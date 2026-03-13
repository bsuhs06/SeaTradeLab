package sources

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bsuhs/shiptracker/ais-collector/internal/database"
)

// DigitTrafficSource fetches AIS data from the Finnish Transport Infrastructure Agency.
// Completely free, no registration or API key required.
// Docs: https://www.digitraffic.fi/en/marine/
//
// Coverage: Gulf of Finland, Baltic Sea - excellent for tracking Russian vessels
// transiting to/from St. Petersburg.
//
// The API provides two main endpoints:
// - /api/ais/v1/locations  (position reports, GeoJSON FeatureCollection)
// - /api/ais/v1/vessels    (vessel metadata, JSON array)
//
// Data refreshes every ~60 seconds. Requires gzip Accept-Encoding.
type DigitTrafficSource struct {
	config    Config
	client    *http.Client
	minDelay  time.Duration
	lastFetch time.Time
}

// NewDigitTrafficSource creates a new Finnish Digitraffic AIS source.
func NewDigitTrafficSource(config Config) *DigitTrafficSource {
	if config.Timeout == 0 {
		config.Timeout = 45 * time.Second
	}

	transport := &http.Transport{
		DisableCompression: false, // We handle gzip per-request
	}
	client := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	return &DigitTrafficSource{
		config:   config,
		client:   client,
		minDelay: 5 * time.Minute, // Conservative: well above the 60s cache
	}
}

func (s *DigitTrafficSource) Name() string {
	return s.config.Name
}

func (s *DigitTrafficSource) Type() string {
	return "api"
}

// --- Response types ---

// dtLocationResponse is a GeoJSON FeatureCollection of vessel positions.
type dtLocationResponse struct {
	Type            string              `json:"type"`
	DataUpdatedTime string              `json:"dataUpdatedTime"`
	Features        []dtLocationFeature `json:"features"`
}

type dtLocationFeature struct {
	MMSI     int             `json:"mmsi"`
	Type     string          `json:"type"`
	Geometry dtGeometry      `json:"geometry"`
	Props    dtLocationProps `json:"properties"`
}

type dtGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"` // [lon, lat]
}

type dtLocationProps struct {
	MMSI              int     `json:"mmsi"`
	SOG               float64 `json:"sog"`
	COG               float64 `json:"cog"`
	NavStat           int     `json:"navStat"`
	ROT               int     `json:"rot"`
	Heading           int     `json:"heading"`
	TimestampExternal int64   `json:"timestampExternal"` // unix millis
}

// dtVesselInfo is the metadata for a single vessel.
type dtVesselInfo struct {
	Name            string `json:"name"`
	MMSI            int    `json:"mmsi"`
	IMO             int    `json:"imo"`
	CallSign        string `json:"callSign"`
	ShipType        int    `json:"shipType"`
	Destination     string `json:"destination"`
	Draught         int    `json:"draught"` // tenths of metres
	ETA             int    `json:"eta"`
	ReferencePointA int    `json:"referencePointA"` // bow
	ReferencePointB int    `json:"referencePointB"` // stern
	ReferencePointC int    `json:"referencePointC"` // port
	ReferencePointD int    `json:"referencePointD"` // starboard
}

// --- Fetch implementation ---

func (s *DigitTrafficSource) Fetch(ctx context.Context) ([]*AISData, error) {
	// Rate limit
	if !s.lastFetch.IsZero() && time.Since(s.lastFetch) < s.minDelay {
		wait := s.minDelay - time.Since(s.lastFetch)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// 1. Fetch positions
	locations, err := s.fetchLocations(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch locations: %w", err)
	}

	// 2. Fetch vessel metadata
	vesselMap, err := s.fetchVessels(ctx)
	if err != nil {
		// Non-fatal: we can still use position data without metadata
		vesselMap = make(map[int]*dtVesselInfo)
	}

	s.lastFetch = time.Now()

	// 3. Merge into AISData
	var results []*AISData
	for _, feat := range locations.Features {
		if feat.MMSI == 0 || len(feat.Geometry.Coordinates) < 2 {
			continue
		}

		lon := feat.Geometry.Coordinates[0]
		lat := feat.Geometry.Coordinates[1]

		// Determine timestamp
		ts := time.Now()
		if feat.Props.TimestampExternal > 0 {
			ts = time.UnixMilli(feat.Props.TimestampExternal)
		}

		navStatusName := NavigationStatusName(feat.Props.NavStat)

		data := &AISData{
			Vessel: &database.Vessel{
				MMSI:       int64(feat.MMSI),
				LastSeenAt: ts,
			},
			Position: &database.AISPosition{
				MMSI:                 int64(feat.MMSI),
				Latitude:             lat,
				Longitude:            lon,
				SpeedOverGround:      floatPtrIfNotZero(feat.Props.SOG),
				CourseOverGround:     floatPtrIfNotZero(feat.Props.COG),
				Heading:              intPtrIfNotZero(feat.Props.Heading),
				NavigationStatus:     &feat.Props.NavStat,
				NavigationStatusName: stringPtrIfNotEmpty(navStatusName),
				Timestamp:            ts,
			},
		}

		// Enrich from vessel metadata if available
		if info, ok := vesselMap[feat.MMSI]; ok {
			data.Vessel.Name = stringPtrIfNotEmpty(strings.TrimSpace(info.Name))
			data.Vessel.CallSign = stringPtrIfNotEmpty(strings.TrimSpace(info.CallSign))
			data.Vessel.VesselType = intPtrIfNotZero(info.ShipType)
			data.Vessel.VesselTypeName = stringPtrIfNotEmpty(VesselTypeName(info.ShipType))
			data.Vessel.Destination = stringPtrIfNotEmpty(strings.TrimSpace(info.Destination))
			data.Vessel.DimensionA = intPtrIfNotZero(info.ReferencePointA)
			data.Vessel.DimensionB = intPtrIfNotZero(info.ReferencePointB)
			data.Vessel.DimensionC = intPtrIfNotZero(info.ReferencePointC)
			data.Vessel.DimensionD = intPtrIfNotZero(info.ReferencePointD)
			if info.IMO > 0 {
				imo := int64(info.IMO)
				data.Vessel.IMONumber = &imo
			}
			if info.Draught > 0 {
				d := float64(info.Draught) / 10.0 // API returns tenths of metres
				data.Vessel.Draught = &d
			}
		}

		results = append(results, data)
	}

	return results, nil
}

// fetchLocations retrieves position reports from the locations endpoint.
func (s *DigitTrafficSource) fetchLocations(ctx context.Context) (*dtLocationResponse, error) {
	baseURL := s.config.BaseURL
	if baseURL == "" {
		baseURL = "https://meri.digitraffic.fi"
	}

	body, err := s.doGet(ctx, baseURL+"/api/ais/v1/locations")
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var resp dtLocationResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode locations: %w", err)
	}
	return &resp, nil
}

// fetchVessels retrieves vessel metadata and returns a map keyed by MMSI.
func (s *DigitTrafficSource) fetchVessels(ctx context.Context) (map[int]*dtVesselInfo, error) {
	baseURL := s.config.BaseURL
	if baseURL == "" {
		baseURL = "https://meri.digitraffic.fi"
	}

	body, err := s.doGet(ctx, baseURL+"/api/ais/v1/vessels")
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var vessels []dtVesselInfo
	if err := json.NewDecoder(body).Decode(&vessels); err != nil {
		return nil, fmt.Errorf("decode vessels: %w", err)
	}

	m := make(map[int]*dtVesselInfo, len(vessels))
	for i := range vessels {
		m[vessels[i].MMSI] = &vessels[i]
	}
	return m, nil
}

// doGet performs a GET request with required gzip encoding and polite headers.
func (s *DigitTrafficSource) doGet(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Digitraffic-User", "SeaTradeLab-Research")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d (%s)", resp.StatusCode, url)
	}

	// Handle gzip
	var reader io.ReadCloser
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		reader = &gzipReadCloser{gzip: gr, body: resp.Body}
	} else {
		reader = resp.Body
	}

	return reader, nil
}

// gzipReadCloser wraps a gzip reader so both the gzip and underlying body get closed.
type gzipReadCloser struct {
	gzip *gzip.Reader
	body io.ReadCloser
}

func (g *gzipReadCloser) Read(p []byte) (int, error) {
	return g.gzip.Read(p)
}

func (g *gzipReadCloser) Close() error {
	g.gzip.Close()
	return g.body.Close()
}

func (s *DigitTrafficSource) HealthCheck(ctx context.Context) error {
	body, err := s.doGet(ctx, "https://meri.digitraffic.fi/api/ais/v1/locations?mmsi=230000000")
	if err != nil {
		return err
	}
	body.Close()
	return nil
}

func (s *DigitTrafficSource) Close() error {
	s.client.CloseIdleConnections()
	return nil
}

// --- helpers (only needed if not already defined) ---

func int64PtrFromStr(s string) *int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v == 0 {
		return nil
	}
	return &v
}

package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bsuhs/shiptracker/ais-collector/internal/database"
	"github.com/gorilla/websocket"
)

// AISStreamSource connects to aisstream.io's WebSocket API and buffers
// incoming AIS messages. The existing collector poll loop calls Fetch()
// which drains the buffer, making this source compatible with the
// periodic-polling architecture used by Digitraffic and other sources.
type AISStreamSource struct {
	config Config
	logger *log.Logger

	// WebSocket
	conn   *websocket.Conn
	connMu sync.Mutex

	// Buffer of parsed AIS messages accumulated between Fetch() calls
	mu     sync.Mutex
	buffer []*AISData

	// Lifecycle
	cancel context.CancelFunc
	done   chan struct{}

	// Reconnection state
	connected bool

	// Stats
	msgCount atomic.Int64
}

// Bounding boxes for target regions: [[lat_min, lon_min], [lat_max, lon_max]]
// AIS Stream format: [[lat_min, lon_min], [lat_max, lon_max]]
// Global coverage excluding the Americas.
var aisStreamBoundingBoxes = [][][2]float64{
	// Europe & Baltic
	{{35.0, -12.0}, {72.0, 45.0}},
	// Black Sea, Med East & Caspian
	{{25.0, 25.0}, {47.0, 55.0}},
	// Persian Gulf, Arabian Sea & Indian Ocean
	{{-10.0, 35.0}, {30.0, 80.0}},
	// East Asia & SE Asia
	{{-12.0, 80.0}, {55.0, 145.0}},
	// Oceania & SW Pacific
	{{-50.0, 100.0}, {-12.0, 180.0}},
	// West & Southern Africa
	{{-40.0, -20.0}, {25.0, 35.0}},
}

// maxBufferSize caps the number of buffered messages to prevent
// unbounded memory growth between Fetch() calls.
const maxBufferSize = 150000

// NewAISStreamSource creates a new AIS Stream WebSocket source.
// The connection is not established until the collector calls Start
// (which triggers the first Fetch). We connect lazily from the
// background reader goroutine.
func NewAISStreamSource(config Config) (*AISStreamSource, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("aisstream: API key is required (set AISSTREAM_API_KEY)")
	}

	s := &AISStreamSource{
		config: config,
		logger: log.New(log.Writer(), "[AISSTREAM] ", log.LstdFlags|log.Lshortfile),
		buffer: make([]*AISData, 0, 1024),
		done:   make(chan struct{}),
	}

	// Start background reader
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go s.readLoop(ctx)

	return s, nil
}

func (s *AISStreamSource) Name() string { return s.config.Name }
func (s *AISStreamSource) Type() string { return "stream" }

// Fetch drains the buffer of AIS messages accumulated since the last call.
func (s *AISStreamSource) Fetch(ctx context.Context) ([]*AISData, error) {
	s.mu.Lock()
	data := s.buffer
	s.buffer = make([]*AISData, 0, 1024)
	s.mu.Unlock()

	if !s.isConnected() && len(data) == 0 {
		return nil, fmt.Errorf("aisstream: websocket not connected, no buffered data")
	}

	return data, nil
}

// HealthCheck verifies the WebSocket connection is alive.
func (s *AISStreamSource) HealthCheck(ctx context.Context) error {
	if !s.isConnected() {
		return fmt.Errorf("aisstream: websocket not connected")
	}
	return nil
}

// Close shuts down the WebSocket connection and background goroutine.
func (s *AISStreamSource) Close() error {
	s.cancel()
	// Wait for readLoop to stop (with timeout)
	select {
	case <-s.done:
	case <-time.After(5 * time.Second):
	}

	s.connMu.Lock()
	defer s.connMu.Unlock()
	if s.conn != nil {
		s.conn.Close()
	}
	return nil
}

func (s *AISStreamSource) isConnected() bool {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	return s.connected
}

func (s *AISStreamSource) setConnected(v bool) {
	s.connMu.Lock()
	s.connected = v
	s.connMu.Unlock()
}

// readLoop manages the WebSocket lifecycle: connect → read → reconnect.
func (s *AISStreamSource) readLoop(ctx context.Context) {
	defer close(s.done)

	backoff := time.Second
	const maxBackoff = 5 * time.Minute

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := s.connect(ctx); err != nil {
			s.logger.Printf("Connection failed: %v (retry in %v)", err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = time.Duration(math.Min(float64(backoff*2), float64(maxBackoff)))
			continue
		}

		// Connected — reset backoff
		backoff = time.Second
		s.logger.Printf("Connected to AIS Stream, reading messages...")

		// Read messages until error or context cancellation
		s.readMessages(ctx)

		s.setConnected(false)
		s.logger.Printf("Disconnected after %d messages, will reconnect in %v", s.msgCount.Load(), backoff)

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}

// connect establishes the WebSocket connection and sends the subscription message.
func (s *AISStreamSource) connect(ctx context.Context) error {
	url := s.config.BaseURL
	if url == "" {
		url = "wss://stream.aisstream.io/v0/stream"
	}

	dialer := websocket.Dialer{
		HandshakeTimeout:  30 * time.Second,
		EnableCompression: true,
	}

	headers := http.Header{}
	headers.Set("User-Agent", "AIS-Collector/1.0")

	conn, resp, err := dialer.DialContext(ctx, url, headers)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Set up ping handler to keep connection alive
	conn.SetPingHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
	})

	// Build subscription message
	sub := aisStreamSubscription{
		APIKey:        s.config.APIKey,
		BoundingBoxes: aisStreamBoundingBoxes,
	}

	subJSON, _ := json.Marshal(sub)
	s.logger.Printf("Sending subscription (%d bytes, %d bounding boxes)", len(subJSON), len(aisStreamBoundingBoxes))

	if err := conn.WriteJSON(sub); err != nil {
		conn.Close()
		return fmt.Errorf("subscribe: %w", err)
	}

	s.connMu.Lock()
	s.conn = conn
	s.connected = true
	s.connMu.Unlock()

	s.msgCount.Store(0)
	return nil
}

// readMessages reads from the WebSocket until an error occurs.
func (s *AISStreamSource) readMessages(ctx context.Context) {
	s.connMu.Lock()
	conn := s.conn
	s.connMu.Unlock()

	if conn == nil {
		return
	}

	conn.SetReadDeadline(time.Now().Add(90 * time.Second))

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			count := s.msgCount.Load()
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				s.logger.Printf("Read error after %d messages: %v", count, err)
			}
			return
		}

		count := s.msgCount.Add(1)
		// Reset read deadline after each successful read
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))

		if count == 1 {
			s.logger.Printf("First message received (%d bytes)", len(msgBytes))
		} else if count%1000 == 0 {
			s.mu.Lock()
			bufLen := len(s.buffer)
			s.mu.Unlock()
			s.logger.Printf("Received %d messages, buffer size: %d", count, bufLen)
		}

		aisData, err := s.parseMessage(msgBytes)
		if err != nil {
			if count <= 5 {
				s.logger.Printf("Parse error on msg %d: %v", count, err)
			}
			continue
		}
		if aisData == nil {
			if count <= 5 {
				s.logger.Printf("Msg %d parsed as nil (type skipped)", count)
			}
			continue
		}

		s.mu.Lock()
		if len(s.buffer) < maxBufferSize {
			s.buffer = append(s.buffer, aisData)
		}
		s.mu.Unlock()
	}
}

// --- AIS Stream JSON message types ---

type aisStreamSubscription struct {
	APIKey        string         `json:"APIKey"`
	BoundingBoxes [][][2]float64 `json:"BoundingBoxes"`
}

// aisStreamMessage is the envelope for all messages from AIS Stream.
type aisStreamMessage struct {
	MessageType string          `json:"MessageType"`
	MetaData    aisStreamMeta   `json:"MetaData"`
	Message     json.RawMessage `json:"Message"`
}

type aisStreamMeta struct {
	MMSI        int64       `json:"MMSI"`
	MMSI_String json.Number `json:"MMSI_String"`
	ShipName    string      `json:"ShipName"`
	Latitude    float64     `json:"latitude"`
	Longitude   float64     `json:"longitude"`
	TimeUtc     string      `json:"time_utc"`
}

// Position report (message types 1, 2, 3)
type aisStreamPositionReport struct {
	Cog                       float64 `json:"Cog"`
	CommunicationState        int     `json:"CommunicationState"`
	Latitude                  float64 `json:"Latitude"`
	Longitude                 float64 `json:"Longitude"`
	MessageID                 int     `json:"MessageID"`
	NavigationalStatus        int     `json:"NavigationalStatus"`
	PositionAccuracy          bool    `json:"PositionAccuracy"`
	Raim                      bool    `json:"Raim"`
	RateOfTurn                int     `json:"RateOfTurn"`
	RepeatIndicator           int     `json:"RepeatIndicator"`
	Sog                       float64 `json:"Sog"`
	Spare                     int     `json:"Spare"`
	SpecialManoeuvreIndicator int     `json:"SpecialManoeuvreIndicator"`
	Timestamp                 int     `json:"Timestamp"`
	TrueHeading               int     `json:"TrueHeading"`
	UserID                    int64   `json:"UserID"`
	Valid                     bool    `json:"Valid"`
}

// Static and voyage data (message type 5)
type aisStreamStaticData struct {
	AisVersion  int    `json:"AisVersion"`
	CallSign    string `json:"CallSign"`
	Destination string `json:"Destination"`
	Dimension   struct {
		A int `json:"A"`
		B int `json:"B"`
		C int `json:"C"`
		D int `json:"D"`
	} `json:"Dimension"`
	Draught float64 `json:"Draught"`
	Dte     bool    `json:"Dte"`
	Eta     struct {
		Day    int `json:"Day"`
		Hour   int `json:"Hour"`
		Minute int `json:"Minute"`
		Month  int `json:"Month"`
	} `json:"Eta"`
	FixType              int     `json:"FixType"`
	ImoNumber            int64   `json:"ImoNumber"`
	MaximumStaticDraught float64 `json:"MaximumStaticDraught"`
	MessageID            int     `json:"MessageID"`
	Name                 string  `json:"Name"`
	RepeatIndicator      int     `json:"RepeatIndicator"`
	Spare                bool    `json:"Spare"`
	Type                 int     `json:"Type"`
	UserID               int64   `json:"UserID"`
	Valid                bool    `json:"Valid"`
}

// parseMessage parses a raw WebSocket message into AISData.
func (s *AISStreamSource) parseMessage(raw []byte) (*AISData, error) {
	var msg aisStreamMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}

	now := time.Now()
	ts := parseAISStreamTime(msg.MetaData.TimeUtc, now)
	mmsi := msg.MetaData.MMSI

	if mmsi == 0 {
		return nil, nil
	}

	switch msg.MessageType {
	case "PositionReport":
		return s.parsePositionReport(msg, ts, now)

	case "ShipStaticData":
		return s.parseStaticData(msg, ts, now)

	case "StandardClassBPositionReport":
		// Class B position reports have the same structure
		return s.parsePositionReport(msg, ts, now)

	case "StandardSearchAndRescueAircraftReport",
		"StaticDataReport",
		"AddressedSafetyMessage",
		"Acknowledge",
		"MultiSlotBinaryMessage",
		"LongRangeAISBroadcastMessage",
		"BaseStationReport",
		"DataLinkManagementMessage",
		"AidsToNavigationReport",
		"UnknownMessage":
		// Known message types we don't process
		return nil, nil

	default:
		// Unknown message type — skip silently
		return nil, nil
	}
}

func (s *AISStreamSource) parsePositionReport(msg aisStreamMessage, ts, now time.Time) (*AISData, error) {
	// Message is wrapped: {"PositionReport": {...}} or {"StandardClassBPositionReport": {...}}
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(msg.Message, &wrapper); err != nil {
		return nil, fmt.Errorf("unmarshal position wrapper: %w", err)
	}

	var inner json.RawMessage
	for _, v := range wrapper {
		inner = v
		break
	}
	if inner == nil {
		return nil, nil
	}

	var pr aisStreamPositionReport
	if err := json.Unmarshal(inner, &pr); err != nil {
		return nil, fmt.Errorf("unmarshal position report: %w", err)
	}

	mmsi := msg.MetaData.MMSI
	lat := pr.Latitude
	lon := pr.Longitude

	// Use metadata lat/lon as fallback
	if lat == 0 && lon == 0 {
		lat = msg.MetaData.Latitude
		lon = msg.MetaData.Longitude
	}

	// Skip invalid positions
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return nil, nil
	}
	if lat == 0 && lon == 0 {
		return nil, nil
	}

	navStatus := pr.NavigationalStatus
	heading := pr.TrueHeading
	if heading == 511 {
		heading = 0 // 511 = not available
	}

	shipName := strings.TrimSpace(msg.MetaData.ShipName)

	return &AISData{
		Vessel: &database.Vessel{
			MMSI:       mmsi,
			Name:       stringPtrIfNotEmpty(shipName),
			LastSeenAt: now,
		},
		Position: &database.AISPosition{
			MMSI:                 mmsi,
			Latitude:             lat,
			Longitude:            lon,
			SpeedOverGround:      floatPtrIfNotZero(pr.Sog),
			CourseOverGround:     floatPtrIfNotZero(pr.Cog),
			Heading:              intPtrIfNotZero(heading),
			NavigationStatus:     intPtrIfNotZero(navStatus),
			NavigationStatusName: stringPtrIfNotEmpty(NavigationStatusName(navStatus)),
			Timestamp:            ts,
		},
	}, nil
}

func (s *AISStreamSource) parseStaticData(msg aisStreamMessage, ts, now time.Time) (*AISData, error) {
	// Message is wrapped: {"ShipStaticData": {...}}
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(msg.Message, &wrapper); err != nil {
		return nil, fmt.Errorf("unmarshal static wrapper: %w", err)
	}

	var inner json.RawMessage
	for _, v := range wrapper {
		inner = v
		break
	}
	if inner == nil {
		return nil, nil
	}

	var sd aisStreamStaticData
	if err := json.Unmarshal(inner, &sd); err != nil {
		return nil, fmt.Errorf("unmarshal static data: %w", err)
	}

	mmsi := msg.MetaData.MMSI

	name := strings.TrimSpace(sd.Name)
	if name == "" {
		name = strings.TrimSpace(msg.MetaData.ShipName)
	}

	vesselType := sd.Type
	vesselTypeName := VesselTypeName(vesselType)

	// Parse ETA
	var eta *time.Time
	if sd.Eta.Month > 0 && sd.Eta.Day > 0 {
		year := now.Year()
		etaTime := time.Date(year, time.Month(sd.Eta.Month), sd.Eta.Day,
			sd.Eta.Hour, sd.Eta.Minute, 0, 0, time.UTC)
		// If ETA is in the past, assume it's next year
		if etaTime.Before(now) {
			etaTime = etaTime.AddDate(1, 0, 0)
		}
		eta = &etaTime
	}

	// Draught: AIS Stream provides it in tenths of meters (like raw AIS)
	var draught *float64
	if sd.Draught > 0 {
		d := sd.Draught / 10.0
		draught = &d
	} else if sd.MaximumStaticDraught > 0 {
		d := sd.MaximumStaticDraught / 10.0
		draught = &d
	}

	lat := msg.MetaData.Latitude
	lon := msg.MetaData.Longitude

	data := &AISData{
		Vessel: &database.Vessel{
			MMSI:           mmsi,
			IMONumber:      int64Ptr(sd.ImoNumber),
			Name:           stringPtrIfNotEmpty(name),
			CallSign:       stringPtrIfNotEmpty(strings.TrimSpace(sd.CallSign)),
			VesselType:     intPtrIfNotZero(vesselType),
			VesselTypeName: stringPtrIfNotEmpty(vesselTypeName),
			DimensionA:     intPtrIfNotZero(sd.Dimension.A),
			DimensionB:     intPtrIfNotZero(sd.Dimension.B),
			DimensionC:     intPtrIfNotZero(sd.Dimension.C),
			DimensionD:     intPtrIfNotZero(sd.Dimension.D),
			Draught:        draught,
			Destination:    stringPtrIfNotEmpty(strings.TrimSpace(sd.Destination)),
			ETA:            eta,
			LastSeenAt:     now,
		},
	}

	// Include position if metadata has valid coordinates
	if lat != 0 || lon != 0 {
		if lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180 {
			data.Position = &database.AISPosition{
				MMSI:      mmsi,
				Latitude:  lat,
				Longitude: lon,
				Timestamp: ts,
			}
		}
	}

	return data, nil
}

// parseAISStreamTime parses the time_utc field from AIS Stream metadata.
// Format: "2024-01-15 12:30:45" or ISO 8601.
func parseAISStreamTime(s string, fallback time.Time) time.Time {
	if s == "" {
		return fallback
	}

	formats := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}

	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}

	return fallback
}

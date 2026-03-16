package api

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bsuhs/shiptracker/web-server/internal/db"
)

type Handler struct {
	repo         *db.Repo
	logger       *log.Logger
	analyticsMu  sync.Mutex
	analyticsRun *analyticsRun
	collectorMu  sync.Mutex
	collectorCmd *exec.Cmd
	collectorLog string
}

type analyticsRun struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	StartedAt string `json:"started_at"`
	Output    string `json:"output"`
	Args      string `json:"args"`
}

func NewHandler(repo *db.Repo, logger *log.Logger) *Handler {
	h := &Handler{repo: repo, logger: logger}
	// Run data retention on startup — purge positions older than 30 days
	go h.autoRetention()
	return h
}

func (h *Handler) autoRetention() {
	ctx := context.Background()
	h.logger.Println("Running data retention (30 day max)...")
	deleted, err := h.repo.PurgeOldPositions(ctx, 30)
	if err != nil {
		h.logger.Printf("Data retention error: %v", err)
		return
	}
	h.logger.Printf("Data retention: purged %d old positions", deleted)
	if deleted > 0 {
		orphans, err := h.repo.PurgeOrphanVessels(ctx)
		if err != nil {
			h.logger.Printf("Orphan vessel cleanup error: %v", err)
			return
		}
		if orphans > 0 {
			h.logger.Printf("Data retention: removed %d orphan vessels", orphans)
		}
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/vessels", h.vessels)
	mux.HandleFunc("/api/vessels/", h.vesselTrack)
	mux.HandleFunc("/api/stats", h.stats)
	mux.HandleFunc("/api/trails", h.trails)
	mux.HandleFunc("/api/sts-events", h.stsEvents)
	mux.HandleFunc("/api/sts-events/", h.stsEventUpdate)
	mux.HandleFunc("/api/search", h.searchVessels)
	mux.HandleFunc("/api/dark-vessels", h.darkVessels)
	mux.HandleFunc("/api/historical", h.historical)
	mux.HandleFunc("/api/time-range", h.timeRange)
	mux.HandleFunc("/api/port-visits", h.portVisits)
	mux.HandleFunc("/api/run-analytics", h.runAnalytics)
	mux.HandleFunc("/api/analytics-status", h.analyticsStatus)
	mux.HandleFunc("/api/collector-status", h.collectorStatus)
	mux.HandleFunc("/api/collector-start", h.collectorStart)
	mux.HandleFunc("/api/collector-stop", h.collectorStop)
	mux.HandleFunc("/api/ports", h.ports)
	mux.HandleFunc("/api/ports/override", h.portOverride)
	mux.HandleFunc("/api/purge", h.purgeOldData)
	mux.HandleFunc("/api/spoofed-vessels", h.spoofedVessels)
	mux.HandleFunc("/api/vessel-registry", h.vesselRegistry)
	mux.HandleFunc("/api/vessel-registry/", h.vesselRegistryDetail)
	mux.HandleFunc("/api/vessel-changes", h.vesselChanges)
	mux.HandleFunc("/api/vessel-tags", h.vesselTags)
	mux.HandleFunc("/api/tainted-vessels", h.taintedVessels)
	mux.HandleFunc("/api/vessel-taint/", h.vesselTaintDetail)
	mux.HandleFunc("/api/taint-chain/", h.taintChain)
}

func (h *Handler) vessels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var bbox *db.BBox
	parse := func(key string) (float64, bool) {
		v := r.URL.Query().Get(key)
		if v == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	}
	if south, ok1 := parse("south"); ok1 {
		if west, ok2 := parse("west"); ok2 {
			if north, ok3 := parse("north"); ok3 {
				if east, ok4 := parse("east"); ok4 {
					bbox = &db.BBox{South: south, West: west, North: north, East: east}
				}
			}
		}
	}

	vessels, err := h.repo.VesselsInBBox(r.Context(), bbox)
	if err != nil {
		h.logger.Printf("Error fetching vessels: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	features := make([]map[string]interface{}, 0, len(vessels))
	for _, v := range vessels {
		props := map[string]interface{}{
			"mmsi":      v.MMSI,
			"timestamp": v.Timestamp,
		}
		if v.Name != nil {
			props["name"] = *v.Name
		}
		if v.VesselTypeName != nil {
			props["vessel_type"] = *v.VesselTypeName
		}
		if v.CallSign != nil {
			props["call_sign"] = *v.CallSign
		}
		if v.IMONumber != nil {
			props["imo"] = *v.IMONumber
		}
		if v.Draught != nil {
			props["draught"] = *v.Draught
		}
		if v.SpeedOverGround != nil {
			props["sog"] = *v.SpeedOverGround
		}
		if v.CourseOverGround != nil {
			props["cog"] = *v.CourseOverGround
		}
		if v.Heading != nil {
			props["heading"] = *v.Heading
		}
		if v.NavigationStatusName != nil {
			props["nav_status"] = *v.NavigationStatusName
		}
		if v.Destination != nil {
			props["destination"] = *v.Destination
		}

		mmsiStr := strconv.FormatInt(v.MMSI, 10)
		props["is_russian"] = strings.HasPrefix(mmsiStr, "273")

		features = append(features, map[string]interface{}{
			"type": "Feature",
			"geometry": map[string]interface{}{
				"type":        "Point",
				"coordinates": []float64{v.Longitude, v.Latitude},
			},
			"properties": props,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=10")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type":     "FeatureCollection",
		"features": features,
	})
}

func (h *Handler) vesselTrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/vessels/"), "/")
	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, "MMSI required", http.StatusBadRequest)
		return
	}

	mmsi, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "Invalid MMSI", http.StatusBadRequest)
		return
	}

	hours := 24
	if hParam := r.URL.Query().Get("hours"); hParam != "" {
		if parsed, err := strconv.Atoi(hParam); err == nil && parsed > 0 && parsed <= 168 {
			hours = parsed
		}
	}

	points, err := h.repo.VesselTrack(r.Context(), mmsi, hours)
	if err != nil {
		h.logger.Printf("Error fetching track for MMSI %d: %v", mmsi, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	coords := make([][]float64, 0, len(points))
	timestamps := make([]string, 0, len(points))
	speeds := make([]interface{}, 0, len(points))
	for _, p := range points {
		coords = append(coords, []float64{p.Longitude, p.Latitude})
		timestamps = append(timestamps, p.Timestamp.Format("2006-01-02T15:04:05Z"))
		if p.SpeedOverGround != nil {
			speeds = append(speeds, *p.SpeedOverGround)
		} else {
			speeds = append(speeds, nil)
		}
	}

	result := map[string]interface{}{
		"mmsi":        mmsi,
		"hours":       hours,
		"point_count": len(points),
		"track": map[string]interface{}{
			"type":        "LineString",
			"coordinates": coords,
		},
		"timestamps": timestamps,
		"speeds":     speeds,
	}

	vessel, err := h.repo.VesselByMMSI(r.Context(), mmsi)
	if err == nil && vessel != nil {
		result["vessel"] = vessel
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) trails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parse := func(key string) (float64, bool) {
		v := r.URL.Query().Get(key)
		if v == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	}

	south, ok1 := parse("south")
	west, ok2 := parse("west")
	north, ok3 := parse("north")
	east, ok4 := parse("east")
	if !ok1 || !ok2 || !ok3 || !ok4 {
		http.Error(w, "south, west, north, east params required", http.StatusBadRequest)
		return
	}

	hours := 24
	if hp := r.URL.Query().Get("hours"); hp != "" {
		if parsed, err := strconv.Atoi(hp); err == nil && parsed > 0 && parsed <= 48 {
			hours = parsed
		}
	}

	trails, err := h.repo.TrailsInBBox(r.Context(), south, west, north, east, hours)
	if err != nil {
		h.logger.Printf("Error fetching trails: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Convert int64 keys to strings for JSON
	out := make(map[string][][3]float64, len(trails))
	for mmsi, coords := range trails {
		out[strconv.FormatInt(mmsi, 10)] = coords
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := h.repo.GetStats(r.Context())
	if err != nil {
		h.logger.Printf("Error fetching stats: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// --- STS Events ---

func (h *Handler) stsEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hours := 168
	if hp := r.URL.Query().Get("hours"); hp != "" {
		if parsed, err := strconv.Atoi(hp); err == nil && parsed > 0 {
			hours = parsed
		}
	}

	limit := 100
	if lp := r.URL.Query().Get("limit"); lp != "" {
		if parsed, err := strconv.Atoi(lp); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	events, err := h.repo.GetSTSEvents(r.Context(), hours, limit)
	if err != nil {
		h.logger.Printf("Error fetching STS events: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	totalCount, _ := h.repo.CountSTSEvents(r.Context(), hours)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"events":      events,
		"count":       len(events),
		"total_count": totalCount,
		"hours":       hours,
	})
}

func (h *Handler) stsEventUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from /api/sts-events/{id}
	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		http.Error(w, "Missing event ID", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	if err != nil {
		http.Error(w, "Invalid event ID", http.StatusBadRequest)
		return
	}

	var body struct {
		Confidence string  `json:"confidence"`
		Reviewed   bool    `json:"reviewed"`
		Tag        *string `json:"tag"`
		Notes      *string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate confidence
	switch body.Confidence {
	case "high", "medium", "low":
	default:
		http.Error(w, "Invalid confidence (high/medium/low)", http.StatusBadRequest)
		return
	}

	if err := h.repo.UpdateSTSEvent(r.Context(), id, body.Confidence, body.Reviewed, body.Tag, body.Notes); err != nil {
		h.logger.Printf("Error updating STS event %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// --- Vessel Search ---

func (h *Handler) searchVessels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "q parameter required", http.StatusBadRequest)
		return
	}

	limit := 100
	if lp := r.URL.Query().Get("limit"); lp != "" {
		if parsed, err := strconv.Atoi(lp); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	vessels, err := h.repo.SearchVessels(r.Context(), q, limit)
	if err != nil {
		h.logger.Printf("Error searching vessels: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	features := h.vesselFeatures(vessels)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type":     "FeatureCollection",
		"features": features,
	})
}

// --- Dark Vessels ---

func (h *Handler) darkVessels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	minHours := 6.0
	if hp := r.URL.Query().Get("min_hours"); hp != "" {
		if parsed, err := strconv.ParseFloat(hp, 64); err == nil && parsed > 0 {
			minHours = parsed
		}
	}

	limit := 100
	if lp := r.URL.Query().Get("limit"); lp != "" {
		if parsed, err := strconv.Atoi(lp); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	vessels, err := h.repo.GetDarkVessels(r.Context(), minHours, limit)
	if err != nil {
		h.logger.Printf("Error fetching dark vessels: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	totalCount, _ := h.repo.CountDarkVessels(r.Context(), minHours)

	features := make([]map[string]interface{}, 0, len(vessels))
	for _, v := range vessels {
		props := map[string]interface{}{"mmsi": v.MMSI, "timestamp": v.Timestamp}
		if v.Name != nil {
			props["name"] = *v.Name
		}
		if v.VesselTypeName != nil {
			props["vessel_type"] = *v.VesselTypeName
		}
		if v.SpeedOverGround != nil {
			props["sog"] = *v.SpeedOverGround
		}
		if v.Heading != nil {
			props["heading"] = *v.Heading
		}
		mmsiStr := strconv.FormatInt(v.MMSI, 10)
		props["is_russian"] = strings.HasPrefix(mmsiStr, "273")
		props["gap_hours"] = math.Round(time.Since(v.Timestamp).Hours()*10) / 10

		features = append(features, map[string]interface{}{
			"type": "Feature",
			"geometry": map[string]interface{}{
				"type":        "Point",
				"coordinates": []float64{v.Longitude, v.Latitude},
			},
			"properties": props,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type":        "FeatureCollection",
		"features":    features,
		"count":       len(features),
		"total_count": totalCount,
	})
}

// --- Historical / Time Slider ---

func (h *Handler) historical(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	timeStr := r.URL.Query().Get("time")
	if timeStr == "" {
		http.Error(w, "time parameter required (ISO 8601)", http.StatusBadRequest)
		return
	}

	targetTime, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		http.Error(w, "Invalid time format, use ISO 8601", http.StatusBadRequest)
		return
	}

	vessels, err := h.repo.GetHistoricalPositions(r.Context(), targetTime)
	if err != nil {
		h.logger.Printf("Error fetching historical positions: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	features := h.vesselFeatures(vessels)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type":     "FeatureCollection",
		"features": features,
		"count":    len(features),
		"time":     targetTime,
	})
}

func (h *Handler) timeRange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	minT, maxT, err := h.repo.GetTimeRange(r.Context())
	if err != nil {
		h.logger.Printf("Error fetching time range: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"min": minT,
		"max": maxT,
	})
}

// ========== Shared: vessel to GeoJSON features ==========

func (h *Handler) portVisits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hours := 720 // ~30 days default
	if hp := r.URL.Query().Get("hours"); hp != "" {
		if parsed, err := strconv.Atoi(hp); err == nil && parsed > 0 {
			hours = parsed
		}
	}

	nonRussianOnly := r.URL.Query().Get("non_russian") == "true"

	limit := 100
	if lp := r.URL.Query().Get("limit"); lp != "" {
		if parsed, err := strconv.Atoi(lp); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	visits, err := h.repo.GetPortVisits(r.Context(), hours, nonRussianOnly, limit)
	if err != nil {
		h.logger.Printf("Error fetching port visits: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	totalCount, _ := h.repo.CountPortVisits(r.Context(), hours, nonRussianOnly)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"visits":      visits,
		"count":       len(visits),
		"total_count": totalCount,
		"hours":       hours,
	})
}

func (h *Handler) vesselFeatures(vessels []*db.VesselPosition) []map[string]interface{} {
	features := make([]map[string]interface{}, 0, len(vessels))
	for _, v := range vessels {
		props := map[string]interface{}{
			"mmsi":      v.MMSI,
			"timestamp": v.Timestamp,
		}
		if v.Name != nil {
			props["name"] = *v.Name
		}
		if v.VesselTypeName != nil {
			props["vessel_type"] = *v.VesselTypeName
		}
		if v.CallSign != nil {
			props["call_sign"] = *v.CallSign
		}
		if v.IMONumber != nil {
			props["imo"] = *v.IMONumber
		}
		if v.Draught != nil {
			props["draught"] = *v.Draught
		}
		if v.SpeedOverGround != nil {
			props["sog"] = *v.SpeedOverGround
		}
		if v.CourseOverGround != nil {
			props["cog"] = *v.CourseOverGround
		}
		if v.Heading != nil {
			props["heading"] = *v.Heading
		}
		if v.NavigationStatusName != nil {
			props["nav_status"] = *v.NavigationStatusName
		}
		if v.Destination != nil {
			props["destination"] = *v.Destination
		}

		mmsiStr := strconv.FormatInt(v.MMSI, 10)
		props["is_russian"] = strings.HasPrefix(mmsiStr, "273")

		features = append(features, map[string]interface{}{
			"type": "Feature",
			"geometry": map[string]interface{}{
				"type":        "Point",
				"coordinates": []float64{v.Longitude, v.Latitude},
			},
			"properties": props,
		})
	}
	return features
}

func (h *Handler) runAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.analyticsMu.Lock()
	if h.analyticsRun != nil && h.analyticsRun.Status == "running" {
		h.analyticsMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":  "analytics already running",
			"status": h.analyticsRun,
		})
		return
	}
	h.analyticsMu.Unlock()

	var req struct {
		Task        string `json:"task"`
		Hours       int    `json:"hours"`
		Distance    int    `json:"distance"`
		Speed       int    `json:"speed"`
		MinDuration int    `json:"min_duration"`
		GapHours    int    `json:"gap_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	args := []string{"-m", "src.run"}
	switch req.Task {
	case "sts":
		args = append(args, "--sts")
	case "gaps":
		args = append(args, "--gaps")
	case "ports":
		args = append(args, "--ports")
	case "russian-ports":
		args = append(args, "--russian-ports")
	case "detect":
		args = append(args, "--detect")
	case "all":
		args = append(args, "--detect", "--sts", "--gaps", "--ports", "--russian-ports")
	default:
		http.Error(w, "invalid task: "+req.Task, http.StatusBadRequest)
		return
	}

	if req.Hours > 0 {
		args = append(args, "--hours", strconv.Itoa(req.Hours))
	}
	if req.Distance > 0 {
		args = append(args, "--distance", strconv.Itoa(req.Distance))
	}
	if req.Speed > 0 {
		args = append(args, "--speed", strconv.Itoa(req.Speed))
	}
	if req.MinDuration > 0 {
		args = append(args, "--min-duration", strconv.Itoa(req.MinDuration))
	}
	if req.GapHours > 0 {
		args = append(args, "--gap-hours", strconv.Itoa(req.GapHours))
	}

	run := &analyticsRun{
		ID:        time.Now().Format("20060102-150405"),
		Status:    "running",
		StartedAt: time.Now().Format(time.RFC3339),
		Args:      strings.Join(args[2:], " "),
	}

	h.analyticsMu.Lock()
	h.analyticsRun = run
	h.analyticsMu.Unlock()

	go func() {
		// Resolve analytics dir: try relative to executable, then common paths
		exePath, _ := os.Executable()
		candidates := []string{
			filepath.Join(filepath.Dir(exePath), "..", "..", "analytics"),
			filepath.Join(filepath.Dir(exePath), "..", "analytics"),
			"../analytics",
			"../../analytics",
		}
		analyticsDir := ""
		for _, c := range candidates {
			if abs, err := filepath.Abs(c); err == nil {
				if _, err := os.Stat(abs); err == nil {
					analyticsDir = abs
					break
				}
			}
		}
		if analyticsDir == "" {
			h.analyticsMu.Lock()
			h.analyticsRun.Status = "failed"
			h.analyticsRun.Output = "Could not find analytics directory. Tried: " + strings.Join(candidates, ", ")
			h.analyticsMu.Unlock()
			return
		}
		cmd := exec.Command("python3", args...)
		cmd.Dir = analyticsDir
		out, err := cmd.CombinedOutput()

		h.analyticsMu.Lock()
		defer h.analyticsMu.Unlock()
		h.analyticsRun.Output = string(out)
		if err != nil {
			h.analyticsRun.Status = "failed"
			h.analyticsRun.Output += "\nError: " + err.Error()
		} else {
			h.analyticsRun.Status = "completed"
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "analytics started",
		"run":     run,
	})
}

func (h *Handler) analyticsStatus(w http.ResponseWriter, r *http.Request) {
	h.analyticsMu.Lock()
	defer h.analyticsMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if h.analyticsRun == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "no runs",
		})
		return
	}
	json.NewEncoder(w).Encode(h.analyticsRun)
}

// ========== Collector Control ==========

func (h *Handler) findCollectorBinary() string {
	exePath, _ := os.Executable()
	candidates := []string{
		filepath.Join(filepath.Dir(exePath), "..", "..", "ais-collector", "bin", "collector"),
		filepath.Join(filepath.Dir(exePath), "..", "ais-collector", "bin", "collector"),
		"../ais-collector/bin/collector",
		"../../ais-collector/bin/collector",
	}
	for _, c := range candidates {
		if abs, err := filepath.Abs(c); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}
	return ""
}

func (h *Handler) findCollectorDir() string {
	bin := h.findCollectorBinary()
	if bin == "" {
		return ""
	}
	// binary is at ais-collector/bin/collector, dir is ais-collector/
	return filepath.Dir(filepath.Dir(bin))
}

func (h *Handler) isCollectorRunning() (bool, int) {
	h.collectorMu.Lock()
	defer h.collectorMu.Unlock()
	if h.collectorCmd != nil && h.collectorCmd.Process != nil {
		// Check if process is still alive
		if err := h.collectorCmd.Process.Signal(syscall.Signal(0)); err == nil {
			return true, h.collectorCmd.Process.Pid
		}
		// Process exited
		h.collectorCmd = nil
	}
	return false, 0
}

func (h *Handler) collectorStatus(w http.ResponseWriter, r *http.Request) {
	running, pid := h.isCollectorRunning()

	// Also check for any external collector processes
	var externalPids []int
	if !running {
		// Check if collector is running externally (not managed by us)
		checkCmd := exec.Command("pgrep", "-f", "bin/collector")
		out, err := checkCmd.Output()
		if err == nil && len(out) > 0 {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if p, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
					externalPids = append(externalPids, p)
				}
			}
		}
	}

	// Get last collection time from stats
	stats, _ := h.repo.GetStats(r.Context())
	var lastCollected *time.Time
	if stats != nil && stats.LastCollectedAt != nil {
		lastCollected = stats.LastCollectedAt
	}

	h.collectorMu.Lock()
	logTail := h.collectorLog
	h.collectorMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"managed_running": running,
		"managed_pid":     pid,
		"external_pids":   externalPids,
		"binary_found":    h.findCollectorBinary() != "",
		"last_collected":  lastCollected,
		"log":             logTail,
	})
}

func (h *Handler) collectorStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	running, _ := h.isCollectorRunning()
	if running {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "collector already running",
		})
		return
	}

	binPath := h.findCollectorBinary()
	if binPath == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "collector binary not found",
		})
		return
	}

	collectorDir := h.findCollectorDir()
	cmd := exec.Command(binPath)
	cmd.Dir = collectorDir

	// Use a pipe to capture output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout pipe

	if err := cmd.Start(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	h.collectorMu.Lock()
	h.collectorLog = ""
	h.collectorCmd = cmd
	h.collectorMu.Unlock()

	// Background goroutine reads output and waits for exit
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				h.collectorMu.Lock()
				// Keep last ~8KB of log
				h.collectorLog += string(buf[:n])
				if len(h.collectorLog) > 8192 {
					h.collectorLog = h.collectorLog[len(h.collectorLog)-8192:]
				}
				h.collectorMu.Unlock()
			}
			if err != nil {
				break
			}
		}
		exitErr := cmd.Wait()
		h.collectorMu.Lock()
		if exitErr != nil {
			h.collectorLog += "\nProcess exited: " + exitErr.Error()
		} else {
			h.collectorLog += "\nProcess exited normally"
		}
		h.collectorCmd = nil
		h.collectorMu.Unlock()
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "collector started",
		"pid":     cmd.Process.Pid,
	})
}

func (h *Handler) collectorStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.collectorMu.Lock()
	cmd := h.collectorCmd
	h.collectorMu.Unlock()

	stoppedManaged := false
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Signal(syscall.SIGTERM)
		stoppedManaged = true
		time.Sleep(500 * time.Millisecond)
		h.collectorMu.Lock()
		h.collectorCmd = nil
		h.collectorMu.Unlock()
	}

	// Also kill any external collector processes
	killCmd := exec.Command("pkill", "-f", "bin/collector")
	killCmd.Run()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":         "collector stopped",
		"stopped_managed": stoppedManaged,
	})
}

// ========== Port Management ==========

type builtinPort struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	RadiusKm  float64 `json:"radius_km"`
	Country   string  `json:"country"`
	PortType  string  `json:"port_type"`
	Source    string  `json:"source"`
}

// Hard-coded port list matching analytics/src/analyzers/port_tracker.py PORTS
var builtinPorts = []builtinPort{
	// ===== REGION 1 — EUROPE =====
	// Russia (Baltic)
	{"Primorsk", 60.355, 29.206, 6.0, "Russia", "oil", "builtin"},
	{"Ust-Luga", 59.680, 28.390, 6.0, "Russia", "oil", "builtin"},
	{"Vysotsk", 60.627, 28.573, 4.0, "Russia", "oil", "builtin"},
	{"St. Petersburg", 59.933, 30.300, 8.0, "Russia", "commercial", "builtin"},
	{"Kaliningrad", 54.710, 20.500, 6.0, "Russia", "commercial", "builtin"},
	{"Kronshtadt", 59.990, 29.770, 4.0, "Russia", "naval", "builtin"},
	{"Vyborg", 60.710, 28.750, 4.0, "Russia", "cargo", "builtin"},
	// Russia (Arctic)
	{"Murmansk", 68.970, 33.060, 6.0, "Russia", "commercial", "builtin"},
	{"Arkhangelsk", 64.540, 40.540, 5.0, "Russia", "cargo", "builtin"},
	// Russia (Pacific)
	{"Vladivostok", 43.110, 131.890, 5.0, "Russia", "commercial", "builtin"},
	{"Nakhodka", 42.820, 132.880, 4.0, "Russia", "cargo", "builtin"},
	{"Vostochny", 42.750, 133.070, 5.0, "Russia", "cargo", "builtin"},
	{"Kozmino", 42.730, 133.040, 4.0, "Russia", "oil", "builtin"},
	{"De-Kastri", 51.470, 140.780, 4.0, "Russia", "oil", "builtin"},
	// Finland
	{"Helsinki", 60.155, 24.955, 4.0, "Finland", "commercial", "builtin"},
	{"Turku", 60.435, 22.230, 4.0, "Finland", "commercial", "builtin"},
	{"Hamina-Kotka", 60.470, 26.950, 5.0, "Finland", "cargo", "builtin"},
	{"Porvoo / Kilpilahti", 60.305, 25.555, 3.0, "Finland", "oil", "builtin"},
	{"Naantali", 60.465, 22.030, 3.0, "Finland", "oil", "builtin"},
	{"Rauma", 61.130, 21.460, 4.0, "Finland", "cargo", "builtin"},
	{"Pori / Tahkoluoto", 61.635, 21.390, 4.0, "Finland", "lng", "builtin"},
	{"Kokkola", 63.840, 23.030, 4.0, "Finland", "cargo", "builtin"},
	{"Pietarsaari / Jakobstad", 63.710, 22.690, 3.0, "Finland", "cargo", "builtin"},
	{"Vaasa", 63.085, 21.575, 3.0, "Finland", "ferry", "builtin"},
	{"Oulu", 65.010, 25.410, 4.0, "Finland", "cargo", "builtin"},
	{"Kemi", 65.740, 24.540, 4.0, "Finland", "cargo", "builtin"},
	{"Tornio / Raahe", 64.680, 24.470, 5.0, "Finland", "cargo", "builtin"},
	{"Raahe", 64.680, 24.470, 4.0, "Finland", "cargo", "builtin"},
	{"Hanko", 59.820, 22.970, 3.0, "Finland", "cargo", "builtin"},
	{"Loviisa", 60.445, 26.240, 3.0, "Finland", "cargo", "builtin"},
	{"Inkoo", 60.045, 24.005, 3.0, "Finland", "lng", "builtin"},
	{"Uusikaupunki", 60.795, 21.395, 3.0, "Finland", "cargo", "builtin"},
	{"Mariehamn", 60.097, 19.935, 3.0, "Finland", "ferry", "builtin"},
	{"Eckero", 60.225, 19.535, 2.0, "Finland", "ferry", "builtin"},
	{"Langnas", 60.115, 20.295, 2.0, "Finland", "ferry", "builtin"},
	// Sweden
	{"Stockholm", 59.325, 18.070, 5.0, "Sweden", "commercial", "builtin"},
	{"Nynashamn", 58.900, 17.950, 3.0, "Sweden", "oil", "builtin"},
	{"Sodertalje", 59.195, 17.625, 3.0, "Sweden", "cargo", "builtin"},
	{"Kapellskar", 59.720, 19.065, 2.0, "Sweden", "ferry", "builtin"},
	{"Grisslehamn", 60.105, 18.820, 2.0, "Sweden", "ferry", "builtin"},
	{"Norrtalje", 59.760, 18.700, 2.0, "Sweden", "cargo", "builtin"},
	{"Oxelosund", 58.665, 17.125, 3.0, "Sweden", "cargo", "builtin"},
	{"Norrkoping", 58.595, 16.200, 4.0, "Sweden", "cargo", "builtin"},
	{"Vastervik", 57.755, 16.655, 2.0, "Sweden", "cargo", "builtin"},
	{"Visby", 57.640, 18.290, 3.0, "Sweden", "commercial", "builtin"},
	{"Slite", 57.710, 18.810, 2.0, "Sweden", "cargo", "builtin"},
	{"Oskarshamn", 57.265, 16.455, 3.0, "Sweden", "cargo", "builtin"},
	{"Kalmar", 56.660, 16.365, 3.0, "Sweden", "cargo", "builtin"},
	{"Karlskrona", 56.160, 15.590, 3.0, "Sweden", "naval", "builtin"},
	{"Karlshamn", 56.165, 14.860, 3.0, "Sweden", "cargo", "builtin"},
	{"Gavle", 60.675, 17.195, 4.0, "Sweden", "lng", "builtin"},
	{"Sundsvall", 62.390, 17.340, 4.0, "Sweden", "cargo", "builtin"},
	{"Harnosand", 62.635, 17.940, 3.0, "Sweden", "cargo", "builtin"},
	{"Ornskoldsvik", 63.290, 18.720, 3.0, "Sweden", "cargo", "builtin"},
	{"Umea", 63.720, 20.270, 4.0, "Sweden", "cargo", "builtin"},
	{"Skelleftea", 64.680, 21.230, 3.0, "Sweden", "cargo", "builtin"},
	{"Lulea", 65.575, 22.145, 5.0, "Sweden", "cargo", "builtin"},
	{"Ystad", 55.425, 13.830, 3.0, "Sweden", "ferry", "builtin"},
	{"Trelleborg", 55.370, 13.160, 3.0, "Sweden", "ferry", "builtin"},
	{"Malmo", 55.615, 13.000, 4.0, "Sweden", "commercial", "builtin"},
	{"Helsingborg", 56.040, 12.695, 3.0, "Sweden", "commercial", "builtin"},
	{"Gothenburg", 57.695, 11.945, 5.0, "Sweden", "commercial", "builtin"},
	{"Lysekil", 58.275, 11.430, 3.0, "Sweden", "oil", "builtin"},
	{"Stenungsund", 58.070, 11.825, 3.0, "Sweden", "oil", "builtin"},
	// Estonia
	{"Tallinn / Muuga", 59.490, 24.960, 5.0, "Estonia", "commercial", "builtin"},
	{"Paldiski", 59.350, 24.050, 3.0, "Estonia", "lng", "builtin"},
	{"Sillamae", 59.400, 27.760, 3.0, "Estonia", "oil", "builtin"},
	{"Parnu", 58.385, 24.495, 3.0, "Estonia", "cargo", "builtin"},
	{"Kuressaare / Roomassaare", 58.225, 22.490, 2.0, "Estonia", "cargo", "builtin"},
	// Latvia
	{"Riga", 57.045, 24.065, 5.0, "Latvia", "commercial", "builtin"},
	{"Ventspils", 57.400, 21.540, 4.0, "Latvia", "oil", "builtin"},
	{"Liepaja", 56.530, 21.000, 4.0, "Latvia", "commercial", "builtin"},
	// Lithuania
	{"Klaipeda", 55.710, 21.120, 5.0, "Lithuania", "lng", "builtin"},
	{"Butinge", 56.070, 21.060, 3.0, "Lithuania", "oil", "builtin"},
	// Poland
	{"Gdansk", 54.395, 18.670, 5.0, "Poland", "commercial", "builtin"},
	{"Gdynia", 54.530, 18.545, 4.0, "Poland", "commercial", "builtin"},
	{"Swinoujscie", 53.910, 14.260, 4.0, "Poland", "lng", "builtin"},
	{"Szczecin", 53.430, 14.570, 5.0, "Poland", "commercial", "builtin"},
	{"Police", 53.565, 14.570, 3.0, "Poland", "cargo", "builtin"},
	{"Elblag", 54.155, 19.400, 3.0, "Poland", "cargo", "builtin"},
	// Germany
	{"Rostock / Warnemunde", 54.180, 12.100, 5.0, "Germany", "commercial", "builtin"},
	{"Wismar", 53.900, 11.460, 3.0, "Germany", "cargo", "builtin"},
	{"Lubeck / Travemunde", 53.960, 10.870, 4.0, "Germany", "commercial", "builtin"},
	{"Kiel", 54.330, 10.150, 4.0, "Germany", "commercial", "builtin"},
	{"Sassnitz / Mukran", 54.515, 13.615, 3.0, "Germany", "cargo", "builtin"},
	{"Stralsund", 54.310, 13.100, 3.0, "Germany", "cargo", "builtin"},
	{"Brunsbuttel", 53.895, 9.140, 3.0, "Germany", "lng", "builtin"},
	{"Hamburg", 53.545, 9.930, 8.0, "Germany", "commercial", "builtin"},
	{"Bremerhaven", 53.540, 8.580, 5.0, "Germany", "commercial", "builtin"},
	{"Wilhelmshaven", 53.515, 8.145, 5.0, "Germany", "oil", "builtin"},
	// Denmark
	{"Copenhagen", 55.690, 12.610, 5.0, "Denmark", "commercial", "builtin"},
	{"Fredericia", 55.565, 9.745, 3.0, "Denmark", "oil", "builtin"},
	{"Aarhus", 56.150, 10.225, 4.0, "Denmark", "commercial", "builtin"},
	{"Kalundborg", 55.680, 11.090, 3.0, "Denmark", "oil", "builtin"},
	{"Ronne (Bornholm)", 55.090, 14.685, 3.0, "Denmark", "ferry", "builtin"},
	{"Helsingor", 56.035, 12.615, 2.0, "Denmark", "ferry", "builtin"},
	{"Gedser", 54.575, 11.925, 2.0, "Denmark", "ferry", "builtin"},
	{"Rodby", 54.655, 11.350, 2.0, "Denmark", "ferry", "builtin"},
	{"Skagen", 57.720, 10.595, 3.0, "Denmark", "commercial", "builtin"},
	// Norway
	{"Oslo", 59.900, 10.740, 5.0, "Norway", "commercial", "builtin"},
	{"Fredrikstad / Borg", 59.220, 10.960, 3.0, "Norway", "cargo", "builtin"},
	{"Larvik", 59.050, 10.030, 3.0, "Norway", "cargo", "builtin"},
	{"Kristiansand", 58.145, 8.000, 4.0, "Norway", "commercial", "builtin"},
	{"Stavanger", 58.970, 5.730, 4.0, "Norway", "oil", "builtin"},
	{"Bergen", 60.395, 5.320, 4.0, "Norway", "commercial", "builtin"},
	{"Mongstad", 60.810, 5.035, 3.0, "Norway", "oil", "builtin"},
	{"Sture", 60.580, 4.875, 2.0, "Norway", "oil", "builtin"},
	{"Hammerfest", 70.665, 23.680, 3.0, "Norway", "lng", "builtin"},
	{"Tromso", 69.650, 18.960, 3.0, "Norway", "commercial", "builtin"},
	{"Narvik", 68.430, 17.430, 3.0, "Norway", "cargo", "builtin"},
	// Netherlands
	{"Rotterdam", 51.905, 4.490, 10.0, "Netherlands", "commercial", "builtin"},
	{"Amsterdam", 52.410, 4.790, 6.0, "Netherlands", "commercial", "builtin"},
	// Belgium
	{"Antwerp", 51.270, 4.350, 8.0, "Belgium", "commercial", "builtin"},
	{"Zeebrugge", 51.350, 3.200, 4.0, "Belgium", "lng", "builtin"},
	// United Kingdom
	{"London / Tilbury", 51.460, 0.360, 5.0, "United Kingdom", "commercial", "builtin"},
	{"Southampton", 50.895, -1.400, 5.0, "United Kingdom", "commercial", "builtin"},
	{"Felixstowe", 51.955, 1.305, 4.0, "United Kingdom", "commercial", "builtin"},
	{"Immingham", 53.630, -0.200, 5.0, "United Kingdom", "cargo", "builtin"},
	{"Liverpool", 53.440, -3.020, 5.0, "United Kingdom", "commercial", "builtin"},
	{"Milford Haven", 51.710, -5.050, 5.0, "United Kingdom", "oil", "builtin"},
	{"Fawley", 50.830, -1.340, 3.0, "United Kingdom", "oil", "builtin"},
	{"Sullom Voe", 60.460, -1.280, 4.0, "United Kingdom", "oil", "builtin"},
	{"Scapa Flow", 58.890, -3.100, 5.0, "United Kingdom", "oil", "builtin"},
	{"Aberdeen", 57.145, -2.070, 4.0, "United Kingdom", "oil", "builtin"},
	{"Grangemouth", 56.035, -3.710, 4.0, "United Kingdom", "oil", "builtin"},
	{"Teesport", 54.610, -1.160, 4.0, "United Kingdom", "cargo", "builtin"},
	{"Hound Point", 56.010, -3.380, 3.0, "United Kingdom", "oil", "builtin"},
	// France
	{"Le Havre", 49.480, 0.110, 5.0, "France", "commercial", "builtin"},
	{"Marseille / Fos", 43.400, 4.870, 6.0, "France", "commercial", "builtin"},
	{"Dunkirk", 51.060, 2.370, 5.0, "France", "commercial", "builtin"},
	{"Nantes / Saint-Nazaire", 47.280, -2.190, 5.0, "France", "commercial", "builtin"},
	{"Brest", 48.380, -4.490, 4.0, "France", "naval", "builtin"},
	// Spain
	{"Barcelona", 41.350, 2.170, 5.0, "Spain", "commercial", "builtin"},
	{"Valencia", 39.445, -0.320, 5.0, "Spain", "commercial", "builtin"},
	{"Algeciras", 36.130, -5.440, 5.0, "Spain", "commercial", "builtin"},
	{"Bilbao", 43.340, -3.050, 4.0, "Spain", "commercial", "builtin"},
	{"Cartagena", 37.580, -0.990, 4.0, "Spain", "oil", "builtin"},
	{"Tarragona", 41.090, 1.230, 4.0, "Spain", "oil", "builtin"},
	{"Las Palmas", 28.140, -15.420, 5.0, "Spain", "commercial", "builtin"},
	{"Huelva", 37.200, -6.940, 4.0, "Spain", "oil", "builtin"},
	// Portugal
	{"Lisbon", 38.700, -9.140, 5.0, "Portugal", "commercial", "builtin"},
	{"Sines", 37.950, -8.880, 5.0, "Portugal", "oil", "builtin"},
	{"Leixoes", 41.185, -8.710, 4.0, "Portugal", "commercial", "builtin"},
	// Italy
	{"Genoa", 44.405, 8.925, 5.0, "Italy", "commercial", "builtin"},
	{"Trieste", 45.640, 13.760, 5.0, "Italy", "oil", "builtin"},
	{"Venice", 45.440, 12.350, 5.0, "Italy", "commercial", "builtin"},
	{"Naples", 40.840, 14.270, 5.0, "Italy", "commercial", "builtin"},
	{"Gioia Tauro", 38.430, 15.890, 5.0, "Italy", "commercial", "builtin"},
	{"Augusta", 37.230, 15.230, 4.0, "Italy", "oil", "builtin"},
	{"Taranto", 40.470, 17.210, 4.0, "Italy", "commercial", "builtin"},
	{"Livorno", 43.550, 10.300, 4.0, "Italy", "commercial", "builtin"},
	{"Ravenna", 44.490, 12.270, 4.0, "Italy", "cargo", "builtin"},
	{"Cagliari", 39.210, 9.110, 4.0, "Italy", "commercial", "builtin"},
	{"Milazzo", 38.230, 15.250, 3.0, "Italy", "oil", "builtin"},
	{"Priolo / Siracusa", 37.160, 15.190, 4.0, "Italy", "oil", "builtin"},
	{"La Spezia", 44.100, 9.840, 4.0, "Italy", "commercial", "builtin"},
	{"Sarroch", 39.070, 9.020, 3.0, "Italy", "oil", "builtin"},
	// Greece
	{"Piraeus", 37.940, 23.630, 5.0, "Greece", "commercial", "builtin"},
	{"Thessaloniki", 40.620, 22.940, 5.0, "Greece", "commercial", "builtin"},
	{"Agioi Theodoroi", 37.930, 23.100, 3.0, "Greece", "oil", "builtin"},
	{"Elefsis", 38.040, 23.530, 3.0, "Greece", "oil", "builtin"},
	{"Revithoussa", 37.960, 23.370, 3.0, "Greece", "lng", "builtin"},
	// Croatia
	{"Rijeka", 45.330, 14.440, 4.0, "Croatia", "commercial", "builtin"},
	{"Omisalj", 45.210, 14.530, 3.0, "Croatia", "oil", "builtin"},
	// Slovenia
	{"Koper", 45.550, 13.740, 4.0, "Slovenia", "commercial", "builtin"},
	// Montenegro
	{"Bar", 42.090, 19.110, 3.0, "Montenegro", "commercial", "builtin"},
	// Albania
	{"Durres", 41.310, 19.440, 3.0, "Albania", "commercial", "builtin"},
	// Malta
	{"Marsaxlokk", 35.830, 14.540, 4.0, "Malta", "commercial", "builtin"},
	// Cyprus
	{"Limassol", 34.660, 33.040, 4.0, "Cyprus", "commercial", "builtin"},
	{"Vasilikos", 34.710, 33.340, 3.0, "Cyprus", "oil", "builtin"},
	// Iceland
	{"Reykjavik", 64.150, -21.930, 4.0, "Iceland", "commercial", "builtin"},

	// ===== REGION 2 — BLACK SEA / MED EAST =====
	// Russia (Black Sea)
	{"Novorossiysk", 44.720, 37.790, 6.0, "Russia", "oil", "builtin"},
	{"Tuapse", 44.100, 39.070, 4.0, "Russia", "oil", "builtin"},
	{"Kavkaz", 45.370, 36.650, 4.0, "Russia", "oil", "builtin"},
	{"Taman", 45.220, 36.720, 4.0, "Russia", "oil", "builtin"},
	{"Sevastopol", 44.620, 33.530, 5.0, "Russia", "naval", "builtin"},
	{"Kerch", 45.350, 36.480, 4.0, "Russia", "commercial", "builtin"},
	// Turkey
	{"Istanbul", 41.010, 28.980, 8.0, "Turkey", "commercial", "builtin"},
	{"Izmir / Aliaga", 38.800, 26.960, 5.0, "Turkey", "oil", "builtin"},
	{"Mersin", 36.790, 34.640, 5.0, "Turkey", "commercial", "builtin"},
	{"Iskenderun", 36.590, 36.170, 4.0, "Turkey", "commercial", "builtin"},
	{"Ceyhan / BTC Terminal", 36.860, 35.770, 4.0, "Turkey", "oil", "builtin"},
	{"Samsun", 41.280, 36.340, 4.0, "Turkey", "commercial", "builtin"},
	{"Trabzon", 41.000, 39.730, 3.0, "Turkey", "commercial", "builtin"},
	{"Dortyol", 36.850, 36.200, 3.0, "Turkey", "oil", "builtin"},
	// Ukraine
	{"Odesa", 46.490, 30.760, 5.0, "Ukraine", "commercial", "builtin"},
	{"Pivdennyi", 46.630, 31.010, 4.0, "Ukraine", "oil", "builtin"},
	{"Mykolaiv", 46.970, 32.000, 4.0, "Ukraine", "cargo", "builtin"},
	// Georgia
	{"Batumi", 41.650, 41.630, 4.0, "Georgia", "oil", "builtin"},
	{"Poti", 42.160, 41.670, 3.0, "Georgia", "commercial", "builtin"},
	{"Supsa", 42.010, 41.770, 3.0, "Georgia", "oil", "builtin"},
	// Romania
	{"Constanta", 44.170, 28.660, 6.0, "Romania", "commercial", "builtin"},
	// Bulgaria
	{"Varna", 43.190, 27.920, 4.0, "Bulgaria", "commercial", "builtin"},
	{"Burgas", 42.490, 27.480, 4.0, "Bulgaria", "commercial", "builtin"},
	// Egypt
	{"Port Said", 31.260, 32.300, 5.0, "Egypt", "commercial", "builtin"},
	{"Suez", 29.960, 32.560, 5.0, "Egypt", "commercial", "builtin"},
	{"Ain Sukhna", 29.610, 32.340, 4.0, "Egypt", "oil", "builtin"},
	{"Alexandria", 31.195, 29.860, 5.0, "Egypt", "commercial", "builtin"},
	{"Damietta", 31.470, 31.810, 4.0, "Egypt", "lng", "builtin"},
	{"El Dekheila", 31.150, 29.790, 3.0, "Egypt", "cargo", "builtin"},
	// Israel
	{"Haifa", 32.820, 35.000, 4.0, "Israel", "commercial", "builtin"},
	{"Ashdod", 31.830, 34.640, 4.0, "Israel", "commercial", "builtin"},
	{"Ashkelon", 31.670, 34.520, 3.0, "Israel", "oil", "builtin"},
	{"Eilat", 29.550, 34.960, 3.0, "Israel", "commercial", "builtin"},
	// Lebanon
	{"Beirut", 33.900, 35.520, 4.0, "Lebanon", "commercial", "builtin"},
	{"Tripoli (Lebanon)", 34.450, 35.820, 3.0, "Lebanon", "oil", "builtin"},
	// Syria
	{"Tartus", 34.890, 35.870, 4.0, "Syria", "naval", "builtin"},
	{"Latakia", 35.520, 35.770, 4.0, "Syria", "commercial", "builtin"},
	{"Baniyas", 35.230, 35.950, 3.0, "Syria", "oil", "builtin"},
	// Libya
	{"Zawiya", 32.760, 12.710, 3.0, "Libya", "oil", "builtin"},
	{"Es Sider", 30.630, 18.350, 4.0, "Libya", "oil", "builtin"},
	{"Ras Lanuf", 30.490, 18.550, 4.0, "Libya", "oil", "builtin"},
	{"Brega", 30.410, 19.580, 3.0, "Libya", "oil", "builtin"},
	{"Zueitina", 30.870, 20.120, 3.0, "Libya", "oil", "builtin"},
	{"Marsa el Hariga / Tobruk", 32.080, 23.960, 3.0, "Libya", "oil", "builtin"},
	// Tunisia
	{"Bizerte", 37.280, 9.870, 3.0, "Tunisia", "commercial", "builtin"},
	{"La Skhirra", 34.300, 10.070, 3.0, "Tunisia", "oil", "builtin"},
	// Algeria
	{"Algiers", 36.770, 3.060, 5.0, "Algeria", "commercial", "builtin"},
	{"Arzew", 35.830, -0.310, 4.0, "Algeria", "lng", "builtin"},
	{"Skikda", 36.880, 6.910, 4.0, "Algeria", "lng", "builtin"},
	{"Bejaia", 36.750, 5.100, 3.0, "Algeria", "oil", "builtin"},
	// Morocco
	{"Tanger Med", 35.890, -5.500, 5.0, "Morocco", "commercial", "builtin"},
	{"Casablanca", 33.600, -7.620, 5.0, "Morocco", "commercial", "builtin"},
	{"Mohammedia", 33.720, -7.400, 3.0, "Morocco", "oil", "builtin"},

	// ===== REGION 3 — PERSIAN GULF / INDIAN OCEAN / RED SEA =====
	// Saudi Arabia
	{"Ras Tanura", 26.640, 50.160, 5.0, "Saudi Arabia", "oil", "builtin"},
	{"Jubail", 27.010, 49.660, 6.0, "Saudi Arabia", "commercial", "builtin"},
	{"Yanbu", 24.090, 38.050, 5.0, "Saudi Arabia", "oil", "builtin"},
	{"Jeddah", 21.480, 39.160, 6.0, "Saudi Arabia", "commercial", "builtin"},
	{"Dammam / King Abdulaziz", 26.470, 50.210, 5.0, "Saudi Arabia", "commercial", "builtin"},
	{"Ras al-Khair", 27.140, 49.250, 4.0, "Saudi Arabia", "cargo", "builtin"},
	{"Jazan", 16.900, 42.570, 4.0, "Saudi Arabia", "oil", "builtin"},
	// UAE
	{"Fujairah", 25.120, 56.340, 5.0, "UAE", "oil", "builtin"},
	{"Jebel Ali", 25.020, 55.060, 8.0, "UAE", "commercial", "builtin"},
	{"Ruwais", 24.110, 52.730, 5.0, "UAE", "oil", "builtin"},
	{"Khor Fakkan", 25.340, 56.350, 4.0, "UAE", "commercial", "builtin"},
	{"Abu Dhabi / Musaffah", 24.440, 54.500, 5.0, "UAE", "commercial", "builtin"},
	{"Das Island", 25.060, 52.870, 4.0, "UAE", "oil", "builtin"},
	// Qatar
	{"Ras Laffan", 25.920, 51.550, 6.0, "Qatar", "lng", "builtin"},
	{"Mesaieed", 24.990, 51.570, 5.0, "Qatar", "oil", "builtin"},
	// Kuwait
	{"Mina al-Ahmadi", 29.060, 48.160, 5.0, "Kuwait", "oil", "builtin"},
	{"Shuwaikh", 29.350, 47.920, 4.0, "Kuwait", "commercial", "builtin"},
	{"Mina Abdullah", 29.000, 48.180, 4.0, "Kuwait", "oil", "builtin"},
	// Bahrain
	{"Khalifa bin Salman", 26.020, 50.590, 4.0, "Bahrain", "commercial", "builtin"},
	{"Sitra", 26.130, 50.650, 3.0, "Bahrain", "oil", "builtin"},
	// Iraq
	{"Basra / Al-Faw", 29.980, 48.460, 6.0, "Iraq", "oil", "builtin"},
	{"ABOT / Al-Bakr", 29.680, 48.800, 5.0, "Iraq", "oil", "builtin"},
	{"Khor al-Amaya", 29.780, 48.830, 4.0, "Iraq", "oil", "builtin"},
	// Iran
	{"Kharg Island", 29.230, 50.310, 5.0, "Iran", "oil", "builtin"},
	{"Bandar Abbas", 27.170, 56.280, 5.0, "Iran", "commercial", "builtin"},
	{"Assaluyeh", 27.480, 52.610, 5.0, "Iran", "lng", "builtin"},
	{"Bandar Imam Khomeini", 30.430, 49.070, 5.0, "Iran", "commercial", "builtin"},
	{"Chabahar", 25.290, 60.620, 4.0, "Iran", "commercial", "builtin"},
	{"Lavan Island", 26.810, 53.360, 4.0, "Iran", "oil", "builtin"},
	{"Sirri Island", 25.900, 54.520, 3.0, "Iran", "oil", "builtin"},
	// Oman
	{"Sohar", 24.370, 56.740, 5.0, "Oman", "commercial", "builtin"},
	{"Salalah", 16.940, 54.010, 5.0, "Oman", "commercial", "builtin"},
	{"Mina al-Fahal", 23.630, 58.520, 4.0, "Oman", "oil", "builtin"},
	// Yemen
	{"Aden", 12.790, 45.020, 5.0, "Yemen", "commercial", "builtin"},
	{"Hodeidah", 14.790, 42.940, 4.0, "Yemen", "commercial", "builtin"},
	{"Ash Shihr", 14.760, 49.600, 3.0, "Yemen", "oil", "builtin"},
	// Djibouti
	{"Djibouti", 11.590, 43.140, 4.0, "Djibouti", "commercial", "builtin"},
	// India
	{"Mumbai / JNPT", 18.950, 72.950, 6.0, "India", "commercial", "builtin"},
	{"Mundra", 22.730, 69.710, 6.0, "India", "commercial", "builtin"},
	{"Sikka", 22.420, 69.840, 4.0, "India", "oil", "builtin"},
	{"Vadinar", 22.390, 69.700, 3.0, "India", "oil", "builtin"},
	{"Kandla", 23.030, 70.210, 4.0, "India", "commercial", "builtin"},
	{"Paradip", 20.260, 86.670, 5.0, "India", "cargo", "builtin"},
	{"Visakhapatnam", 17.680, 83.290, 5.0, "India", "commercial", "builtin"},
	{"Chennai", 13.090, 80.290, 5.0, "India", "commercial", "builtin"},
	{"Kochi", 9.970, 76.260, 4.0, "India", "commercial", "builtin"},
	{"Mangalore / NMPT", 12.920, 74.810, 4.0, "India", "commercial", "builtin"},
	{"Haldia", 22.030, 88.060, 4.0, "India", "cargo", "builtin"},
	{"Hazira", 21.110, 72.630, 4.0, "India", "lng", "builtin"},
	{"Dahej", 21.690, 72.590, 3.0, "India", "lng", "builtin"},
	{"Ennore / Kamarajar", 13.230, 80.330, 4.0, "India", "cargo", "builtin"},
	{"Tuticorin", 8.770, 78.190, 4.0, "India", "commercial", "builtin"},
	{"Krishnapatnam", 14.250, 80.120, 4.0, "India", "cargo", "builtin"},
	// Pakistan
	{"Karachi", 24.820, 66.980, 5.0, "Pakistan", "commercial", "builtin"},
	{"Port Qasim", 24.780, 67.350, 5.0, "Pakistan", "commercial", "builtin"},
	{"Gwadar", 25.130, 62.330, 4.0, "Pakistan", "commercial", "builtin"},
	// Sri Lanka
	{"Colombo", 6.940, 79.850, 5.0, "Sri Lanka", "commercial", "builtin"},
	{"Hambantota", 6.120, 81.110, 4.0, "Sri Lanka", "commercial", "builtin"},
	// Bangladesh
	{"Chittagong", 22.310, 91.810, 5.0, "Bangladesh", "commercial", "builtin"},
	// Myanmar
	{"Yangon / Thilawa", 16.750, 96.250, 5.0, "Myanmar", "commercial", "builtin"},
	// Eritrea
	{"Assab", 13.010, 42.740, 3.0, "Eritrea", "commercial", "builtin"},
	// Sudan
	{"Port Sudan", 19.610, 37.220, 4.0, "Sudan", "commercial", "builtin"},
	// Jordan
	{"Aqaba", 29.500, 35.010, 4.0, "Jordan", "commercial", "builtin"},

	// ===== REGION 4 — EAST ASIA / SE ASIA =====
	// China
	{"Shanghai", 31.390, 121.510, 8.0, "China", "commercial", "builtin"},
	{"Ningbo-Zhoushan", 29.950, 121.830, 8.0, "China", "commercial", "builtin"},
	{"Guangzhou", 22.880, 113.510, 6.0, "China", "commercial", "builtin"},
	{"Shenzhen / Yantian", 22.560, 114.270, 5.0, "China", "commercial", "builtin"},
	{"Qingdao", 36.070, 120.320, 6.0, "China", "commercial", "builtin"},
	{"Tianjin", 38.980, 117.740, 8.0, "China", "commercial", "builtin"},
	{"Dalian", 38.930, 121.650, 6.0, "China", "commercial", "builtin"},
	{"Xiamen", 24.450, 118.080, 5.0, "China", "commercial", "builtin"},
	{"Zhanjiang", 21.200, 110.400, 4.0, "China", "oil", "builtin"},
	{"Rizhao", 35.390, 119.550, 4.0, "China", "cargo", "builtin"},
	{"Tangshan / Caofeidian", 39.230, 118.920, 5.0, "China", "cargo", "builtin"},
	{"Nanjing", 32.070, 118.720, 5.0, "China", "commercial", "builtin"},
	{"Dongguan", 22.790, 113.680, 4.0, "China", "commercial", "builtin"},
	{"Haikou", 20.020, 110.340, 4.0, "China", "commercial", "builtin"},
	{"Quanzhou", 24.800, 118.630, 4.0, "China", "commercial", "builtin"},
	{"Zhuhai", 22.130, 113.480, 4.0, "China", "commercial", "builtin"},
	{"Yangshan", 30.630, 122.070, 6.0, "China", "commercial", "builtin"},
	{"Dafeng", 33.200, 120.760, 4.0, "China", "lng", "builtin"},
	{"Yingkou", 40.660, 122.250, 4.0, "China", "cargo", "builtin"},
	{"Maoming", 21.450, 110.860, 3.0, "China", "oil", "builtin"},
	// Japan
	{"Yokohama", 35.450, 139.650, 5.0, "Japan", "commercial", "builtin"},
	{"Tokyo", 35.630, 139.790, 5.0, "Japan", "commercial", "builtin"},
	{"Osaka / Kobe", 34.660, 135.210, 6.0, "Japan", "commercial", "builtin"},
	{"Nagoya", 35.070, 136.870, 5.0, "Japan", "commercial", "builtin"},
	{"Chiba", 35.570, 140.080, 5.0, "Japan", "oil", "builtin"},
	{"Kashima", 35.920, 140.690, 4.0, "Japan", "cargo", "builtin"},
	{"Kiire", 31.380, 130.600, 3.0, "Japan", "oil", "builtin"},
	{"Mizushima", 34.480, 133.740, 4.0, "Japan", "oil", "builtin"},
	{"Kawasaki", 35.510, 139.750, 4.0, "Japan", "oil", "builtin"},
	{"Kitakyushu", 33.900, 130.970, 4.0, "Japan", "commercial", "builtin"},
	{"Hakata / Fukuoka", 33.610, 130.400, 4.0, "Japan", "commercial", "builtin"},
	// South Korea
	{"Busan", 35.080, 129.040, 6.0, "South Korea", "commercial", "builtin"},
	{"Ulsan", 35.490, 129.370, 5.0, "South Korea", "oil", "builtin"},
	{"Yeosu / Gwangyang", 34.870, 127.760, 5.0, "South Korea", "commercial", "builtin"},
	{"Incheon", 37.450, 126.600, 5.0, "South Korea", "commercial", "builtin"},
	{"Daesan", 36.930, 126.420, 4.0, "South Korea", "oil", "builtin"},
	{"Pyeongtaek / Dangjin", 36.960, 126.830, 4.0, "South Korea", "commercial", "builtin"},
	// Taiwan
	{"Kaohsiung", 22.610, 120.280, 5.0, "Taiwan", "commercial", "builtin"},
	{"Keelung", 25.160, 121.740, 4.0, "Taiwan", "commercial", "builtin"},
	{"Taichung", 24.280, 120.510, 4.0, "Taiwan", "commercial", "builtin"},
	{"Mailiao", 23.790, 120.200, 4.0, "Taiwan", "oil", "builtin"},
	// Hong Kong
	{"Hong Kong", 22.280, 114.160, 6.0, "Hong Kong", "commercial", "builtin"},
	// Singapore
	{"Singapore", 1.260, 103.830, 8.0, "Singapore", "commercial", "builtin"},
	// Malaysia
	{"Port Klang", 2.990, 101.370, 5.0, "Malaysia", "commercial", "builtin"},
	{"Tanjung Pelepas", 1.360, 103.550, 5.0, "Malaysia", "commercial", "builtin"},
	{"Penang", 5.420, 100.350, 4.0, "Malaysia", "commercial", "builtin"},
	{"Bintulu", 3.180, 113.040, 4.0, "Malaysia", "lng", "builtin"},
	{"Labuan", 5.280, 115.240, 3.0, "Malaysia", "oil", "builtin"},
	{"Kerteh", 4.520, 103.440, 3.0, "Malaysia", "oil", "builtin"},
	{"Kemaman", 4.230, 103.440, 3.0, "Malaysia", "oil", "builtin"},
	// Indonesia
	{"Tanjung Priok / Jakarta", -6.100, 106.880, 5.0, "Indonesia", "commercial", "builtin"},
	{"Cilacap", -7.740, 109.010, 4.0, "Indonesia", "oil", "builtin"},
	{"Balikpapan", -1.270, 116.810, 5.0, "Indonesia", "oil", "builtin"},
	{"Dumai", 1.680, 101.450, 4.0, "Indonesia", "oil", "builtin"},
	{"Belawan / Medan", 3.790, 98.690, 4.0, "Indonesia", "commercial", "builtin"},
	{"Bontang", 0.100, 117.490, 4.0, "Indonesia", "lng", "builtin"},
	{"Merak", -5.930, 106.000, 3.0, "Indonesia", "cargo", "builtin"},
	{"Surabaya / Tanjung Perak", -7.200, 112.740, 5.0, "Indonesia", "commercial", "builtin"},
	// Thailand
	{"Laem Chabang", 13.080, 100.880, 5.0, "Thailand", "commercial", "builtin"},
	{"Map Ta Phut", 12.720, 101.180, 4.0, "Thailand", "oil", "builtin"},
	{"Bangkok / Klong Toey", 13.700, 100.580, 5.0, "Thailand", "commercial", "builtin"},
	{"Si Racha", 13.160, 100.920, 3.0, "Thailand", "oil", "builtin"},
	// Vietnam
	{"Ho Chi Minh City / Cat Lai", 10.760, 106.770, 5.0, "Vietnam", "commercial", "builtin"},
	{"Hai Phong", 20.860, 106.680, 5.0, "Vietnam", "commercial", "builtin"},
	{"Vung Tau", 10.340, 107.090, 4.0, "Vietnam", "oil", "builtin"},
	{"Da Nang", 16.080, 108.220, 4.0, "Vietnam", "commercial", "builtin"},
	// Philippines
	{"Manila", 14.580, 120.960, 5.0, "Philippines", "commercial", "builtin"},
	{"Batangas", 13.730, 121.050, 4.0, "Philippines", "commercial", "builtin"},
	{"Cebu", 10.300, 123.900, 4.0, "Philippines", "commercial", "builtin"},
	{"Subic Bay", 14.790, 120.280, 4.0, "Philippines", "commercial", "builtin"},
	// Cambodia
	{"Sihanoukville", 10.630, 103.500, 4.0, "Cambodia", "commercial", "builtin"},
	// Brunei
	{"Muara", 5.020, 115.080, 3.0, "Brunei", "commercial", "builtin"},
	{"Lumut (Brunei)", 4.620, 114.440, 3.0, "Brunei", "lng", "builtin"},

	// ===== REGION 5 — OCEANIA =====
	// Australia
	{"Port Hedland", -20.310, 118.580, 6.0, "Australia", "cargo", "builtin"},
	{"Dampier", -20.660, 116.710, 5.0, "Australia", "cargo", "builtin"},
	{"Gladstone", -23.850, 151.270, 5.0, "Australia", "lng", "builtin"},
	{"Newcastle (AU)", -32.920, 151.790, 5.0, "Australia", "cargo", "builtin"},
	{"Melbourne", -37.840, 144.920, 6.0, "Australia", "commercial", "builtin"},
	{"Sydney", -33.860, 151.190, 5.0, "Australia", "commercial", "builtin"},
	{"Fremantle", -32.050, 115.740, 5.0, "Australia", "commercial", "builtin"},
	{"Hay Point / Dalrymple Bay", -21.280, 149.280, 5.0, "Australia", "cargo", "builtin"},
	{"Brisbane", -27.380, 153.170, 5.0, "Australia", "commercial", "builtin"},
	{"Darwin", -12.440, 130.850, 5.0, "Australia", "commercial", "builtin"},
	{"Whyalla", -33.020, 137.530, 4.0, "Australia", "cargo", "builtin"},
	{"Geelong", -38.130, 144.360, 4.0, "Australia", "oil", "builtin"},
	{"Kwinana", -32.230, 115.770, 4.0, "Australia", "oil", "builtin"},
	{"Abbot Point", -19.890, 148.090, 4.0, "Australia", "cargo", "builtin"},
	{"Barrow Island", -20.810, 115.440, 4.0, "Australia", "lng", "builtin"},
	{"Weipa", -12.680, 141.870, 4.0, "Australia", "cargo", "builtin"},
	{"Port Adelaide", -34.780, 138.520, 4.0, "Australia", "commercial", "builtin"},
	{"Bonython / Port Bonython", -32.980, 137.770, 3.0, "Australia", "lng", "builtin"},
	// New Zealand
	{"Tauranga", -37.640, 176.190, 4.0, "New Zealand", "commercial", "builtin"},
	{"Auckland", -36.840, 174.760, 5.0, "New Zealand", "commercial", "builtin"},
	{"Lyttelton", -43.610, 172.720, 3.0, "New Zealand", "commercial", "builtin"},
	{"Marsden Point", -35.830, 174.500, 3.0, "New Zealand", "oil", "builtin"},
	// Papua New Guinea
	{"Lae", -6.730, 147.000, 4.0, "Papua New Guinea", "cargo", "builtin"},

	// ===== REGION 6 — WEST / SOUTHERN AFRICA =====
	// South Africa
	{"Durban", -29.870, 31.040, 5.0, "South Africa", "commercial", "builtin"},
	{"Richards Bay", -28.790, 32.090, 5.0, "South Africa", "cargo", "builtin"},
	{"Cape Town", -33.910, 18.440, 5.0, "South Africa", "commercial", "builtin"},
	{"Saldanha Bay", -33.020, 17.930, 5.0, "South Africa", "cargo", "builtin"},
	{"Port Elizabeth / Gqeberha", -33.770, 25.640, 4.0, "South Africa", "commercial", "builtin"},
	{"Mossel Bay", -34.180, 22.150, 3.0, "South Africa", "oil", "builtin"},
	// Nigeria
	{"Lagos / Apapa", 6.440, 3.380, 5.0, "Nigeria", "commercial", "builtin"},
	{"Bonny", 4.430, 7.170, 4.0, "Nigeria", "oil", "builtin"},
	{"Qua Iboe", 4.530, 8.020, 4.0, "Nigeria", "oil", "builtin"},
	{"Brass", 4.310, 6.240, 4.0, "Nigeria", "oil", "builtin"},
	{"Forcados", 5.350, 5.360, 3.0, "Nigeria", "oil", "builtin"},
	// Ghana
	{"Tema", 5.630, 0.010, 4.0, "Ghana", "commercial", "builtin"},
	{"Takoradi", 4.890, -1.740, 3.0, "Ghana", "commercial", "builtin"},
	// Cote d'Ivoire
	{"Abidjan", 5.290, -4.020, 5.0, "Cote d'Ivoire", "commercial", "builtin"},
	// Senegal
	{"Dakar", 14.680, -17.430, 5.0, "Senegal", "commercial", "builtin"},
	// Cameroon
	{"Douala", 4.040, 9.710, 4.0, "Cameroon", "commercial", "builtin"},
	{"Kribi", 2.950, 9.900, 3.0, "Cameroon", "oil", "builtin"},
	// Angola
	{"Luanda", -8.800, 13.250, 5.0, "Angola", "commercial", "builtin"},
	{"Lobito", -12.340, 13.560, 3.0, "Angola", "commercial", "builtin"},
	{"Soyo", -6.130, 12.350, 3.0, "Angola", "oil", "builtin"},
	// Republic of Congo
	{"Pointe-Noire", -4.780, 11.850, 4.0, "Republic of Congo", "oil", "builtin"},
	// Equatorial Guinea
	{"Malabo", 3.750, 8.780, 3.0, "Equatorial Guinea", "oil", "builtin"},
	{"Bata", 1.860, 9.770, 3.0, "Equatorial Guinea", "commercial", "builtin"},
	{"Punta Europa", 3.770, 8.710, 3.0, "Equatorial Guinea", "lng", "builtin"},
	// Gabon
	{"Port-Gentil", -0.720, 8.780, 4.0, "Gabon", "oil", "builtin"},
	{"Libreville / Owendo", 0.290, 9.490, 4.0, "Gabon", "commercial", "builtin"},
	// Namibia
	{"Walvis Bay", -22.960, 14.510, 4.0, "Namibia", "commercial", "builtin"},
	// Mauritania
	{"Nouadhibou", 20.920, -17.050, 4.0, "Mauritania", "cargo", "builtin"},
	// Togo
	{"Lome", 6.140, 1.290, 4.0, "Togo", "commercial", "builtin"},
	// Benin
	{"Cotonou", 6.350, 2.430, 4.0, "Benin", "commercial", "builtin"},
	// Kenya
	{"Mombasa", -4.040, 39.660, 5.0, "Kenya", "commercial", "builtin"},
	// Tanzania
	{"Dar es Salaam", -6.830, 39.290, 5.0, "Tanzania", "commercial", "builtin"},
	// Mozambique
	{"Maputo", -25.960, 32.590, 4.0, "Mozambique", "commercial", "builtin"},
	{"Beira", -19.830, 34.870, 4.0, "Mozambique", "commercial", "builtin"},
	// Madagascar
	{"Toamasina", -18.150, 49.410, 4.0, "Madagascar", "commercial", "builtin"},
	// Mauritius
	{"Port Louis", -20.160, 57.500, 4.0, "Mauritius", "commercial", "builtin"},
}

func (h *Handler) ports(w http.ResponseWriter, r *http.Request) {
	// Build effective port list: builtins + custom adds - removes
	overrides, err := h.repo.GetPortOverrides(r.Context())
	if err != nil {
		h.logger.Printf("port overrides query: %v", err)
		overrides = nil
	}

	// Collect removed port names
	removed := make(map[string]bool)
	for _, o := range overrides {
		if o.Action == "remove" {
			removed[o.Name] = true
		}
	}

	var result []builtinPort
	for _, p := range builtinPorts {
		if !removed[p.Name] {
			result = append(result, p)
		}
	}

	// Add custom ports
	for _, o := range overrides {
		if o.Action == "add" {
			result = append(result, builtinPort{
				Name: o.Name, Latitude: o.Latitude, Longitude: o.Longitude,
				RadiusKm: o.RadiusKm, Country: o.Country, PortType: o.PortType,
				Source: "custom",
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ports":     result,
		"total":     len(result),
		"builtin":   len(builtinPorts),
		"overrides": overrides,
	})
}

func (h *Handler) portOverride(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			RadiusKm  float64 `json:"radius_km"`
			Country   string  `json:"country"`
			PortType  string  `json:"port_type"`
			Action    string  `json:"action"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		if req.Action == "" {
			req.Action = "add"
		}
		if req.Action != "add" && req.Action != "remove" {
			http.Error(w, "action must be 'add' or 'remove'", http.StatusBadRequest)
			return
		}
		if req.RadiusKm <= 0 {
			req.RadiusKm = 4.0
		}
		if req.PortType == "" {
			req.PortType = "commercial"
		}

		po := &db.PortOverride{
			Name: req.Name, Latitude: req.Latitude, Longitude: req.Longitude,
			RadiusKm: req.RadiusKm, Country: req.Country, PortType: req.PortType,
			Action: req.Action,
		}
		if err := h.repo.AddPortOverride(r.Context(), po); err != nil {
			h.logger.Printf("add port override: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "saved", "override": po})

	case http.MethodDelete:
		idStr := r.URL.Query().Get("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		if err := h.repo.DeletePortOverride(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "deleted"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) purgeOldData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	daysStr := r.URL.Query().Get("days")
	days := 30
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d >= 7 {
			days = d
		}
	}

	deleted, err := h.repo.PurgeOldPositions(r.Context(), days)
	if err != nil {
		h.logger.Printf("purge error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var orphans int64
	if deleted > 0 {
		orphans, _ = h.repo.PurgeOrphanVessels(r.Context())
	}
	h.logger.Printf("Manual purge: %d positions, %d orphan vessels (retention: %d days)", deleted, orphans, days)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"positions_deleted": deleted,
		"vessels_deleted":   orphans,
		"retention_days":    days,
	})
}

func (h *Handler) spoofedVessels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hours := 24
	if hp := r.URL.Query().Get("hours"); hp != "" {
		if parsed, err := strconv.Atoi(hp); err == nil && parsed > 0 && parsed <= 720 {
			hours = parsed
		}
	}

	limit := 100
	if lp := r.URL.Query().Get("limit"); lp != "" {
		if parsed, err := strconv.Atoi(lp); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	vessels, err := h.repo.GetSpoofedVessels(r.Context(), hours, limit)
	if err != nil {
		h.logger.Printf("Error fetching spoofed vessels: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"vessels": vessels,
		"count":   len(vessels),
		"hours":   hours,
	})
}

// ========== Vessel Registry ==========

func (h *Handler) vesselRegistry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query().Get("q")
	tag := r.URL.Query().Get("tag")
	limit := 100
	if lp := r.URL.Query().Get("limit"); lp != "" {
		if parsed, err := strconv.Atoi(lp); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	entries, err := h.repo.GetVesselRegistry(r.Context(), q, tag, limit)
	if err != nil {
		h.logger.Printf("Error fetching vessel registry: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"vessels": entries,
		"count":   len(entries),
	})
}

func (h *Handler) vesselRegistryDetail(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/vessel-registry/"), "/")
	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, "MMSI required", http.StatusBadRequest)
		return
	}

	mmsi, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "Invalid MMSI", http.StatusBadRequest)
		return
	}

	// Sub-route: /api/vessel-registry/{mmsi}/notes
	if len(parts) >= 2 && parts[1] == "notes" {
		h.vesselNotes(w, r, mmsi)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vessel, err := h.repo.VesselByMMSI(r.Context(), mmsi)
	if err != nil {
		h.logger.Printf("Error fetching vessel %d: %v", mmsi, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if vessel == nil {
		http.Error(w, "Vessel not found", http.StatusNotFound)
		return
	}

	history, err := h.repo.GetVesselHistory(r.Context(), mmsi, 100)
	if err != nil {
		h.logger.Printf("Error fetching vessel history %d: %v", mmsi, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	notes, err := h.repo.GetVesselNotes(r.Context(), mmsi)
	if err != nil {
		h.logger.Printf("Error fetching vessel notes %d: %v", mmsi, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"vessel":  vessel,
		"history": history,
		"notes":   notes,
	})
}

func (h *Handler) vesselNotes(w http.ResponseWriter, r *http.Request, mmsi int64) {
	switch r.Method {
	case http.MethodPost:
		var body struct {
			Tag  string  `json:"tag"`
			Note *string `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if body.Tag == "" {
			http.Error(w, "tag is required", http.StatusBadRequest)
			return
		}
		if len(body.Tag) > 50 {
			http.Error(w, "tag too long", http.StatusBadRequest)
			return
		}
		if err := h.repo.UpsertVesselNote(r.Context(), mmsi, body.Tag, body.Note); err != nil {
			h.logger.Printf("Error upserting vessel note: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	case http.MethodDelete:
		tag := r.URL.Query().Get("tag")
		if tag == "" {
			http.Error(w, "tag query param required", http.StatusBadRequest)
			return
		}
		if err := h.repo.DeleteVesselNote(r.Context(), mmsi, tag); err != nil {
			h.logger.Printf("Error deleting vessel note: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) vesselChanges(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 100
	if lp := r.URL.Query().Get("limit"); lp != "" {
		if parsed, err := strconv.Atoi(lp); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	changes, err := h.repo.GetRecentChanges(r.Context(), limit)
	if err != nil {
		h.logger.Printf("Error fetching vessel changes: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"changes": changes,
		"count":   len(changes),
	})
}

func (h *Handler) vesselTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tags, err := h.repo.GetAllTags(r.Context())
	if err != nil {
		h.logger.Printf("Error fetching tags: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tags": tags,
	})
}

func (h *Handler) taintedVessels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 100000
	if lp := r.URL.Query().Get("limit"); lp != "" {
		if parsed, err := strconv.Atoi(lp); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	tainted, err := h.repo.GetTaintedVessels(r.Context(), limit)
	if err != nil {
		h.logger.Printf("Error fetching tainted vessels: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	total, _ := h.repo.CountTaintedVessels(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tainted":     tainted,
		"count":       len(tainted),
		"total_count": total,
	})
}

func (h *Handler) vesselTaintDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse MMSI from /api/vessel-taint/{mmsi}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/vessel-taint/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "MMSI required", http.StatusBadRequest)
		return
	}
	mmsi, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "Invalid MMSI", http.StatusBadRequest)
		return
	}

	taint, err := h.repo.GetVesselTaint(r.Context(), mmsi)
	if err != nil {
		h.logger.Printf("Error fetching vessel taint: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	portCalls, err := h.repo.GetVesselPortCalls(r.Context(), mmsi)
	if err != nil {
		h.logger.Printf("Error fetching port calls: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	encounters, err := h.repo.GetVesselEncounters(r.Context(), mmsi)
	if err != nil {
		h.logger.Printf("Error fetching encounters: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mmsi":       mmsi,
		"taint":      taint,
		"port_calls": portCalls,
		"encounters": encounters,
	})
}

func (h *Handler) taintChain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse taint ID from /api/taint-chain/{id}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/taint-chain/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Taint ID required", http.StatusBadRequest)
		return
	}
	taintID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "Invalid taint ID", http.StatusBadRequest)
		return
	}

	chain, err := h.repo.GetTaintChain(r.Context(), taintID)
	if err != nil {
		h.logger.Printf("Error fetching taint chain: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"chain": chain,
		"count": len(chain),
	})
}

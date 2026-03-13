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
}

func (h *Handler) vessels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vessels, err := h.repo.AllVesselsWithPositions(r.Context())
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
		if v.Sources != nil {
			props["sources"] = *v.Sources
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
	w.Header().Set("Cache-Control", "public, max-age=30")
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
	out := make(map[string][][2]float64, len(trails))
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
		if parsed, err := strconv.Atoi(lp); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	events, err := h.repo.GetSTSEvents(r.Context(), hours, limit)
	if err != nil {
		h.logger.Printf("Error fetching STS events: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"events": events,
		"count":  len(events),
		"hours":  hours,
	})
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

	limit := 20
	if lp := r.URL.Query().Get("limit"); lp != "" {
		if parsed, err := strconv.Atoi(lp); err == nil && parsed > 0 && parsed <= 100 {
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

	vessels, err := h.repo.GetDarkVessels(r.Context(), minHours, 500)
	if err != nil {
		h.logger.Printf("Error fetching dark vessels: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

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
		"type":     "FeatureCollection",
		"features": features,
		"count":    len(features),
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

	limit := 200
	if lp := r.URL.Query().Get("limit"); lp != "" {
		if parsed, err := strconv.Atoi(lp); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	visits, err := h.repo.GetPortVisits(r.Context(), hours, nonRussianOnly, limit)
	if err != nil {
		h.logger.Printf("Error fetching port visits: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"visits": visits,
		"count":  len(visits),
		"hours":  hours,
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
	// Russia
	{"Primorsk", 60.355, 29.206, 6.0, "Russia", "oil", "builtin"},
	{"Ust-Luga", 59.680, 28.390, 6.0, "Russia", "oil", "builtin"},
	{"Vysotsk", 60.627, 28.573, 4.0, "Russia", "oil", "builtin"},
	{"St. Petersburg", 59.933, 30.300, 8.0, "Russia", "commercial", "builtin"},
	{"Kaliningrad", 54.710, 20.500, 6.0, "Russia", "commercial", "builtin"},
	{"Kronshtadt", 59.990, 29.770, 4.0, "Russia", "naval", "builtin"},
	{"Vyborg", 60.710, 28.750, 4.0, "Russia", "cargo", "builtin"},
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
	{"Hanko", 59.820, 22.970, 3.0, "Finland", "cargo", "builtin"},
	{"Inkoo", 60.045, 24.005, 3.0, "Finland", "lng", "builtin"},
	{"Mariehamn", 60.097, 19.935, 3.0, "Finland", "ferry", "builtin"},
	// Sweden
	{"Stockholm", 59.325, 18.070, 5.0, "Sweden", "commercial", "builtin"},
	{"Nynashamn", 58.900, 17.950, 3.0, "Sweden", "oil", "builtin"},
	{"Kapellskar", 59.720, 19.065, 2.0, "Sweden", "ferry", "builtin"},
	{"Oxelosund", 58.665, 17.125, 3.0, "Sweden", "cargo", "builtin"},
	{"Visby", 57.640, 18.290, 3.0, "Sweden", "commercial", "builtin"},
	{"Gavle", 60.675, 17.195, 4.0, "Sweden", "lng", "builtin"},
	{"Lulea", 65.575, 22.145, 5.0, "Sweden", "cargo", "builtin"},
	{"Gothenburg", 57.695, 11.945, 5.0, "Sweden", "commercial", "builtin"},
	{"Lysekil", 58.275, 11.430, 3.0, "Sweden", "oil", "builtin"},
	// Estonia
	{"Tallinn / Muuga", 59.490, 24.960, 5.0, "Estonia", "commercial", "builtin"},
	{"Paldiski", 59.350, 24.050, 3.0, "Estonia", "lng", "builtin"},
	{"Sillamae", 59.400, 27.760, 3.0, "Estonia", "oil", "builtin"},
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
	// Germany
	{"Rostock", 54.180, 12.100, 5.0, "Germany", "commercial", "builtin"},
	{"Lubeck", 53.960, 10.870, 4.0, "Germany", "commercial", "builtin"},
	{"Kiel", 54.330, 10.150, 4.0, "Germany", "commercial", "builtin"},
	{"Brunsbuttel", 53.895, 9.140, 3.0, "Germany", "lng", "builtin"},
	// Denmark
	{"Copenhagen", 55.690, 12.610, 5.0, "Denmark", "commercial", "builtin"},
	{"Fredericia", 55.565, 9.745, 3.0, "Denmark", "oil", "builtin"},
	// Norway
	{"Oslo", 59.900, 10.740, 5.0, "Norway", "commercial", "builtin"},
	{"Stavanger", 58.970, 5.730, 4.0, "Norway", "oil", "builtin"},
	{"Bergen", 60.395, 5.320, 4.0, "Norway", "commercial", "builtin"},
	{"Mongstad", 60.810, 5.035, 3.0, "Norway", "oil", "builtin"},
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

package sources

import (
	"context"
	"time"

	"github.com/bsuhs/shiptracker/ais-collector/internal/database"
)

// AISData represents the combined vessel and position data from a source
type AISData struct {
	Vessel   *database.Vessel
	Position *database.AISPosition
}

// Source defines the interface that all AIS data sources must implement
type Source interface {
	// Name returns the unique name of this source
	Name() string

	// Type returns the source type (e.g., "api", "stream", "file")
	Type() string

	// Fetch retrieves AIS data from the source
	// Returns a slice of AISData containing vessel and position information
	Fetch(ctx context.Context) ([]*AISData, error)

	// HealthCheck verifies the source is accessible and functioning
	HealthCheck(ctx context.Context) error

	// Close cleans up any resources used by the source
	Close() error
}

// Config holds common configuration for sources
type Config struct {
	Name       string
	APIKey     string
	BaseURL    string
	Timeout    time.Duration
	MaxRetries int
}

// NavigationStatusName converts navigation status code to name
func NavigationStatusName(status int) string {
	statusMap := map[int]string{
		0:  "Under way using engine",
		1:  "At anchor",
		2:  "Not under command",
		3:  "Restricted manoeuverability",
		4:  "Constrained by her draught",
		5:  "Moored",
		6:  "Aground",
		7:  "Engaged in fishing",
		8:  "Under way sailing",
		9:  "Reserved for HSC",
		10: "Reserved for WIG",
		11: "Power-driven vessel towing astern",
		12: "Power-driven vessel pushing ahead",
		13: "Reserved",
		14: "AIS-SART",
		15: "Undefined",
	}

	if name, ok := statusMap[status]; ok {
		return name
	}
	return "Unknown"
}

// VesselTypeName converts vessel type code to name
func VesselTypeName(vesselType int) string {
	switch {
	case vesselType >= 20 && vesselType <= 29:
		return "Wing in ground (WIG)"
	case vesselType == 30:
		return "Fishing"
	case vesselType >= 31 && vesselType <= 32:
		return "Towing"
	case vesselType == 33:
		return "Dredging or underwater ops"
	case vesselType == 34:
		return "Diving ops"
	case vesselType == 35:
		return "Military ops"
	case vesselType == 36:
		return "Sailing"
	case vesselType == 37:
		return "Pleasure craft"
	case vesselType >= 40 && vesselType <= 49:
		return "High speed craft (HSC)"
	case vesselType == 50:
		return "Pilot vessel"
	case vesselType == 51:
		return "Search and rescue vessel"
	case vesselType == 52:
		return "Tug"
	case vesselType == 53:
		return "Port tender"
	case vesselType == 54:
		return "Anti-pollution equipment"
	case vesselType == 55:
		return "Law enforcement"
	case vesselType >= 60 && vesselType <= 69:
		return "Passenger"
	case vesselType >= 70 && vesselType <= 79:
		return "Cargo"
	case vesselType >= 80 && vesselType <= 89:
		return "Tanker"
	case vesselType >= 90 && vesselType <= 99:
		return "Other type"
	default:
		return "Unknown"
	}
}

// Helper functions for building optional pointers

func stringPtrIfNotEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intPtrIfNotZero(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}

func floatPtrIfNotZero(f float64) *float64 {
	if f == 0.0 {
		return nil
	}
	return &f
}

func int64Ptr(i int64) *int64 {
	if i == 0 {
		return nil
	}
	return &i
}

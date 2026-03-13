package database

import (
	"context"
	"time"
)

// Vessel represents a ship/vessel in the database
type Vessel struct {
	MMSI           int64      `json:"mmsi"`
	IMONumber      *int64     `json:"imo_number,omitempty"`
	Name           *string    `json:"name,omitempty"`
	CallSign       *string    `json:"call_sign,omitempty"`
	VesselType     *int       `json:"vessel_type,omitempty"`
	VesselTypeName *string    `json:"vessel_type_name,omitempty"`
	DimensionA     *int       `json:"dimension_a,omitempty"`
	DimensionB     *int       `json:"dimension_b,omitempty"`
	DimensionC     *int       `json:"dimension_c,omitempty"`
	DimensionD     *int       `json:"dimension_d,omitempty"`
	Draught        *float64   `json:"draught,omitempty"`
	Destination    *string    `json:"destination,omitempty"`
	ETA            *time.Time `json:"eta,omitempty"`
	FirstSeenAt    time.Time  `json:"first_seen_at"`
	LastSeenAt     time.Time  `json:"last_seen_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// AISPosition represents a position report from AIS
type AISPosition struct {
	ID                   int64     `json:"id"`
	MMSI                 int64     `json:"mmsi"`
	SourceID             *int      `json:"source_id,omitempty"`
	Latitude             float64   `json:"latitude"`
	Longitude            float64   `json:"longitude"`
	SpeedOverGround      *float64  `json:"speed_over_ground,omitempty"`
	CourseOverGround     *float64  `json:"course_over_ground,omitempty"`
	Heading              *int      `json:"heading,omitempty"`
	NavigationStatus     *int      `json:"navigation_status,omitempty"`
	NavigationStatusName *string   `json:"navigation_status_name,omitempty"`
	Timestamp            time.Time `json:"timestamp"`
	ReceivedAt           time.Time `json:"received_at"`
	CreatedAt            time.Time `json:"created_at"`
}

// AISSource represents a data source/provider
type AISSource struct {
	ID         int        `json:"id"`
	Name       string     `json:"name"`
	SourceType string     `json:"source_type"`
	Enabled    bool       `json:"enabled"`
	LastPollAt *time.Time `json:"last_poll_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// VesselWithPosition represents a vessel joined with its latest position
type VesselWithPosition struct {
	MMSI                 int64     `json:"mmsi"`
	Name                 *string   `json:"name,omitempty"`
	VesselTypeName       *string   `json:"vessel_type_name,omitempty"`
	Latitude             float64   `json:"latitude"`
	Longitude            float64   `json:"longitude"`
	SpeedOverGround      *float64  `json:"speed_over_ground,omitempty"`
	CourseOverGround     *float64  `json:"course_over_ground,omitempty"`
	Heading              *int      `json:"heading,omitempty"`
	NavigationStatusName *string   `json:"navigation_status_name,omitempty"`
	Timestamp            time.Time `json:"timestamp"`
	Destination          *string   `json:"destination,omitempty"`
	CallSign             *string   `json:"call_sign,omitempty"`
	IMONumber            *int64    `json:"imo_number,omitempty"`
	Draught              *float64  `json:"draught,omitempty"`
}

// Repository defines database operations
type Repository interface {
	// Vessel operations
	UpsertVessel(ctx context.Context, vessel *Vessel) error
	GetVessel(ctx context.Context, mmsi int64) (*Vessel, error)
	UpdateVesselLastSeen(ctx context.Context, mmsi int64, lastSeen time.Time) error

	// Position operations
	InsertPosition(ctx context.Context, position *AISPosition) error
	InsertPositions(ctx context.Context, positions []*AISPosition) error
	GetLatestPositions(ctx context.Context, limit int) ([]*AISPosition, error)

	// Web/API operations
	GetAllVesselsWithPositions(ctx context.Context) ([]*VesselWithPosition, error)
	GetVesselTrack(ctx context.Context, mmsi int64, hours int) ([]*AISPosition, error)
	GetStats(ctx context.Context) (*DBStats, error)

	// Source operations
	GetSource(ctx context.Context, name string) (*AISSource, error)
	UpsertSource(ctx context.Context, source *AISSource) error
	UpdateSourcePollTime(ctx context.Context, sourceID int, pollTime time.Time) error
	GetEnabledSources(ctx context.Context) ([]*AISSource, error)

	// Utility
	Close() error
}

// DBStats holds summary statistics
type DBStats struct {
	TotalVessels    int        `json:"total_vessels"`
	RussianVessels  int        `json:"russian_vessels"`
	TotalPositions  int64      `json:"total_positions"`
	LastCollectedAt *time.Time `json:"last_collected_at,omitempty"`
}

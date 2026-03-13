package sources

import (
	"context"
	"net/http"
	"time"

	"github.com/bsuhs/shiptracker/ais-collector/internal/database"
)

// MockSource is a sample implementation for testing
// Replace this with real source implementations (AISHub, MarineTraffic, etc.)
type MockSource struct {
	config Config
	client *http.Client
}

// NewMockSource creates a new mock AIS data source
func NewMockSource(config Config) *MockSource {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	return &MockSource{
		config: config,
		client: client,
	}
}

func (s *MockSource) Name() string {
	return s.config.Name
}

func (s *MockSource) Type() string {
	return "api"
}

func (s *MockSource) Fetch(ctx context.Context) ([]*AISData, error) {
	// This is a mock implementation
	// In a real implementation, you would:
	// 1. Make HTTP request to the AIS API
	// 2. Parse the response
	// 3. Transform to AISData format

	// Example mock data
	now := time.Now()
	mockData := []*AISData{
		{
			Vessel: &database.Vessel{
				MMSI:           367123456,
				Name:           stringPtr("EXAMPLE SHIP"),
				CallSign:       stringPtr("WDA1234"),
				VesselType:     intPtr(70),
				VesselTypeName: stringPtr("Cargo"),
				LastSeenAt:     now,
			},
			Position: &database.AISPosition{
				MMSI:                 367123456,
				Latitude:             37.7749,
				Longitude:            -122.4194,
				SpeedOverGround:      floatPtr(12.5),
				CourseOverGround:     floatPtr(185.0),
				Heading:              intPtr(180),
				NavigationStatus:     intPtr(0),
				NavigationStatusName: stringPtr("Under way using engine"),
				Timestamp:            now,
			},
		},
	}

	return mockData, nil
}

func (s *MockSource) HealthCheck(ctx context.Context) error {
	// Implement health check for your specific source
	// For example, ping the API endpoint
	return nil
}

func (s *MockSource) Close() error {
	s.client.CloseIdleConnections()
	return nil
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func floatPtr(f float64) *float64 {
	return &f
}

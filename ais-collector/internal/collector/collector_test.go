package collector

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/bsuhs/shiptracker/ais-collector/internal/database"
	"github.com/bsuhs/shiptracker/ais-collector/internal/sources"
)

// MockRepository implements a mock database repository for testing
type MockRepository struct {
	vessels   map[int64]*database.Vessel
	positions []*database.AISPosition
	sources   map[string]*database.AISSource
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		vessels:   make(map[int64]*database.Vessel),
		positions: make([]*database.AISPosition, 0),
		sources:   make(map[string]*database.AISSource),
	}
}

func (m *MockRepository) UpsertVessel(ctx context.Context, vessel *database.Vessel) error {
	m.vessels[vessel.MMSI] = vessel
	return nil
}

func (m *MockRepository) GetVessel(ctx context.Context, mmsi int64) (*database.Vessel, error) {
	return m.vessels[mmsi], nil
}

func (m *MockRepository) UpdateVesselLastSeen(ctx context.Context, mmsi int64, lastSeen time.Time) error {
	if v, ok := m.vessels[mmsi]; ok {
		v.LastSeenAt = lastSeen
	}
	return nil
}

func (m *MockRepository) InsertPosition(ctx context.Context, position *database.AISPosition) error {
	m.positions = append(m.positions, position)
	return nil
}

func (m *MockRepository) InsertPositions(ctx context.Context, positions []*database.AISPosition) error {
	m.positions = append(m.positions, positions...)
	return nil
}

func (m *MockRepository) GetLatestPositions(ctx context.Context, limit int) ([]*database.AISPosition, error) {
	if len(m.positions) < limit {
		return m.positions, nil
	}
	return m.positions[:limit], nil
}

func (m *MockRepository) GetSource(ctx context.Context, name string) (*database.AISSource, error) {
	return m.sources[name], nil
}

func (m *MockRepository) UpsertSource(ctx context.Context, source *database.AISSource) error {
	source.ID = len(m.sources) + 1
	m.sources[source.Name] = source
	return nil
}

func (m *MockRepository) UpdateSourcePollTime(ctx context.Context, sourceID int, pollTime time.Time) error {
	return nil
}

func (m *MockRepository) GetEnabledSources(ctx context.Context) ([]*database.AISSource, error) {
	result := make([]*database.AISSource, 0)
	for _, source := range m.sources {
		if source.Enabled {
			result = append(result, source)
		}
	}
	return result, nil
}

func (m *MockRepository) GetAllVesselsWithPositions(ctx context.Context) ([]*database.VesselWithPosition, error) {
	return nil, nil
}

func (m *MockRepository) GetVesselTrack(ctx context.Context, mmsi int64, hours int) ([]*database.AISPosition, error) {
	return nil, nil
}

func (m *MockRepository) GetStats(ctx context.Context) (*database.DBStats, error) {
	return &database.DBStats{}, nil
}

func (m *MockRepository) Close() error {
	return nil
}

func TestCollectorCreation(t *testing.T) {
	repo := NewMockRepository()
	mockSource := sources.NewMockSource(sources.Config{Name: "test", Timeout: 10 * time.Second})
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	collector := New(repo, []sources.Source{mockSource}, 1*time.Minute, logger)

	if collector == nil {
		t.Fatal("Expected collector to be created")
	}

	if len(collector.sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(collector.sources))
	}
}

func TestSourceRegistration(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	mockSource := sources.NewMockSource(sources.Config{Name: "test-source", Timeout: 10 * time.Second})
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	collector := New(repo, []sources.Source{mockSource}, 1*time.Minute, logger)

	if err := collector.registerSources(ctx); err != nil {
		t.Fatalf("Failed to register sources: %v", err)
	}

	// Verify source was registered
	source, err := repo.GetSource(ctx, "test-source")
	if err != nil {
		t.Fatalf("Failed to get source: %v", err)
	}

	if source == nil {
		t.Fatal("Expected source to be registered")
	}

	if source.Name != "test-source" {
		t.Errorf("Expected source name 'test-source', got '%s'", source.Name)
	}
}

func TestCollectFromSource(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	mockSource := sources.NewMockSource(sources.Config{Name: "test-source", Timeout: 10 * time.Second})
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	collector := New(repo, []sources.Source{mockSource}, 1*time.Minute, logger)

	// Register source first
	if err := collector.registerSources(ctx); err != nil {
		t.Fatalf("Failed to register sources: %v", err)
	}

	// Collect data
	if err := collector.collectFromSource(ctx, mockSource); err != nil {
		t.Fatalf("Failed to collect from source: %v", err)
	}

	// Verify data was collected
	if len(repo.positions) == 0 {
		t.Error("Expected positions to be collected")
	}

	if len(repo.vessels) == 0 {
		t.Error("Expected vessels to be collected")
	}
}

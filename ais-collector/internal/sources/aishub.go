package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bsuhs/shiptracker/ais-collector/internal/database"
)

// AISHubSource is a template for AISHub.com integration.
// Requires registration at https://www.aishub.net/ for a free API key.
// Currently a stub — not fully implemented.
type AISHubSource struct {
	config Config
	client *http.Client
}

func NewAISHubSource(config Config) *AISHubSource {
	return &AISHubSource{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
	}
}

func (s *AISHubSource) Name() string { return s.config.Name }
func (s *AISHubSource) Type() string { return "api" }

type AISHubResponse struct {
	Error     bool            `json:"ERROR"`
	ErrorText string          `json:"ERRORTXT,omitempty"`
	Vessels   [][]interface{} `json:"VESSELS"`
}

func (s *AISHubSource) Fetch(ctx context.Context) ([]*AISData, error) {
	url := fmt.Sprintf("%s?username=%s&format=1&output=json&compress=0",
		s.config.BaseURL, s.config.APIKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("aishub: create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("aishub: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("aishub: status %d: %s", resp.StatusCode, string(body))
	}

	var aisResp AISHubResponse
	if err := json.NewDecoder(resp.Body).Decode(&aisResp); err != nil {
		return nil, fmt.Errorf("aishub: decode: %w", err)
	}

	if aisResp.Error {
		return nil, fmt.Errorf("aishub: API error: %s", aisResp.ErrorText)
	}

	var results []*AISData
	for _, vessel := range aisResp.Vessels {
		if len(vessel) < 10 {
			continue
		}
		mmsi, ok := vessel[0].(float64)
		if !ok {
			continue
		}
		results = append(results, &AISData{
			Vessel: &database.Vessel{
				MMSI:       int64(mmsi),
				LastSeenAt: time.Now(),
			},
			Position: &database.AISPosition{
				MMSI:      int64(mmsi),
				Timestamp: time.Now(),
			},
		})
	}

	return results, nil
}

func (s *AISHubSource) HealthCheck(ctx context.Context) error { return nil }

func (s *AISHubSource) Close() error {
	s.client.CloseIdleConnections()
	return nil
}

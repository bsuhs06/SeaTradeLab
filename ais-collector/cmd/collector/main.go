package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bsuhs/shiptracker/ais-collector/internal/collector"
	"github.com/bsuhs/shiptracker/ais-collector/internal/database"
	"github.com/bsuhs/shiptracker/ais-collector/internal/sources"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Setup logger
	logger := log.New(os.Stdout, "[AIS-COLLECTOR] ", log.LstdFlags|log.Lshortfile)

	// Get configuration from environment
	databaseURL := getEnv("DATABASE_URL", "postgres://localhost:5432/ais_data?sslmode=disable")
	pollIntervalMinutes := getEnvAsInt("POLL_INTERVAL_MINUTES", 10)

	logger.Printf("Starting AIS Data Collector")
	logger.Printf("Database: %s", maskDatabaseURL(databaseURL))
	logger.Printf("Poll interval: %d minutes", pollIntervalMinutes)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize database connection
	repo, err := database.NewPgRepository(ctx, databaseURL)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer repo.Close()

	logger.Println("Successfully connected to database")

	// Initialize data sources via registry
	dataSources := sources.CreateSources(sources.DefaultRegistry, buildSourceDefs(), logger)
	if len(dataSources) == 0 {
		logger.Fatal("No data sources configured")
	}

	// Create collector
	pollInterval := time.Duration(pollIntervalMinutes) * time.Minute
	col := collector.New(repo, dataSources, pollInterval, logger)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start collector in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := col.Start(ctx); err != nil {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		logger.Println("Received shutdown signal")
		cancel()
		if err := col.Stop(); err != nil {
			logger.Printf("Error during shutdown: %v", err)
		}
	case err := <-errChan:
		logger.Printf("Collector error: %v", err)
		cancel()
		col.Stop()
	}

	logger.Println("Collector stopped successfully")
}

// buildSourceDefs builds source definitions from environment variables.
// Each source type can be enabled/disabled independently.
// To add a new source: register it in sources/registry.go and add an env block here.
func buildSourceDefs() []sources.SourceDef {
	return []sources.SourceDef{
		{
			Type:    "digitraffic",
			Enabled: getEnv("ENABLE_DIGITRAFFIC", "true") == "true",
			Config: sources.Config{
				Name:       "digitraffic",
				BaseURL:    getEnv("DIGITRAFFIC_URL", "https://meri.digitraffic.fi"),
				Timeout:    45 * time.Second,
				MaxRetries: 2,
			},
		},
		{
			Type:    "mock",
			Enabled: getEnv("ENABLE_MOCK_SOURCE", "false") == "true",
			Config: sources.Config{
				Name:       "mock",
				Timeout:    30 * time.Second,
				MaxRetries: 3,
			},
		},
		{
			Type:    "aishub",
			Enabled: os.Getenv("AISHUB_USERNAME") != "",
			Config: sources.Config{
				Name:       "aishub",
				APIKey:     os.Getenv("AISHUB_USERNAME"),
				BaseURL:    getEnv("AISHUB_BASE_URL", "http://data.aishub.net/ws.php"),
				Timeout:    30 * time.Second,
				MaxRetries: 3,
			},
		},
		{
			Type:    "aisstream",
			Enabled: os.Getenv("AISSTREAM_API_KEY") != "",
			Config: sources.Config{
				Name:       "aisstream",
				APIKey:     os.Getenv("AISSTREAM_API_KEY"),
				BaseURL:    getEnv("AISSTREAM_URL", "wss://stream.aisstream.io/v0/stream"),
				Timeout:    30 * time.Second,
				MaxRetries: 3,
			},
		},
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt gets an environment variable as an integer with a default value
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// maskDatabaseURL masks sensitive parts of the database URL for logging
func maskDatabaseURL(url string) string {
	if len(url) < 20 {
		return "***"
	}
	// Simple masking - show only protocol and host
	return url[:10] + "***"
}

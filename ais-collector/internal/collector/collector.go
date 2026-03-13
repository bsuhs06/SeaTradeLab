package collector

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bsuhs/shiptracker/ais-collector/internal/database"
	"github.com/bsuhs/shiptracker/ais-collector/internal/sources"
)

// Collector manages the collection of AIS data from multiple sources
type Collector struct {
	repo     database.Repository
	sources  []sources.Source
	interval time.Duration
	logger   *log.Logger
}

// New creates a new Collector
func New(repo database.Repository, sources []sources.Source, interval time.Duration, logger *log.Logger) *Collector {
	return &Collector{
		repo:     repo,
		sources:  sources,
		interval: interval,
		logger:   logger,
	}
}

// Start begins the collection process
func (c *Collector) Start(ctx context.Context) error {
	c.logger.Printf("Starting AIS data collector with %d sources", len(c.sources))
	
	// Register all sources in the database
	if err := c.registerSources(ctx); err != nil {
		return fmt.Errorf("failed to register sources: %w", err)
	}

	// Initial collection
	if err := c.collectAll(ctx); err != nil {
		c.logger.Printf("Initial collection error: %v", err)
	}

	// Start periodic collection
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Println("Collector stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := c.collectAll(ctx); err != nil {
				c.logger.Printf("Collection error: %v", err)
			}
		}
	}
}

// registerSources ensures all sources are registered in the database
func (c *Collector) registerSources(ctx context.Context) error {
	for _, src := range c.sources {
		source := &database.AISSource{
			Name:       src.Name(),
			SourceType: src.Type(),
			Enabled:    true,
		}

		if err := c.repo.UpsertSource(ctx, source); err != nil {
			return fmt.Errorf("failed to register source %s: %w", src.Name(), err)
		}

		c.logger.Printf("Registered source: %s (ID: %d)", source.Name, source.ID)
	}

	return nil
}

// collectAll collects data from all sources concurrently
func (c *Collector) collectAll(ctx context.Context) error {
	c.logger.Println("Starting data collection cycle")
	startTime := time.Now()

	var wg sync.WaitGroup
	errChan := make(chan error, len(c.sources))

	for _, src := range c.sources {
		wg.Add(1)
		go func(source sources.Source) {
			defer wg.Done()
			if err := c.collectFromSource(ctx, source); err != nil {
				errChan <- fmt.Errorf("source %s: %w", source.Name(), err)
			}
		}(src)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Collect any errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
		c.logger.Printf("Collection error: %v", err)
	}

	duration := time.Since(startTime)
	c.logger.Printf("Collection cycle completed in %v", duration)

	if len(errors) > 0 {
		return fmt.Errorf("collection completed with %d errors", len(errors))
	}

	return nil
}

// collectFromSource collects data from a single source
func (c *Collector) collectFromSource(ctx context.Context, source sources.Source) error {
	c.logger.Printf("Fetching data from source: %s", source.Name())

	// Get source from database to get its ID
	dbSource, err := c.repo.GetSource(ctx, source.Name())
	if err != nil {
		return fmt.Errorf("failed to get source: %w", err)
	}
	if dbSource == nil {
		return fmt.Errorf("source not found in database")
	}

	// Fetch data
	data, err := source.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch data: %w", err)
	}

	c.logger.Printf("Fetched %d records from %s", len(data), source.Name())

	// Process and store data
	var positionsStored, vesselsUpserted int
	for _, item := range data {
		// Upsert vessel information
		if item.Vessel != nil {
			if err := c.repo.UpsertVessel(ctx, item.Vessel); err != nil {
				c.logger.Printf("Failed to upsert vessel %d: %v", item.Vessel.MMSI, err)
				continue
			}
			vesselsUpserted++
		}

		// Insert position data
		if item.Position != nil {
			item.Position.SourceID = &dbSource.ID
			if err := c.repo.InsertPosition(ctx, item.Position); err != nil {
				c.logger.Printf("Failed to insert position for MMSI %d: %v", item.Position.MMSI, err)
				continue
			}
			positionsStored++
		}
	}

	// Update source last poll time
	if err := c.repo.UpdateSourcePollTime(ctx, dbSource.ID, time.Now()); err != nil {
		c.logger.Printf("Failed to update poll time for source %s: %v", source.Name(), err)
	}

	c.logger.Printf("Source %s: stored %d positions, upserted %d vessels",
		source.Name(), positionsStored, vesselsUpserted)

	return nil
}

// Stop gracefully shuts down the collector
func (c *Collector) Stop() error {
	c.logger.Println("Shutting down collector...")

	var errors []error
	for _, src := range c.sources {
		if err := src.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close source %s: %w", src.Name(), err))
		}
	}

	if err := c.repo.Close(); err != nil {
		errors = append(errors, fmt.Errorf("failed to close repository: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown completed with errors: %v", errors)
	}

	return nil
}

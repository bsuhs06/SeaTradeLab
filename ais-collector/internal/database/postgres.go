package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgRepository implements Repository using PostgreSQL
type PgRepository struct {
	pool *pgxpool.Pool
}

// NewPgRepository creates a new PostgreSQL repository
func NewPgRepository(ctx context.Context, databaseURL string) (*PgRepository, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return &PgRepository{pool: pool}, nil
}

// Close closes the database connection pool
func (r *PgRepository) Close() error {
	r.pool.Close()
	return nil
}

// UpsertVessel inserts or updates a vessel
func (r *PgRepository) UpsertVessel(ctx context.Context, vessel *Vessel) error {
	query := `
		INSERT INTO vessels (
			mmsi, imo_number, name, call_sign, vessel_type, vessel_type_name,
			dimension_a, dimension_b, dimension_c, dimension_d, draught,
			destination, eta, last_seen_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
		ON CONFLICT (mmsi) DO UPDATE SET
			imo_number = COALESCE(EXCLUDED.imo_number, vessels.imo_number),
			name = COALESCE(EXCLUDED.name, vessels.name),
			call_sign = COALESCE(EXCLUDED.call_sign, vessels.call_sign),
			vessel_type = COALESCE(EXCLUDED.vessel_type, vessels.vessel_type),
			vessel_type_name = COALESCE(EXCLUDED.vessel_type_name, vessels.vessel_type_name),
			dimension_a = COALESCE(EXCLUDED.dimension_a, vessels.dimension_a),
			dimension_b = COALESCE(EXCLUDED.dimension_b, vessels.dimension_b),
			dimension_c = COALESCE(EXCLUDED.dimension_c, vessels.dimension_c),
			dimension_d = COALESCE(EXCLUDED.dimension_d, vessels.dimension_d),
			draught = COALESCE(EXCLUDED.draught, vessels.draught),
			destination = COALESCE(EXCLUDED.destination, vessels.destination),
			eta = COALESCE(EXCLUDED.eta, vessels.eta),
			last_seen_at = EXCLUDED.last_seen_at,
			updated_at = NOW()
	`

	_, err := r.pool.Exec(ctx, query,
		vessel.MMSI, vessel.IMONumber, vessel.Name, vessel.CallSign,
		vessel.VesselType, vessel.VesselTypeName, vessel.DimensionA,
		vessel.DimensionB, vessel.DimensionC, vessel.DimensionD,
		vessel.Draught, vessel.Destination, vessel.ETA, vessel.LastSeenAt,
	)

	return err
}

// GetVessel retrieves a vessel by MMSI
func (r *PgRepository) GetVessel(ctx context.Context, mmsi int64) (*Vessel, error) {
	query := `
		SELECT mmsi, imo_number, name, call_sign, vessel_type, vessel_type_name,
			   dimension_a, dimension_b, dimension_c, dimension_d, draught,
			   destination, eta, first_seen_at, last_seen_at, created_at, updated_at
		FROM vessels WHERE mmsi = $1
	`

	var vessel Vessel
	err := r.pool.QueryRow(ctx, query, mmsi).Scan(
		&vessel.MMSI, &vessel.IMONumber, &vessel.Name, &vessel.CallSign,
		&vessel.VesselType, &vessel.VesselTypeName, &vessel.DimensionA,
		&vessel.DimensionB, &vessel.DimensionC, &vessel.DimensionD,
		&vessel.Draught, &vessel.Destination, &vessel.ETA,
		&vessel.FirstSeenAt, &vessel.LastSeenAt, &vessel.CreatedAt, &vessel.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &vessel, nil
}

// UpdateVesselLastSeen updates the last seen timestamp
func (r *PgRepository) UpdateVesselLastSeen(ctx context.Context, mmsi int64, lastSeen time.Time) error {
	query := `UPDATE vessels SET last_seen_at = $1, updated_at = NOW() WHERE mmsi = $2`
	_, err := r.pool.Exec(ctx, query, lastSeen, mmsi)
	return err
}

// InsertPosition inserts a single position report
func (r *PgRepository) InsertPosition(ctx context.Context, position *AISPosition) error {
	query := `
		INSERT INTO ais_positions (
			mmsi, source_id, latitude, longitude, position,
			speed_over_ground, course_over_ground, heading,
			navigation_status, navigation_status_name, timestamp
		) VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($5, $6), 4326)::geography,
				  $7, $8, $9, $10, $11, $12)
	`

	_, err := r.pool.Exec(ctx, query,
		position.MMSI, position.SourceID,
		position.Latitude, position.Longitude,
		position.Longitude, position.Latitude,
		position.SpeedOverGround, position.CourseOverGround, position.Heading,
		position.NavigationStatus, position.NavigationStatusName, position.Timestamp,
	)

	return err
}

// InsertPositions inserts multiple position reports in a batch
func (r *PgRepository) InsertPositions(ctx context.Context, positions []*AISPosition) error {
	batch := &pgx.Batch{}

	query := `
		INSERT INTO ais_positions (
			mmsi, source_id, latitude, longitude, position,
			speed_over_ground, course_over_ground, heading,
			navigation_status, navigation_status_name, timestamp
		) VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($5, $6), 4326)::geography,
				  $7, $8, $9, $10, $11, $12)
	`

	for _, pos := range positions {
		batch.Queue(query,
			pos.MMSI, pos.SourceID,
			pos.Latitude, pos.Longitude,
			pos.Longitude, pos.Latitude,
			pos.SpeedOverGround, pos.CourseOverGround, pos.Heading,
			pos.NavigationStatus, pos.NavigationStatusName, pos.Timestamp,
		)
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(positions); i++ {
		_, err := br.Exec()
		if err != nil {
			return fmt.Errorf("failed to insert position %d: %w", i, err)
		}
	}

	return nil
}

// GetLatestPositions retrieves the most recent position reports
func (r *PgRepository) GetLatestPositions(ctx context.Context, limit int) ([]*AISPosition, error) {
	query := `
		SELECT id, mmsi, source_id, latitude, longitude,
			   speed_over_ground, course_over_ground, heading,
			   navigation_status, navigation_status_name,
			   timestamp, received_at, created_at
		FROM ais_positions
		ORDER BY timestamp DESC
		LIMIT $1
	`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []*AISPosition
	for rows.Next() {
		var pos AISPosition
		if err := rows.Scan(
			&pos.ID, &pos.MMSI, &pos.SourceID,
			&pos.Latitude, &pos.Longitude,
			&pos.SpeedOverGround, &pos.CourseOverGround, &pos.Heading,
			&pos.NavigationStatus, &pos.NavigationStatusName,
			&pos.Timestamp, &pos.ReceivedAt, &pos.CreatedAt,
		); err != nil {
			return nil, err
		}
		positions = append(positions, &pos)
	}

	return positions, rows.Err()
}

// GetSource retrieves a source by name
func (r *PgRepository) GetSource(ctx context.Context, name string) (*AISSource, error) {
	query := `
		SELECT id, name, source_type, enabled, last_poll_at, created_at, updated_at
		FROM ais_sources WHERE name = $1
	`

	var source AISSource
	err := r.pool.QueryRow(ctx, query, name).Scan(
		&source.ID, &source.Name, &source.SourceType, &source.Enabled,
		&source.LastPollAt, &source.CreatedAt, &source.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &source, nil
}

// UpsertSource inserts or updates a source
func (r *PgRepository) UpsertSource(ctx context.Context, source *AISSource) error {
	query := `
		INSERT INTO ais_sources (name, source_type, enabled, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (name) DO UPDATE SET
			source_type = EXCLUDED.source_type,
			enabled = EXCLUDED.enabled,
			updated_at = NOW()
		RETURNING id
	`

	return r.pool.QueryRow(ctx, query, source.Name, source.SourceType, source.Enabled).Scan(&source.ID)
}

// UpdateSourcePollTime updates the last poll time for a source
func (r *PgRepository) UpdateSourcePollTime(ctx context.Context, sourceID int, pollTime time.Time) error {
	query := `UPDATE ais_sources SET last_poll_at = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, pollTime, sourceID)
	return err
}

// GetEnabledSources retrieves all enabled sources
func (r *PgRepository) GetEnabledSources(ctx context.Context) ([]*AISSource, error) {
	query := `
		SELECT id, name, source_type, enabled, last_poll_at, created_at, updated_at
		FROM ais_sources WHERE enabled = true
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []*AISSource
	for rows.Next() {
		var source AISSource
		if err := rows.Scan(
			&source.ID, &source.Name, &source.SourceType, &source.Enabled,
			&source.LastPollAt, &source.CreatedAt, &source.UpdatedAt,
		); err != nil {
			return nil, err
		}
		sources = append(sources, &source)
	}

	return sources, rows.Err()
}

// GetAllVesselsWithPositions returns all vessels with their latest position
func (r *PgRepository) GetAllVesselsWithPositions(ctx context.Context) ([]*VesselWithPosition, error) {
	query := `
		SELECT
			v.mmsi, v.name, v.vessel_type_name, v.call_sign, v.imo_number, v.draught,
			ap.latitude, ap.longitude, ap.speed_over_ground, ap.course_over_ground,
			ap.heading, ap.navigation_status_name, ap.timestamp, v.destination
		FROM vessels v
		INNER JOIN LATERAL (
			SELECT latitude, longitude, speed_over_ground, course_over_ground,
				   heading, navigation_status_name, timestamp
			FROM ais_positions
			WHERE mmsi = v.mmsi
			ORDER BY timestamp DESC
			LIMIT 1
		) ap ON true
		ORDER BY v.mmsi
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vessels []*VesselWithPosition
	for rows.Next() {
		var vp VesselWithPosition
		if err := rows.Scan(
			&vp.MMSI, &vp.Name, &vp.VesselTypeName, &vp.CallSign, &vp.IMONumber, &vp.Draught,
			&vp.Latitude, &vp.Longitude, &vp.SpeedOverGround, &vp.CourseOverGround,
			&vp.Heading, &vp.NavigationStatusName, &vp.Timestamp, &vp.Destination,
		); err != nil {
			return nil, err
		}
		vessels = append(vessels, &vp)
	}

	return vessels, rows.Err()
}

// GetVesselTrack returns position history for a vessel over the given hours
func (r *PgRepository) GetVesselTrack(ctx context.Context, mmsi int64, hours int) ([]*AISPosition, error) {
	query := `
		SELECT id, mmsi, source_id, latitude, longitude,
			   speed_over_ground, course_over_ground, heading,
			   navigation_status, navigation_status_name,
			   timestamp, received_at, created_at
		FROM ais_positions
		WHERE mmsi = $1 AND timestamp > NOW() - ($2 || ' hours')::interval
		ORDER BY timestamp ASC
	`

	rows, err := r.pool.Query(ctx, query, mmsi, fmt.Sprintf("%d", hours))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []*AISPosition
	for rows.Next() {
		var pos AISPosition
		if err := rows.Scan(
			&pos.ID, &pos.MMSI, &pos.SourceID,
			&pos.Latitude, &pos.Longitude,
			&pos.SpeedOverGround, &pos.CourseOverGround, &pos.Heading,
			&pos.NavigationStatus, &pos.NavigationStatusName,
			&pos.Timestamp, &pos.ReceivedAt, &pos.CreatedAt,
		); err != nil {
			return nil, err
		}
		positions = append(positions, &pos)
	}

	return positions, rows.Err()
}

// GetStats returns summary statistics
func (r *PgRepository) GetStats(ctx context.Context) (*DBStats, error) {
	stats := &DBStats{}

	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM vessels`).Scan(&stats.TotalVessels)
	if err != nil {
		return nil, err
	}

	err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM vessels WHERE mmsi::text LIKE '273%'`).Scan(&stats.RussianVessels)
	if err != nil {
		return nil, err
	}

	err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM ais_positions`).Scan(&stats.TotalPositions)
	if err != nil {
		return nil, err
	}

	err = r.pool.QueryRow(ctx, `SELECT MAX(last_poll_at) FROM ais_sources WHERE enabled = true`).Scan(&stats.LastCollectedAt)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

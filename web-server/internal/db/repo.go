package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VesselPosition struct {
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
	Sources              *string   `json:"sources,omitempty"`
}

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
}

type TrackPoint struct {
	Latitude         float64   `json:"latitude"`
	Longitude        float64   `json:"longitude"`
	SpeedOverGround  *float64  `json:"speed_over_ground,omitempty"`
	CourseOverGround *float64  `json:"course_over_ground,omitempty"`
	Heading          *int      `json:"heading,omitempty"`
	NavStatus        *string   `json:"nav_status,omitempty"`
	Timestamp        time.Time `json:"timestamp"`
}

type Stats struct {
	TotalVessels    int        `json:"total_vessels"`
	RussianVessels  int        `json:"russian_vessels"`
	TotalPositions  int64      `json:"total_positions"`
	LastCollectedAt *time.Time `json:"last_collected_at,omitempty"`
}

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(ctx context.Context, databaseURL string) (*Repo, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	config.MaxConns = 10

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Repo{pool: pool}, nil
}

func (r *Repo) Close() {
	r.pool.Close()
}

func (r *Repo) AllVesselsWithPositions(ctx context.Context) ([]*VesselPosition, error) {
	query := `
		SELECT v.mmsi, v.name, v.vessel_type_name, v.call_sign, v.imo_number, v.draught,
			ap.latitude, ap.longitude, ap.speed_over_ground, ap.course_over_ground,
			ap.heading, ap.navigation_status_name, ap.timestamp, v.destination,
			(
				SELECT STRING_AGG(DISTINCT s.name, ', ' ORDER BY s.name)
				FROM ais_positions p2
				JOIN ais_sources s ON p2.source_id = s.id
				WHERE p2.mmsi = v.mmsi
			) AS sources
		FROM vessels v
		INNER JOIN LATERAL (
			SELECT latitude, longitude, speed_over_ground, course_over_ground,
				heading, navigation_status_name, timestamp
			FROM ais_positions WHERE mmsi = v.mmsi ORDER BY timestamp DESC LIMIT 1
		) ap ON true
		WHERE ap.timestamp > NOW() - interval '7 days'
		ORDER BY v.mmsi`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*VesselPosition
	for rows.Next() {
		var vp VesselPosition
		if err := rows.Scan(&vp.MMSI, &vp.Name, &vp.VesselTypeName, &vp.CallSign,
			&vp.IMONumber, &vp.Draught, &vp.Latitude, &vp.Longitude,
			&vp.SpeedOverGround, &vp.CourseOverGround, &vp.Heading,
			&vp.NavigationStatusName, &vp.Timestamp, &vp.Destination,
			&vp.Sources); err != nil {
			return nil, err
		}
		out = append(out, &vp)
	}
	return out, rows.Err()
}

func (r *Repo) VesselByMMSI(ctx context.Context, mmsi int64) (*Vessel, error) {
	query := `SELECT mmsi, imo_number, name, call_sign, vessel_type, vessel_type_name,
		dimension_a, dimension_b, dimension_c, dimension_d, draught,
		destination, eta, first_seen_at, last_seen_at FROM vessels WHERE mmsi = $1`
	var v Vessel
	err := r.pool.QueryRow(ctx, query, mmsi).Scan(
		&v.MMSI, &v.IMONumber, &v.Name, &v.CallSign, &v.VesselType, &v.VesselTypeName,
		&v.DimensionA, &v.DimensionB, &v.DimensionC, &v.DimensionD,
		&v.Draught, &v.Destination, &v.ETA, &v.FirstSeenAt, &v.LastSeenAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &v, err
}

func (r *Repo) VesselTrack(ctx context.Context, mmsi int64, hours int) ([]*TrackPoint, error) {
	query := `SELECT latitude, longitude, speed_over_ground, course_over_ground,
		heading, navigation_status_name, timestamp
		FROM ais_positions
		WHERE mmsi = $1 AND timestamp > NOW() - make_interval(hours => $2)
		ORDER BY timestamp ASC`

	rows, err := r.pool.Query(ctx, query, mmsi, hours)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*TrackPoint
	for rows.Next() {
		var p TrackPoint
		if err := rows.Scan(&p.Latitude, &p.Longitude, &p.SpeedOverGround,
			&p.CourseOverGround, &p.Heading, &p.NavStatus, &p.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

type TrailPoint struct {
	MMSI      int64   `json:"mmsi"`
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
}

func (r *Repo) TrailsInBBox(ctx context.Context, south, west, north, east float64, hours int) (map[int64][][2]float64, error) {
	query := `WITH visible AS (
		SELECT DISTINCT mmsi FROM ais_positions
		WHERE timestamp > NOW() - make_interval(hours => $5)
		AND latitude BETWEEN $1 AND $2
		AND longitude BETWEEN $3 AND $4
		LIMIT 500
	)
	SELECT p.mmsi, p.latitude, p.longitude
	FROM ais_positions p
	JOIN visible v ON p.mmsi = v.mmsi
	WHERE p.timestamp > NOW() - make_interval(hours => $5)
	ORDER BY p.mmsi, p.timestamp ASC`

	rows, err := r.pool.Query(ctx, query, south, north, west, east, hours)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	trails := make(map[int64][][2]float64)
	for rows.Next() {
		var mmsi int64
		var lat, lng float64
		if err := rows.Scan(&mmsi, &lat, &lng); err != nil {
			return nil, err
		}
		trails[mmsi] = append(trails[mmsi], [2]float64{lng, lat})
	}
	return trails, rows.Err()
}

func (r *Repo) GetStats(ctx context.Context) (*Stats, error) {
	s := &Stats{}
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM vessels`).Scan(&s.TotalVessels); err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM vessels WHERE mmsi::text LIKE '273%'`).Scan(&s.RussianVessels); err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM ais_positions`).Scan(&s.TotalPositions); err != nil {
		return nil, err
	}
	_ = r.pool.QueryRow(ctx, `SELECT MAX(last_poll_at) FROM ais_sources WHERE enabled = true`).Scan(&s.LastCollectedAt)
	return s, nil
}

// --- STS Events ---

type STSEvent struct {
	ID              int64     `json:"id"`
	MMSIA           int64     `json:"mmsi_a"`
	MMSIB           int64     `json:"mmsi_b"`
	NameA           *string   `json:"name_a,omitempty"`
	NameB           *string   `json:"name_b,omitempty"`
	TypeA           *string   `json:"type_a,omitempty"`
	TypeB           *string   `json:"type_b,omitempty"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time"`
	DurationMinutes int       `json:"duration_minutes"`
	MinDistanceM    *float64  `json:"min_distance_m,omitempty"`
	AvgLat          *float64  `json:"avg_lat,omitempty"`
	AvgLon          *float64  `json:"avg_lon,omitempty"`
	LatA            *float64  `json:"lat_a,omitempty"`
	LonA            *float64  `json:"lon_a,omitempty"`
	LatB            *float64  `json:"lat_b,omitempty"`
	LonB            *float64  `json:"lon_b,omitempty"`
	Observations    *int      `json:"observations,omitempty"`
	Confidence      string    `json:"confidence"`
	DetectedAt      time.Time `json:"detected_at"`
}

func (r *Repo) GetSTSEvents(ctx context.Context, hours int, limit int) ([]*STSEvent, error) {
	query := `SELECT se.id, se.mmsi_a, se.mmsi_b, se.name_a, se.name_b, se.type_a, se.type_b,
		se.start_time, se.end_time, se.duration_minutes, se.min_distance_m,
		se.avg_lat, se.avg_lon,
		pa.latitude, pa.longitude, pb.latitude, pb.longitude,
		se.observations, se.confidence, se.detected_at
		FROM sts_events se
		LEFT JOIN LATERAL (
			SELECT latitude, longitude FROM ais_positions
			WHERE mmsi = se.mmsi_a AND timestamp <= se.start_time
			ORDER BY timestamp DESC LIMIT 1
		) pa ON true
		LEFT JOIN LATERAL (
			SELECT latitude, longitude FROM ais_positions
			WHERE mmsi = se.mmsi_b AND timestamp <= se.start_time
			ORDER BY timestamp DESC LIMIT 1
		) pb ON true
		WHERE se.start_time > NOW() - make_interval(hours => $1)
		ORDER BY se.start_time DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, hours, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*STSEvent
	for rows.Next() {
		var e STSEvent
		if err := rows.Scan(&e.ID, &e.MMSIA, &e.MMSIB, &e.NameA, &e.NameB,
			&e.TypeA, &e.TypeB, &e.StartTime, &e.EndTime, &e.DurationMinutes,
			&e.MinDistanceM, &e.AvgLat, &e.AvgLon,
			&e.LatA, &e.LonA, &e.LatB, &e.LonB,
			&e.Observations,
			&e.Confidence, &e.DetectedAt); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// --- Vessel Search ---

func (r *Repo) SearchVessels(ctx context.Context, q string, limit int) ([]*VesselPosition, error) {
	query := `
		SELECT v.mmsi, v.name, v.vessel_type_name, v.call_sign, v.imo_number, v.draught,
			ap.latitude, ap.longitude, ap.speed_over_ground, ap.course_over_ground,
			ap.heading, ap.navigation_status_name, ap.timestamp, v.destination
		FROM vessels v
		INNER JOIN LATERAL (
			SELECT latitude, longitude, speed_over_ground, course_over_ground,
				heading, navigation_status_name, timestamp
			FROM ais_positions WHERE mmsi = v.mmsi ORDER BY timestamp DESC LIMIT 1
		) ap ON true
		WHERE v.name ILIKE '%' || $1 || '%'
			OR v.mmsi::text LIKE $1 || '%'
			OR v.call_sign ILIKE '%' || $1 || '%'
			OR COALESCE(v.imo_number::text, '') = $1
		ORDER BY v.name
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*VesselPosition
	for rows.Next() {
		var vp VesselPosition
		if err := rows.Scan(&vp.MMSI, &vp.Name, &vp.VesselTypeName, &vp.CallSign,
			&vp.IMONumber, &vp.Draught, &vp.Latitude, &vp.Longitude,
			&vp.SpeedOverGround, &vp.CourseOverGround, &vp.Heading,
			&vp.NavigationStatusName, &vp.Timestamp, &vp.Destination); err != nil {
			return nil, err
		}
		out = append(out, &vp)
	}
	return out, rows.Err()
}

// --- Dark Vessels ---

func (r *Repo) GetDarkVessels(ctx context.Context, minGapHours float64, limit int) ([]*VesselPosition, error) {
	query := `
		SELECT v.mmsi, v.name, v.vessel_type_name, v.call_sign, v.imo_number, v.draught,
			ap.latitude, ap.longitude, ap.speed_over_ground, ap.course_over_ground,
			ap.heading, ap.navigation_status_name, ap.timestamp, v.destination
		FROM vessels v
		INNER JOIN LATERAL (
			SELECT latitude, longitude, speed_over_ground, course_over_ground,
				heading, navigation_status_name, timestamp
			FROM ais_positions WHERE mmsi = v.mmsi ORDER BY timestamp DESC LIMIT 1
		) ap ON true
		WHERE EXTRACT(EPOCH FROM (NOW() - ap.timestamp))/3600 >= $1
		ORDER BY ap.timestamp ASC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, minGapHours, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*VesselPosition
	for rows.Next() {
		var vp VesselPosition
		if err := rows.Scan(&vp.MMSI, &vp.Name, &vp.VesselTypeName, &vp.CallSign,
			&vp.IMONumber, &vp.Draught, &vp.Latitude, &vp.Longitude,
			&vp.SpeedOverGround, &vp.CourseOverGround, &vp.Heading,
			&vp.NavigationStatusName, &vp.Timestamp, &vp.Destination); err != nil {
			return nil, err
		}
		out = append(out, &vp)
	}
	return out, rows.Err()
}

// --- Historical / Time Slider ---

func (r *Repo) GetHistoricalPositions(ctx context.Context, targetTime time.Time) ([]*VesselPosition, error) {
	windowStart := targetTime.Add(-1 * time.Hour)
	query := `
		SELECT DISTINCT ON (v.mmsi)
			v.mmsi, v.name, v.vessel_type_name, v.call_sign, v.imo_number, v.draught,
			ap.latitude, ap.longitude, ap.speed_over_ground, ap.course_over_ground,
			ap.heading, ap.navigation_status_name, ap.timestamp, v.destination
		FROM vessels v
		INNER JOIN ais_positions ap ON v.mmsi = ap.mmsi
		WHERE ap.timestamp BETWEEN $1 AND $2
		ORDER BY v.mmsi, ap.timestamp DESC`

	rows, err := r.pool.Query(ctx, query, windowStart, targetTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*VesselPosition
	for rows.Next() {
		var vp VesselPosition
		if err := rows.Scan(&vp.MMSI, &vp.Name, &vp.VesselTypeName, &vp.CallSign,
			&vp.IMONumber, &vp.Draught, &vp.Latitude, &vp.Longitude,
			&vp.SpeedOverGround, &vp.CourseOverGround, &vp.Heading,
			&vp.NavigationStatusName, &vp.Timestamp, &vp.Destination); err != nil {
			return nil, err
		}
		out = append(out, &vp)
	}
	return out, rows.Err()
}

func (r *Repo) GetTimeRange(ctx context.Context) (*time.Time, *time.Time, error) {
	var minT, maxT *time.Time
	err := r.pool.QueryRow(ctx, `SELECT MIN(timestamp), MAX(timestamp) FROM ais_positions`).Scan(&minT, &maxT)
	return minT, maxT, err
}

// --- Russian Port Visits ---

type PortVisit struct {
	ID            int64      `json:"id"`
	MMSI          int64      `json:"mmsi"`
	VesselName    *string    `json:"vessel_name,omitempty"`
	VesselType    *string    `json:"vessel_type,omitempty"`
	CallSign      *string    `json:"call_sign,omitempty"`
	IMONumber     *int64     `json:"imo_number,omitempty"`
	FlagCountry   *string    `json:"flag_country,omitempty"`
	IsRussian     bool       `json:"is_russian"`
	PortName      string     `json:"port_name"`
	PortLat       *float64   `json:"port_lat,omitempty"`
	PortLon       *float64   `json:"port_lon,omitempty"`
	ArrivalTime   time.Time  `json:"arrival_time"`
	DepartureTime *time.Time `json:"departure_time,omitempty"`
	DurationHours *float64   `json:"duration_hours,omitempty"`
	Observations  *int       `json:"observations,omitempty"`
	StillInPort   bool       `json:"still_in_port"`
	DetectedAt    time.Time  `json:"detected_at"`
}

func (r *Repo) GetPortVisits(ctx context.Context, hours int, nonRussianOnly bool, limit int) ([]*PortVisit, error) {
	var query string
	var args []interface{}

	if nonRussianOnly {
		query = `SELECT id, mmsi, vessel_name, vessel_type, call_sign, imo_number,
			flag_country, is_russian, port_name, port_lat, port_lon,
			arrival_time, departure_time, duration_hours, observations,
			still_in_port, detected_at
			FROM russian_port_visits
			WHERE arrival_time > NOW() - make_interval(hours => $1)
			  AND is_russian = false
			ORDER BY arrival_time DESC
			LIMIT $2`
		args = []interface{}{hours, limit}
	} else {
		query = `SELECT id, mmsi, vessel_name, vessel_type, call_sign, imo_number,
			flag_country, is_russian, port_name, port_lat, port_lon,
			arrival_time, departure_time, duration_hours, observations,
			still_in_port, detected_at
			FROM russian_port_visits
			WHERE arrival_time > NOW() - make_interval(hours => $1)
			ORDER BY arrival_time DESC
			LIMIT $2`
		args = []interface{}{hours, limit}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*PortVisit
	for rows.Next() {
		var pv PortVisit
		if err := rows.Scan(&pv.ID, &pv.MMSI, &pv.VesselName, &pv.VesselType,
			&pv.CallSign, &pv.IMONumber, &pv.FlagCountry, &pv.IsRussian,
			&pv.PortName, &pv.PortLat, &pv.PortLon, &pv.ArrivalTime,
			&pv.DepartureTime, &pv.DurationHours, &pv.Observations,
			&pv.StillInPort, &pv.DetectedAt); err != nil {
			return nil, err
		}
		out = append(out, &pv)
	}
	return out, rows.Err()
}

// ========== Port Overrides ==========

type PortOverride struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	RadiusKm  float64   `json:"radius_km"`
	Country   string    `json:"country"`
	PortType  string    `json:"port_type"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *Repo) GetPortOverrides(ctx context.Context) ([]*PortOverride, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, latitude, longitude, radius_km, country, port_type, action, created_at
		 FROM port_overrides ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*PortOverride
	for rows.Next() {
		var po PortOverride
		if err := rows.Scan(&po.ID, &po.Name, &po.Latitude, &po.Longitude,
			&po.RadiusKm, &po.Country, &po.PortType, &po.Action, &po.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &po)
	}
	return out, rows.Err()
}

func (r *Repo) AddPortOverride(ctx context.Context, po *PortOverride) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO port_overrides (name, latitude, longitude, radius_km, country, port_type, action)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (name, action) DO UPDATE SET
		   latitude = EXCLUDED.latitude, longitude = EXCLUDED.longitude,
		   radius_km = EXCLUDED.radius_km, country = EXCLUDED.country,
		   port_type = EXCLUDED.port_type, created_at = NOW()
		 RETURNING id`,
		po.Name, po.Latitude, po.Longitude, po.RadiusKm, po.Country, po.PortType, po.Action,
	).Scan(&po.ID)
}

func (r *Repo) DeletePortOverride(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM port_overrides WHERE id = $1`, id)
	return err
}

// PurgeOldPositions deletes AIS positions older than the given number of days.
// Returns the number of rows deleted.
func (r *Repo) PurgeOldPositions(ctx context.Context, retentionDays int) (int64, error) {
	result, err := r.pool.Exec(ctx,
		`DELETE FROM ais_positions WHERE timestamp < NOW() - make_interval(days => $1)`,
		retentionDays)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// PurgeOrphanVessels removes vessels that have no remaining positions.
func (r *Repo) PurgeOrphanVessels(ctx context.Context) (int64, error) {
	result, err := r.pool.Exec(ctx,
		`DELETE FROM vessels v WHERE NOT EXISTS (
			SELECT 1 FROM ais_positions ap WHERE ap.mmsi = v.mmsi
		)`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

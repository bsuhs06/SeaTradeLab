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
	return r.VesselsInBBox(ctx, nil)
}

// BBox holds bounding box coordinates for viewport filtering.
type BBox struct {
	South, West, North, East float64
}

func (r *Repo) VesselsInBBox(ctx context.Context, bbox *BBox) ([]*VesselPosition, error) {
	var query string
	var args []interface{}

	if bbox != nil {
		query = `
		SELECT v.mmsi, v.name, v.vessel_type_name, v.call_sign, v.imo_number, v.draught,
			ap.latitude, ap.longitude, ap.speed_over_ground, ap.course_over_ground,
			ap.heading, ap.navigation_status_name, ap.timestamp, v.destination
		FROM vessels v
		INNER JOIN LATERAL (
			SELECT latitude, longitude, speed_over_ground, course_over_ground,
				heading, navigation_status_name, timestamp
			FROM ais_positions WHERE mmsi = v.mmsi ORDER BY timestamp DESC LIMIT 1
		) ap ON true
		WHERE ap.timestamp > NOW() - interval '7 days'
		  AND ap.latitude BETWEEN $1 AND $2
		  AND ap.longitude BETWEEN $3 AND $4
		ORDER BY v.mmsi`
		args = []interface{}{bbox.South, bbox.North, bbox.West, bbox.East}
	} else {
		query = `
		SELECT v.mmsi, v.name, v.vessel_type_name, v.call_sign, v.imo_number, v.draught,
			ap.latitude, ap.longitude, ap.speed_over_ground, ap.course_over_ground,
			ap.heading, ap.navigation_status_name, ap.timestamp, v.destination
		FROM vessels v
		INNER JOIN LATERAL (
			SELECT latitude, longitude, speed_over_ground, course_over_ground,
				heading, navigation_status_name, timestamp
			FROM ais_positions WHERE mmsi = v.mmsi ORDER BY timestamp DESC LIMIT 1
		) ap ON true
		WHERE ap.timestamp > NOW() - interval '7 days'
		ORDER BY v.mmsi`
	}

	rows, err := r.pool.Query(ctx, query, args...)
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

func (r *Repo) TrailsInBBox(ctx context.Context, south, west, north, east float64, hours int) (map[int64][][3]float64, error) {
	query := `WITH visible AS (
		SELECT DISTINCT mmsi FROM ais_positions
		WHERE timestamp > NOW() - make_interval(hours => $5)
		AND latitude BETWEEN $1 AND $2
		AND longitude BETWEEN $3 AND $4
		LIMIT 500
	)
	SELECT p.mmsi, p.latitude, p.longitude, EXTRACT(EPOCH FROM p.timestamp)::bigint
	FROM ais_positions p
	JOIN visible v ON p.mmsi = v.mmsi
	WHERE p.timestamp > NOW() - make_interval(hours => $5)
	ORDER BY p.mmsi, p.timestamp ASC`

	rows, err := r.pool.Query(ctx, query, south, north, west, east, hours)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	trails := make(map[int64][][3]float64)
	for rows.Next() {
		var mmsi int64
		var lat, lng float64
		var ts int64
		if err := rows.Scan(&mmsi, &lat, &lng, &ts); err != nil {
			return nil, err
		}
		trails[mmsi] = append(trails[mmsi], [3]float64{lng, lat, float64(ts)})
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
	Reviewed        bool      `json:"reviewed"`
	Tag             *string   `json:"tag,omitempty"`
	Notes           *string   `json:"notes,omitempty"`
	DetectedAt      time.Time `json:"detected_at"`
}

func (r *Repo) GetSTSEvents(ctx context.Context, hours int, limit int) ([]*STSEvent, error) {
	query := `SELECT se.id, se.mmsi_a, se.mmsi_b, se.name_a, se.name_b, se.type_a, se.type_b,
		se.start_time, se.end_time, se.duration_minutes, se.min_distance_m,
		se.avg_lat, se.avg_lon,
		pa.latitude, pa.longitude, pb.latitude, pb.longitude,
		se.observations, se.confidence, se.reviewed, se.tag, se.notes, se.detected_at
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
			&e.Confidence, &e.Reviewed, &e.Tag, &e.Notes, &e.DetectedAt); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

func (r *Repo) UpdateSTSEvent(ctx context.Context, id int64, confidence string, reviewed bool, tag *string, notes *string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE sts_events SET confidence = $2, reviewed = $3, tag = $4, notes = $5 WHERE id = $1`,
		id, confidence, reviewed, tag, notes)
	return err
}

func (r *Repo) CountSTSEvents(ctx context.Context, hours int) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM sts_events WHERE start_time > NOW() - make_interval(hours => $1)`, hours).Scan(&count)
	return count, err
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

func (r *Repo) CountDarkVessels(ctx context.Context, minGapHours float64) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM vessels v
		INNER JOIN LATERAL (
			SELECT timestamp FROM ais_positions WHERE mmsi = v.mmsi ORDER BY timestamp DESC LIMIT 1
		) ap ON true
		WHERE EXTRACT(EPOCH FROM (NOW() - ap.timestamp))/3600 >= $1`, minGapHours).Scan(&count)
	return count, err
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

func (r *Repo) CountPortVisits(ctx context.Context, hours int, nonRussianOnly bool) (int, error) {
	var count int
	var err error
	if nonRussianOnly {
		err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM russian_port_visits WHERE arrival_time > NOW() - make_interval(hours => $1) AND is_russian = false`, hours).Scan(&count)
	} else {
		err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM russian_port_visits WHERE arrival_time > NOW() - make_interval(hours => $1)`, hours).Scan(&count)
	}
	return count, err
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

// --- Spoofed AIS Detection ---

type SpoofedVessel struct {
	MMSI       int64   `json:"mmsi"`
	Name       *string `json:"name,omitempty"`
	VesselType *string `json:"vessel_type,omitempty"`
	Reason     string  `json:"reason"`
	LatFrom    float64 `json:"lat_from"`
	LonFrom    float64 `json:"lon_from"`
	LatTo      float64 `json:"lat_to"`
	LonTo      float64 `json:"lon_to"`
	SpeedKnots float64 `json:"speed_knots"`
	DistanceKm float64 `json:"distance_km"`
	TimeDeltaS int     `json:"time_delta_s"`
	Timestamp1 string  `json:"timestamp_1"`
	Timestamp2 string  `json:"timestamp_2"`
}

func (r *Repo) GetSpoofedVessels(ctx context.Context, hours int, limit int) ([]*SpoofedVessel, error) {
	// Detect impossible speed (>60 knots between consecutive positions)
	// and positions on land (lat/lon clearly invalid like lat>90 or on known land)
	query := `
		WITH consecutive AS (
			SELECT mmsi, latitude AS lat1, longitude AS lon1, timestamp AS ts1,
				LEAD(latitude) OVER w AS lat2,
				LEAD(longitude) OVER w AS lon2,
				LEAD(timestamp) OVER w AS ts2
			FROM ais_positions
			WHERE timestamp > NOW() - make_interval(hours => $1)
			WINDOW w AS (PARTITION BY mmsi ORDER BY timestamp)
		),
		anomalies AS (
			SELECT mmsi, lat1, lon1, lat2, lon2, ts1, ts2,
				EXTRACT(EPOCH FROM ts2 - ts1) AS dt_s,
				ST_DistanceSphere(
					ST_MakePoint(lon1, lat1),
					ST_MakePoint(lon2, lat2)
				) / 1000.0 AS dist_km
			FROM consecutive
			WHERE lat2 IS NOT NULL
				AND EXTRACT(EPOCH FROM ts2 - ts1) > 0
				AND EXTRACT(EPOCH FROM ts2 - ts1) < 7200
		)
		SELECT a.mmsi, v.name, v.vessel_type_name,
			CASE
				WHEN (a.dist_km / (a.dt_s / 3600.0)) > 200 THEN 'teleport'
				WHEN (a.dist_km / (a.dt_s / 3600.0)) > 60 THEN 'impossible_speed'
				ELSE 'suspicious_speed'
			END AS reason,
			a.lat1, a.lon1, a.lat2, a.lon2,
			(a.dist_km / (a.dt_s / 3600.0)) * 0.539957 AS speed_knots,
			a.dist_km, a.dt_s::int, a.ts1::text, a.ts2::text
		FROM anomalies a
		JOIN vessels v ON v.mmsi = a.mmsi
		WHERE (a.dist_km / (a.dt_s / 3600.0)) > 60
		ORDER BY speed_knots DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, hours, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*SpoofedVessel
	for rows.Next() {
		var s SpoofedVessel
		if err := rows.Scan(&s.MMSI, &s.Name, &s.VesselType, &s.Reason,
			&s.LatFrom, &s.LonFrom, &s.LatTo, &s.LonTo,
			&s.SpeedKnots, &s.DistanceKm, &s.TimeDeltaS,
			&s.Timestamp1, &s.Timestamp2); err != nil {
			return nil, err
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}

// ========== Vessel Registry ==========

type VesselRegistryEntry struct {
	MMSI           int64     `json:"mmsi"`
	IMONumber      *int64    `json:"imo_number,omitempty"`
	Name           *string   `json:"name,omitempty"`
	CallSign       *string   `json:"call_sign,omitempty"`
	VesselType     *int      `json:"vessel_type,omitempty"`
	VesselTypeName *string   `json:"vessel_type_name,omitempty"`
	Draught        *float64  `json:"draught,omitempty"`
	Destination    *string   `json:"destination,omitempty"`
	FirstSeenAt    time.Time `json:"first_seen_at"`
	LastSeenAt     time.Time `json:"last_seen_at"`
	ChangeCount    int       `json:"change_count"`
	Tags           *string   `json:"tags,omitempty"`
}

type VesselHistoryRecord struct {
	ID        int64     `json:"id"`
	MMSI      int64     `json:"mmsi"`
	FieldName string    `json:"field_name"`
	OldValue  *string   `json:"old_value,omitempty"`
	NewValue  *string   `json:"new_value,omitempty"`
	ChangedAt time.Time `json:"changed_at"`
}

type VesselNote struct {
	ID        int       `json:"id"`
	MMSI      int64     `json:"mmsi"`
	Tag       string    `json:"tag"`
	Note      *string   `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r *Repo) GetVesselRegistry(ctx context.Context, q string, tag string, limit int) ([]*VesselRegistryEntry, error) {
	query := `
		SELECT v.mmsi, v.imo_number, v.name, v.call_sign, v.vessel_type, v.vessel_type_name,
			v.draught, v.destination, v.first_seen_at, v.last_seen_at,
			COALESCE(hc.cnt, 0) AS change_count,
			nt.tags
		FROM vessels v
		LEFT JOIN (
			SELECT mmsi, COUNT(*) AS cnt FROM vessel_history GROUP BY mmsi
		) hc ON hc.mmsi = v.mmsi
		LEFT JOIN (
			SELECT mmsi, STRING_AGG(tag, ',' ORDER BY tag) AS tags
			FROM vessel_notes GROUP BY mmsi
		) nt ON nt.mmsi = v.mmsi
		WHERE 1=1`

	var args []interface{}
	argIdx := 1

	if q != "" {
		query += fmt.Sprintf(` AND (v.name ILIKE '%%' || $%d || '%%'
			OR v.mmsi::text LIKE $%d || '%%'
			OR v.call_sign ILIKE '%%' || $%d || '%%'
			OR COALESCE(v.imo_number::text, '') = $%d)`, argIdx, argIdx, argIdx, argIdx)
		args = append(args, q)
		argIdx++
	}

	if tag != "" {
		query += fmt.Sprintf(` AND EXISTS (SELECT 1 FROM vessel_notes vn WHERE vn.mmsi = v.mmsi AND vn.tag = $%d)`, argIdx)
		args = append(args, tag)
		argIdx++
	}

	query += fmt.Sprintf(` ORDER BY v.last_seen_at DESC LIMIT $%d`, argIdx)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*VesselRegistryEntry
	for rows.Next() {
		var e VesselRegistryEntry
		if err := rows.Scan(&e.MMSI, &e.IMONumber, &e.Name, &e.CallSign,
			&e.VesselType, &e.VesselTypeName, &e.Draught, &e.Destination,
			&e.FirstSeenAt, &e.LastSeenAt, &e.ChangeCount, &e.Tags); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

func (r *Repo) GetVesselHistory(ctx context.Context, mmsi int64, limit int) ([]*VesselHistoryRecord, error) {
	query := `SELECT id, mmsi, field_name, old_value, new_value, changed_at
		FROM vessel_history WHERE mmsi = $1
		ORDER BY changed_at DESC LIMIT $2`

	rows, err := r.pool.Query(ctx, query, mmsi, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*VesselHistoryRecord
	for rows.Next() {
		var h VesselHistoryRecord
		if err := rows.Scan(&h.ID, &h.MMSI, &h.FieldName, &h.OldValue, &h.NewValue, &h.ChangedAt); err != nil {
			return nil, err
		}
		out = append(out, &h)
	}
	return out, rows.Err()
}

func (r *Repo) GetVesselNotes(ctx context.Context, mmsi int64) ([]*VesselNote, error) {
	query := `SELECT id, mmsi, tag, note, created_at, updated_at
		FROM vessel_notes WHERE mmsi = $1 ORDER BY created_at`

	rows, err := r.pool.Query(ctx, query, mmsi)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*VesselNote
	for rows.Next() {
		var n VesselNote
		if err := rows.Scan(&n.ID, &n.MMSI, &n.Tag, &n.Note, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &n)
	}
	return out, rows.Err()
}

func (r *Repo) UpsertVesselNote(ctx context.Context, mmsi int64, tag string, note *string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO vessel_notes (mmsi, tag, note)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (mmsi, tag) DO UPDATE SET note = EXCLUDED.note, updated_at = NOW()`,
		mmsi, tag, note)
	return err
}

func (r *Repo) DeleteVesselNote(ctx context.Context, mmsi int64, tag string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM vessel_notes WHERE mmsi = $1 AND tag = $2`, mmsi, tag)
	return err
}

func (r *Repo) GetAllTags(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT tag FROM vessel_notes ORDER BY tag`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *Repo) GetRecentChanges(ctx context.Context, limit int) ([]*VesselHistoryRecord, error) {
	query := `SELECT id, mmsi, field_name, old_value, new_value, changed_at
		FROM vessel_history
		ORDER BY changed_at DESC LIMIT $1`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*VesselHistoryRecord
	for rows.Next() {
		var h VesselHistoryRecord
		if err := rows.Scan(&h.ID, &h.MMSI, &h.FieldName, &h.OldValue, &h.NewValue, &h.ChangedAt); err != nil {
			return nil, err
		}
		out = append(out, &h)
	}
	return out, rows.Err()
}

// ========== Vessel Taint Tracking ==========

type VesselPortCall struct {
	ID            int64      `json:"id"`
	MMSI          int64      `json:"mmsi"`
	VesselName    *string    `json:"vessel_name,omitempty"`
	VesselType    *string    `json:"vessel_type,omitempty"`
	FlagCountry   *string    `json:"flag_country,omitempty"`
	PortName      string     `json:"port_name"`
	PortCountry   *string    `json:"port_country,omitempty"`
	PortLat       *float64   `json:"port_lat,omitempty"`
	PortLon       *float64   `json:"port_lon,omitempty"`
	ArrivalTime   time.Time  `json:"arrival_time"`
	DepartureTime *time.Time `json:"departure_time,omitempty"`
	DurationHours *float64   `json:"duration_hours,omitempty"`
	StillInPort   bool       `json:"still_in_port"`
}

type VesselEncounter struct {
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
	MaxSogA         *float64  `json:"max_sog_a,omitempty"`
	MaxSogB         *float64  `json:"max_sog_b,omitempty"`
}

type VesselTaintRecord struct {
	ID            int64     `json:"id"`
	MMSI          int64     `json:"mmsi"`
	VesselName    *string   `json:"vessel_name,omitempty"`
	TaintType     string    `json:"taint_type"`
	Reason        *string   `json:"reason,omitempty"`
	SourceMMSI    *int64    `json:"source_mmsi,omitempty"`
	SourceName    *string   `json:"source_name,omitempty"`
	SourceTaintID *int64    `json:"source_taint_id,omitempty"`
	PortCallID    *int64    `json:"port_call_id,omitempty"`
	EncounterID   *int64    `json:"encounter_id,omitempty"`
	TaintedAt     time.Time `json:"tainted_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	Active        bool      `json:"active"`
}

type TaintChainLink struct {
	Taint     VesselTaintRecord `json:"taint"`
	PortCall  *VesselPortCall   `json:"port_call,omitempty"`
	Encounter *VesselEncounter  `json:"encounter,omitempty"`
}

func (r *Repo) GetTaintedVessels(ctx context.Context, limit int) ([]*VesselTaintRecord, error) {
	query := `SELECT id, mmsi, vessel_name, taint_type, reason,
		source_mmsi, source_name, source_taint_id, port_call_id, encounter_id,
		tainted_at, expires_at, active
		FROM vessel_taint WHERE active = true
		ORDER BY tainted_at DESC LIMIT $1`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*VesselTaintRecord
	for rows.Next() {
		var t VesselTaintRecord
		if err := rows.Scan(&t.ID, &t.MMSI, &t.VesselName, &t.TaintType, &t.Reason,
			&t.SourceMMSI, &t.SourceName, &t.SourceTaintID, &t.PortCallID, &t.EncounterID,
			&t.TaintedAt, &t.ExpiresAt, &t.Active); err != nil {
			return nil, err
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}

func (r *Repo) CountTaintedVessels(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(DISTINCT mmsi) FROM vessel_taint WHERE active = true`).Scan(&count)
	return count, err
}

func (r *Repo) GetVesselPortCalls(ctx context.Context, mmsi int64) ([]*VesselPortCall, error) {
	query := `SELECT id, mmsi, vessel_name, vessel_type, flag_country,
		port_name, port_country, port_lat, port_lon,
		arrival_time, departure_time, duration_hours, still_in_port
		FROM vessel_port_calls WHERE mmsi = $1
		ORDER BY arrival_time DESC`

	rows, err := r.pool.Query(ctx, query, mmsi)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*VesselPortCall
	for rows.Next() {
		var pc VesselPortCall
		if err := rows.Scan(&pc.ID, &pc.MMSI, &pc.VesselName, &pc.VesselType, &pc.FlagCountry,
			&pc.PortName, &pc.PortCountry, &pc.PortLat, &pc.PortLon,
			&pc.ArrivalTime, &pc.DepartureTime, &pc.DurationHours, &pc.StillInPort); err != nil {
			return nil, err
		}
		out = append(out, &pc)
	}
	return out, rows.Err()
}

func (r *Repo) GetVesselEncounters(ctx context.Context, mmsi int64) ([]*VesselEncounter, error) {
	query := `SELECT id, mmsi_a, mmsi_b, name_a, name_b, type_a, type_b,
		start_time, end_time, duration_minutes, min_distance_m,
		avg_lat, avg_lon, max_sog_a, max_sog_b
		FROM vessel_encounters
		WHERE mmsi_a = $1 OR mmsi_b = $1
		ORDER BY start_time DESC`

	rows, err := r.pool.Query(ctx, query, mmsi)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*VesselEncounter
	for rows.Next() {
		var e VesselEncounter
		if err := rows.Scan(&e.ID, &e.MMSIA, &e.MMSIB, &e.NameA, &e.NameB,
			&e.TypeA, &e.TypeB, &e.StartTime, &e.EndTime, &e.DurationMinutes,
			&e.MinDistanceM, &e.AvgLat, &e.AvgLon, &e.MaxSogA, &e.MaxSogB); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

func (r *Repo) GetVesselTaint(ctx context.Context, mmsi int64) ([]*VesselTaintRecord, error) {
	query := `SELECT id, mmsi, vessel_name, taint_type, reason,
		source_mmsi, source_name, source_taint_id, port_call_id, encounter_id,
		tainted_at, expires_at, active
		FROM vessel_taint WHERE mmsi = $1
		ORDER BY tainted_at DESC`

	rows, err := r.pool.Query(ctx, query, mmsi)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*VesselTaintRecord
	for rows.Next() {
		var t VesselTaintRecord
		if err := rows.Scan(&t.ID, &t.MMSI, &t.VesselName, &t.TaintType, &t.Reason,
			&t.SourceMMSI, &t.SourceName, &t.SourceTaintID, &t.PortCallID, &t.EncounterID,
			&t.TaintedAt, &t.ExpiresAt, &t.Active); err != nil {
			return nil, err
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}

func (r *Repo) GetTaintChain(ctx context.Context, taintID int64) ([]*TaintChainLink, error) {
	var chain []*TaintChainLink
	currentID := taintID

	for i := 0; i < 20; i++ { // max depth to prevent infinite loops
		var t VesselTaintRecord
		err := r.pool.QueryRow(ctx, `SELECT id, mmsi, vessel_name, taint_type, reason,
			source_mmsi, source_name, source_taint_id, port_call_id, encounter_id,
			tainted_at, expires_at, active
			FROM vessel_taint WHERE id = $1`, currentID).Scan(
			&t.ID, &t.MMSI, &t.VesselName, &t.TaintType, &t.Reason,
			&t.SourceMMSI, &t.SourceName, &t.SourceTaintID, &t.PortCallID, &t.EncounterID,
			&t.TaintedAt, &t.ExpiresAt, &t.Active)
		if err != nil {
			break
		}

		link := &TaintChainLink{Taint: t}

		// Load associated port call
		if t.PortCallID != nil {
			var pc VesselPortCall
			err := r.pool.QueryRow(ctx, `SELECT id, mmsi, vessel_name, vessel_type, flag_country,
				port_name, port_country, port_lat, port_lon,
				arrival_time, departure_time, duration_hours, still_in_port
				FROM vessel_port_calls WHERE id = $1`, *t.PortCallID).Scan(
				&pc.ID, &pc.MMSI, &pc.VesselName, &pc.VesselType, &pc.FlagCountry,
				&pc.PortName, &pc.PortCountry, &pc.PortLat, &pc.PortLon,
				&pc.ArrivalTime, &pc.DepartureTime, &pc.DurationHours, &pc.StillInPort)
			if err == nil {
				link.PortCall = &pc
			}
		}

		// Load associated encounter
		if t.EncounterID != nil {
			var e VesselEncounter
			err := r.pool.QueryRow(ctx, `SELECT id, mmsi_a, mmsi_b, name_a, name_b, type_a, type_b,
				start_time, end_time, duration_minutes, min_distance_m,
				avg_lat, avg_lon, max_sog_a, max_sog_b
				FROM vessel_encounters WHERE id = $1`, *t.EncounterID).Scan(
				&e.ID, &e.MMSIA, &e.MMSIB, &e.NameA, &e.NameB, &e.TypeA, &e.TypeB,
				&e.StartTime, &e.EndTime, &e.DurationMinutes, &e.MinDistanceM,
				&e.AvgLat, &e.AvgLon, &e.MaxSogA, &e.MaxSogB)
			if err == nil {
				link.Encounter = &e
			}
		}

		chain = append(chain, link)

		// Follow the chain
		if t.SourceTaintID != nil {
			currentID = *t.SourceTaintID
		} else {
			break
		}
	}

	return chain, nil
}

// --- Vessel Favorites ---

type VesselFavorite struct {
	ID         int64     `json:"id"`
	MMSI       int64     `json:"mmsi"`
	VesselName *string   `json:"vessel_name,omitempty"`
	VesselType *string   `json:"vessel_type,omitempty"`
	Notes      *string   `json:"notes,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type FavoriteWithPosition struct {
	VesselFavorite
	Latitude        *float64   `json:"latitude,omitempty"`
	Longitude       *float64   `json:"longitude,omitempty"`
	SpeedOverGround *float64   `json:"speed_over_ground,omitempty"`
	Heading         *int       `json:"heading,omitempty"`
	Destination     *string    `json:"destination,omitempty"`
	LastSeen        *time.Time `json:"last_seen,omitempty"`
	FlagCountry     string     `json:"flag_country"`
}

func (r *Repo) GetFavorites(ctx context.Context) ([]*FavoriteWithPosition, error) {
	query := `SELECT f.id, f.mmsi, f.vessel_name, f.vessel_type, f.notes, f.created_at,
		ap.latitude, ap.longitude, ap.speed_over_ground, ap.heading, v.destination, ap.timestamp
		FROM vessel_favorites f
		LEFT JOIN vessels v ON f.mmsi = v.mmsi
		LEFT JOIN LATERAL (
			SELECT latitude, longitude, speed_over_ground, heading, timestamp
			FROM ais_positions WHERE mmsi = f.mmsi ORDER BY timestamp DESC LIMIT 1
		) ap ON true
		ORDER BY f.created_at DESC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*FavoriteWithPosition
	for rows.Next() {
		var f FavoriteWithPosition
		if err := rows.Scan(&f.ID, &f.MMSI, &f.VesselName, &f.VesselType, &f.Notes, &f.CreatedAt,
			&f.Latitude, &f.Longitude, &f.SpeedOverGround, &f.Heading, &f.Destination, &f.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, &f)
	}
	return out, rows.Err()
}

func (r *Repo) AddFavorite(ctx context.Context, mmsi int64, vesselName, vesselType, notes *string) (*VesselFavorite, error) {
	var f VesselFavorite
	err := r.pool.QueryRow(ctx, `INSERT INTO vessel_favorites (mmsi, vessel_name, vessel_type, notes)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (mmsi) DO UPDATE SET vessel_name = COALESCE(EXCLUDED.vessel_name, vessel_favorites.vessel_name),
			vessel_type = COALESCE(EXCLUDED.vessel_type, vessel_favorites.vessel_type),
			notes = COALESCE(EXCLUDED.notes, vessel_favorites.notes)
		RETURNING id, mmsi, vessel_name, vessel_type, notes, created_at`,
		mmsi, vesselName, vesselType, notes).Scan(&f.ID, &f.MMSI, &f.VesselName, &f.VesselType, &f.Notes, &f.CreatedAt)
	return &f, err
}

func (r *Repo) RemoveFavorite(ctx context.Context, mmsi int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM vessel_favorites WHERE mmsi = $1`, mmsi)
	return err
}

func (r *Repo) UpdateFavoriteNotes(ctx context.Context, mmsi int64, notes *string) error {
	_, err := r.pool.Exec(ctx, `UPDATE vessel_favorites SET notes = $2 WHERE mmsi = $1`, mmsi, notes)
	return err
}

func (r *Repo) IsFavorite(ctx context.Context, mmsi int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vessel_favorites WHERE mmsi = $1)`, mmsi).Scan(&exists)
	return exists, err
}

func (r *Repo) GetFavoriteMMSIs(ctx context.Context) (map[int64]bool, error) {
	rows, err := r.pool.Query(ctx, `SELECT mmsi FROM vessel_favorites`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64]bool)
	for rows.Next() {
		var mmsi int64
		if err := rows.Scan(&mmsi); err != nil {
			return nil, err
		}
		out[mmsi] = true
	}
	return out, rows.Err()
}

// ========== Destination Anomalies ==========

type DestinationAnomaly struct {
	MMSI           int64     `json:"mmsi"`
	Name           *string   `json:"name,omitempty"`
	VesselTypeName *string   `json:"vessel_type_name,omitempty"`
	Destination    string    `json:"destination"`
	Reason         string    `json:"reason"`
	LastSeenAt     time.Time `json:"last_seen_at"`
	ChangeCount    int       `json:"change_count"`
	Latitude       float64   `json:"latitude"`
	Longitude      float64   `json:"longitude"`
}

type DestinationChange struct {
	ID        int64     `json:"id"`
	MMSI      int64     `json:"mmsi"`
	Name      *string   `json:"name,omitempty"`
	OldValue  *string   `json:"old_value,omitempty"`
	NewValue  *string   `json:"new_value,omitempty"`
	ChangedAt time.Time `json:"changed_at"`
}

func (r *Repo) GetDestinationAnomalies(ctx context.Context, limit int) ([]*DestinationAnomaly, error) {
	// Flag destinations that look like messages rather than port names:
	// 1. Contains known message/protest keywords
	// 2. Contains crew/personnel words (not port names)
	// 3. Long multi-word (>30 chars, 3+ words)
	// 4. 3+ words (most real ports are 1-2 words)
	// 5. Frequent destination changes
	query := `
		WITH dest_changes AS (
			SELECT mmsi, COUNT(*) AS change_count
			FROM vessel_history
			WHERE field_name = 'destination'
			  AND changed_at > NOW() - interval '30 days'
			GROUP BY mmsi
		)
		SELECT v.mmsi, v.name, v.vessel_type_name, v.destination,
			CASE
				WHEN v.destination ~* '\m(SOS|HELP|MAYDAY|ATTACK|CONFLICT|WAR|PROTEST|SANCTION|MILITARY|IRAN|HOUTHI|MISSILE|STRIKE|SEIZED|HIJACK|PIRAT|EMBARGO|THREAT|WEAPON|BOMB|NOT INVOLVED|NO PART|DO NOT)\M'
					THEN 'message_keywords'
				WHEN v.destination ~* '\m(CREW|OWNER|MASTER|CAPTAIN|SAILING|ANCHOR|DRIFTING|REPAIR|BROKEN|UNSAFE|DISTRESS|OVERDUE|MISSING|ABANDONED|REFUGEE|ILLEGAL|POLICE|COAST GUARD|NAVY|CHINESE|INDIAN|IRANIAN|RUSSIAN|KOREAN|FILIPINO|TURKISH|SYRIAN|YEMENI|SOMALI|LIBYAN|NORTH KOREAN|VENEZUELAN)\M'
					THEN 'message_keywords'
				WHEN LENGTH(v.destination) > 30
					AND array_length(string_to_array(TRIM(v.destination), ' '), 1) >= 3
					THEN 'long_multi_word'
				WHEN array_length(string_to_array(TRIM(v.destination), ' '), 1) >= 3
					THEN 'multi_word_message'
				WHEN COALESCE(dc.change_count, 0) >= 5
					THEN 'frequent_changes'
				ELSE 'unusual_format'
			END AS reason,
			v.last_seen_at,
			COALESCE(dc.change_count, 0) AS change_count,
			COALESCE(lp.latitude, 0) AS latitude,
			COALESCE(lp.longitude, 0) AS longitude
		FROM vessels v
		LEFT JOIN dest_changes dc ON dc.mmsi = v.mmsi
		LEFT JOIN LATERAL (
			SELECT latitude, longitude FROM ais_positions
			WHERE mmsi = v.mmsi ORDER BY timestamp DESC LIMIT 1
		) lp ON true
		WHERE v.destination IS NOT NULL
		  AND v.destination <> ''
		  AND v.last_seen_at > NOW() - interval '30 days'
		  AND (
			-- Contains protest/message keywords
			v.destination ~* '\m(SOS|HELP|MAYDAY|ATTACK|CONFLICT|WAR|PROTEST|SANCTION|MILITARY|IRAN|HOUTHI|MISSILE|STRIKE|SEIZED|HIJACK|PIRAT|EMBARGO|THREAT|WEAPON|BOMB|NOT INVOLVED|NO PART|DO NOT)\M'
			-- Contains crew/personnel/status words
			OR v.destination ~* '\m(CREW|OWNER|MASTER|CAPTAIN|SAILING|ANCHOR|DRIFTING|REPAIR|BROKEN|UNSAFE|DISTRESS|OVERDUE|MISSING|ABANDONED|REFUGEE|ILLEGAL|POLICE|COAST GUARD|NAVY|CHINESE|INDIAN|IRANIAN|RUSSIAN|KOREAN|FILIPINO|TURKISH|SYRIAN|YEMENI|SOMALI|LIBYAN|NORTH KOREAN|VENEZUELAN)\M'
			-- Long multi-word string (likely a message)
			OR (LENGTH(v.destination) > 30
				AND array_length(string_to_array(TRIM(v.destination), ' '), 1) >= 3)
			-- 3+ words in destination (unusual for port names)
			OR array_length(string_to_array(TRIM(v.destination), ' '), 1) >= 3
			-- Frequent destination changes
			OR COALESCE(dc.change_count, 0) >= 5
		  )
		ORDER BY
			CASE
				WHEN v.destination ~* '\m(SOS|HELP|MAYDAY|ATTACK|CONFLICT|WAR|PROTEST|SANCTION|MILITARY|IRAN|HOUTHI|MISSILE|STRIKE|SEIZED|HIJACK|PIRAT|EMBARGO|THREAT|WEAPON|BOMB|NOT INVOLVED|NO PART|DO NOT)\M' THEN 0
				WHEN v.destination ~* '\m(CREW|OWNER|MASTER|CAPTAIN|SAILING|ANCHOR|DRIFTING|REPAIR|BROKEN|UNSAFE|DISTRESS|OVERDUE|MISSING|ABANDONED|REFUGEE|ILLEGAL|POLICE|COAST GUARD|NAVY|CHINESE|INDIAN|IRANIAN|RUSSIAN|KOREAN|FILIPINO|TURKISH|SYRIAN|YEMENI|SOMALI|LIBYAN|NORTH KOREAN|VENEZUELAN)\M' THEN 0
				ELSE 1
			END,
			COALESCE(dc.change_count, 0) DESC,
			v.last_seen_at DESC
		LIMIT $1`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*DestinationAnomaly
	for rows.Next() {
		var a DestinationAnomaly
		if err := rows.Scan(&a.MMSI, &a.Name, &a.VesselTypeName, &a.Destination,
			&a.Reason, &a.LastSeenAt, &a.ChangeCount, &a.Latitude, &a.Longitude); err != nil {
			return nil, err
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}

func (r *Repo) GetDestinationChanges(ctx context.Context, mmsi int64, limit int) ([]*DestinationChange, error) {
	query := `SELECT vh.id, vh.mmsi, v.name, vh.old_value, vh.new_value, vh.changed_at
		FROM vessel_history vh
		JOIN vessels v ON v.mmsi = vh.mmsi
		WHERE vh.field_name = 'destination'
		  AND vh.mmsi = $1
		ORDER BY vh.changed_at DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, mmsi, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*DestinationChange
	for rows.Next() {
		var c DestinationChange
		if err := rows.Scan(&c.ID, &c.MMSI, &c.Name, &c.OldValue, &c.NewValue, &c.ChangedAt); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

func (r *Repo) GetRecentDestinationChanges(ctx context.Context, hours int, limit int) ([]*DestinationChange, error) {
	query := `SELECT vh.id, vh.mmsi, v.name, vh.old_value, vh.new_value, vh.changed_at
		FROM vessel_history vh
		JOIN vessels v ON v.mmsi = vh.mmsi
		WHERE vh.field_name = 'destination'
		  AND vh.changed_at > NOW() - make_interval(hours => $1)
		ORDER BY vh.changed_at DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, hours, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*DestinationChange
	for rows.Next() {
		var c DestinationChange
		if err := rows.Scan(&c.ID, &c.MMSI, &c.Name, &c.OldValue, &c.NewValue, &c.ChangedAt); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

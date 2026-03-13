-- Migration: 001_initial_schema.up.sql
-- Create AIS data tables with PostGIS support

-- Enable PostGIS extension
CREATE EXTENSION IF NOT EXISTS postgis;

-- Table for AIS data sources/providers
CREATE TABLE ais_sources (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    source_type VARCHAR(50) NOT NULL, -- 'api', 'stream', 'file'
    enabled BOOLEAN DEFAULT true,
    last_poll_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Table for vessel information
CREATE TABLE vessels (
    mmsi BIGINT PRIMARY KEY, -- Maritime Mobile Service Identity (unique vessel ID)
    imo_number BIGINT, -- International Maritime Organization number
    name VARCHAR(255),
    call_sign VARCHAR(50),
    vessel_type INT,
    vessel_type_name VARCHAR(100),
    dimension_a INT, -- Distance to bow
    dimension_b INT, -- Distance to stern
    dimension_c INT, -- Distance to port
    dimension_d INT, -- Distance to starboard
    draught NUMERIC(5,2), -- Draft in meters
    destination VARCHAR(255),
    eta TIMESTAMPTZ,
    first_seen_at TIMESTAMPTZ DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Table for position reports (the main AIS data)
CREATE TABLE ais_positions (
    id BIGSERIAL PRIMARY KEY,
    mmsi BIGINT NOT NULL REFERENCES vessels(mmsi),
    source_id INT REFERENCES ais_sources(id),
    position GEOGRAPHY(POINT, 4326) NOT NULL, -- PostGIS geographic point
    latitude NUMERIC(10, 7) NOT NULL,
    longitude NUMERIC(11, 7) NOT NULL,
    speed_over_ground NUMERIC(5,2), -- Speed in knots
    course_over_ground NUMERIC(5,2), -- Course in degrees
    heading INT, -- True heading in degrees
    navigation_status INT,
    navigation_status_name VARCHAR(50),
    timestamp TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX idx_ais_positions_mmsi ON ais_positions(mmsi);
CREATE INDEX idx_ais_positions_timestamp ON ais_positions(timestamp DESC);
CREATE INDEX idx_ais_positions_received_at ON ais_positions(received_at DESC);
CREATE INDEX idx_ais_positions_position ON ais_positions USING GIST(position);
CREATE INDEX idx_vessels_last_seen ON vessels(last_seen_at DESC);
CREATE INDEX idx_vessels_name ON vessels(name);

-- Create a view for latest vessel positions
CREATE VIEW latest_vessel_positions AS
SELECT DISTINCT ON (v.mmsi)
    v.mmsi,
    v.name,
    v.vessel_type_name,
    ap.latitude,
    ap.longitude,
    ap.speed_over_ground,
    ap.course_over_ground,
    ap.heading,
    ap.navigation_status_name,
    ap.timestamp,
    v.destination,
    v.eta
FROM vessels v
INNER JOIN ais_positions ap ON v.mmsi = ap.mmsi
ORDER BY v.mmsi, ap.timestamp DESC;

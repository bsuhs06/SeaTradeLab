-- Migration: 002 - STS events and AIS gaps tables

-- Table for detected Ship-to-Ship transfer events
CREATE TABLE IF NOT EXISTS sts_events (
    id BIGSERIAL PRIMARY KEY,
    mmsi_a BIGINT NOT NULL,
    mmsi_b BIGINT NOT NULL,
    name_a VARCHAR(255),
    name_b VARCHAR(255),
    type_a VARCHAR(100),
    type_b VARCHAR(100),
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    duration_minutes INT NOT NULL,
    min_distance_m NUMERIC(10,1),
    avg_lat NUMERIC(10,7),
    avg_lon NUMERIC(11,7),
    observations INT DEFAULT 1,
    confidence VARCHAR(20) DEFAULT 'medium',
    reviewed BOOLEAN DEFAULT false,
    notes TEXT,
    detected_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Table for detected AIS gaps (dark periods)
CREATE TABLE IF NOT EXISTS ais_gaps (
    id BIGSERIAL PRIMARY KEY,
    mmsi BIGINT NOT NULL,
    last_position_time TIMESTAMPTZ NOT NULL,
    last_lat NUMERIC(10,7),
    last_lon NUMERIC(11,7),
    last_sog NUMERIC(5,2),
    last_cog NUMERIC(5,2),
    reappear_time TIMESTAMPTZ,
    reappear_lat NUMERIC(10,7),
    reappear_lon NUMERIC(11,7),
    gap_hours NUMERIC(8,2),
    distance_km NUMERIC(10,2),
    is_active BOOLEAN DEFAULT true,
    detected_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for STS events
CREATE INDEX IF NOT EXISTS idx_sts_events_time ON sts_events(start_time DESC);
CREATE INDEX IF NOT EXISTS idx_sts_events_mmsi_a ON sts_events(mmsi_a);
CREATE INDEX IF NOT EXISTS idx_sts_events_mmsi_b ON sts_events(mmsi_b);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sts_events_unique ON sts_events(mmsi_a, mmsi_b, start_time);

-- Indexes for AIS gaps
CREATE INDEX IF NOT EXISTS idx_ais_gaps_mmsi ON ais_gaps(mmsi);
CREATE INDEX IF NOT EXISTS idx_ais_gaps_active ON ais_gaps(is_active) WHERE is_active = true;

-- Composite index for time-slider queries (major perf improvement)
CREATE INDEX IF NOT EXISTS idx_ais_positions_mmsi_ts ON ais_positions(mmsi, timestamp DESC);

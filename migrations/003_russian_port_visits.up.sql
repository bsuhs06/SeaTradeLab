-- Migration: 003 - Russian port visits table
-- Tracks all vessels visiting Russian ports, especially non-Russian flagged

CREATE TABLE IF NOT EXISTS russian_port_visits (
    id BIGSERIAL PRIMARY KEY,
    mmsi BIGINT NOT NULL,
    vessel_name VARCHAR(255),
    vessel_type VARCHAR(100),
    call_sign VARCHAR(50),
    imo_number BIGINT,
    flag_country VARCHAR(100),
    is_russian BOOLEAN NOT NULL DEFAULT false,
    port_name VARCHAR(100) NOT NULL,
    port_lat NUMERIC(10,7),
    port_lon NUMERIC(11,7),
    arrival_time TIMESTAMPTZ NOT NULL,
    departure_time TIMESTAMPTZ,
    duration_hours NUMERIC(8,2),
    min_speed_kts NUMERIC(5,2),
    avg_speed_kts NUMERIC(5,2),
    observations INT DEFAULT 1,
    still_in_port BOOLEAN DEFAULT false,
    detected_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_rpv_mmsi ON russian_port_visits(mmsi);
CREATE INDEX IF NOT EXISTS idx_rpv_port ON russian_port_visits(port_name);
CREATE INDEX IF NOT EXISTS idx_rpv_arrival ON russian_port_visits(arrival_time DESC);
CREATE INDEX IF NOT EXISTS idx_rpv_non_russian ON russian_port_visits(is_russian) WHERE is_russian = false;
CREATE UNIQUE INDEX IF NOT EXISTS idx_rpv_unique ON russian_port_visits(mmsi, port_name, arrival_time);

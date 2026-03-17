-- Migration 008: Vessel Taint Tracking System
-- Tracks all port calls, close encounters, and taint propagation chains.

-- 1. All port calls for all vessels (not just Russian ports)
CREATE TABLE IF NOT EXISTS vessel_port_calls (
    id              BIGSERIAL PRIMARY KEY,
    mmsi            BIGINT NOT NULL,
    vessel_name     TEXT,
    vessel_type     TEXT,
    flag_country    TEXT,
    port_name       TEXT NOT NULL,
    port_country    TEXT NOT NULL,
    port_lat        DOUBLE PRECISION,
    port_lon        DOUBLE PRECISION,
    arrival_time    TIMESTAMPTZ NOT NULL,
    departure_time  TIMESTAMPTZ,
    duration_hours  DOUBLE PRECISION,
    still_in_port   BOOLEAN DEFAULT false,
    detected_at     TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(mmsi, port_name, arrival_time)
);

CREATE INDEX IF NOT EXISTS idx_vpc_mmsi ON vessel_port_calls(mmsi);
CREATE INDEX IF NOT EXISTS idx_vpc_arrival ON vessel_port_calls(arrival_time DESC);
CREATE INDEX IF NOT EXISTS idx_vpc_port_country ON vessel_port_calls(port_country);
CREATE INDEX IF NOT EXISTS idx_vpc_mmsi_arrival ON vessel_port_calls(mmsi, arrival_time DESC);

-- 2. Vessel-to-vessel encounters (within 100m for 30+ min at <2kts)
CREATE TABLE IF NOT EXISTS vessel_encounters (
    id              BIGSERIAL PRIMARY KEY,
    mmsi_a          BIGINT NOT NULL,
    mmsi_b          BIGINT NOT NULL,
    name_a          TEXT,
    name_b          TEXT,
    type_a          TEXT,
    type_b          TEXT,
    start_time      TIMESTAMPTZ NOT NULL,
    end_time        TIMESTAMPTZ NOT NULL,
    duration_minutes INTEGER NOT NULL,
    min_distance_m  DOUBLE PRECISION,
    avg_lat         DOUBLE PRECISION,
    avg_lon         DOUBLE PRECISION,
    max_sog_a       DOUBLE PRECISION,
    max_sog_b       DOUBLE PRECISION,
    detected_at     TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(mmsi_a, mmsi_b, start_time)
);

CREATE INDEX IF NOT EXISTS idx_ve_mmsi_a ON vessel_encounters(mmsi_a);
CREATE INDEX IF NOT EXISTS idx_ve_mmsi_b ON vessel_encounters(mmsi_b);
CREATE INDEX IF NOT EXISTS idx_ve_start ON vessel_encounters(start_time DESC);

-- 3. Taint records — tracks why a vessel is tainted and the chain
CREATE TABLE IF NOT EXISTS vessel_taint (
    id              BIGSERIAL PRIMARY KEY,
    mmsi            BIGINT NOT NULL,
    vessel_name     TEXT,
    taint_type      TEXT NOT NULL,  -- 'russian_port', 'encounter', 'no_subsequent_port'
    reason          TEXT NOT NULL,  -- human-readable: "Visited Primorsk on 2026-03-01"
    source_mmsi     BIGINT,        -- NULL for direct taint, set for encounter-propagated
    source_name     TEXT,           -- name of the source vessel
    source_taint_id BIGINT REFERENCES vessel_taint(id), -- links to the taint that propagated
    port_call_id    BIGINT REFERENCES vessel_port_calls(id),
    encounter_id    BIGINT REFERENCES vessel_encounters(id),
    tainted_at      TIMESTAMPTZ NOT NULL,  -- when the tainting event occurred
    detected_at     TIMESTAMPTZ DEFAULT NOW(),
    expires_at      TIMESTAMPTZ,   -- 90 days from tainted_at for port-based taint
    active          BOOLEAN DEFAULT true,
    UNIQUE(mmsi, taint_type, tainted_at)
);

CREATE INDEX IF NOT EXISTS idx_vt_mmsi ON vessel_taint(mmsi);
CREATE INDEX IF NOT EXISTS idx_vt_active ON vessel_taint(active) WHERE active = true;
CREATE INDEX IF NOT EXISTS idx_vt_source ON vessel_taint(source_mmsi);
CREATE INDEX IF NOT EXISTS idx_vt_type ON vessel_taint(taint_type);
CREATE INDEX IF NOT EXISTS idx_vt_tainted_at ON vessel_taint(tainted_at DESC);

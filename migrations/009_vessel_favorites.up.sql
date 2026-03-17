CREATE TABLE IF NOT EXISTS vessel_favorites (
    id SERIAL PRIMARY KEY,
    mmsi BIGINT NOT NULL UNIQUE,
    vessel_name TEXT,
    vessel_type TEXT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vessel_favorites_mmsi ON vessel_favorites(mmsi);

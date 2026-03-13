-- Port overrides: user-defined port additions and removals
-- 'add' = custom port to include in filtering
-- 'remove' = suppress a built-in port from filtering
CREATE TABLE IF NOT EXISTS port_overrides (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    radius_km DOUBLE PRECISION NOT NULL DEFAULT 4.0,
    country VARCHAR(100) NOT NULL DEFAULT '',
    port_type VARCHAR(50) NOT NULL DEFAULT 'commercial',
    action VARCHAR(10) NOT NULL DEFAULT 'add' CHECK (action IN ('add', 'remove')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(name, action)
);

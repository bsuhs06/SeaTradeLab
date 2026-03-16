-- Migration: 007_vessel_registry.up.sql
-- Vessel history tracking and enrichment notes

-- Track changes to key vessel identity fields
CREATE TABLE IF NOT EXISTS vessel_history (
    id BIGSERIAL PRIMARY KEY,
    mmsi BIGINT NOT NULL REFERENCES vessels(mmsi) ON DELETE CASCADE,
    field_name VARCHAR(50) NOT NULL,    -- 'name', 'call_sign', 'imo_number', 'vessel_type_name', 'destination'
    old_value TEXT,
    new_value TEXT,
    changed_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_vessel_history_mmsi ON vessel_history(mmsi);
CREATE INDEX idx_vessel_history_changed_at ON vessel_history(changed_at DESC);
CREATE INDEX idx_vessel_history_field ON vessel_history(field_name);

-- User-managed vessel notes / tags / classifications
CREATE TABLE IF NOT EXISTS vessel_notes (
    id SERIAL PRIMARY KEY,
    mmsi BIGINT NOT NULL REFERENCES vessels(mmsi) ON DELETE CASCADE,
    tag VARCHAR(50) NOT NULL,           -- 'icebreaker', 'watchlist', 'sanctions', 'military', etc.
    note TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(mmsi, tag)
);

CREATE INDEX idx_vessel_notes_mmsi ON vessel_notes(mmsi);
CREATE INDEX idx_vessel_notes_tag ON vessel_notes(tag);

-- Trigger function: log changes to tracked fields
CREATE OR REPLACE FUNCTION log_vessel_changes() RETURNS TRIGGER AS $$
BEGIN
    -- Only fire on UPDATE (not INSERT)
    IF TG_OP = 'UPDATE' THEN
        -- Name change
        IF COALESCE(OLD.name, '') <> COALESCE(NEW.name, '') AND OLD.name IS NOT NULL THEN
            INSERT INTO vessel_history (mmsi, field_name, old_value, new_value)
            VALUES (NEW.mmsi, 'name', OLD.name, NEW.name);
        END IF;

        -- Call sign change
        IF COALESCE(OLD.call_sign, '') <> COALESCE(NEW.call_sign, '') AND OLD.call_sign IS NOT NULL THEN
            INSERT INTO vessel_history (mmsi, field_name, old_value, new_value)
            VALUES (NEW.mmsi, 'call_sign', OLD.call_sign, NEW.call_sign);
        END IF;

        -- IMO number change (suspicious — IMO should be permanent)
        IF COALESCE(OLD.imo_number, 0) <> COALESCE(NEW.imo_number, 0) AND OLD.imo_number IS NOT NULL THEN
            INSERT INTO vessel_history (mmsi, field_name, old_value, new_value)
            VALUES (NEW.mmsi, 'imo_number', OLD.imo_number::text, NEW.imo_number::text);
        END IF;

        -- Vessel type change
        IF COALESCE(OLD.vessel_type_name, '') <> COALESCE(NEW.vessel_type_name, '') AND OLD.vessel_type_name IS NOT NULL THEN
            INSERT INTO vessel_history (mmsi, field_name, old_value, new_value)
            VALUES (NEW.mmsi, 'vessel_type_name', OLD.vessel_type_name, NEW.vessel_type_name);
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Attach the trigger to the vessels table
DROP TRIGGER IF EXISTS trg_vessel_changes ON vessels;
CREATE TRIGGER trg_vessel_changes
    AFTER UPDATE ON vessels
    FOR EACH ROW
    EXECUTE FUNCTION log_vessel_changes();

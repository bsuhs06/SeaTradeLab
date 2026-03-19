-- Migration: 010_destination_tracking.up.sql
-- Add destination change tracking to the vessel history trigger

CREATE OR REPLACE FUNCTION log_vessel_changes() RETURNS TRIGGER AS $$
BEGIN
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

        -- IMO number change
        IF COALESCE(OLD.imo_number, 0) <> COALESCE(NEW.imo_number, 0) AND OLD.imo_number IS NOT NULL THEN
            INSERT INTO vessel_history (mmsi, field_name, old_value, new_value)
            VALUES (NEW.mmsi, 'imo_number', OLD.imo_number::text, NEW.imo_number::text);
        END IF;

        -- Vessel type change
        IF COALESCE(OLD.vessel_type_name, '') <> COALESCE(NEW.vessel_type_name, '') AND OLD.vessel_type_name IS NOT NULL THEN
            INSERT INTO vessel_history (mmsi, field_name, old_value, new_value)
            VALUES (NEW.mmsi, 'vessel_type_name', OLD.vessel_type_name, NEW.vessel_type_name);
        END IF;

        -- Destination change
        IF COALESCE(OLD.destination, '') <> COALESCE(NEW.destination, '') AND OLD.destination IS NOT NULL THEN
            INSERT INTO vessel_history (mmsi, field_name, old_value, new_value)
            VALUES (NEW.mmsi, 'destination', OLD.destination, NEW.destination);
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

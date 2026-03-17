-- Composite index for the LATERAL join in AllVesselsWithPositions (mmsi + timestamp DESC)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ais_positions_mmsi_ts
  ON ais_positions (mmsi, timestamp DESC);

-- Composite index for bbox + time filtering in TrailsInBBox
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ais_positions_ts_lat_lon
  ON ais_positions (timestamp DESC, latitude, longitude);

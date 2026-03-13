-- Migration: 001_initial_schema.down.sql
-- Rollback the initial schema

DROP VIEW IF EXISTS latest_vessel_positions;
DROP TABLE IF EXISTS ais_positions;
DROP TABLE IF EXISTS vessels;
DROP TABLE IF EXISTS ais_sources;
DROP EXTENSION IF EXISTS postgis;

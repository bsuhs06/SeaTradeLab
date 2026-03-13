# AIS Collector

Go service that polls AIS data sources and stores vessel positions in PostgreSQL + PostGIS.

Currently pulls from Finnish Digitraffic (free, no signup) which covers the Baltic Sea and Gulf of Finland — about 18,000 vessels per poll including ~1,300 Russian. See [GHOST_FLEET_TRACKING.md](GHOST_FLEET_TRACKING.md) for what we're actually looking for.

## Setup

```bash
make db-up          # start postgres + postgis in docker, runs migrations
cp .env.example .env  # defaults work fine for local dev
make run            # start collecting
```

You should see something like:
```
[AIS-COLLECTOR] Initialized source: digitraffic (type=digitraffic)
[AIS-COLLECTOR] Fetched 18241 records from digitraffic
[AIS-COLLECTOR] Collection cycle completed in 4.2s
```

Poll interval defaults to 10 minutes. Digitraffic caches for ~60s so polling faster than every 5 minutes is pointless — the collector enforces this.

## Adding a data source

The collector uses a source registry. Three steps:

1. Create `internal/sources/yoursource.go` implementing the `Source` interface (see `interface.go`)
2. Register it in `registry.go`'s `init()`:
   ```go
   DefaultRegistry.Register("yoursource", func(cfg Config) (Source, error) {
       return NewYourSource(cfg), nil
   })
   ```
3. Wire it up in `main.go`'s `buildSourceDefs()` with env vars

That's it — collector picks it up on next run.

## Current sources

**Digitraffic** — on by default. Free Finnish maritime API. Gulf of Finland + Baltic. No key needed.

**AISStream** — optional WebSocket stream from aisstream.io. Set `AISSTREAM_API_KEY` in `.env`. Covers configurable bounding boxes (Baltic, Med, Middle East, SE Asia).

**AISHub** — partially implemented. Set `AISHUB_USERNAME` to enable.

**Mock** — for testing. `ENABLE_MOCK_SOURCE=true`.

## Database

Tables: `vessels` (ship metadata, keyed by MMSI), `ais_positions` (time-series positions with PostGIS geography), `ais_sources` (source tracking).

PostGIS lets you do spatial queries:
```sql
-- ships within 10nm of a point
SELECT * FROM latest_vessel_positions
WHERE ST_DWithin(position, ST_SetSRID(ST_MakePoint(24.9, 60.1), 4326)::geography, 18520);
```

## Environment variables

| Variable | Default | What it does |
|----------|---------|-------------|
| `DATABASE_URL` | — | Postgres connection (required) |
| `POLL_INTERVAL_MINUTES` | `10` | How often to poll |
| `ENABLE_DIGITRAFFIC` | `true` | Finnish AIS |
| `ENABLE_MOCK_SOURCE` | `false` | Fake data for dev |
| `AISSTREAM_API_KEY` | — | Enables AISStream when set |
| `AISHUB_USERNAME` | — | Enables AISHub when set |

## Useful commands

```bash
make build         # compile to bin/collector
make test          # run tests
make db-shell      # psql into the database
make db-down       # stop postgres
```

## Deploying

Docker image:
```bash
docker build -t ais-collector .
```

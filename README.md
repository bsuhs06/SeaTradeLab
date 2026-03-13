# SeaTradeLab

Tracks maritime vessels in the Baltic Sea using free AIS data, with a focus on detecting shadow fleet activity — ship-to-ship transfers, AIS gaps, and non-Russian vessels visiting Russian oil terminals.

## What's in here

**[ais-collector/](./ais-collector/)** — Go service that polls AIS sources (primarily Finnish Digitraffic) and stores vessel positions in Postgres. Runs on a 10-minute interval. See its README for setup.

**[web-server/](./web-server/)** — Go web server with a Leaflet satellite map. Shows live vessel positions, track history, STS events, dark vessels. `go run ./cmd/server/` → http://localhost:8080

**[frontend/](./frontend/)** — Vue 3 + TypeScript SPA. Dashboard and live map views.

**[analytics/](./analytics/)** — Python detection algorithms:
- STS transfer detection (KD-tree proximity search, geodesic distance, port exclusion)
- AIS gap detection (vessels going dark)
- Russian port visit tracking (flags non-Russian vessels at Primorsk, Ust-Luga, etc.)
- Port arrival/departure monitoring for ~90 Baltic ports

## Quick start

```bash
cp .env.example .env                    # configure database credentials
docker compose up -d                    # postgres + postgis
cd ais-collector && make run            # start collecting
cd web-server && go run ./cmd/server/   # API on :8080
cd frontend && npm run dev              # frontend on :5173
cd analytics && python -m src.run       # run detectors
```

## Database

All services share one PostgreSQL + PostGIS instance. Configure your connection string in `.env`:

```
DATABASE_URL=postgres://<user>:<password>@localhost:5432/<database>?sslmode=disable
```

## Stack

Go, Python, PostgreSQL 16 + PostGIS, Vue 3, Docker. Digitraffic for AIS data (free, no signup).

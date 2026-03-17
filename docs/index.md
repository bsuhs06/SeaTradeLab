---
layout: default
title: Home
---

Track shadow fleet activity in the Baltic Sea using free AIS data. Detect ship-to-ship transfers, AIS gaps, and sanctioned port visits.

## What it detects

**Ship-to-Ship Transfers** — KD-tree proximity search with geodesic distance, speed filtering, and port exclusion zones to find vessels rendezvousing at sea.

**AIS Gaps** — Vessels going dark — transponders turned off to avoid tracking. Flags gaps with last-known position to distinguish from coverage edges.

**Port Visit Tracking** — Detects non-Russian vessels visiting Russian oil terminals (Primorsk, Ust-Luga, Vysotsk, etc.) using MMSI prefix flag identification.

**Live Vessel Map** — Satellite view of ~18,000 vessels with track history, flag filtering, and real-time position updates from Baltic AIS sources.

## Architecture

| Component | Description |
|-----------|-------------|
| **AIS Collector** | Go service polling AIS sources on a 10-minute interval. Stores positions in PostGIS. |
| **Web Server** | Go REST API serving vessel data, tracks, STS events, and analytics results. |
| **Frontend** | Vue 3 + TypeScript SPA with Leaflet satellite map and analytics dashboard. |
| **Analytics** | Python detection algorithms — STS, AIS gaps, port visits. Runs on-demand or scheduled. |
| **Database** | PostgreSQL 16 + PostGIS for spatial queries. Shared by all services. |
| **Data Sources** | Finnish Digitraffic maritime API. Free, no signup, covers the Baltic Sea. aisstream.io. Global live ais streaming. |

## Quick start

```bash
# Clone and configure
git clone https://github.com/bsuhs/seatradelab.git
cd seatradelab
cp .env.example .env           # set your database credentials

# Start the database
docker compose up -d

# Collect AIS data
cd ais-collector && make run

# Start the API server
cd web-server && go run ./cmd/server/

# Start the frontend (dev mode)
cd frontend && npm run dev

# Run detection algorithms
cd analytics && python -m src.run --detect
```

## Data sources

- **Finnish Digitraffic** — Baltic Sea + Gulf of Finland. ~18,000 vessels per poll including ~1,300 Russian-flagged. *Free*
- **AISStream** — WebSocket stream with configurable bounding boxes. Global coverage. *API key required*
- **AISHub** — Community AIS data sharing network. *Registration required*

The collector has a pluggable source architecture — add new sources by implementing a single Go interface.

## Why this exists

Commercial maritime intelligence platforms cost $50K+/year. This project explores what's possible with free public AIS data, PostGIS spatial queries, and straightforward detection algorithms. It's a research tool, not a production surveillance system.

---

## Latest from the blog

<ul class="post-list">
{% for post in site.posts limit:3 %}
<li>
  <strong><a href="{{ post.url | relative_url }}">{{ post.title }}</a></strong><br>
  <small>{{ post.date | date: "%B %d, %Y" }}{% if post.tags.size > 0 %} · {% for tag in post.tags %}<span class="post-tag">{{ tag }}</span> {% endfor %}{% endif %}</small><br>
  {{ post.excerpt | strip_html | truncatewords: 40 }}
</li>
{% endfor %}
</ul>

{% if site.posts.size > 3 %}
[See all posts →]({{ '/blog' | relative_url }})
{% endif %}

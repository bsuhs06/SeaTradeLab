---
layout: default
title: About
description: What SeaTradeLab is and why it exists.
permalink: /about/
---

# About

## What is SeaTradeLab?

SeaTradeLab is an open-source maritime intelligence platform that tracks shadow fleet activity in the Baltic Sea using free AIS (Automatic Identification System) data. It detects ship-to-ship transfers, AIS gaps, and non-Russian vessels visiting Russian oil terminals.

Commercial maritime intelligence platforms cost $50K+/year. This project explores what's possible with free public AIS data, PostGIS spatial queries, and straightforward detection algorithms. It's a research tool, not a production surveillance system.

## The Stack

- **AIS Collector** — Go service polling AIS sources on a 10-minute interval, storing vessel positions in PostGIS
- **Web Server** — Go REST API serving vessel data, tracks, STS events, and analytics results
- **Frontend** — Vue 3 + TypeScript SPA with Leaflet satellite map and analytics dashboard
- **Analytics** — Python detection algorithms for STS transfers, AIS gaps, and port visits
- **Database** — PostgreSQL 16 + PostGIS for spatial queries

## Data Sources

The primary data source is the [Finnish Digitraffic maritime API](https://www.digitraffic.fi/en/marine-traffic/), which provides free AIS data covering the Baltic Sea and Gulf of Finland — roughly 18,000 vessels per poll including ~1,300 Russian-flagged vessels.

## Contact

Find the project on [GitHub](https://github.com/bsuhs/seatradelab).

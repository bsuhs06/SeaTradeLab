# Changelog

Notes on what changed between sessions. Not exhaustive.

## Recent

- STS detector was flagging fishing boats anchored together near Kotka. Added 10km port exclusion radius and commercial vessel type filter to cut the noise.
- Digitraffic sometimes returns 0 results around 3-4am UTC (probably maintenance window). Collector handles this gracefully now — logs it and moves on instead of upserting empty data.
- Added port_overrides table so you can add/remove ports without touching code. Was getting tired of editing the PORTS list every time.
- Russian port visit detector now derives flag country from MMSI prefix instead of hardcoding "Russia = 273". Has a full MID mapping for ~150 countries. Turns out a surprising number of Cameroon-flagged tankers visit Primorsk.
- AIS gap detector had a false positive problem — vessels sailing out of Digitraffic coverage (leaving the Baltic) would show as "dark." Can't really fix this without a second data source, but the gap detector now notes the last known position so you can tell if the vessel was near the edge of coverage.
- Reduced hosting costs significantly by switching to a simpler deployment setup.

## TODO

- [ ] Add Norwegian Kystverket source for Murmansk/Barents coverage
- [ ] Vessel ownership data (Equasis or similar) — right now we can see a ship but not who actually owns it
- [ ] Better handling of MMSI changes / identity switching
- [ ] Alert system (email/webhook when specific patterns are detected)
- [ ] Historical data export for offline analysis

# Ghost Fleet Tracking

The shadow/dark fleet is a network of mostly old tankers used to move sanctioned Russian oil without getting caught. They do things like turn off AIS, swap identities, and do ship-to-ship transfers in open water so the crude can't be traced back to a Russian port.

This project watches for those patterns using free AIS data from the Baltic.

## Why the Baltic?

Most Russian crude leaving the western side of the country goes through a handful of ports in the Gulf of Finland — Primorsk, Ust-Luga, Vysotsk, St. Petersburg. All of those are in range of the Finnish Digitraffic AIS API, which is free and doesn't require signup. That's our primary data source.

Coverage is roughly 54-60°N, 10-30°E. It catches the Gulf of Finland, the approaches to Kaliningrad, and the Danish straits (the only way out of the Baltic to the North Sea). Not global, but it's where a lot of the action is.

Eventually I'd like to add Norwegian Kystverket data for Murmansk/Barents coverage and Danish Maritime Authority for better strait visibility. The source registry makes this straightforward — see the README for how to add a new source.

## What we detect

**AIS gaps** — vessels that stop broadcasting. Could be equipment failure, could be deliberate. The detector flags anything over 6 hours by default. Caveat: a ship sailing out of Digitraffic's coverage area will look "dark" even if it's still transmitting, so gaps near the edge of coverage are less interesting than gaps mid-Gulf of Finland.

**Ship-to-ship transfers** — two vessels parked within 500m of each other, both going under 3 knots, for at least 15 minutes. Uses a KD-tree for fast proximity searching, then geodesic distance for accuracy. Only flags pairs where at least one vessel is a tanker/cargo type, and only if they're 10+ km from any port (otherwise you're just looking at an anchorage).

**Russian port visits** — flags every vessel that enters a Russian port boundary, and highlights the non-Russian ones. A Cypriot-flagged tanker spending 8 hours at Primorsk is more interesting than a Russian tug doing the same thing.

**Port tracking** — arrivals and departures at ~90 Baltic ports. Mostly useful context for the other detectors.

## Quick queries

Once you have data, these are useful:

```sql
-- Ships that haven't reported in 24+ hours
SELECT v.mmsi, v.name,
       MAX(ap.timestamp) as last_seen,
       NOW() - MAX(ap.timestamp) as gap
FROM vessels v
JOIN ais_positions ap ON v.mmsi = ap.mmsi
GROUP BY v.mmsi, v.name
HAVING NOW() - MAX(ap.timestamp) > INTERVAL '24 hours'
ORDER BY gap DESC;

-- Slow-movers (loitering, possible transfer)
SELECT v.mmsi, v.name, ap.latitude, ap.longitude, ap.speed_over_ground
FROM vessels v
JOIN ais_positions ap ON v.mmsi = ap.mmsi
WHERE ap.speed_over_ground BETWEEN 0.1 AND 2.0
  AND ap.timestamp > NOW() - INTERVAL '24 hours'
ORDER BY ap.timestamp DESC;
```

## Data volume

Digitraffic returns ~18,000 vessel positions per poll. At 10-minute intervals that's about 5-20 MB/day of database growth. Very manageable.

## Things to keep in mind

MMSI prefix 273 = Russian Federation. Useful for quick filtering but not definitive — vessels change flags. The russian_port_visits analyzer has a full MMSI-to-country mapping for ~150 countries.

The Digitraffic API caches for ~60 seconds. The collector enforces a 5-minute minimum between requests. Polling more often than that is pointless and rude.

## Resources

- OFAC sanctions list: https://sanctionssearch.ofac.treas.gov/
- EU sanctions map: https://www.sanctionsmap.eu/
- C4ADS (sanctions research): https://c4ads.org/

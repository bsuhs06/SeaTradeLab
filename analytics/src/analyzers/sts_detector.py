"""
STS (ship-to-ship) transfer detection.

Finds pairs of vessels parked near each other at sea — slow, close together,
for a sustained period. Filters out port anchorages and non-commercial vessels.
"""

import numpy as np
import pandas as pd
from geopy.distance import geodesic
from scipy.spatial import cKDTree

from ..database import all_positions_in_window, query_df
from .port_tracker import PORTS

# Rough conversion: 1 degree latitude ~ 111km
DEG_TO_KM = 111.0

# Vessel types considered "commercial" — at least one vessel in an STS pair
# must be one of these for the event to be relevant.
COMMERCIAL_TYPES = {
    "tanker", "cargo", "oil tanker", "chemical tanker",
    "lng tanker", "lpg tanker", "bulk carrier", "container ship",
    "general cargo", "oil/chemical tanker",
}


def _is_commercial_type(vessel_type: str | None) -> bool:
    if vessel_type is None or not isinstance(vessel_type, str):
        return False
    vt = vessel_type.strip().lower()
    # Direct match
    if vt in COMMERCIAL_TYPES:
        return True
    # Partial match — vessel_type_name from Digitraffic can be verbose
    for ct in COMMERCIAL_TYPES:
        if ct in vt:
            return True
    return False


def _is_in_any_port(lat: float, lon: float) -> bool:
    lat, lon = float(lat), float(lon)
    for name, plat, plon, radius_km, _country, _ptype in PORTS:
        # Quick degree-based pre-filter (~1 deg ≈ 111km) to avoid expensive geodesic
        dlat = abs(lat - plat)
        dlon = abs(lon - plon)
        if dlat > radius_km / 80.0 or dlon > radius_km / 80.0:
            continue
        if geodesic((lat, lon), (plat, plon)).km <= radius_km:
            return True
    return False


def _min_distance_to_port_km(lat: float, lon: float) -> float:
    """Return the distance in km to the nearest known port."""
    lat, lon = float(lat), float(lon)
    min_dist = float("inf")
    for _name, plat, plon, _radius, _country, _ptype in PORTS:
        # Quick pre-filter: skip ports obviously far away
        dlat = abs(lat - plat)
        dlon = abs(lon - plon)
        approx_km = max(dlat, dlon) * 111.0
        if approx_km > min_dist:
            continue
        d = geodesic((lat, lon), (plat, plon)).km
        if d < min_dist:
            min_dist = d
    return min_dist


def find_proximity_events(
    hours: int = 12,
    distance_threshold_m: float = 500,
    speed_threshold_kts: float = 3.0,
    min_duration_minutes: int = 15,
    min_distance_from_port_km: float = 10.0,
) -> pd.DataFrame:
    """
    Scan recent AIS data for potential ship-to-ship transfer events.

    Args:
        hours: How many hours back to look.
        distance_threshold_m: Max distance in meters to count as "close".
        speed_threshold_kts: Max SOG to count as "loitering".
        min_duration_minutes: Minimum duration of proximity to flag.
        min_distance_from_port_km: Minimum distance from any port (km).
            Events closer than this are likely port/anchorage activity.

    Returns:
        DataFrame of potential STS events with columns:
        mmsi_a, mmsi_b, name_a, name_b, start_time, end_time,
        duration_minutes, min_distance_m, avg_lat, avg_lon
    """
    df = all_positions_in_window(hours)
    if df.empty:
        return pd.DataFrame()

    # Round timestamps to 10-minute windows for matching
    df["time_bucket"] = df["timestamp"].dt.floor("10min")

    events = []

    for bucket, group in df.groupby("time_bucket"):
        # Filter to slow-moving vessels
        slow = group[
            (group["speed_over_ground"].notna())
            & (group["speed_over_ground"] <= speed_threshold_kts)
        ]
        if len(slow) < 2:
            continue

        # Use a KD-tree for fast proximity search
        coords = slow[["latitude", "longitude"]].values
        # Convert distance threshold to approximate degrees
        threshold_deg = distance_threshold_m / (DEG_TO_KM * 1000)
        tree = cKDTree(coords)
        pairs = tree.query_pairs(r=threshold_deg)

        for i, j in pairs:
            row_a = slow.iloc[i]
            row_b = slow.iloc[j]

            # Skip same vessel
            if row_a["mmsi"] == row_b["mmsi"]:
                continue

            # Compute precise geodesic distance
            dist = geodesic(
                (row_a["latitude"], row_a["longitude"]),
                (row_b["latitude"], row_b["longitude"]),
            ).meters

            if dist <= distance_threshold_m:
                # At least one vessel must be a commercial type (tanker/cargo)
                type_a = row_a.get("vessel_type_name")
                type_b = row_b.get("vessel_type_name")
                if not _is_commercial_type(type_a) and not _is_commercial_type(type_b):
                    continue

                avg_lat = (row_a["latitude"] + row_b["latitude"]) / 2
                avg_lon = (row_a["longitude"] + row_b["longitude"]) / 2

                # Skip events where EITHER vessel is inside a port area
                if _is_in_any_port(row_a["latitude"], row_a["longitude"]):
                    continue
                if _is_in_any_port(row_b["latitude"], row_b["longitude"]):
                    continue

                # Must be far enough from any port (anchorage/coastal filter)
                if _min_distance_to_port_km(avg_lat, avg_lon) < min_distance_from_port_km:
                    continue

                events.append(
                    {
                        "mmsi_a": int(row_a["mmsi"]),
                        "mmsi_b": int(row_b["mmsi"]),
                        "name_a": row_a.get("name"),
                        "name_b": row_b.get("name"),
                        "time": bucket,
                        "distance_m": round(dist, 1),
                        "lat": round(avg_lat, 5),
                        "lon": round(avg_lon, 5),
                        "sog_a": row_a["speed_over_ground"],
                        "sog_b": row_b["speed_over_ground"],
                        "type_a": row_a.get("vessel_type_name"),
                        "type_b": row_b.get("vessel_type_name"),
                    }
                )

    if not events:
        return pd.DataFrame()

    result = pd.DataFrame(events)

    # Normalize pairs so mmsi_a < mmsi_b (avoid duplicates)
    mask = result["mmsi_a"] > result["mmsi_b"]
    for col in ["mmsi", "name", "sog", "type"]:
        a_col, b_col = f"{col}_a", f"{col}_b"
        result.loc[mask, [a_col, b_col]] = result.loc[mask, [b_col, a_col]].values

    # Group consecutive time buckets for the same pair
    result = result.sort_values(["mmsi_a", "mmsi_b", "time"])
    result = (
        result.groupby(["mmsi_a", "mmsi_b"])
        .agg(
            name_a=("name_a", "first"),
            name_b=("name_b", "first"),
            type_a=("type_a", "first"),
            type_b=("type_b", "first"),
            start_time=("time", "min"),
            end_time=("time", "max"),
            observations=("time", "count"),
            min_distance_m=("distance_m", "min"),
            avg_lat=("lat", "mean"),
            avg_lon=("lon", "mean"),
        )
        .reset_index()
    )

    # Calculate duration and filter
    result["duration_minutes"] = (
        (result["end_time"] - result["start_time"]).dt.total_seconds() / 60
    )
    result = result[result["duration_minutes"] >= min_duration_minutes]

    return result.sort_values("duration_minutes", ascending=False)


def flag_russian_sts(hours: int = 24) -> pd.DataFrame:
    """
    Find STS events involving at least one Russian-flagged vessel.
    These are the most interesting for ghost fleet tracking.
    """
    events = find_proximity_events(hours=hours)
    if events.empty:
        return events

    # Filter to events where at least one vessel is Russian (273 prefix)
    mask = events["mmsi_a"].astype(str).str.startswith("273") | events[
        "mmsi_b"
    ].astype(str).str.startswith("273")
    return events[mask]


def persist_events(events_df, min_distance_from_port_km: float = 10.0):
    """
    Write detected STS events to the sts_events table.
    Also cleans up invalid STS events (in-port, non-commercial, too close to port).
    Uses ON CONFLICT to avoid duplicate entries.
    Returns the number of rows inserted.
    """
    from ..database import get_connection

    # Clean up existing STS events that fail current filters
    with get_connection() as conn:
        with conn.cursor() as cur:
            cur.execute("SELECT id, avg_lat, avg_lon, type_a, type_b FROM sts_events")
            rows = cur.fetchall()
            ids_to_delete = []
            for row_id, lat, lon, type_a, type_b in rows:
                reason = None
                if lat is not None and lon is not None:
                    if _is_in_any_port(lat, lon):
                        reason = "in-port"
                    elif _min_distance_to_port_km(lat, lon) < min_distance_from_port_km:
                        reason = "too close to port"
                if not _is_commercial_type(type_a) and not _is_commercial_type(type_b):
                    reason = "non-commercial"
                if reason:
                    ids_to_delete.append(row_id)
            if ids_to_delete:
                cur.execute(
                    "DELETE FROM sts_events WHERE id = ANY(%s)",
                    (ids_to_delete,),
                )
                conn.commit()
                print(f"  Cleaned up {len(ids_to_delete)} invalid STS events from database")

    if events_df.empty:
        return 0

    from ..database import get_connection

    with get_connection() as conn:
        with conn.cursor() as cur:
            count = 0
            for _, row in events_df.iterrows():
                duration = int(row.get("duration_minutes", 0))
                confidence = "high" if duration >= 60 else ("medium" if duration >= 15 else "low")
                cur.execute(
                    """
                    INSERT INTO sts_events (mmsi_a, mmsi_b, name_a, name_b, type_a, type_b,
                        start_time, end_time, duration_minutes, min_distance_m,
                        avg_lat, avg_lon, observations, confidence)
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                    ON CONFLICT (mmsi_a, mmsi_b, start_time) DO NOTHING
                    """,
                    (
                        int(row["mmsi_a"]),
                        int(row["mmsi_b"]),
                        row.get("name_a"),
                        row.get("name_b"),
                        row.get("type_a"),
                        row.get("type_b"),
                        row["start_time"],
                        row["end_time"],
                        duration,
                        row.get("min_distance_m"),
                        row.get("avg_lat"),
                        row.get("avg_lon"),
                        row.get("observations"),
                        confidence,
                    ),
                )
                count += cur.rowcount
            conn.commit()
    return count

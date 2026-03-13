"""
AIS gap detection — finds vessels that stopped transmitting.
"""

import pandas as pd
from ..database import get_connection, query_df


def find_dark_vessels(min_gap_hours=6):
    return query_df(
        """
        SELECT v.mmsi, v.name, v.vessel_type_name,
            ap.latitude, ap.longitude, ap.speed_over_ground as sog,
            ap.course_over_ground as cog, ap.heading,
            ap.timestamp as last_seen,
            EXTRACT(EPOCH FROM (NOW() - ap.timestamp))/3600 as gap_hours
        FROM vessels v
        INNER JOIN LATERAL (
            SELECT latitude, longitude, speed_over_ground, course_over_ground,
                heading, timestamp
            FROM ais_positions WHERE mmsi = v.mmsi ORDER BY timestamp DESC LIMIT 1
        ) ap ON true
        WHERE EXTRACT(EPOCH FROM (NOW() - ap.timestamp))/3600 >= %s
        ORDER BY ap.timestamp ASC
        """,
        params=(min_gap_hours,),
    )


def detect_ais_gaps(hours_back=48, min_gap_hours=2):
    """
    Scan position history for significant AIS gaps within a time window.

    A gap is when a vessel has no position reports for >= min_gap_hours
    between two consecutive reports.

    Returns DataFrame with columns: mmsi, last_position_time, last_lat,
    last_lon, last_sog, last_cog, reappear_time, reappear_lat, reappear_lon,
    gap_hours
    """
    df = query_df(
        """
        SELECT mmsi, latitude, longitude, speed_over_ground,
               course_over_ground, timestamp
        FROM ais_positions
        WHERE timestamp > NOW() - make_interval(hours => %s)
        ORDER BY mmsi, timestamp ASC
        """,
        params=(hours_back,),
    )

    if df.empty:
        return pd.DataFrame()

    gaps = []
    for mmsi, vessel_data in df.groupby("mmsi"):
        vessel_data = vessel_data.sort_values("timestamp")
        times = vessel_data["timestamp"].values

        for i in range(1, len(times)):
            gap = (times[i] - times[i - 1]) / pd.Timedelta(hours=1)
            if gap >= min_gap_hours:
                before = vessel_data.iloc[i - 1]
                after = vessel_data.iloc[i]
                gaps.append(
                    {
                        "mmsi": int(mmsi),
                        "last_position_time": before["timestamp"],
                        "last_lat": before["latitude"],
                        "last_lon": before["longitude"],
                        "last_sog": before["speed_over_ground"],
                        "last_cog": before["course_over_ground"],
                        "reappear_time": after["timestamp"],
                        "reappear_lat": after["latitude"],
                        "reappear_lon": after["longitude"],
                        "gap_hours": round(gap, 2),
                    }
                )

    if not gaps:
        return pd.DataFrame()

    return pd.DataFrame(gaps).sort_values("gap_hours", ascending=False)


def persist_gaps(gaps_df):
    """Write detected AIS gaps to the ais_gaps table."""
    if gaps_df.empty:
        return 0

    with get_connection() as conn:
        with conn.cursor() as cur:
            count = 0
            for _, row in gaps_df.iterrows():
                cur.execute(
                    """
                    INSERT INTO ais_gaps (mmsi, last_position_time, last_lat, last_lon,
                        last_sog, last_cog, reappear_time, reappear_lat, reappear_lon,
                        gap_hours, is_active)
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        int(row["mmsi"]),
                        row["last_position_time"],
                        row.get("last_lat"),
                        row.get("last_lon"),
                        row.get("last_sog"),
                        row.get("last_cog"),
                        row.get("reappear_time"),
                        row.get("reappear_lat"),
                        row.get("reappear_lon"),
                        row.get("gap_hours"),
                        row.get("reappear_time") is None,
                    ),
                )
                count += cur.rowcount
            conn.commit()
    return count

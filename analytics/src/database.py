"""
Database connection helper for the analytics module.
Connects to the same PostgreSQL/PostGIS database used by the AIS collector.
"""

import os
import psycopg2
import psycopg2.extras
import pandas as pd
from dotenv import load_dotenv

load_dotenv()

DATABASE_URL = os.getenv("DATABASE_URL")
if not DATABASE_URL:
    raise RuntimeError("DATABASE_URL environment variable is required")


def get_connection():
    """Return a psycopg2 connection to the AIS database."""
    return psycopg2.connect(DATABASE_URL)


def query_df(sql: str, params=None) -> pd.DataFrame:
    """Run a SQL query and return results as a pandas DataFrame."""
    with get_connection() as conn:
        return pd.read_sql_query(sql, conn, params=params)


def russian_vessels() -> pd.DataFrame:
    """Return all Russian-flagged vessels (MMSI prefix 273)."""
    return query_df(
        "SELECT * FROM vessels WHERE mmsi::text LIKE '273%' ORDER BY last_seen_at DESC"
    )


def vessel_track(mmsi: int, hours: int = 24) -> pd.DataFrame:
    """Return position history for a single vessel."""
    return query_df(
        """
        SELECT mmsi, latitude, longitude, speed_over_ground, course_over_ground,
               heading, navigation_status_name, timestamp
        FROM ais_positions
        WHERE mmsi = %s AND timestamp > NOW() - make_interval(hours => %s)
        ORDER BY timestamp ASC
        """,
        params=(mmsi, hours),
    )


def all_positions_in_window(hours: int = 6) -> pd.DataFrame:
    """Return all positions from the last N hours."""
    return query_df(
        """
        SELECT ap.mmsi, v.name, v.vessel_type_name,
               ap.latitude, ap.longitude, ap.speed_over_ground,
               ap.course_over_ground, ap.timestamp
        FROM ais_positions ap
        JOIN vessels v ON v.mmsi = ap.mmsi
        WHERE ap.timestamp > NOW() - make_interval(hours => %s)
        ORDER BY ap.timestamp ASC
        """,
        params=(hours,),
    )

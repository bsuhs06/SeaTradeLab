"""
Port arrival/departure tracking for ~90 Baltic ports.
"""

import pandas as pd
import numpy as np
from geopy.distance import geodesic

from ..database import query_df, get_connection

# Port definitions: (name, lat, lon, radius_km, country, port_type)
# Radius defines a circle around the port center
# port_type: oil, lng, commercial, cargo, naval, ferry, fishing, multi
PORTS = [
    # ===== RUSSIA =====
    ("Primorsk", 60.355, 29.206, 6.0, "Russia", "oil"),
    ("Ust-Luga", 59.680, 28.390, 6.0, "Russia", "oil"),
    ("Vysotsk", 60.627, 28.573, 4.0, "Russia", "oil"),
    ("St. Petersburg", 59.933, 30.300, 8.0, "Russia", "commercial"),
    ("Kaliningrad", 54.710, 20.500, 6.0, "Russia", "commercial"),
    ("Kronshtadt", 59.990, 29.770, 4.0, "Russia", "naval"),
    ("Vyborg", 60.710, 28.750, 4.0, "Russia", "cargo"),

    # ===== FINLAND =====
    ("Helsinki", 60.155, 24.955, 4.0, "Finland", "commercial"),
    ("Turku", 60.435, 22.230, 4.0, "Finland", "commercial"),
    ("Hamina-Kotka", 60.470, 26.950, 5.0, "Finland", "cargo"),
    ("Porvoo / Kilpilahti", 60.305, 25.555, 3.0, "Finland", "oil"),
    ("Naantali", 60.465, 22.030, 3.0, "Finland", "oil"),
    ("Rauma", 61.130, 21.460, 4.0, "Finland", "cargo"),
    ("Pori / Tahkoluoto", 61.635, 21.390, 4.0, "Finland", "lng"),
    ("Kokkola", 63.840, 23.030, 4.0, "Finland", "cargo"),
    ("Pietarsaari / Jakobstad", 63.710, 22.690, 3.0, "Finland", "cargo"),
    ("Vaasa", 63.085, 21.575, 3.0, "Finland", "ferry"),
    ("Oulu", 65.010, 25.410, 4.0, "Finland", "cargo"),
    ("Kemi", 65.740, 24.540, 4.0, "Finland", "cargo"),
    ("Tornio / Raahe", 64.680, 24.470, 5.0, "Finland", "cargo"),
    ("Raahe", 64.680, 24.470, 4.0, "Finland", "cargo"),
    ("Hanko", 59.820, 22.970, 3.0, "Finland", "cargo"),
    ("Loviisa", 60.445, 26.240, 3.0, "Finland", "cargo"),
    ("Inkoo", 60.045, 24.005, 3.0, "Finland", "lng"),
    ("Uusikaupunki", 60.795, 21.395, 3.0, "Finland", "cargo"),
    ("Mariehamn", 60.097, 19.935, 3.0, "Finland", "ferry"),
    ("Eckerö", 60.225, 19.535, 2.0, "Finland", "ferry"),
    ("Långnäs", 60.115, 20.295, 2.0, "Finland", "ferry"),

    # ===== SWEDEN (Baltic coast) =====
    ("Stockholm", 59.325, 18.070, 5.0, "Sweden", "commercial"),
    ("Nynäshamn", 58.900, 17.950, 3.0, "Sweden", "oil"),
    ("Södertälje", 59.195, 17.625, 3.0, "Sweden", "cargo"),
    ("Kapellskär", 59.720, 19.065, 2.0, "Sweden", "ferry"),
    ("Grisslehamn", 60.105, 18.820, 2.0, "Sweden", "ferry"),
    ("Norrtälje", 59.760, 18.700, 2.0, "Sweden", "cargo"),
    ("Oxelösund", 58.665, 17.125, 3.0, "Sweden", "cargo"),
    ("Norrköping", 58.595, 16.200, 4.0, "Sweden", "cargo"),
    ("Västervik", 57.755, 16.655, 2.0, "Sweden", "cargo"),
    ("Visby", 57.640, 18.290, 3.0, "Sweden", "commercial"),
    ("Slite", 57.710, 18.810, 2.0, "Sweden", "cargo"),
    ("Oskarshamn", 57.265, 16.455, 3.0, "Sweden", "cargo"),
    ("Kalmar", 56.660, 16.365, 3.0, "Sweden", "cargo"),
    ("Karlskrona", 56.160, 15.590, 3.0, "Sweden", "naval"),
    ("Karlshamn", 56.165, 14.860, 3.0, "Sweden", "cargo"),
    ("Gävle", 60.675, 17.195, 4.0, "Sweden", "lng"),
    ("Sundsvall", 62.390, 17.340, 4.0, "Sweden", "cargo"),
    ("Härnösand", 62.635, 17.940, 3.0, "Sweden", "cargo"),
    ("Örnsköldsvik", 63.290, 18.720, 3.0, "Sweden", "cargo"),
    ("Umeå", 63.720, 20.270, 4.0, "Sweden", "cargo"),
    ("Skellefteå", 64.680, 21.230, 3.0, "Sweden", "cargo"),
    ("Luleå", 65.575, 22.145, 5.0, "Sweden", "cargo"),
    ("Ystad", 55.425, 13.830, 3.0, "Sweden", "ferry"),
    ("Trelleborg", 55.370, 13.160, 3.0, "Sweden", "ferry"),
    ("Malmö", 55.615, 13.000, 4.0, "Sweden", "commercial"),
    ("Helsingborg", 56.040, 12.695, 3.0, "Sweden", "commercial"),
    ("Gothenburg", 57.695, 11.945, 5.0, "Sweden", "commercial"),
    ("Lysekil", 58.275, 11.430, 3.0, "Sweden", "oil"),
    ("Stenungsund", 58.070, 11.825, 3.0, "Sweden", "oil"),

    # ===== ESTONIA =====
    ("Tallinn / Muuga", 59.490, 24.960, 5.0, "Estonia", "commercial"),
    ("Paldiski", 59.350, 24.050, 3.0, "Estonia", "lng"),
    ("Sillamäe", 59.400, 27.760, 3.0, "Estonia", "oil"),
    ("Pärnu", 58.385, 24.495, 3.0, "Estonia", "cargo"),
    ("Kuressaare / Roomassaare", 58.225, 22.490, 2.0, "Estonia", "cargo"),

    # ===== LATVIA =====
    ("Riga", 57.045, 24.065, 5.0, "Latvia", "commercial"),
    ("Ventspils", 57.400, 21.540, 4.0, "Latvia", "oil"),
    ("Liepāja", 56.530, 21.000, 4.0, "Latvia", "commercial"),

    # ===== LITHUANIA =====
    ("Klaipėda", 55.710, 21.120, 5.0, "Lithuania", "lng"),
    ("Būtingė", 56.070, 21.060, 3.0, "Lithuania", "oil"),

    # ===== POLAND =====
    ("Gdańsk", 54.395, 18.670, 5.0, "Poland", "commercial"),
    ("Gdynia", 54.530, 18.545, 4.0, "Poland", "commercial"),
    ("Świnoujście", 53.910, 14.260, 4.0, "Poland", "lng"),
    ("Szczecin", 53.430, 14.570, 5.0, "Poland", "commercial"),
    ("Police", 53.565, 14.570, 3.0, "Poland", "cargo"),
    ("Elbląg", 54.155, 19.400, 3.0, "Poland", "cargo"),

    # ===== GERMANY (Baltic) =====
    ("Rostock / Warnemünde", 54.180, 12.100, 5.0, "Germany", "commercial"),
    ("Wismar", 53.900, 11.460, 3.0, "Germany", "cargo"),
    ("Lübeck / Travemünde", 53.960, 10.870, 4.0, "Germany", "commercial"),
    ("Kiel", 54.330, 10.150, 4.0, "Germany", "commercial"),
    ("Sassnitz / Mukran", 54.515, 13.615, 3.0, "Germany", "cargo"),
    ("Stralsund", 54.310, 13.100, 3.0, "Germany", "cargo"),
    ("Brunsbüttel", 53.895, 9.140, 3.0, "Germany", "lng"),

    # ===== DENMARK =====
    ("Copenhagen", 55.690, 12.610, 5.0, "Denmark", "commercial"),
    ("Fredericia", 55.565, 9.745, 3.0, "Denmark", "oil"),
    ("Aarhus", 56.150, 10.225, 4.0, "Denmark", "commercial"),
    ("Kalundborg", 55.680, 11.090, 3.0, "Denmark", "oil"),
    ("Rønne (Bornholm)", 55.090, 14.685, 3.0, "Denmark", "ferry"),
    ("Helsingør", 56.035, 12.615, 2.0, "Denmark", "ferry"),
    ("Gedser", 54.575, 11.925, 2.0, "Denmark", "ferry"),
    ("Rødby", 54.655, 11.350, 2.0, "Denmark", "ferry"),

    # ===== NORWAY (south/east) =====
    ("Oslo", 59.900, 10.740, 5.0, "Norway", "commercial"),
    ("Fredrikstad / Borg", 59.220, 10.960, 3.0, "Norway", "cargo"),
    ("Larvik", 59.050, 10.030, 3.0, "Norway", "cargo"),
    ("Kristiansand", 58.145, 8.000, 4.0, "Norway", "commercial"),
    ("Stavanger", 58.970, 5.730, 4.0, "Norway", "oil"),
    ("Bergen", 60.395, 5.320, 4.0, "Norway", "commercial"),
    ("Mongstad", 60.810, 5.035, 3.0, "Norway", "oil"),
    ("Sture", 60.580, 4.875, 2.0, "Norway", "oil"),
]

# Convenience: Russian ports only (for russian_port_visits module)
RUSSIAN_PORTS = [(n, lat, lon, r) for n, lat, lon, r, country, _ in PORTS if country == "Russia"]


def _in_port(lat: float, lon: float, port_lat: float, port_lon: float, radius_km: float) -> bool:
    """Check if a position is within a port's radius."""
    return geodesic((lat, lon), (port_lat, port_lon)).km <= radius_km


def classify_positions(hours: int = 24) -> pd.DataFrame:
    """
    Tag each position with the port it's in (if any).

    Returns a DataFrame with an added 'port' column.
    """
    df = query_df(
        """
        SELECT ap.mmsi, v.name, v.vessel_type_name,
               ap.latitude, ap.longitude, ap.speed_over_ground,
               ap.timestamp
        FROM ais_positions ap
        JOIN vessels v ON v.mmsi = ap.mmsi
        WHERE ap.timestamp > NOW() - make_interval(hours => %s)
          AND v.mmsi::text LIKE '273%%'
        ORDER BY ap.mmsi, ap.timestamp ASC
        """,
        params=(hours,),
    )
    if df.empty:
        return df

    def find_port(row):
        for name, plat, plon, radius, _country, _ptype in PORTS:
            if _in_port(row["latitude"], row["longitude"], plat, plon, radius):
                return name
        return None

    df["port"] = df.apply(find_port, axis=1)
    return df


def detect_port_visits(hours: int = 48) -> pd.DataFrame:
    """
    Detect port arrival/departure events for Russian vessels.

    Returns a DataFrame with columns:
        mmsi, name, port, arrival_time, departure_time, duration_hours
    """
    df = classify_positions(hours)
    if df.empty:
        return pd.DataFrame()

    visits = []

    for mmsi, vessel_data in df.groupby("mmsi"):
        vessel_data = vessel_data.sort_values("timestamp")
        name = vessel_data["name"].iloc[0]
        vtype = vessel_data["vessel_type_name"].iloc[0]

        current_port = None
        arrival_time = None

        for _, row in vessel_data.iterrows():
            port = row["port"]

            if port is not None and current_port is None:
                # Arrived at port
                current_port = port
                arrival_time = row["timestamp"]

            elif port is None and current_port is not None:
                # Left port
                visits.append(
                    {
                        "mmsi": mmsi,
                        "name": name,
                        "vessel_type": vtype,
                        "port": current_port,
                        "arrival_time": arrival_time,
                        "departure_time": row["timestamp"],
                        "duration_hours": round(
                            (row["timestamp"] - arrival_time).total_seconds() / 3600, 1
                        ),
                    }
                )
                current_port = None
                arrival_time = None

            elif port is not None and port != current_port:
                # Moved to different port (close ports might overlap)
                if current_port is not None:
                    visits.append(
                        {
                            "mmsi": mmsi,
                            "name": name,
                            "vessel_type": vtype,
                            "port": current_port,
                            "arrival_time": arrival_time,
                            "departure_time": row["timestamp"],
                            "duration_hours": round(
                                (row["timestamp"] - arrival_time).total_seconds() / 3600,
                                1,
                            ),
                        }
                    )
                current_port = port
                arrival_time = row["timestamp"]

        # If still in port at end of data
        if current_port is not None:
            visits.append(
                {
                    "mmsi": mmsi,
                    "name": name,
                    "vessel_type": vtype,
                    "port": current_port,
                    "arrival_time": arrival_time,
                    "departure_time": None,
                    "duration_hours": None,
                }
            )

    if not visits:
        return pd.DataFrame()

    return pd.DataFrame(visits).sort_values("arrival_time", ascending=False)

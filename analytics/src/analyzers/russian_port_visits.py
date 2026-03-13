"""
Flags vessels visiting Russian ports. Highlights non-Russian ships.
"""

import pandas as pd
import numpy as np
from geopy.distance import geodesic

from ..database import get_connection, query_df
from .port_tracker import RUSSIAN_PORTS

# MMSI MID (Maritime Identification Digits) to country mapping
# Source: ITU Maritime Identification Digits assignments
MID_COUNTRY = {
    "201": "Albania", "202": "Andorra", "203": "Austria", "204": "Azores",
    "205": "Belgium", "206": "Belarus", "207": "Bulgaria", "208": "Vatican",
    "209": "Cyprus", "210": "Cyprus", "211": "Germany", "212": "Cyprus",
    "213": "Georgia", "214": "Moldova", "215": "Malta", "216": "Armenia",
    "218": "Germany", "219": "Denmark", "220": "Denmark", "224": "Spain",
    "225": "Spain", "226": "France", "227": "France", "228": "France",
    "229": "Malta", "230": "Finland", "231": "Faroe Islands",
    "232": "United Kingdom", "233": "United Kingdom", "234": "United Kingdom",
    "235": "United Kingdom", "236": "Gibraltar", "237": "Greece",
    "238": "Croatia", "239": "Greece", "240": "Greece", "241": "Greece",
    "242": "Morocco", "243": "Hungary", "244": "Netherlands",
    "245": "Netherlands", "246": "Netherlands", "247": "Italy",
    "248": "Malta", "249": "Malta", "250": "Ireland", "251": "Iceland",
    "252": "Liechtenstein", "253": "Luxembourg", "254": "Madagascar",
    "255": "Madeira", "256": "Malta", "257": "Norway", "258": "Norway",
    "259": "Norway", "261": "Poland", "263": "Portugal", "264": "Romania",
    "265": "Sweden", "266": "Sweden", "267": "Sweden",
    "268": "San Marino", "269": "Switzerland", "270": "Czech Republic",
    "271": "Turkey", "272": "Ukraine", "273": "Russia",
    "274": "North Macedonia", "275": "Latvia", "276": "Estonia",
    "277": "Lithuania", "278": "Slovenia", "279": "Serbia",
    "301": "Anguilla", "303": "Alaska (USA)", "304": "Antigua & Barbuda",
    "305": "Antigua & Barbuda", "306": "Curacao", "307": "Aruba",
    "308": "Bahamas", "309": "Bahamas", "310": "Bermuda",
    "311": "Bahamas", "312": "Belize", "314": "Barbados",
    "316": "Canada", "319": "Cayman Islands",
    "321": "Costa Rica", "323": "Cuba", "325": "Dominica",
    "327": "Dominican Republic", "329": "Guadeloupe",
    "330": "Grenada", "331": "Greenland", "332": "Guatemala",
    "334": "Honduras", "336": "Haiti",
    "338": "United States", "339": "Jamaica", "341": "Saint Kitts & Nevis",
    "343": "Saint Lucia", "345": "Mexico",
    "347": "Martinique", "348": "Montserrat",
    "350": "Nicaragua", "351": "Panama", "352": "Panama",
    "353": "Panama", "354": "Panama", "355": "Panama",
    "356": "Panama", "357": "Panama",
    "358": "Puerto Rico", "359": "El Salvador",
    "361": "Saint Pierre & Miquelon",
    "362": "Trinidad & Tobago", "364": "Turks & Caicos",
    "366": "United States", "367": "United States", "368": "United States",
    "369": "United States", "370": "Panama",
    "371": "Panama", "372": "Panama", "373": "Panama",
    "374": "Panama", "375": "Saint Vincent & Grenadines",
    "376": "Saint Vincent & Grenadines",
    "377": "Saint Vincent & Grenadines",
    "378": "British Virgin Islands", "379": "US Virgin Islands",
    "401": "Afghanistan", "403": "Saudi Arabia",
    "405": "Bangladesh", "408": "Bahrain",
    "410": "Bhutan", "412": "China", "413": "China", "414": "China",
    "416": "Taiwan", "417": "Sri Lanka",
    "419": "India", "422": "Iran", "423": "Azerbaijan",
    "425": "Iraq", "428": "Israel", "431": "Japan",
    "432": "Japan", "434": "Turkmenistan",
    "436": "Kazakhstan", "437": "Uzbekistan",
    "438": "Jordan", "440": "South Korea", "441": "South Korea",
    "443": "Palestine", "445": "North Korea",
    "447": "Kuwait", "450": "Lebanon",
    "451": "Kyrgyzstan", "453": "Macao",
    "455": "Maldives", "457": "Mongolia",
    "459": "Nepal", "461": "Oman",
    "463": "Pakistan", "466": "Qatar",
    "468": "Syria", "470": "UAE",
    "471": "UAE", "472": "Tajikistan",
    "473": "Yemen", "475": "Yemen",
    "477": "Hong Kong", "478": "Bosnia & Herzegovina",
    "501": "Adelie Land (FR)", "503": "Australia",
    "506": "Myanmar", "508": "Brunei",
    "510": "Micronesia", "511": "Palau",
    "512": "New Zealand", "514": "Cambodia",
    "515": "Cambodia", "516": "Christmas Island",
    "518": "Cook Islands", "520": "Fiji",
    "523": "Cocos Islands", "525": "Indonesia",
    "529": "Kiribati", "531": "Laos",
    "533": "Malaysia", "536": "Northern Mariana Islands",
    "538": "Marshall Islands", "540": "New Caledonia",
    "542": "Niue", "544": "Nauru",
    "546": "French Polynesia", "548": "Philippines",
    "553": "Papua New Guinea",
    "555": "Pitcairn Island", "557": "Solomon Islands",
    "559": "American Samoa", "561": "Samoa",
    "563": "Singapore", "564": "Singapore",
    "565": "Singapore", "566": "Singapore",
    "567": "Thailand", "570": "Tonga",
    "572": "Tuvalu", "574": "Vietnam",
    "576": "Vanuatu", "577": "Vanuatu",
    "578": "Wallis & Futuna",
    "601": "South Africa", "603": "Angola",
    "605": "Algeria", "607": "Burkina Faso",
    "609": "Burundi", "610": "Benin",
    "611": "Botswana", "612": "Central African Republic",
    "613": "Cameroon", "615": "Congo",
    "616": "Comoros", "617": "Cabo Verde",
    "618": "Cote d'Ivoire", "619": "Djibouti",
    "620": "Egypt", "621": "Equatorial Guinea",
    "622": "Ethiopia", "624": "Eritrea",
    "625": "Gabon", "626": "Ghana",
    "627": "Gambia", "629": "Guinea-Bissau",
    "630": "Guinea", "631": "Liberia",
    "632": "Liberia", "633": "Liberia",
    "634": "Liberia", "635": "Liberia",
    "636": "Liberia", "637": "Liberia",
    "638": "South Sudan", "642": "Libya",
    "644": "Lesotho", "645": "Mauritius",
    "647": "Madagascar", "649": "Mali",
    "650": "Mozambique", "654": "Mauritania",
    "655": "Malawi", "656": "Niger",
    "657": "Nigeria", "659": "Namibia",
    "660": "Reunion", "661": "Rwanda",
    "662": "Sudan", "663": "Senegal",
    "664": "Seychelles", "665": "Saint Helena",
    "666": "Somalia", "667": "Sierra Leone",
    "668": "Sao Tome & Principe", "669": "Swaziland",
    "670": "Chad", "671": "Togo",
    "672": "Tunisia", "674": "Tanzania",
    "675": "Uganda", "676": "DR Congo",
    "677": "Tanzania", "678": "Zambia",
    "679": "Zimbabwe",
}


def mid_to_country(mmsi: int) -> str:
    """Convert MMSI to country name using MID prefix."""
    mid = str(mmsi)[:3]
    return MID_COUNTRY.get(mid, f"Unknown ({mid})")


def _in_port(lat, lon, port_lat, port_lon, radius_km):
    """Check if a position is within a port's radius."""
    return geodesic((lat, lon), (port_lat, port_lon)).km <= radius_km


def detect_russian_port_visits(hours: int = 168, min_duration_minutes: int = 30) -> pd.DataFrame:
    """
    Detect ALL vessels visiting Russian ports.

    Args:
        hours: How far back to scan
        min_duration_minutes: Minimum time in port to count as a visit

    Returns:
        DataFrame with visit records including flag info
    """
    # Build a bounding box query that covers all Russian ports with margin
    # This is much faster than scanning all positions
    lats = [p[1] for p in RUSSIAN_PORTS]
    lons = [p[2] for p in RUSSIAN_PORTS]
    max_radius_deg = 0.15  # ~15km margin in degrees

    min_lat = min(lats) - max_radius_deg
    max_lat = max(lats) + max_radius_deg
    min_lon = min(lons) - max_radius_deg
    max_lon = max(lons) + max_radius_deg

    print(f"  Scanning positions near Russian ports (lat {min_lat:.1f}-{max_lat:.1f}, lon {min_lon:.1f}-{max_lon:.1f})...")

    df = query_df(
        """
        SELECT ap.mmsi, v.name, v.vessel_type_name, v.call_sign, v.imo_number,
               ap.latitude, ap.longitude, ap.speed_over_ground, ap.timestamp
        FROM ais_positions ap
        JOIN vessels v ON v.mmsi = ap.mmsi
        WHERE ap.timestamp > NOW() - make_interval(hours => %s)
          AND ap.latitude BETWEEN %s AND %s
          AND ap.longitude BETWEEN %s AND %s
        ORDER BY ap.mmsi, ap.timestamp ASC
        """,
        params=(hours, min_lat, max_lat, min_lon, max_lon),
    )

    if df.empty:
        print("  No positions found near Russian ports.")
        return pd.DataFrame()

    print(f"  Found {len(df)} positions from {df['mmsi'].nunique()} vessels near Russian ports")

    # Classify each position by Russian port
    def find_russian_port(row):
        for name, plat, plon, radius in RUSSIAN_PORTS:
            if _in_port(row["latitude"], row["longitude"], plat, plon, radius):
                return name
        return None

    df["port"] = df.apply(find_russian_port, axis=1)
    in_port = df[df["port"].notna()]

    if in_port.empty:
        print("  No positions matched Russian port boundaries.")
        return pd.DataFrame()

    print(f"  {len(in_port)} positions from {in_port['mmsi'].nunique()} vessels inside Russian ports")

    # Group into visits
    visits = []
    for mmsi, vessel_data in in_port.groupby("mmsi"):
        vessel_data = vessel_data.sort_values("timestamp")
        name = vessel_data["name"].iloc[0]
        vtype = vessel_data["vessel_type_name"].iloc[0]
        call_sign = vessel_data["call_sign"].iloc[0]
        imo = vessel_data["imo_number"].iloc[0]
        is_russian = str(mmsi).startswith("273")
        country = mid_to_country(mmsi)

        current_port = None
        arrival_time = None
        speeds = []
        obs_count = 0

        for _, row in vessel_data.iterrows():
            port = row["port"]

            if current_port is None:
                # Start new visit
                current_port = port
                arrival_time = row["timestamp"]
                speeds = [row["speed_over_ground"] if pd.notna(row["speed_over_ground"]) else 0]
                obs_count = 1
            elif port == current_port:
                # Continue visit
                speeds.append(row["speed_over_ground"] if pd.notna(row["speed_over_ground"]) else 0)
                obs_count += 1
            else:
                # Port changed - close current visit
                duration_h = (row["timestamp"] - arrival_time).total_seconds() / 3600
                duration_min = duration_h * 60
                if duration_min >= min_duration_minutes or obs_count >= 3:
                    port_info = next((p for p in RUSSIAN_PORTS if p[0] == current_port), None)
                    visits.append({
                        "mmsi": mmsi,
                        "vessel_name": name,
                        "vessel_type": vtype,
                        "call_sign": call_sign,
                        "imo_number": imo if pd.notna(imo) else None,
                        "flag_country": country,
                        "is_russian": is_russian,
                        "port_name": current_port,
                        "port_lat": port_info[1] if port_info else None,
                        "port_lon": port_info[2] if port_info else None,
                        "arrival_time": arrival_time,
                        "departure_time": row["timestamp"],
                        "duration_hours": round(duration_h, 2),
                        "min_speed_kts": round(min(speeds), 2) if speeds else None,
                        "avg_speed_kts": round(np.mean(speeds), 2) if speeds else None,
                        "observations": obs_count,
                        "still_in_port": False,
                    })
                # Start new port visit
                current_port = port
                arrival_time = row["timestamp"]
                speeds = [row["speed_over_ground"] if pd.notna(row["speed_over_ground"]) else 0]
                obs_count = 1

        # Close final visit (vessel still in port or last known position was in port)
        if current_port is not None:
            last_row = vessel_data.iloc[-1]
            duration_h = (last_row["timestamp"] - arrival_time).total_seconds() / 3600
            duration_min = duration_h * 60
            if duration_min >= min_duration_minutes or obs_count >= 3:
                port_info = next((p for p in RUSSIAN_PORTS if p[0] == current_port), None)
                visits.append({
                    "mmsi": mmsi,
                    "vessel_name": name,
                    "vessel_type": vtype,
                    "call_sign": call_sign,
                    "imo_number": imo if pd.notna(imo) else None,
                    "flag_country": country,
                    "is_russian": is_russian,
                    "port_name": current_port,
                    "port_lat": port_info[1] if port_info else None,
                    "port_lon": port_info[2] if port_info else None,
                    "arrival_time": arrival_time,
                    "departure_time": last_row["timestamp"],
                    "duration_hours": round(duration_h, 2),
                    "min_speed_kts": round(min(speeds), 2) if speeds else None,
                    "avg_speed_kts": round(np.mean(speeds), 2) if speeds else None,
                    "observations": obs_count,
                    "still_in_port": True,
                })

    if not visits:
        print("  No qualifying port visits found.")
        return pd.DataFrame()

    result = pd.DataFrame(visits).sort_values("arrival_time", ascending=False)
    non_russian = result[~result["is_russian"]]
    print(f"  Detected {len(result)} port visits ({len(non_russian)} non-Russian vessels)")

    return result


def persist_port_visits(visits: pd.DataFrame) -> int:
    """Persist port visits to the russian_port_visits table."""
    if visits.empty:
        return 0

    conn = get_connection()
    cur = conn.cursor()
    count = 0

    for _, v in visits.iterrows():
        try:
            cur.execute(
                """
                INSERT INTO russian_port_visits
                    (mmsi, vessel_name, vessel_type, call_sign, imo_number,
                     flag_country, is_russian, port_name, port_lat, port_lon,
                     arrival_time, departure_time, duration_hours,
                     min_speed_kts, avg_speed_kts, observations, still_in_port)
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                ON CONFLICT (mmsi, port_name, arrival_time) DO UPDATE SET
                    departure_time = EXCLUDED.departure_time,
                    duration_hours = EXCLUDED.duration_hours,
                    observations = EXCLUDED.observations,
                    still_in_port = EXCLUDED.still_in_port
                """,
                (
                    int(v["mmsi"]),
                    v["vessel_name"],
                    v["vessel_type"],
                    v["call_sign"],
                    int(v["imo_number"]) if v["imo_number"] is not None and pd.notna(v["imo_number"]) else None,
                    v["flag_country"],
                    bool(v["is_russian"]),
                    v["port_name"],
                    v["port_lat"],
                    v["port_lon"],
                    v["arrival_time"],
                    v["departure_time"],
                    v["duration_hours"],
                    v["min_speed_kts"],
                    v["avg_speed_kts"],
                    int(v["observations"]),
                    bool(v["still_in_port"]),
                ),
            )
            count += 1
        except Exception as e:
            print(f"  Warning: failed to persist visit for MMSI {v['mmsi']}: {e}")
            conn.rollback()
            continue

    conn.commit()
    cur.close()
    conn.close()
    return count

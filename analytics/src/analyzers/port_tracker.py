"""
Port arrival/departure tracking for global ports.
"""

import pandas as pd
import numpy as np
from geopy.distance import geodesic

from ..database import query_df, get_connection

# Port definitions: (name, lat, lon, radius_km, country, port_type)
# Radius defines a circle around the port center
# port_type: oil, lng, commercial, cargo, naval, ferry, fishing, multi
PORTS = [
    # ============================================================
    #  REGION 1 — EUROPE (Baltic, North Sea, Atlantic)
    # ============================================================

    # ===== RUSSIA (Baltic) =====
    ("Primorsk", 60.355, 29.206, 6.0, "Russia", "oil"),
    ("Ust-Luga", 59.680, 28.390, 6.0, "Russia", "oil"),
    ("Vysotsk", 60.627, 28.573, 4.0, "Russia", "oil"),
    ("St. Petersburg", 59.933, 30.300, 8.0, "Russia", "commercial"),
    ("Kaliningrad", 54.710, 20.500, 6.0, "Russia", "commercial"),
    ("Kronshtadt", 59.990, 29.770, 4.0, "Russia", "naval"),
    ("Vyborg", 60.710, 28.750, 4.0, "Russia", "cargo"),

    # ===== RUSSIA (Arctic) =====
    ("Murmansk", 68.970, 33.060, 6.0, "Russia", "commercial"),
    ("Arkhangelsk", 64.540, 40.540, 5.0, "Russia", "cargo"),

    # ===== RUSSIA (Pacific) =====
    ("Vladivostok", 43.110, 131.890, 5.0, "Russia", "commercial"),
    ("Nakhodka", 42.820, 132.880, 4.0, "Russia", "cargo"),
    ("Vostochny", 42.750, 133.070, 5.0, "Russia", "cargo"),
    ("Kozmino", 42.730, 133.040, 4.0, "Russia", "oil"),
    ("De-Kastri", 51.470, 140.780, 4.0, "Russia", "oil"),

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

    # ===== SWEDEN =====
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

    # ===== GERMANY =====
    ("Rostock / Warnemünde", 54.180, 12.100, 5.0, "Germany", "commercial"),
    ("Wismar", 53.900, 11.460, 3.0, "Germany", "cargo"),
    ("Lübeck / Travemünde", 53.960, 10.870, 4.0, "Germany", "commercial"),
    ("Kiel", 54.330, 10.150, 4.0, "Germany", "commercial"),
    ("Sassnitz / Mukran", 54.515, 13.615, 3.0, "Germany", "cargo"),
    ("Stralsund", 54.310, 13.100, 3.0, "Germany", "cargo"),
    ("Brunsbüttel", 53.895, 9.140, 3.0, "Germany", "lng"),
    ("Hamburg", 53.545, 9.930, 8.0, "Germany", "commercial"),
    ("Bremerhaven", 53.540, 8.580, 5.0, "Germany", "commercial"),
    ("Wilhelmshaven", 53.515, 8.145, 5.0, "Germany", "oil"),

    # ===== DENMARK =====
    ("Copenhagen", 55.690, 12.610, 5.0, "Denmark", "commercial"),
    ("Fredericia", 55.565, 9.745, 3.0, "Denmark", "oil"),
    ("Aarhus", 56.150, 10.225, 4.0, "Denmark", "commercial"),
    ("Kalundborg", 55.680, 11.090, 3.0, "Denmark", "oil"),
    ("Rønne (Bornholm)", 55.090, 14.685, 3.0, "Denmark", "ferry"),
    ("Helsingør", 56.035, 12.615, 2.0, "Denmark", "ferry"),
    ("Gedser", 54.575, 11.925, 2.0, "Denmark", "ferry"),
    ("Rødby", 54.655, 11.350, 2.0, "Denmark", "ferry"),
    ("Skagen", 57.720, 10.595, 3.0, "Denmark", "commercial"),

    # ===== NORWAY =====
    ("Oslo", 59.900, 10.740, 5.0, "Norway", "commercial"),
    ("Fredrikstad / Borg", 59.220, 10.960, 3.0, "Norway", "cargo"),
    ("Larvik", 59.050, 10.030, 3.0, "Norway", "cargo"),
    ("Kristiansand", 58.145, 8.000, 4.0, "Norway", "commercial"),
    ("Stavanger", 58.970, 5.730, 4.0, "Norway", "oil"),
    ("Bergen", 60.395, 5.320, 4.0, "Norway", "commercial"),
    ("Mongstad", 60.810, 5.035, 3.0, "Norway", "oil"),
    ("Sture", 60.580, 4.875, 2.0, "Norway", "oil"),
    ("Hammerfest", 70.665, 23.680, 3.0, "Norway", "lng"),
    ("Tromsø", 69.650, 18.960, 3.0, "Norway", "commercial"),
    ("Narvik", 68.430, 17.430, 3.0, "Norway", "cargo"),

    # ===== NETHERLANDS =====
    ("Rotterdam", 51.905, 4.490, 10.0, "Netherlands", "commercial"),
    ("Amsterdam", 52.410, 4.790, 6.0, "Netherlands", "commercial"),

    # ===== BELGIUM =====
    ("Antwerp", 51.270, 4.350, 8.0, "Belgium", "commercial"),
    ("Zeebrugge", 51.350, 3.200, 4.0, "Belgium", "lng"),

    # ===== UNITED KINGDOM =====
    ("London / Tilbury", 51.460, 0.360, 5.0, "United Kingdom", "commercial"),
    ("Southampton", 50.895, -1.400, 5.0, "United Kingdom", "commercial"),
    ("Felixstowe", 51.955, 1.305, 4.0, "United Kingdom", "commercial"),
    ("Immingham", 53.630, -0.200, 5.0, "United Kingdom", "cargo"),
    ("Liverpool", 53.440, -3.020, 5.0, "United Kingdom", "commercial"),
    ("Milford Haven", 51.710, -5.050, 5.0, "United Kingdom", "oil"),
    ("Fawley", 50.830, -1.340, 3.0, "United Kingdom", "oil"),
    ("Sullom Voe", 60.460, -1.280, 4.0, "United Kingdom", "oil"),
    ("Scapa Flow", 58.890, -3.100, 5.0, "United Kingdom", "oil"),
    ("Aberdeen", 57.145, -2.070, 4.0, "United Kingdom", "oil"),
    ("Grangemouth", 56.035, -3.710, 4.0, "United Kingdom", "oil"),
    ("Teesport", 54.610, -1.160, 4.0, "United Kingdom", "cargo"),
    ("Hound Point", 56.010, -3.380, 3.0, "United Kingdom", "oil"),

    # ===== FRANCE =====
    ("Le Havre", 49.480, 0.110, 5.0, "France", "commercial"),
    ("Marseille / Fos", 43.400, 4.870, 6.0, "France", "commercial"),
    ("Dunkirk", 51.060, 2.370, 5.0, "France", "commercial"),
    ("Nantes / Saint-Nazaire", 47.280, -2.190, 5.0, "France", "commercial"),
    ("Brest", 48.380, -4.490, 4.0, "France", "naval"),

    # ===== SPAIN =====
    ("Barcelona", 41.350, 2.170, 5.0, "Spain", "commercial"),
    ("Valencia", 39.445, -0.320, 5.0, "Spain", "commercial"),
    ("Algeciras", 36.130, -5.440, 5.0, "Spain", "commercial"),
    ("Bilbao", 43.340, -3.050, 4.0, "Spain", "commercial"),
    ("Cartagena", 37.580, -0.990, 4.0, "Spain", "oil"),
    ("Tarragona", 41.090, 1.230, 4.0, "Spain", "oil"),
    ("Las Palmas", 28.140, -15.420, 5.0, "Spain", "commercial"),
    ("Huelva", 37.200, -6.940, 4.0, "Spain", "oil"),

    # ===== PORTUGAL =====
    ("Lisbon", 38.700, -9.140, 5.0, "Portugal", "commercial"),
    ("Sines", 37.950, -8.880, 5.0, "Portugal", "oil"),
    ("Leixões", 41.185, -8.710, 4.0, "Portugal", "commercial"),

    # ===== ITALY =====
    ("Genoa", 44.405, 8.925, 5.0, "Italy", "commercial"),
    ("Trieste", 45.640, 13.760, 5.0, "Italy", "oil"),
    ("Venice", 45.440, 12.350, 5.0, "Italy", "commercial"),
    ("Naples", 40.840, 14.270, 5.0, "Italy", "commercial"),
    ("Gioia Tauro", 38.430, 15.890, 5.0, "Italy", "commercial"),
    ("Augusta", 37.230, 15.230, 4.0, "Italy", "oil"),
    ("Taranto", 40.470, 17.210, 4.0, "Italy", "commercial"),
    ("Livorno", 43.550, 10.300, 4.0, "Italy", "commercial"),
    ("Ravenna", 44.490, 12.270, 4.0, "Italy", "cargo"),
    ("Cagliari", 39.210, 9.110, 4.0, "Italy", "commercial"),
    ("Milazzo", 38.230, 15.250, 3.0, "Italy", "oil"),
    ("Priolo / Siracusa", 37.160, 15.190, 4.0, "Italy", "oil"),
    ("La Spezia", 44.100, 9.840, 4.0, "Italy", "commercial"),
    ("Sarroch", 39.070, 9.020, 3.0, "Italy", "oil"),

    # ===== GREECE =====
    ("Piraeus", 37.940, 23.630, 5.0, "Greece", "commercial"),
    ("Thessaloniki", 40.620, 22.940, 5.0, "Greece", "commercial"),
    ("Agioi Theodoroi", 37.930, 23.100, 3.0, "Greece", "oil"),
    ("Elefsis", 38.040, 23.530, 3.0, "Greece", "oil"),
    ("Revithoussa", 37.960, 23.370, 3.0, "Greece", "lng"),

    # ===== CROATIA =====
    ("Rijeka", 45.330, 14.440, 4.0, "Croatia", "commercial"),
    ("Omišalj", 45.210, 14.530, 3.0, "Croatia", "oil"),

    # ===== SLOVENIA =====
    ("Koper", 45.550, 13.740, 4.0, "Slovenia", "commercial"),

    # ===== MONTENEGRO =====
    ("Bar", 42.090, 19.110, 3.0, "Montenegro", "commercial"),

    # ===== ALBANIA =====
    ("Durrës", 41.310, 19.440, 3.0, "Albania", "commercial"),

    # ===== MALTA =====
    ("Marsaxlokk", 35.830, 14.540, 4.0, "Malta", "commercial"),

    # ===== CYPRUS =====
    ("Limassol", 34.660, 33.040, 4.0, "Cyprus", "commercial"),
    ("Vasilikos", 34.710, 33.340, 3.0, "Cyprus", "oil"),

    # ===== ICELAND =====
    ("Reykjavik", 64.150, -21.930, 4.0, "Iceland", "commercial"),

    # ============================================================
    #  REGION 2 — BLACK SEA / MEDITERRANEAN EAST
    # ============================================================

    # ===== RUSSIA (Black Sea) =====
    ("Novorossiysk", 44.720, 37.790, 6.0, "Russia", "oil"),
    ("Tuapse", 44.100, 39.070, 4.0, "Russia", "oil"),
    ("Kavkaz", 45.370, 36.650, 4.0, "Russia", "oil"),
    ("Taman", 45.220, 36.720, 4.0, "Russia", "oil"),
    ("Sevastopol", 44.620, 33.530, 5.0, "Russia", "naval"),
    ("Kerch", 45.350, 36.480, 4.0, "Russia", "commercial"),

    # ===== TURKEY =====
    ("Istanbul", 41.010, 28.980, 8.0, "Turkey", "commercial"),
    ("Izmir / Aliaga", 38.800, 26.960, 5.0, "Turkey", "oil"),
    ("Mersin", 36.790, 34.640, 5.0, "Turkey", "commercial"),
    ("Iskenderun", 36.590, 36.170, 4.0, "Turkey", "commercial"),
    ("Ceyhan / BTC Terminal", 36.860, 35.770, 4.0, "Turkey", "oil"),
    ("Samsun", 41.280, 36.340, 4.0, "Turkey", "commercial"),
    ("Trabzon", 41.000, 39.730, 3.0, "Turkey", "commercial"),
    ("Dortyol", 36.850, 36.200, 3.0, "Turkey", "oil"),

    # ===== UKRAINE =====
    ("Odesa", 46.490, 30.760, 5.0, "Ukraine", "commercial"),
    ("Pivdennyi", 46.630, 31.010, 4.0, "Ukraine", "oil"),
    ("Mykolaiv", 46.970, 32.000, 4.0, "Ukraine", "cargo"),

    # ===== GEORGIA =====
    ("Batumi", 41.650, 41.630, 4.0, "Georgia", "oil"),
    ("Poti", 42.160, 41.670, 3.0, "Georgia", "commercial"),
    ("Supsa", 42.010, 41.770, 3.0, "Georgia", "oil"),

    # ===== ROMANIA =====
    ("Constanța", 44.170, 28.660, 6.0, "Romania", "commercial"),

    # ===== BULGARIA =====
    ("Varna", 43.190, 27.920, 4.0, "Bulgaria", "commercial"),
    ("Burgas", 42.490, 27.480, 4.0, "Bulgaria", "commercial"),

    # ===== EGYPT =====
    ("Port Said", 31.260, 32.300, 5.0, "Egypt", "commercial"),
    ("Suez", 29.960, 32.560, 5.0, "Egypt", "commercial"),
    ("Ain Sukhna", 29.610, 32.340, 4.0, "Egypt", "oil"),
    ("Alexandria", 31.195, 29.860, 5.0, "Egypt", "commercial"),
    ("Damietta", 31.470, 31.810, 4.0, "Egypt", "lng"),
    ("El Dekheila", 31.150, 29.790, 3.0, "Egypt", "cargo"),

    # ===== ISRAEL =====
    ("Haifa", 32.820, 35.000, 4.0, "Israel", "commercial"),
    ("Ashdod", 31.830, 34.640, 4.0, "Israel", "commercial"),
    ("Ashkelon", 31.670, 34.520, 3.0, "Israel", "oil"),
    ("Eilat", 29.550, 34.960, 3.0, "Israel", "commercial"),

    # ===== LEBANON =====
    ("Beirut", 33.900, 35.520, 4.0, "Lebanon", "commercial"),
    ("Tripoli (Lebanon)", 34.450, 35.820, 3.0, "Lebanon", "oil"),

    # ===== SYRIA =====
    ("Tartus", 34.890, 35.870, 4.0, "Syria", "naval"),
    ("Latakia", 35.520, 35.770, 4.0, "Syria", "commercial"),
    ("Baniyas", 35.230, 35.950, 3.0, "Syria", "oil"),

    # ===== LIBYA =====
    ("Zawiya", 32.760, 12.710, 3.0, "Libya", "oil"),
    ("Es Sider", 30.630, 18.350, 4.0, "Libya", "oil"),
    ("Ras Lanuf", 30.490, 18.550, 4.0, "Libya", "oil"),
    ("Brega", 30.410, 19.580, 3.0, "Libya", "oil"),
    ("Zueitina", 30.870, 20.120, 3.0, "Libya", "oil"),
    ("Marsa el Hariga / Tobruk", 32.080, 23.960, 3.0, "Libya", "oil"),

    # ===== TUNISIA =====
    ("Bizerte", 37.280, 9.870, 3.0, "Tunisia", "commercial"),
    ("La Skhirra", 34.300, 10.070, 3.0, "Tunisia", "oil"),

    # ===== ALGERIA =====
    ("Algiers", 36.770, 3.060, 5.0, "Algeria", "commercial"),
    ("Arzew", 35.830, -0.310, 4.0, "Algeria", "lng"),
    ("Skikda", 36.880, 6.910, 4.0, "Algeria", "lng"),
    ("Bejaia", 36.750, 5.100, 3.0, "Algeria", "oil"),

    # ===== MOROCCO =====
    ("Tanger Med", 35.890, -5.500, 5.0, "Morocco", "commercial"),
    ("Casablanca", 33.600, -7.620, 5.0, "Morocco", "commercial"),
    ("Mohammedia", 33.720, -7.400, 3.0, "Morocco", "oil"),

    # ============================================================
    #  REGION 3 — PERSIAN GULF / INDIAN OCEAN / RED SEA
    # ============================================================

    # ===== SAUDI ARABIA =====
    ("Ras Tanura", 26.640, 50.160, 5.0, "Saudi Arabia", "oil"),
    ("Jubail", 27.010, 49.660, 6.0, "Saudi Arabia", "commercial"),
    ("Yanbu", 24.090, 38.050, 5.0, "Saudi Arabia", "oil"),
    ("Jeddah", 21.480, 39.160, 6.0, "Saudi Arabia", "commercial"),
    ("Dammam / King Abdulaziz", 26.470, 50.210, 5.0, "Saudi Arabia", "commercial"),
    ("Ras al-Khair", 27.140, 49.250, 4.0, "Saudi Arabia", "cargo"),
    ("Jazan", 16.900, 42.570, 4.0, "Saudi Arabia", "oil"),

    # ===== UAE =====
    ("Fujairah", 25.120, 56.340, 5.0, "UAE", "oil"),
    ("Jebel Ali", 25.020, 55.060, 8.0, "UAE", "commercial"),
    ("Ruwais", 24.110, 52.730, 5.0, "UAE", "oil"),
    ("Khor Fakkan", 25.340, 56.350, 4.0, "UAE", "commercial"),
    ("Abu Dhabi / Musaffah", 24.440, 54.500, 5.0, "UAE", "commercial"),
    ("Das Island", 25.060, 52.870, 4.0, "UAE", "oil"),

    # ===== QATAR =====
    ("Ras Laffan", 25.920, 51.550, 6.0, "Qatar", "lng"),
    ("Mesaieed", 24.990, 51.570, 5.0, "Qatar", "oil"),

    # ===== KUWAIT =====
    ("Mina al-Ahmadi", 29.060, 48.160, 5.0, "Kuwait", "oil"),
    ("Shuwaikh", 29.350, 47.920, 4.0, "Kuwait", "commercial"),
    ("Mina Abdullah", 29.000, 48.180, 4.0, "Kuwait", "oil"),

    # ===== BAHRAIN =====
    ("Khalifa bin Salman", 26.020, 50.590, 4.0, "Bahrain", "commercial"),
    ("Sitra", 26.130, 50.650, 3.0, "Bahrain", "oil"),

    # ===== IRAQ =====
    ("Basra / Al-Faw", 29.980, 48.460, 6.0, "Iraq", "oil"),
    ("ABOT / Al-Bakr", 29.680, 48.800, 5.0, "Iraq", "oil"),
    ("Khor al-Amaya", 29.780, 48.830, 4.0, "Iraq", "oil"),

    # ===== IRAN =====
    ("Kharg Island", 29.230, 50.310, 5.0, "Iran", "oil"),
    ("Bandar Abbas", 27.170, 56.280, 5.0, "Iran", "commercial"),
    ("Assaluyeh", 27.480, 52.610, 5.0, "Iran", "lng"),
    ("Bandar Imam Khomeini", 30.430, 49.070, 5.0, "Iran", "commercial"),
    ("Chabahar", 25.290, 60.620, 4.0, "Iran", "commercial"),
    ("Lavan Island", 26.810, 53.360, 4.0, "Iran", "oil"),
    ("Sirri Island", 25.900, 54.520, 3.0, "Iran", "oil"),

    # ===== OMAN =====
    ("Sohar", 24.370, 56.740, 5.0, "Oman", "commercial"),
    ("Salalah", 16.940, 54.010, 5.0, "Oman", "commercial"),
    ("Mina al-Fahal", 23.630, 58.520, 4.0, "Oman", "oil"),

    # ===== YEMEN =====
    ("Aden", 12.790, 45.020, 5.0, "Yemen", "commercial"),
    ("Hodeidah", 14.790, 42.940, 4.0, "Yemen", "commercial"),
    ("Ash Shihr", 14.760, 49.600, 3.0, "Yemen", "oil"),

    # ===== DJIBOUTI =====
    ("Djibouti", 11.590, 43.140, 4.0, "Djibouti", "commercial"),

    # ===== INDIA =====
    ("Mumbai / JNPT", 18.950, 72.950, 6.0, "India", "commercial"),
    ("Mundra", 22.730, 69.710, 6.0, "India", "commercial"),
    ("Sikka", 22.420, 69.840, 4.0, "India", "oil"),
    ("Vadinar", 22.390, 69.700, 3.0, "India", "oil"),
    ("Kandla", 23.030, 70.210, 4.0, "India", "commercial"),
    ("Paradip", 20.260, 86.670, 5.0, "India", "cargo"),
    ("Visakhapatnam", 17.680, 83.290, 5.0, "India", "commercial"),
    ("Chennai", 13.090, 80.290, 5.0, "India", "commercial"),
    ("Kochi", 9.970, 76.260, 4.0, "India", "commercial"),
    ("Mangalore / NMPT", 12.920, 74.810, 4.0, "India", "commercial"),
    ("Haldia", 22.030, 88.060, 4.0, "India", "cargo"),
    ("Hazira", 21.110, 72.630, 4.0, "India", "lng"),
    ("Dahej", 21.690, 72.590, 3.0, "India", "lng"),
    ("Ennore / Kamarajar", 13.230, 80.330, 4.0, "India", "cargo"),
    ("Tuticorin", 8.770, 78.190, 4.0, "India", "commercial"),
    ("Krishnapatnam", 14.250, 80.120, 4.0, "India", "cargo"),

    # ===== PAKISTAN =====
    ("Karachi", 24.820, 66.980, 5.0, "Pakistan", "commercial"),
    ("Port Qasim", 24.780, 67.350, 5.0, "Pakistan", "commercial"),
    ("Gwadar", 25.130, 62.330, 4.0, "Pakistan", "commercial"),

    # ===== SRI LANKA =====
    ("Colombo", 6.940, 79.850, 5.0, "Sri Lanka", "commercial"),
    ("Hambantota", 6.120, 81.110, 4.0, "Sri Lanka", "commercial"),

    # ===== BANGLADESH =====
    ("Chittagong", 22.310, 91.810, 5.0, "Bangladesh", "commercial"),

    # ===== MYANMAR =====
    ("Yangon / Thilawa", 16.750, 96.250, 5.0, "Myanmar", "commercial"),

    # ===== ERITREA =====
    ("Assab", 13.010, 42.740, 3.0, "Eritrea", "commercial"),

    # ===== SUDAN =====
    ("Port Sudan", 19.610, 37.220, 4.0, "Sudan", "commercial"),

    # ===== JORDAN =====
    ("Aqaba", 29.500, 35.010, 4.0, "Jordan", "commercial"),

    # ============================================================
    #  REGION 4 — EAST ASIA / SE ASIA
    # ============================================================

    # ===== CHINA =====
    ("Shanghai", 31.390, 121.510, 8.0, "China", "commercial"),
    ("Ningbo-Zhoushan", 29.950, 121.830, 8.0, "China", "commercial"),
    ("Guangzhou", 22.880, 113.510, 6.0, "China", "commercial"),
    ("Shenzhen / Yantian", 22.560, 114.270, 5.0, "China", "commercial"),
    ("Qingdao", 36.070, 120.320, 6.0, "China", "commercial"),
    ("Tianjin", 38.980, 117.740, 8.0, "China", "commercial"),
    ("Dalian", 38.930, 121.650, 6.0, "China", "commercial"),
    ("Xiamen", 24.450, 118.080, 5.0, "China", "commercial"),
    ("Zhanjiang", 21.200, 110.400, 4.0, "China", "oil"),
    ("Rizhao", 35.390, 119.550, 4.0, "China", "cargo"),
    ("Tangshan / Caofeidian", 39.230, 118.920, 5.0, "China", "cargo"),
    ("Nanjing", 32.070, 118.720, 5.0, "China", "commercial"),
    ("Dongguan", 22.790, 113.680, 4.0, "China", "commercial"),
    ("Haikou", 20.020, 110.340, 4.0, "China", "commercial"),
    ("Quanzhou", 24.800, 118.630, 4.0, "China", "commercial"),
    ("Zhuhai", 22.130, 113.480, 4.0, "China", "commercial"),
    ("Yangshan", 30.630, 122.070, 6.0, "China", "commercial"),
    ("Dafeng", 33.200, 120.760, 4.0, "China", "lng"),
    ("Yingkou", 40.660, 122.250, 4.0, "China", "cargo"),
    ("Maoming", 21.450, 110.860, 3.0, "China", "oil"),

    # ===== JAPAN =====
    ("Yokohama", 35.450, 139.650, 5.0, "Japan", "commercial"),
    ("Tokyo", 35.630, 139.790, 5.0, "Japan", "commercial"),
    ("Osaka / Kobe", 34.660, 135.210, 6.0, "Japan", "commercial"),
    ("Nagoya", 35.070, 136.870, 5.0, "Japan", "commercial"),
    ("Chiba", 35.570, 140.080, 5.0, "Japan", "oil"),
    ("Kashima", 35.920, 140.690, 4.0, "Japan", "cargo"),
    ("Kiire", 31.380, 130.600, 3.0, "Japan", "oil"),
    ("Mizushima", 34.480, 133.740, 4.0, "Japan", "oil"),
    ("Kawasaki", 35.510, 139.750, 4.0, "Japan", "oil"),
    ("Kitakyushu", 33.900, 130.970, 4.0, "Japan", "commercial"),
    ("Hakata / Fukuoka", 33.610, 130.400, 4.0, "Japan", "commercial"),

    # ===== SOUTH KOREA =====
    ("Busan", 35.080, 129.040, 6.0, "South Korea", "commercial"),
    ("Ulsan", 35.490, 129.370, 5.0, "South Korea", "oil"),
    ("Yeosu / Gwangyang", 34.870, 127.760, 5.0, "South Korea", "commercial"),
    ("Incheon", 37.450, 126.600, 5.0, "South Korea", "commercial"),
    ("Daesan", 36.930, 126.420, 4.0, "South Korea", "oil"),
    ("Pyeongtaek / Dangjin", 36.960, 126.830, 4.0, "South Korea", "commercial"),

    # ===== TAIWAN =====
    ("Kaohsiung", 22.610, 120.280, 5.0, "Taiwan", "commercial"),
    ("Keelung", 25.160, 121.740, 4.0, "Taiwan", "commercial"),
    ("Taichung", 24.280, 120.510, 4.0, "Taiwan", "commercial"),
    ("Mailiao", 23.790, 120.200, 4.0, "Taiwan", "oil"),

    # ===== HONG KONG =====
    ("Hong Kong", 22.280, 114.160, 6.0, "Hong Kong", "commercial"),

    # ===== SINGAPORE =====
    ("Singapore", 1.260, 103.830, 8.0, "Singapore", "commercial"),

    # ===== MALAYSIA =====
    ("Port Klang", 2.990, 101.370, 5.0, "Malaysia", "commercial"),
    ("Tanjung Pelepas", 1.360, 103.550, 5.0, "Malaysia", "commercial"),
    ("Penang", 5.420, 100.350, 4.0, "Malaysia", "commercial"),
    ("Bintulu", 3.180, 113.040, 4.0, "Malaysia", "lng"),
    ("Labuan", 5.280, 115.240, 3.0, "Malaysia", "oil"),
    ("Kerteh", 4.520, 103.440, 3.0, "Malaysia", "oil"),
    ("Kemaman", 4.230, 103.440, 3.0, "Malaysia", "oil"),

    # ===== INDONESIA =====
    ("Tanjung Priok / Jakarta", -6.100, 106.880, 5.0, "Indonesia", "commercial"),
    ("Cilacap", -7.740, 109.010, 4.0, "Indonesia", "oil"),
    ("Balikpapan", -1.270, 116.810, 5.0, "Indonesia", "oil"),
    ("Dumai", 1.680, 101.450, 4.0, "Indonesia", "oil"),
    ("Belawan / Medan", 3.790, 98.690, 4.0, "Indonesia", "commercial"),
    ("Bontang", 0.100, 117.490, 4.0, "Indonesia", "lng"),
    ("Merak", -5.930, 106.000, 3.0, "Indonesia", "cargo"),
    ("Surabaya / Tanjung Perak", -7.200, 112.740, 5.0, "Indonesia", "commercial"),

    # ===== THAILAND =====
    ("Laem Chabang", 13.080, 100.880, 5.0, "Thailand", "commercial"),
    ("Map Ta Phut", 12.720, 101.180, 4.0, "Thailand", "oil"),
    ("Bangkok / Klong Toey", 13.700, 100.580, 5.0, "Thailand", "commercial"),
    ("Si Racha", 13.160, 100.920, 3.0, "Thailand", "oil"),

    # ===== VIETNAM =====
    ("Ho Chi Minh City / Cat Lai", 10.760, 106.770, 5.0, "Vietnam", "commercial"),
    ("Hai Phong", 20.860, 106.680, 5.0, "Vietnam", "commercial"),
    ("Vung Tau", 10.340, 107.090, 4.0, "Vietnam", "oil"),
    ("Da Nang", 16.080, 108.220, 4.0, "Vietnam", "commercial"),

    # ===== PHILIPPINES =====
    ("Manila", 14.580, 120.960, 5.0, "Philippines", "commercial"),
    ("Batangas", 13.730, 121.050, 4.0, "Philippines", "commercial"),
    ("Cebu", 10.300, 123.900, 4.0, "Philippines", "commercial"),
    ("Subic Bay", 14.790, 120.280, 4.0, "Philippines", "commercial"),

    # ===== CAMBODIA =====
    ("Sihanoukville", 10.630, 103.500, 4.0, "Cambodia", "commercial"),

    # ===== BRUNEI =====
    ("Muara", 5.020, 115.080, 3.0, "Brunei", "commercial"),
    ("Lumut (Brunei)", 4.620, 114.440, 3.0, "Brunei", "lng"),

    # ============================================================
    #  REGION 5 — OCEANIA
    # ============================================================

    # ===== AUSTRALIA =====
    ("Port Hedland", -20.310, 118.580, 6.0, "Australia", "cargo"),
    ("Dampier", -20.660, 116.710, 5.0, "Australia", "cargo"),
    ("Gladstone", -23.850, 151.270, 5.0, "Australia", "lng"),
    ("Newcastle (AU)", -32.920, 151.790, 5.0, "Australia", "cargo"),
    ("Melbourne", -37.840, 144.920, 6.0, "Australia", "commercial"),
    ("Sydney", -33.860, 151.190, 5.0, "Australia", "commercial"),
    ("Fremantle", -32.050, 115.740, 5.0, "Australia", "commercial"),
    ("Hay Point / Dalrymple Bay", -21.280, 149.280, 5.0, "Australia", "cargo"),
    ("Brisbane", -27.380, 153.170, 5.0, "Australia", "commercial"),
    ("Darwin", -12.440, 130.850, 5.0, "Australia", "commercial"),
    ("Whyalla", -33.020, 137.530, 4.0, "Australia", "cargo"),
    ("Geelong", -38.130, 144.360, 4.0, "Australia", "oil"),
    ("Kwinana", -32.230, 115.770, 4.0, "Australia", "oil"),
    ("Abbot Point", -19.890, 148.090, 4.0, "Australia", "cargo"),
    ("Barrow Island", -20.810, 115.440, 4.0, "Australia", "lng"),
    ("Weipa", -12.680, 141.870, 4.0, "Australia", "cargo"),
    ("Port Adelaide", -34.780, 138.520, 4.0, "Australia", "commercial"),
    ("Bonython / Port Bonython", -32.980, 137.770, 3.0, "Australia", "lng"),

    # ===== NEW ZEALAND =====
    ("Tauranga", -37.640, 176.190, 4.0, "New Zealand", "commercial"),
    ("Auckland", -36.840, 174.760, 5.0, "New Zealand", "commercial"),
    ("Lyttelton", -43.610, 172.720, 3.0, "New Zealand", "commercial"),
    ("Marsden Point", -35.830, 174.500, 3.0, "New Zealand", "oil"),

    # ===== PAPUA NEW GUINEA =====
    ("Lae", -6.730, 147.000, 4.0, "Papua New Guinea", "cargo"),

    # ============================================================
    #  REGION 6 — WEST / SOUTHERN AFRICA
    # ============================================================

    # ===== SOUTH AFRICA =====
    ("Durban", -29.870, 31.040, 5.0, "South Africa", "commercial"),
    ("Richards Bay", -28.790, 32.090, 5.0, "South Africa", "cargo"),
    ("Cape Town", -33.910, 18.440, 5.0, "South Africa", "commercial"),
    ("Saldanha Bay", -33.020, 17.930, 5.0, "South Africa", "cargo"),
    ("Port Elizabeth / Gqeberha", -33.770, 25.640, 4.0, "South Africa", "commercial"),
    ("Mossel Bay", -34.180, 22.150, 3.0, "South Africa", "oil"),

    # ===== NIGERIA =====
    ("Lagos / Apapa", 6.440, 3.380, 5.0, "Nigeria", "commercial"),
    ("Bonny", 4.430, 7.170, 4.0, "Nigeria", "oil"),
    ("Qua Iboe", 4.530, 8.020, 4.0, "Nigeria", "oil"),
    ("Brass", 4.310, 6.240, 4.0, "Nigeria", "oil"),
    ("Forcados", 5.350, 5.360, 3.0, "Nigeria", "oil"),

    # ===== GHANA =====
    ("Tema", 5.630, 0.010, 4.0, "Ghana", "commercial"),
    ("Takoradi", 4.890, -1.740, 3.0, "Ghana", "commercial"),

    # ===== CÔTE D'IVOIRE =====
    ("Abidjan", 5.290, -4.020, 5.0, "Côte d'Ivoire", "commercial"),

    # ===== SENEGAL =====
    ("Dakar", 14.680, -17.430, 5.0, "Senegal", "commercial"),

    # ===== CAMEROON =====
    ("Douala", 4.040, 9.710, 4.0, "Cameroon", "commercial"),
    ("Kribi", 2.950, 9.900, 3.0, "Cameroon", "oil"),

    # ===== ANGOLA =====
    ("Luanda", -8.800, 13.250, 5.0, "Angola", "commercial"),
    ("Lobito", -12.340, 13.560, 3.0, "Angola", "commercial"),
    ("Soyo", -6.130, 12.350, 3.0, "Angola", "oil"),

    # ===== REPUBLIC OF CONGO =====
    ("Pointe-Noire", -4.780, 11.850, 4.0, "Republic of Congo", "oil"),

    # ===== EQUATORIAL GUINEA =====
    ("Malabo", 3.750, 8.780, 3.0, "Equatorial Guinea", "oil"),
    ("Bata", 1.860, 9.770, 3.0, "Equatorial Guinea", "commercial"),
    ("Punta Europa", 3.770, 8.710, 3.0, "Equatorial Guinea", "lng"),

    # ===== GABON =====
    ("Port-Gentil", -0.720, 8.780, 4.0, "Gabon", "oil"),
    ("Libreville / Owendo", 0.290, 9.490, 4.0, "Gabon", "commercial"),

    # ===== NAMIBIA =====
    ("Walvis Bay", -22.960, 14.510, 4.0, "Namibia", "commercial"),

    # ===== MAURITANIA =====
    ("Nouadhibou", 20.920, -17.050, 4.0, "Mauritania", "cargo"),

    # ===== TOGO =====
    ("Lomé", 6.140, 1.290, 4.0, "Togo", "commercial"),

    # ===== BENIN =====
    ("Cotonou", 6.350, 2.430, 4.0, "Benin", "commercial"),

    # ===== KENYA =====
    ("Mombasa", -4.040, 39.660, 5.0, "Kenya", "commercial"),

    # ===== TANZANIA =====
    ("Dar es Salaam", -6.830, 39.290, 5.0, "Tanzania", "commercial"),

    # ===== MOZAMBIQUE =====
    ("Maputo", -25.960, 32.590, 4.0, "Mozambique", "commercial"),
    ("Beira", -19.830, 34.870, 4.0, "Mozambique", "commercial"),

    # ===== MADAGASCAR =====
    ("Toamasina", -18.150, 49.410, 4.0, "Madagascar", "commercial"),

    # ===== MAURITIUS =====
    ("Port Louis", -20.160, 57.500, 4.0, "Mauritius", "commercial"),
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

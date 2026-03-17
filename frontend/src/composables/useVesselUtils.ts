import type { VesselProperties, VesselCategory } from '@/types/vessel'

const VESSEL_COLORS: Record<VesselCategory, string> = {
  tanker: '#e67e22',
  cargo: '#1a73e8',
  passenger: '#2ecc71',
  tug: '#9b59b6',
  fishing: '#16a085',
  other: '#95a5a6',
}

export function classifyVessel(p: VesselProperties): VesselCategory {
  const t = (p.vessel_type || '').toLowerCase()
  if (t.includes('tanker')) return 'tanker'
  if (t.includes('cargo') || t.includes('container')) return 'cargo'
  if (t.includes('passenger')) return 'passenger'
  if (t.includes('tug') || t.includes('tow') || t.includes('pilot')) return 'tug'
  if (t.includes('fish')) return 'fishing'
  return 'other'
}

export function getVesselColor(p: VesselProperties): string {
  if (p.is_russian) return '#d32f2f'
  return VESSEL_COLORS[classifyVessel(p)]
}

export function isStaleAIS(p: VesselProperties): boolean {
  if (!p.timestamp) return false
  return (Date.now() - new Date(p.timestamp).getTime()) / 3600000 >= 6
}

export function formatAgo(ts: string): string {
  const h = (Date.now() - new Date(ts).getTime()) / 3600000
  if (h < 1) return `${Math.round(h * 60)}m ago`
  if (h < 24) return `${Math.round(h)}h ago`
  return `${Math.round(h / 24)}d ago`
}

export function formatTime(ts: string): string {
  const d = new Date(ts)
  return (
    d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }) +
    ' ' +
    d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })
  )
}

export function formatDuration(h: number | null | undefined): string {
  if (h == null) return '--'
  if (h < 1) return `${Math.round(h * 60)}m`
  if (h < 24) return `${h.toFixed(1)}h`
  return `${(h / 24).toFixed(1)}d`
}

export function formatNumber(n: number | null | undefined): string {
  if (n == null) return '--'
  return Number(n).toLocaleString()
}

// Maritime Identification Digits → country flag emoji + code
const MID_FLAGS: Record<string, [string, string]> = {
  '201': ['🇦🇱', 'AL'], '202': ['🇦🇩', 'AD'], '203': ['🇦🇹', 'AT'], '204': ['🇵🇹', 'PT'], '205': ['🇧🇪', 'BE'],
  '206': ['🇧🇾', 'BY'], '207': ['🇧🇬', 'BG'], '208': ['🇻🇦', 'VA'], '209': ['🇨🇾', 'CY'], '210': ['🇨🇾', 'CY'],
  '211': ['🇩🇪', 'DE'], '212': ['🇨🇾', 'CY'], '213': ['🇬🇪', 'GE'], '214': ['🇲🇩', 'MD'], '215': ['🇲🇹', 'MT'],
  '216': ['🇦🇲', 'AM'], '218': ['🇩🇪', 'DE'], '219': ['🇩🇰', 'DK'], '220': ['🇩🇰', 'DK'], '224': ['🇪🇸', 'ES'],
  '225': ['🇪🇸', 'ES'], '226': ['🇫🇷', 'FR'], '227': ['🇫🇷', 'FR'], '228': ['🇫🇷', 'FR'], '229': ['🇲🇹', 'MT'],
  '230': ['🇫🇮', 'FI'], '231': ['🇫🇴', 'FO'], '232': ['🇬🇧', 'GB'], '233': ['🇬🇧', 'GB'], '234': ['🇬🇧', 'GB'],
  '235': ['🇬🇧', 'GB'], '236': ['🇬🇮', 'GI'], '237': ['🇬🇷', 'GR'], '238': ['🇭🇷', 'HR'], '239': ['🇬🇷', 'GR'],
  '240': ['🇬🇷', 'GR'], '241': ['🇬🇷', 'GR'], '242': ['🇲🇦', 'MA'], '243': ['🇭🇺', 'HU'], '244': ['🇳🇱', 'NL'],
  '245': ['🇳🇱', 'NL'], '246': ['🇳🇱', 'NL'], '247': ['🇮🇹', 'IT'], '248': ['🇲🇹', 'MT'], '249': ['🇲🇹', 'MT'],
  '250': ['🇮🇪', 'IE'], '251': ['🇮🇸', 'IS'], '252': ['🇱🇮', 'LI'], '253': ['🇱🇺', 'LU'], '254': ['🇲🇨', 'MC'],
  '255': ['🇵🇹', 'PT'], '256': ['🇲🇹', 'MT'], '257': ['🇳🇴', 'NO'], '258': ['🇳🇴', 'NO'], '259': ['🇳🇴', 'NO'],
  '261': ['🇵🇱', 'PL'], '263': ['🇵🇹', 'PT'], '264': ['🇷🇴', 'RO'], '265': ['🇸🇪', 'SE'], '266': ['🇸🇪', 'SE'],
  '267': ['🇸🇰', 'SK'], '268': ['🇸🇲', 'SM'], '269': ['🇨🇭', 'CH'], '270': ['🇨🇿', 'CZ'], '271': ['🇹🇷', 'TR'],
  '272': ['🇺🇦', 'UA'], '273': ['🇷🇺', 'RU'], '274': ['🇲🇰', 'MK'], '275': ['🇱🇻', 'LV'], '276': ['🇪🇪', 'EE'],
  '277': ['🇱🇹', 'LT'], '278': ['🇸🇮', 'SI'], '279': ['🇷🇸', 'RS'],
  '301': ['🇦🇮', 'AI'], '303': ['🇺🇸', 'US'], '304': ['🇦🇬', 'AG'], '305': ['🇦🇬', 'AG'],
  '306': ['🇨🇼', 'CW'], '307': ['🇦🇼', 'AW'], '308': ['🇧🇸', 'BS'], '309': ['🇧🇸', 'BS'],
  '310': ['🇧🇲', 'BM'], '311': ['🇧🇸', 'BS'], '312': ['🇧🇿', 'BZ'], '314': ['🇧🇧', 'BB'],
  '316': ['🇨🇦', 'CA'], '319': ['🇰🇾', 'KY'], '321': ['🇨🇷', 'CR'], '323': ['🇨🇺', 'CU'],
  '325': ['🇩🇲', 'DM'], '327': ['🇩🇴', 'DO'], '329': ['🇬🇵', 'GP'], '330': ['🇬🇩', 'GD'],
  '331': ['🇬🇱', 'GL'], '332': ['🇬🇹', 'GT'], '334': ['🇭🇳', 'HN'], '336': ['🇭🇹', 'HT'],
  '338': ['🇺🇸', 'US'], '339': ['🇯🇲', 'JM'], '341': ['🇰🇳', 'KN'], '343': ['🇱🇨', 'LC'],
  '345': ['🇲🇽', 'MX'], '347': ['🇲🇶', 'MQ'], '348': ['🇳🇮', 'NI'], '350': ['🇵🇦', 'PA'],
  '351': ['🇵🇦', 'PA'], '352': ['🇵🇦', 'PA'], '353': ['🇵🇦', 'PA'], '354': ['🇵🇦', 'PA'],
  '355': ['🇵🇦', 'PA'], '356': ['🇵🇦', 'PA'], '357': ['🇵🇦', 'PA'],
  '358': ['🇵🇷', 'PR'], '359': ['🇸🇻', 'SV'], '361': ['🇵🇲', 'PM'],
  '362': ['🇹🇹', 'TT'], '364': ['🇹🇨', 'TC'], '366': ['🇺🇸', 'US'], '367': ['🇺🇸', 'US'],
  '368': ['🇺🇸', 'US'], '369': ['🇺🇸', 'US'], '370': ['🇵🇦', 'PA'],
  '371': ['🇵🇦', 'PA'], '372': ['🇵🇦', 'PA'], '373': ['🇵🇦', 'PA'], '374': ['🇵🇦', 'PA'],
  '375': ['🇻🇨', 'VC'], '376': ['🇻🇬', 'VG'], '377': ['🇻🇮', 'VI'],
  '378': ['🇻🇪', 'VE'],
  '401': ['🇦🇫', 'AF'], '403': ['🇸🇦', 'SA'], '405': ['🇧🇩', 'BD'], '408': ['🇧🇭', 'BH'],
  '410': ['🇧🇹', 'BT'], '412': ['🇨🇳', 'CN'], '413': ['🇨🇳', 'CN'], '414': ['🇨🇳', 'CN'],
  '416': ['🇹🇼', 'TW'], '417': ['🇱🇰', 'LK'], '419': ['🇮🇳', 'IN'],
  '422': ['🇮🇷', 'IR'], '423': ['🇦🇿', 'AZ'], '425': ['🇮🇶', 'IQ'],
  '428': ['🇮🇱', 'IL'], '431': ['🇯🇵', 'JP'], '432': ['🇯🇵', 'JP'],
  '434': ['🇹🇲', 'TM'], '436': ['🇰🇿', 'KZ'], '437': ['🇺🇿', 'UZ'],
  '438': ['🇯🇴', 'JO'], '440': ['🇰🇷', 'KR'], '441': ['🇰🇷', 'KR'],
  '443': ['🇵🇸', 'PS'], '445': ['🇰🇵', 'KP'], '447': ['🇰🇼', 'KW'],
  '450': ['🇱🇧', 'LB'], '451': ['🇰🇬', 'KG'], '453': ['🇲🇴', 'MO'],
  '455': ['🇲🇻', 'MV'], '457': ['🇲🇳', 'MN'], '459': ['🇳🇵', 'NP'],
  '461': ['🇴🇲', 'OM'], '463': ['🇵🇰', 'PK'], '466': ['🇶🇦', 'QA'],
  '468': ['🇸🇾', 'SY'], '470': ['🇦🇪', 'AE'], '471': ['🇦🇪', 'AE'],
  '472': ['🇹🇯', 'TJ'], '473': ['🇾🇪', 'YE'], '475': ['🇾🇪', 'YE'],
  '477': ['🇭🇰', 'HK'],
  '501': ['🇫🇷', 'FR'], '503': ['🇦🇺', 'AU'], '506': ['🇲🇲', 'MM'],
  '508': ['🇧🇳', 'BN'], '510': ['🇫🇲', 'FM'], '511': ['🇵🇼', 'PW'],
  '512': ['🇳🇿', 'NZ'], '514': ['🇰🇭', 'KH'], '515': ['🇰🇭', 'KH'],
  '516': ['🇨🇽', 'CX'], '518': ['🇨🇰', 'CK'], '520': ['🇫🇯', 'FJ'],
  '523': ['🇨🇨', 'CC'], '525': ['🇮🇩', 'ID'], '529': ['🇰🇮', 'KI'],
  '531': ['🇱🇦', 'LA'], '533': ['🇲🇾', 'MY'], '536': ['🇲🇵', 'MP'],
  '538': ['🇲🇭', 'MH'], '540': ['🇳🇨', 'NC'], '542': ['🇳🇺', 'NU'],
  '544': ['🇳🇷', 'NR'], '546': ['🇵🇫', 'PF'], '548': ['🇵🇭', 'PH'],
  '553': ['🇵🇬', 'PG'], '555': ['🇵🇳', 'PN'], '557': ['🇸🇧', 'SB'],
  '559': ['🇦🇸', 'AS'], '561': ['🇼🇸', 'WS'], '563': ['🇸🇬', 'SG'],
  '564': ['🇸🇬', 'SG'], '565': ['🇸🇬', 'SG'], '566': ['🇸🇬', 'SG'],
  '567': ['🇹🇭', 'TH'], '570': ['🇹🇴', 'TO'], '572': ['🇹🇻', 'TV'],
  '574': ['🇻🇳', 'VN'], '576': ['🇻🇺', 'VU'], '577': ['🇻🇺', 'VU'],
  '578': ['🇼🇫', 'WF'],
  '601': ['🇿🇦', 'ZA'], '603': ['🇦🇴', 'AO'], '605': ['🇩🇿', 'DZ'],
  '607': ['🇫🇷', 'FR'], '608': ['🇬🇧', 'GB'], '609': ['🇧🇮', 'BI'],
  '610': ['🇧🇯', 'BJ'], '611': ['🇧🇼', 'BW'], '612': ['🇨🇫', 'CF'],
  '613': ['🇨🇲', 'CM'], '615': ['🇨🇬', 'CG'], '616': ['🇰🇲', 'KM'],
  '617': ['🇨🇻', 'CV'], '618': ['🇫🇷', 'FR'], '619': ['🇨🇮', 'CI'],
  '620': ['🇰🇲', 'KM'], '621': ['🇩🇯', 'DJ'], '622': ['🇪🇬', 'EG'],
  '624': ['🇪🇹', 'ET'], '625': ['🇪🇷', 'ER'], '626': ['🇬🇦', 'GA'],
  '627': ['🇬🇭', 'GH'], '629': ['🇬🇲', 'GM'], '630': ['🇬🇼', 'GW'],
  '631': ['🇬🇶', 'GQ'], '632': ['🇬🇳', 'GN'], '633': ['🇧🇫', 'BF'],
  '634': ['🇰🇪', 'KE'], '635': ['🇱🇷', 'LR'], '636': ['🇱🇷', 'LR'],
  '637': ['🇱🇷', 'LR'], '642': ['🇱🇾', 'LY'], '644': ['🇱🇸', 'LS'],
  '645': ['🇲🇺', 'MU'], '647': ['🇲🇬', 'MG'], '649': ['🇲🇱', 'ML'],
  '650': ['🇲🇿', 'MZ'], '654': ['🇲🇷', 'MR'], '655': ['🇲🇼', 'MW'],
  '656': ['🇳🇪', 'NE'], '657': ['🇳🇬', 'NG'], '659': ['🇳🇦', 'NA'],
  '660': ['🇷🇪', 'RE'], '661': ['🇷🇼', 'RW'], '662': ['🇸🇩', 'SD'],
  '663': ['🇸🇳', 'SN'], '664': ['🇸🇨', 'SC'], '665': ['🇸🇱', 'SL'],
  '666': ['🇸🇴', 'SO'], '667': ['🇸🇱', 'SL'], '668': ['🇸🇿', 'SZ'],
  '669': ['🇹🇩', 'TD'], '670': ['🇹🇬', 'TG'], '671': ['🇹🇳', 'TN'],
  '672': ['🇹🇿', 'TZ'], '674': ['🇺🇬', 'UG'], '675': ['🇨🇩', 'CD'],
  '676': ['🇹🇿', 'TZ'], '677': ['🇹🇿', 'TZ'], '678': ['🇿🇲', 'ZM'],
  '679': ['🇿🇼', 'ZW'],
}

export function mmsiToFlag(mmsi: number): [string, string] {
  const mid = String(mmsi).substring(0, 3)
  return MID_FLAGS[mid] || ['🏴', '??']
}

export function coordsToRegion(lat: number, lon: number): string {
  // Persian Gulf / Arabian Sea
  if (lat >= 22 && lat <= 32 && lon >= 44 && lon <= 60) return 'Persian Gulf'
  if (lat >= 12 && lat <= 22 && lon >= 38 && lon <= 60) return 'Arabian Sea'
  if (lat >= 10 && lat <= 22 && lon >= 60 && lon <= 78) return 'W Indian Ocean'

  // Red Sea / Suez
  if (lat >= 12 && lat <= 30 && lon >= 32 && lon <= 44) return 'Red Sea'

  // Mediterranean
  if (lat >= 30 && lat <= 46 && lon >= -6 && lon <= 36) return 'Mediterranean'

  // Black Sea / Turkish Straits
  if (lat >= 40 && lat <= 47 && lon >= 27 && lon <= 42) return 'Black Sea'

  // Baltic
  if (lat >= 53 && lat <= 66 && lon >= 9 && lon <= 31) return 'Baltic Sea'

  // North Sea / English Channel
  if (lat >= 48 && lat <= 62 && lon >= -5 && lon <= 9) return 'North Sea'

  // Norwegian Sea
  if (lat >= 62 && lat <= 72 && lon >= -5 && lon <= 20) return 'Norwegian Sea'

  // Strait of Malacca / Singapore
  if (lat >= -2 && lat <= 8 && lon >= 95 && lon <= 108) return 'Strait of Malacca'
  if (lat >= 0 && lat <= 2 && lon >= 103 && lon <= 105) return 'Near Singapore'

  // South China Sea
  if (lat >= 0 && lat <= 23 && lon >= 105 && lon <= 121) return 'South China Sea'

  // East China Sea / Yellow Sea
  if (lat >= 23 && lat <= 41 && lon >= 117 && lon <= 132) return 'East China Sea'

  // Sea of Japan
  if (lat >= 33 && lat <= 52 && lon >= 127 && lon <= 142) return 'Sea of Japan'

  // Indian Ocean
  if (lat >= -40 && lat <= 10 && lon >= 40 && lon <= 100) return 'Indian Ocean'

  // West Africa
  if (lat >= -10 && lat <= 20 && lon >= -25 && lon <= 15) return 'West Africa'

  // East Africa
  if (lat >= -30 && lat <= 10 && lon >= 30 && lon <= 55) return 'East Africa'

  // South Africa
  if (lat >= -40 && lat <= -25 && lon >= 10 && lon <= 40) return 'Southern Africa'

  // Bay of Bengal
  if (lat >= 5 && lat <= 23 && lon >= 78 && lon <= 95) return 'Bay of Bengal'

  // SE Asia / Indonesia
  if (lat >= -12 && lat <= 8 && lon >= 95 && lon <= 145) return 'SE Asia'

  // Australia / Oceania
  if (lat >= -50 && lat <= -10 && lon >= 110 && lon <= 180) return 'Oceania'

  // NW Pacific
  if (lat >= 0 && lat <= 55 && lon >= 132 && lon <= 180) return 'NW Pacific'

  // Atlantic
  if (lat >= 35 && lat <= 65 && lon >= -40 && lon <= -5) return 'N Atlantic'

  return `${lat.toFixed(1)}°${lat >= 0 ? 'N' : 'S'}, ${lon.toFixed(1)}°${lon >= 0 ? 'E' : 'W'}`
}

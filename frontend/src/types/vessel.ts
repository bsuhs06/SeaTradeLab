export interface VesselProperties {
  mmsi: number
  name?: string
  vessel_type?: string
  call_sign?: string
  imo?: number
  draught?: number
  sog?: number
  cog?: number
  heading?: number
  nav_status?: string
  destination?: string
  sources?: string
  timestamp: string
  is_russian: boolean
  gap_hours?: number
}

export interface VesselFeature {
  type: 'Feature'
  geometry: {
    type: 'Point'
    coordinates: [number, number] // [lng, lat]
  }
  properties: VesselProperties
}

export interface VesselFeatureCollection {
  type: 'FeatureCollection'
  features: VesselFeature[]
}

export interface VesselTrack {
  mmsi: number
  hours: number
  point_count: number
  track: {
    type: 'LineString'
    coordinates: [number, number][]
  }
  timestamps: string[]
  speeds: (number | null)[]
  vessel?: VesselDetail
}

export interface VesselDetail {
  mmsi: number
  imo_number?: number
  name?: string
  call_sign?: string
  vessel_type?: number
  vessel_type_name?: string
  draught?: number
  destination?: string
  first_seen_at: string
  last_seen_at: string
}

export interface Stats {
  total_vessels: number
  russian_vessels: number
  total_positions: number
  last_collected_at?: string
}

export interface STSEvent {
  id: number
  mmsi_a: number
  mmsi_b: number
  name_a?: string
  name_b?: string
  type_a?: string
  type_b?: string
  start_time: string
  end_time: string
  duration_minutes: number
  min_distance_m?: number
  avg_lat?: number
  avg_lon?: number
  lat_a?: number
  lon_a?: number
  lat_b?: number
  lon_b?: number
  confidence: 'high' | 'medium' | 'low'
  reviewed: boolean
  tag?: string
  notes?: string
}

export interface STSResponse {
  events: STSEvent[]
  count: number
  total_count: number
  hours: number
}

export interface DarkVesselResponse {
  type: 'FeatureCollection'
  features: VesselFeature[]
  count: number
  total_count: number
}

export interface PortVisit {
  mmsi: number
  vessel_name?: string
  vessel_type?: string
  flag_country?: string
  is_russian: boolean
  port_name: string
  port_lat?: number
  port_lon?: number
  arrival_time: string
  departure_time?: string
  duration_hours?: number
  still_in_port: boolean
}

export interface PortVisitResponse {
  visits: PortVisit[]
  count: number
  total_count: number
  hours: number
}

export interface Port {
  name: string
  country: string
  port_type: string
  latitude: number
  longitude: number
  radius_km: number
  source: 'builtin' | 'custom'
  override_id?: number
}

export interface TimeRange {
  min: string
  max: string
}

export interface TrailsMap {
  [mmsi: string]: [number, number, number][]  // [lng, lat, unix_timestamp]
}

export type VesselCategory = 'tanker' | 'cargo' | 'passenger' | 'tug' | 'fishing' | 'other'

export interface AnalyticsRequest {
  task: string
  hours: number
  distance: number
  speed: number
  min_duration: number
  gap_hours: number
}

export interface AnalyticsStatus {
  id?: string
  status: 'running' | 'completed' | 'failed'
  started_at?: string
  output?: string
  args?: string
}

export interface CollectorStatus {
  managed_running: boolean
  managed_pid?: number
  external_pids?: number[]
  last_collected?: string
  binary_found: boolean
  log?: string
}

export interface SpoofedVessel {
  mmsi: number
  name?: string
  vessel_type?: string
  reason: 'teleport' | 'impossible_speed' | 'suspicious_speed'
  lat_from: number
  lon_from: number
  lat_to: number
  lon_to: number
  speed_knots: number
  distance_km: number
  time_delta_s: number
  timestamp_1: string
  timestamp_2: string
}

export interface SpoofedResponse {
  vessels: SpoofedVessel[]
  count: number
  hours: number
}

export interface VesselRegistryEntry {
  mmsi: number
  imo_number?: number
  name?: string
  call_sign?: string
  vessel_type?: number
  vessel_type_name?: string
  draught?: number
  destination?: string
  first_seen_at: string
  last_seen_at: string
  change_count: number
  tags?: string
}

export interface VesselRegistryResponse {
  vessels: VesselRegistryEntry[]
  count: number
}

export interface VesselHistoryRecord {
  id: number
  mmsi: number
  field_name: string
  old_value?: string
  new_value?: string
  changed_at: string
}

export interface VesselNote {
  id: number
  mmsi: number
  tag: string
  note?: string
  created_at: string
  updated_at: string
}

export interface VesselRegistryDetail {
  vessel: VesselDetail
  history: VesselHistoryRecord[]
  notes: VesselNote[]
}

export interface VesselChangesResponse {
  changes: VesselHistoryRecord[]
  count: number
}

// ========== Vessel Taint Tracking ==========

export interface VesselPortCall {
  id: number
  mmsi: number
  vessel_name?: string
  vessel_type?: string
  flag_country?: string
  port_name: string
  port_country?: string
  port_lat?: number
  port_lon?: number
  arrival_time: string
  departure_time?: string
  duration_hours?: number
  still_in_port: boolean
}

export interface VesselEncounter {
  id: number
  mmsi_a: number
  mmsi_b: number
  name_a?: string
  name_b?: string
  type_a?: string
  type_b?: string
  start_time: string
  end_time: string
  duration_minutes: number
  min_distance_m?: number
  avg_lat?: number
  avg_lon?: number
  max_sog_a?: number
  max_sog_b?: number
}

export interface VesselTaintRecord {
  id: number
  mmsi: number
  vessel_name?: string
  taint_type: 'russian_port' | 'encounter' | 'no_subsequent_port'
  reason?: string
  source_mmsi?: number
  source_name?: string
  source_taint_id?: number
  port_call_id?: number
  encounter_id?: number
  tainted_at: string
  expires_at: string
  active: boolean
}

export interface TaintChainLink {
  taint: VesselTaintRecord
  port_call?: VesselPortCall
  encounter?: VesselEncounter
}

export interface TaintedVesselsResponse {
  tainted: VesselTaintRecord[]
  count: number
  total_count: number
}

export interface VesselTaintDetailResponse {
  mmsi: number
  taint: VesselTaintRecord[]
  port_calls: VesselPortCall[]
  encounters: VesselEncounter[]
}

export interface TaintChainResponse {
  chain: TaintChainLink[]
  count: number
}

// ========== Vessel Favorites ==========

export interface VesselFavorite {
  id: number
  mmsi: number
  vessel_name?: string
  vessel_type?: string
  notes?: string
  created_at: string
  latitude?: number
  longitude?: number
  speed_over_ground?: number
  heading?: number
  destination?: string
  last_seen?: string
  flag_country?: string
}

export interface FavoritesResponse {
  favorites: VesselFavorite[]
  count: number
}

// ========== Destination Anomalies ==========

export interface DestinationAnomaly {
  mmsi: number
  name?: string
  vessel_type_name?: string
  destination: string
  reason: 'message_keywords' | 'long_multi_word' | 'multi_word_message' | 'frequent_changes' | 'unusual_format'
  last_seen_at: string
  change_count: number
  latitude: number
  longitude: number
}

export interface DestinationChange {
  id: number
  mmsi: number
  name?: string
  old_value?: string
  new_value?: string
  changed_at: string
}

export interface DestinationAnomaliesResponse {
  anomalies: DestinationAnomaly[]
  changes: DestinationChange[]
  anomaly_count: number
  change_count: number
  hours: number
}

export interface DestinationChangesResponse {
  changes: DestinationChange[]
  count: number
  mmsi: number
}

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
  hours: number
}

export interface DarkVesselResponse {
  type: 'FeatureCollection'
  features: VesselFeature[]
  count: number
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
  [mmsi: string]: [number, number][]
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

import type {
  VesselFeatureCollection,
  VesselTrack,
  Stats,
  STSResponse,
  DarkVesselResponse,
  PortVisitResponse,
  Port,
  TimeRange,
  TrailsMap,
  AnalyticsRequest,
  AnalyticsStatus,
  CollectorStatus,
} from '@/types/vessel'

const BASE = '/api'

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

async function post<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

async function del<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

export const api = {
  getVessels: () => get<VesselFeatureCollection>('/vessels'),

  getVesselTrack: (mmsi: number, hours = 168) =>
    get<VesselTrack>(`/vessels/${mmsi}/track?hours=${hours}`),

  getStats: () => get<Stats>('/stats'),

  getTrails: (south: number, west: number, north: number, east: number, hours = 24) =>
    get<TrailsMap>(`/trails?south=${south}&west=${west}&north=${north}&east=${east}&hours=${hours}`),

  getSTSEvents: (hours = 168, limit = 100) =>
    get<STSResponse>(`/sts-events?hours=${hours}&limit=${limit}`),

  searchVessels: (q: string, limit = 20) =>
    get<VesselFeatureCollection>(`/search?q=${encodeURIComponent(q)}&limit=${limit}`),

  getDarkVessels: (minHours = 6) =>
    get<DarkVesselResponse>(`/dark-vessels?min_hours=${minHours}`),

  getHistorical: (time: string) =>
    get<VesselFeatureCollection>(`/historical?time=${encodeURIComponent(time)}`),

  getTimeRange: () => get<TimeRange>('/time-range'),

  getPortVisits: (hours = 720, nonRussian = false, limit = 200) => {
    let url = `/port-visits?hours=${hours}&limit=${limit}`
    if (nonRussian) url += '&non_russian=true'
    return get<PortVisitResponse>(url)
  },

  getPorts: () => get<Port[]>('/ports'),

  addPort: (port: { name: string; latitude: number; longitude: number; radius_km: number; country: string; port_type: string }) =>
    post<unknown>('/ports/override', { ...port, action: 'add' }),

  removePort: (id: number) => del<unknown>(`/ports/override?id=${id}`),

  excludeBuiltinPort: (name: string) =>
    post<unknown>('/ports/override', { name, action: 'remove' }),

  runAnalytics: (params: AnalyticsRequest) =>
    post<{ message: string; run: AnalyticsStatus }>('/run-analytics', params),

  getAnalyticsStatus: () => get<AnalyticsStatus>('/analytics-status'),

  getCollectorStatus: () => get<CollectorStatus>('/collector-status'),

  startCollector: () => post<{ message?: string; error?: string }>('/collector-start', {}),

  stopCollector: () => post<{ message?: string }>('/collector-stop', {}),

  purgeOldData: (days: number) =>
    post<{ deleted: number }>('/purge', { days }),
}

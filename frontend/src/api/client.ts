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
  SpoofedResponse,
  VesselRegistryResponse,
  VesselRegistryDetail,
  VesselChangesResponse,
  TaintedVesselsResponse,
  VesselTaintDetailResponse,
  TaintChainResponse,
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

async function patch<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

export const api = {
  getVessels: (bbox?: { south: number; west: number; north: number; east: number }) => {
    let url = '/vessels'
    if (bbox) url += `?south=${bbox.south}&west=${bbox.west}&north=${bbox.north}&east=${bbox.east}`
    return get<VesselFeatureCollection>(url)
  },

  getVesselTrack: (mmsi: number, hours = 168) =>
    get<VesselTrack>(`/vessels/${mmsi}/track?hours=${hours}`),

  getStats: () => get<Stats>('/stats'),

  getTrails: (south: number, west: number, north: number, east: number, hours = 24) =>
    get<TrailsMap>(`/trails?south=${south}&west=${west}&north=${north}&east=${east}&hours=${hours}`),

  getSTSEvents: (hours = 168, limit = 100000) =>
    get<STSResponse>(`/sts-events?hours=${hours}&limit=${limit}`),

  updateSTSEvent: (id: number, data: { confidence: string; reviewed: boolean; tag?: string | null; notes?: string | null }) =>
    patch<{ status: string }>(`/sts-events/${id}`, data),

  getSpoofedVessels: (hours = 24, limit = 100000) =>
    get<SpoofedResponse>(`/spoofed-vessels?hours=${hours}&limit=${limit}`),

  searchVessels: (q: string, limit = 100) =>
    get<VesselFeatureCollection>(`/search?q=${encodeURIComponent(q)}&limit=${limit}`),

  getDarkVessels: (minHours = 6) =>
    get<DarkVesselResponse>(`/dark-vessels?min_hours=${minHours}&limit=100000`),

  getHistorical: (time: string) =>
    get<VesselFeatureCollection>(`/historical?time=${encodeURIComponent(time)}`),

  getTimeRange: () => get<TimeRange>('/time-range'),

  getPortVisits: (hours = 720, nonRussian = false, limit = 100000) => {
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

  getVesselRegistry: (q = '', tag = '', limit = 100000) => {
    const params = new URLSearchParams()
    if (q) params.set('q', q)
    if (tag) params.set('tag', tag)
    params.set('limit', String(limit))
    return get<VesselRegistryResponse>(`/vessel-registry?${params}`)
  },

  getVesselRegistryDetail: (mmsi: number) =>
    get<VesselRegistryDetail>(`/vessel-registry/${mmsi}`),

  getVesselChanges: (limit = 100000) =>
    get<VesselChangesResponse>(`/vessel-changes?limit=${limit}`),

  getVesselTags: () =>
    get<{ tags: string[] }>('/vessel-tags'),

  addVesselNote: (mmsi: number, tag: string, note?: string) =>
    post<{ status: string }>(`/vessel-registry/${mmsi}/notes`, { tag, note }),

  deleteVesselNote: (mmsi: number, tag: string) =>
    del<{ status: string }>(`/vessel-registry/${mmsi}/notes?tag=${encodeURIComponent(tag)}`),

  getTaintedVessels: (limit = 100000) =>
    get<TaintedVesselsResponse>(`/tainted-vessels?limit=${limit}`),

  getVesselTaintDetail: (mmsi: number) =>
    get<VesselTaintDetailResponse>(`/vessel-taint/${mmsi}`),

  getTaintChain: (taintId: number) =>
    get<TaintChainResponse>(`/taint-chain/${taintId}`),
}

import { ref, shallowRef, onMounted, onUnmounted, type Ref } from 'vue'
import L from 'leaflet'
import 'leaflet.markercluster'
import { api } from '@/api/client'
import { getVesselColor, isStaleAIS, formatAgo } from './useVesselUtils'
import type { VesselFeatureCollection, VesselFeature, VesselProperties, TrailsMap } from '@/types/vessel'

export interface MapFilters {
  russian: boolean
  nonRussian: boolean
  cargo: boolean
  tanker: boolean
  passenger: boolean
  tug: boolean
  fishing: boolean
  other: boolean
  hideStale: boolean
  darkOnly: boolean
  darkMinHours: number
  movingOnly: boolean
  showTrails: boolean
  showSTS: boolean
  satellite: boolean
}

export function useMap(containerRef: Ref<HTMLElement | null>) {
  const map = shallowRef<L.Map | null>(null)
  const vesselData = ref<VesselFeatureCollection | null>(null)
  const visibleCount = ref(0)
  const isLive = ref(true)

  let markersLayer: L.MarkerClusterGroup
  let trailsLayer: L.LayerGroup | null = null
  let stsLayer: L.LayerGroup | null = null
  let activeTrack: L.LayerGroup | null = null
  let satelliteTiles: L.TileLayer
  let osmTiles: L.TileLayer
  let refreshInterval: ReturnType<typeof setInterval> | null = null
  let trailTimer: ReturnType<typeof setTimeout> | null = null

  const filters = ref<MapFilters>({
    russian: true,
    nonRussian: true,
    cargo: true,
    tanker: true,
    passenger: true,
    tug: false,
    fishing: false,
    other: true,
    hideStale: true,
    darkOnly: false,
    darkMinHours: 6,
    movingOnly: false,
    showTrails: true,
    showSTS: false,
    satellite: true,
  })

  function initMap() {
    if (!containerRef.value) return
    const m = L.map(containerRef.value, { center: [59.9, 24.9], zoom: 7, zoomControl: true, preferCanvas: true })
    satelliteTiles = L.tileLayer(
      'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
      { attribution: 'Tiles: Esri', maxZoom: 18 },
    )
    osmTiles = L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: 'OSM',
      maxZoom: 19,
    })

    if (filters.value.satellite) {
      satelliteTiles.addTo(m)
    } else {
      osmTiles.addTo(m)
    }

    markersLayer = L.markerClusterGroup({
      maxClusterRadius: 40,
      disableClusteringAtZoom: 12,
      spiderfyOnMaxZoom: true,
      showCoverageOnHover: false,
      chunkedLoading: true,
      chunkInterval: 100,
      chunkDelay: 10,
    })
    m.addLayer(markersLayer)

    m.on('zoomend', () => renderMarkers())
    m.on('moveend', () => {
      if (isLive.value) fetchTrails()
    })

    map.value = m
  }

  function toggleSatellite(on: boolean) {
    if (!map.value) return
    if (on) {
      map.value.removeLayer(osmTiles)
      satelliteTiles.addTo(map.value)
    } else {
      map.value.removeLayer(satelliteTiles)
      osmTiles.addTo(map.value)
    }
  }

  // --- Vessel classification for filtering ---
  function classify(p: VesselProperties): string {
    const t = (p.vessel_type || '').toLowerCase()
    if (t.includes('tanker')) return 'tanker'
    if (t.includes('cargo') || t.includes('container')) return 'cargo'
    if (t.includes('passenger')) return 'passenger'
    if (t.includes('tug') || t.includes('tow') || t.includes('pilot')) return 'tug'
    if (t.includes('fish')) return 'fishing'
    return 'other'
  }

  function shouldShow(p: VesselProperties): boolean {
    const f = filters.value
    if (p.is_russian && !f.russian) return false
    if (!p.is_russian && !f.nonRussian) return false

    const cat = classify(p)
    const catMap: Record<string, keyof MapFilters> = {
      cargo: 'cargo', tanker: 'tanker', passenger: 'passenger', tug: 'tug', fishing: 'fishing', other: 'other',
    }
    if (catMap[cat] && !f[catMap[cat]]) return false

    if (f.movingOnly && (p.sog === undefined || p.sog <= 0.5)) return false

    if (isLive.value && f.hideStale) {
      const gapH = (Date.now() - new Date(p.timestamp).getTime()) / 3600000
      if (gapH > 24) return false
    }

    if (f.darkOnly) {
      const gapH = (Date.now() - new Date(p.timestamp).getTime()) / 3600000
      if (gapH < f.darkMinHours) return false
    }

    return true
  }

  // --- Arrow icon ---
  function getArrowSize(z: number): number {
    if (z >= 13) return 22
    if (z >= 11) return 18
    if (z >= 9) return 14
    if (z >= 7) return 10
    return 8
  }

  function makeArrowIcon(p: VesselProperties, z: number): L.DivIcon {
    const color = getVesselColor(p)
    const size = p.is_russian ? getArrowSize(z) + 4 : getArrowSize(z)
    const rot = p.heading !== undefined && p.heading < 360 ? p.heading : (p.cog ?? 0)

    const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 24 24"><path d="M12 2 L20 20 L12 16 L4 20 Z" fill="${color}" stroke="rgba(255,255,255,0.7)" stroke-width="1.2"/></svg>`
    let style = `transform:rotate(${rot}deg);width:${size}px;height:${size}px;`
    if (p.is_russian) style += 'filter:drop-shadow(0 0 3px rgba(255,0,0,0.6));'

    const stale = isStaleAIS(p)
    if (stale) {
      const dSize = Math.max(10, Math.floor(size * 0.55))
      const disconnectSvg = `<svg xmlns="http://www.w3.org/2000/svg" width="${dSize}" height="${dSize}" viewBox="0 0 24 24" style="position:absolute;top:-${dSize + 2}px;left:50%;transform:translateX(-50%);filter:drop-shadow(0 0 2px rgba(0,0,0,0.8))"><line x1="2" y1="2" x2="22" y2="22" stroke="#ff4444" stroke-width="2.5" stroke-linecap="round"/><line x1="2" y1="2" x2="22" y2="22" stroke="#fff" stroke-width="1" stroke-linecap="round"/></svg>`
      const totalH = size + dSize + 2
      const half = Math.floor(size / 2)
      return L.divIcon({
        html: `<div style="position:relative;width:${size}px;height:${size}px;">${disconnectSvg}<div style="${style}">${svg}</div></div>`,
        className: '',
        iconSize: [size, totalH],
        iconAnchor: [half, totalH],
        popupAnchor: [0, -totalH],
      })
    }

    const half = Math.floor(size / 2)
    return L.divIcon({
      html: `<div style="${style}">${svg}</div>`,
      className: '',
      iconSize: [size, size],
      iconAnchor: [half, half],
      popupAnchor: [0, -half],
    })
  }

  // --- Popup ---
  function buildPopup(p: VesselProperties): string {
    const isR = p.is_russian
    const name = (p.name || 'Unknown') + (isR ? ' [RUS]' : '')
    const nc = isR ? 'vessel-name russian' : 'vessel-name'
    let h = `<div class="vessel-popup"><div class="${nc}">${name}</div>`
    h += `<div class="vessel-mmsi">MMSI: ${p.mmsi}</div><table>`
    if (p.vessel_type) h += `<tr><td>Type</td><td>${p.vessel_type}</td></tr>`
    if (p.destination) h += `<tr><td>Dest</td><td>${p.destination}</td></tr>`
    if (p.sog !== undefined) h += `<tr><td>Speed</td><td>${p.sog} kn</td></tr>`
    if (p.cog !== undefined) h += `<tr><td>Course</td><td>${p.cog}&deg;</td></tr>`
    if (p.heading !== undefined) h += `<tr><td>Heading</td><td>${p.heading}&deg;</td></tr>`
    if (p.nav_status) h += `<tr><td>Status</td><td>${p.nav_status}</td></tr>`
    if (p.call_sign) h += `<tr><td>Call sign</td><td>${p.call_sign}</td></tr>`
    if (p.imo) h += `<tr><td>IMO</td><td>${p.imo}</td></tr>`
    if (p.draught) h += `<tr><td>Draught</td><td>${p.draught}m</td></tr>`
    if (p.sources) h += `<tr><td>Sources</td><td>${p.sources}</td></tr>`

    const ts = new Date(p.timestamp)
    const agoMin = Math.round((Date.now() - ts.getTime()) / 60000)
    const agoStr = agoMin < 60 ? `${agoMin} min ago` : `${Math.floor(agoMin / 60)}h ${agoMin % 60}m ago`
    h += `<tr><td>Last AIS</td><td><b>${agoStr}</b> (${ts.toLocaleTimeString()})</td></tr>`
    h += '</table>'
    h += `<button class="track-btn" onclick="window.__loadTrack(${p.mmsi})">Show 7-day track</button>`
    if (activeTrack) h += `<button class="track-btn clear-btn" onclick="window.__clearTrack()">Clear track</button>`
    h += '</div>'
    return h
  }

  // --- Render markers ---
  function renderMarkers() {
    if (!vesselData.value || !map.value) return
    markersLayer.clearLayers()
    let count = 0
    const zoom = map.value.getZoom()
    for (const f of vesselData.value.features) {
      const p = f.properties
      if (!shouldShow(p)) continue
      const [lng, lat] = f.geometry.coordinates
      if (lat === 0 && lng === 0) continue
      const marker = L.marker([lat, lng], { icon: makeArrowIcon(p, zoom) })
      marker.bindPopup(() => buildPopup(p), { maxWidth: 300 })
      markersLayer.addLayer(marker)
      count++
    }
    visibleCount.value = count
    if (isLive.value) fetchTrails()
  }

  // --- Trails ---
  function fetchTrails() {
    if (!map.value) return
    if (trailsLayer) {
      map.value.removeLayer(trailsLayer)
      trailsLayer = null
    }
    if (!filters.value.showTrails || map.value.getZoom() < 10) return
    if (trailTimer) clearTimeout(trailTimer)
    trailTimer = setTimeout(doFetchTrails, 500)
  }

  async function doFetchTrails() {
    if (!map.value) return
    const b = map.value.getBounds()
    try {
      const trails: TrailsMap = await api.getTrails(b.getSouth(), b.getWest(), b.getNorth(), b.getEast())
      if (trailsLayer && map.value) map.value.removeLayer(trailsLayer)
      trailsLayer = L.layerGroup().addTo(map.value!)
      for (const [mmsiStr, coords] of Object.entries(trails)) {
        if (coords.length < 2) continue
        const latlngs: [number, number][] = coords.map(([lng, lat]) => [lat, lng])
        let color: string | null = '#888'
        if (vesselData.value) {
          for (const f of vesselData.value.features) {
            if (String(f.properties.mmsi) === mmsiStr) {
              if (!shouldShow(f.properties)) { color = null; break }
              color = getVesselColor(f.properties)
              break
            }
          }
        }
        if (color === null) continue
        L.polyline(latlngs, { color, weight: 2, opacity: 0.6, dashArray: '4,4', interactive: false }).addTo(trailsLayer!)
      }
    } catch (e) {
      console.error('Trail error:', e)
    }
  }

  // --- STS markers ---
  async function loadSTSMarkers() {
    if (!map.value) return
    if (stsLayer) {
      map.value.removeLayer(stsLayer)
      stsLayer = null
    }
    if (!filters.value.showSTS) return
    try {
      const data = await api.getSTSEvents(168)
      if (!data.events?.length) return
      stsLayer = L.layerGroup().addTo(map.value!)
      for (const e of data.events) {
        if (!e.avg_lat || !e.avg_lon) continue
        const icon = L.divIcon({
          html: '<div class="sts-marker"></div>',
          className: '',
          iconSize: [20, 20],
          iconAnchor: [10, 10],
        })
        const popup = `<div style="font-size:13px"><b>STS Event</b><br>
          <b>${e.name_a || 'MMSI ' + e.mmsi_a}</b> + <b>${e.name_b || 'MMSI ' + e.mmsi_b}</b><br>
          Time: ${new Date(e.start_time).toLocaleString()}<br>
          Duration: ${e.duration_minutes} min<br>
          Distance: ${e.min_distance_m ? Math.round(e.min_distance_m) + 'm' : '--'}<br>
          <span style="color:${e.confidence === 'high' ? '#ff6b6b' : e.confidence === 'medium' ? '#ffa726' : '#66bb6a'}">${e.confidence.toUpperCase()}</span>
          </div>`
        L.marker([e.avg_lat, e.avg_lon], { icon }).bindPopup(popup).addTo(stsLayer!)
      }
    } catch { /* ignore */ }
  }

  // --- Track loading ---
  async function loadTrack(mmsi: number) {
    clearTrack()
    if (!map.value) return
    try {
      const data = await api.getVesselTrack(mmsi, 168)
      if (!data.track?.coordinates?.length || data.track.coordinates.length < 2) return
      const coords: [number, number][] = data.track.coordinates.map(([lng, lat]: [number, number]) => [lat, lng] as [number, number])
      if (coords.length < 2) return
      activeTrack = L.layerGroup().addTo(map.value!)
      L.polyline(coords, { color: '#ff0', weight: 3, opacity: 0.85, dashArray: '8,6' }).addTo(activeTrack)
      L.circleMarker(coords[0]!, { radius: 5, fillColor: '#0f0', fillOpacity: 1, color: '#fff', weight: 1 })
        .addTo(activeTrack).bindTooltip(`Start (${data.point_count} pts)`)
      L.circleMarker(coords[coords.length - 1]!, { radius: 5, fillColor: '#f00', fillOpacity: 1, color: '#fff', weight: 1 })
        .addTo(activeTrack).bindTooltip('Current')
      map.value.fitBounds(L.polyline(coords).getBounds(), { padding: [40, 40] })
    } catch (e) {
      console.error('Track error:', e)
    }
  }

  function clearTrack() {
    if (activeTrack && map.value) {
      map.value.removeLayer(activeTrack)
      activeTrack = null
    }
  }

  // Expose to window for popup onclick handlers
  if (typeof window !== 'undefined') {
    ;(window as any).__loadTrack = loadTrack
    ;(window as any).__clearTrack = clearTrack
  }

  // --- Data loading ---
  async function fetchVessels() {
    try {
      vesselData.value = await api.getVessels()
      renderMarkers()
    } catch (e) {
      console.error('Fetch vessels error:', e)
    }
  }

  async function loadHistorical(isoTime: string) {
    try {
      vesselData.value = await api.getHistorical(isoTime)
      renderMarkers()
    } catch (e) {
      console.error('Historical error:', e)
    }
  }

  function flyTo(lat: number, lng: number, zoom = 14) {
    map.value?.setView([lat, lng], zoom)
  }

  function startAutoRefresh(intervalMs = 120000) {
    stopAutoRefresh()
    refreshInterval = setInterval(() => {
      if (isLive.value) fetchVessels()
    }, intervalMs)
  }

  function stopAutoRefresh() {
    if (refreshInterval) {
      clearInterval(refreshInterval)
      refreshInterval = null
    }
  }

  onMounted(() => {
    initMap()
    fetchVessels()
    startAutoRefresh()
  })

  onUnmounted(() => {
    stopAutoRefresh()
    map.value?.remove()
  })

  return {
    map,
    vesselData,
    visibleCount,
    isLive,
    filters,
    renderMarkers,
    fetchVessels,
    loadHistorical,
    loadTrack,
    clearTrack,
    loadSTSMarkers,
    toggleSatellite,
    flyTo,
  }
}

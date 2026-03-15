import { ref, shallowRef, onMounted, onUnmounted, type Ref } from 'vue'
import L from 'leaflet'
import 'leaflet.markercluster'
import { api } from '@/api/client'
import { getVesselColor, isStaleAIS } from './useVesselUtils'
import type { VesselFeatureCollection, VesselProperties, TrailsMap } from '@/types/vessel'

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
  const focusedMMSIs = ref<Set<number>>(new Set())

  let markersLayer: L.MarkerClusterGroup
  let markersByMmsi = new Map<number, L.Marker | L.CircleMarker>()
  let trailsLayer: L.LayerGroup | null = null
  let stsLayer: L.LayerGroup | null = null
  let activeTrack: L.LayerGroup | null = null
  let focusLayer: L.LayerGroup | null = null
  let satelliteTiles: L.TileLayer
  let osmTiles: L.TileLayer
  let refreshInterval: ReturnType<typeof setInterval> | null = null
  let trailTimer: ReturnType<typeof setTimeout> | null = null
  let fetchTimer: ReturnType<typeof setTimeout> | null = null
  let lastZoom = 7
  let isFetching = false

  // Icon cache: keyed by "color|size|headingBucket|stale|russian"
  const iconCache = new Map<string, L.DivIcon>()
  function clearIconCache() { iconCache.clear() }

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
    satellite: false,
  })

  function getMapBBox() {
    if (!map.value) return undefined
    const b = map.value.getBounds()
    // Pad bounds by 20% to prefetch nearby vessels
    const latPad = (b.getNorth() - b.getSouth()) * 0.2
    const lngPad = (b.getEast() - b.getWest()) * 0.2
    return {
      south: b.getSouth() - latPad,
      west: b.getWest() - lngPad,
      north: b.getNorth() + latPad,
      east: b.getEast() + lngPad,
    }
  }

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
      chunkInterval: 200,
      chunkDelay: 50,
      iconCreateFunction(cluster: L.MarkerCluster) {
        const count = cluster.getChildCount()
        let size: string
        let px: number
        let ring: string
        if (count < 20) { size = 'small'; px = 36; ring = '#3b82f6' }
        else if (count < 100) { size = 'medium'; px = 44; ring = '#f59e0b' }
        else { size = 'large'; px = 52; ring = '#ef4444' }
        const r = px / 2
        const ir = r - 4
        const svg = `<svg width="${px}" height="${px}" viewBox="0 0 ${px} ${px}">` +
          `<circle cx="${r}" cy="${r}" r="${r - 1}" fill="${ring}" opacity="0.25"/>` +
          `<circle cx="${r}" cy="${r}" r="${ir}" fill="#1a1a2e" stroke="${ring}" stroke-width="3"/>` +
          `<text x="${r}" y="${r}" dy="0.35em" text-anchor="middle" fill="#fff" font-size="${size === 'large' ? 14 : size === 'medium' ? 13 : 12}" font-weight="700">${count}</text>` +
          `</svg>`
        return L.divIcon({
          html: svg,
          className: 'vessel-cluster',
          iconSize: L.point(px, px),
        })
      },
    })
    m.addLayer(markersLayer)

    // Debounced viewport change: re-fetch data on pan/zoom
    m.on('zoomend', () => {
      const newZoom = m.getZoom()
      if (newZoom !== lastZoom) {
        clearIconCache() // zoom changed, icons change size
        lastZoom = newZoom
      }
      debouncedFetchVessels()
    })
    m.on('moveend', () => {
      debouncedFetchVessels()
    })

    map.value = m
    lastZoom = m.getZoom()
  }

  // Debounce viewport-triggered fetches (300ms)
  function debouncedFetchVessels() {
    if (fetchTimer) clearTimeout(fetchTimer)
    fetchTimer = setTimeout(() => {
      if (isLive.value) fetchVessels()
    }, 300)
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

  // --- Arrow icon with caching ---
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
    // Bucket heading to nearest 5 degrees for cache efficiency
    const rotBucket = Math.round(rot / 5) * 5
    const stale = isStaleAIS(p)
    const cacheKey = `${color}|${size}|${rotBucket}|${stale ? 1 : 0}|${p.is_russian ? 1 : 0}`

    const cached = iconCache.get(cacheKey)
    if (cached) return cached

    const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 24 24"><path d="M12 2 L20 20 L12 16 L4 20 Z" fill="${color}" stroke="rgba(255,255,255,0.7)" stroke-width="1.2"/></svg>`
    let style = `transform:rotate(${rotBucket}deg);width:${size}px;height:${size}px;`
    if (p.is_russian) style += 'filter:drop-shadow(0 0 3px rgba(255,0,0,0.6));'

    let icon: L.DivIcon
    if (stale) {
      const dSize = Math.max(10, Math.floor(size * 0.55))
      const disconnectSvg = `<svg xmlns="http://www.w3.org/2000/svg" width="${dSize}" height="${dSize}" viewBox="0 0 24 24" style="position:absolute;top:-${dSize + 2}px;left:50%;transform:translateX(-50%);filter:drop-shadow(0 0 2px rgba(0,0,0,0.8))"><line x1="2" y1="2" x2="22" y2="22" stroke="#ff4444" stroke-width="2.5" stroke-linecap="round"/><line x1="2" y1="2" x2="22" y2="22" stroke="#fff" stroke-width="1" stroke-linecap="round"/></svg>`
      const totalH = size + dSize + 2
      const half = Math.floor(size / 2)
      icon = L.divIcon({
        html: `<div style="position:relative;width:${size}px;height:${size}px;">${disconnectSvg}<div style="${style}">${svg}</div></div>`,
        className: '',
        iconSize: [size, totalH],
        iconAnchor: [half, totalH],
        popupAnchor: [0, -totalH],
      })
    } else {
      const half = Math.floor(size / 2)
      icon = L.divIcon({
        html: `<div style="${style}">${svg}</div>`,
        className: '',
        iconSize: [size, size],
        iconAnchor: [half, half],
        popupAnchor: [0, -half],
      })
    }

    iconCache.set(cacheKey, icon)
    return icon
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

  // --- Render markers (diff-based, no flash) ---
  function renderMarkers() {
    if (!vesselData.value || !map.value) return
    const zoom = map.value.getZoom()
    const useCircles = zoom < 9 // lightweight canvas circles at low zoom
    const seen = new Set<number>()
    let count = 0

    for (const f of vesselData.value.features) {
      const p = f.properties
      if (!shouldShow(p)) continue
      const [lng, lat] = f.geometry.coordinates
      if (lat === 0 && lng === 0) continue
      seen.add(p.mmsi)
      count++

      const existing = markersByMmsi.get(p.mmsi)
      if (existing) {
        existing.setLatLng([lat, lng])
        if (!useCircles && existing instanceof L.Marker) {
          existing.setIcon(makeArrowIcon(p, zoom))
          existing.getPopup()?.setContent(buildPopup(p))
        } else if (useCircles && existing instanceof L.CircleMarker && !(existing instanceof L.Marker)) {
          // CircleMarker — just update position (already done) and style
          const color = getVesselColor(p)
          existing.setStyle({ fillColor: color, color })
        } else {
          // type changed (circle<->marker), remove and re-add
          markersLayer.removeLayer(existing)
          const newMarker = createMarker(p, lat, lng, zoom, useCircles)
          markersLayer.addLayer(newMarker)
          markersByMmsi.set(p.mmsi, newMarker)
        }
      } else {
        const marker = createMarker(p, lat, lng, zoom, useCircles)
        markersLayer.addLayer(marker)
        markersByMmsi.set(p.mmsi, marker)
      }
    }

    // remove markers that are no longer in the data
    markersByMmsi.forEach((marker, mmsi) => {
      if (!seen.has(mmsi)) {
        markersLayer.removeLayer(marker)
        markersByMmsi.delete(mmsi)
      }
    })

    visibleCount.value = count

    // Apply focus dimming/highlighting
    applyFocusStyling()

    // Fetch trails (debounced, only if zoomed in enough)
    if (isLive.value) scheduleTrails()
  }

  function createMarker(p: VesselProperties, lat: number, lng: number, zoom: number, useCircles: boolean): L.Marker | L.CircleMarker {
    if (useCircles) {
      const color = getVesselColor(p)
      const radius = p.is_russian ? 4 : 3
      const cm = L.circleMarker([lat, lng], {
        radius,
        fillColor: color,
        color,
        weight: 1,
        fillOpacity: 0.8,
        opacity: 0.9,
      })
      cm.bindPopup(() => buildPopup(p), { maxWidth: 300 })
      return cm
    } else {
      const marker = L.marker([lat, lng], { icon: makeArrowIcon(p, zoom) })
      marker.bindPopup(() => buildPopup(p), { maxWidth: 300 })
      return marker
    }
  }

  // --- Trails ---
  function scheduleTrails() {
    if (!map.value) return
    if (!filters.value.showTrails || map.value.getZoom() < 10) {
      if (trailsLayer && map.value) {
        map.value.removeLayer(trailsLayer)
        trailsLayer = null
      }
      return
    }
    if (trailTimer) clearTimeout(trailTimer)
    trailTimer = setTimeout(doFetchTrails, 600)
  }

  async function doFetchTrails() {
    if (!map.value) return
    const b = map.value.getBounds()
    try {
      const trails: TrailsMap = await api.getTrails(b.getSouth(), b.getWest(), b.getNorth(), b.getEast())
      if (trailsLayer && map.value) map.value.removeLayer(trailsLayer)
      trailsLayer = L.layerGroup().addTo(map.value!)

      // Build MMSI->color lookup once instead of O(N*M) linear scan
      const vesselColorMap = new Map<string, string | null>()
      if (vesselData.value) {
        for (const f of vesselData.value.features) {
          const p = f.properties
          if (!shouldShow(p)) {
            vesselColorMap.set(String(p.mmsi), null)
          } else {
            vesselColorMap.set(String(p.mmsi), getVesselColor(p))
          }
        }
      }

      for (const [mmsiStr, coords] of Object.entries(trails)) {
        if (coords.length < 2) continue
        const latlngs: [number, number][] = coords.map(([lng, lat]) => [lat, lng])
        const color = vesselColorMap.get(mmsiStr) ?? '#888'
        if (vesselColorMap.has(mmsiStr) && vesselColorMap.get(mmsiStr) === null) continue
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

  // --- Data loading (viewport-aware) ---
  async function fetchVessels() {
    if (isFetching) return // prevent overlapping fetches
    isFetching = true
    try {
      const bbox = getMapBBox()
      vesselData.value = await api.getVessels(bbox)
      renderMarkers()
    } catch (e) {
      console.error('Fetch vessels error:', e)
    } finally {
      isFetching = false
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

  // --- STS Focus Mode ---
  function setFocusMode(mmsiA: number, mmsiB: number) {
    focusedMMSIs.value = new Set([mmsiA, mmsiB])
    applyFocusStyling()
  }

  function clearFocusMode() {
    focusedMMSIs.value = new Set()
    if (focusLayer && map.value) {
      map.value.removeLayer(focusLayer)
      focusLayer = null
    }
    // Restore all marker opacities
    markersByMmsi.forEach((marker) => {
      if (marker instanceof L.CircleMarker && !(marker instanceof L.Marker)) {
        marker.setStyle({ fillOpacity: 0.8, opacity: 0.9 })
      } else if (marker instanceof L.Marker) {
        marker.setOpacity(1)
      }
    })
  }

  function applyFocusStyling() {
    if (!map.value || focusedMMSIs.value.size === 0) return

    // Remove old focus highlights
    if (focusLayer && map.value) {
      map.value.removeLayer(focusLayer)
    }
    focusLayer = L.layerGroup().addTo(map.value!)

    markersByMmsi.forEach((marker, mmsi) => {
      const isFocused = focusedMMSIs.value.has(mmsi)
      if (marker instanceof L.CircleMarker && !(marker instanceof L.Marker)) {
        marker.setStyle({
          fillOpacity: isFocused ? 1.0 : 0.08,
          opacity: isFocused ? 1.0 : 0.08,
        })
      } else if (marker instanceof L.Marker) {
        marker.setOpacity(isFocused ? 1 : 0.1)
      }
      // Add pulsing rings for focused vessels
      if (isFocused && focusLayer) {
        const ll = marker.getLatLng()
        L.circleMarker(ll, {
          radius: 18,
          fillColor: '#fff',
          fillOpacity: 0,
          color: '#00ffff',
          weight: 3,
          opacity: 0.9,
          className: 'sts-focus-ring',
        }).addTo(focusLayer!)
        L.circleMarker(ll, {
          radius: 28,
          fillColor: '#fff',
          fillOpacity: 0,
          color: '#00ffff',
          weight: 1.5,
          opacity: 0.5,
          className: 'sts-focus-ring-outer',
        }).addTo(focusLayer!)
      }
    })
  }

  function startAutoRefresh(intervalMs = 15000) {
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
    if (fetchTimer) clearTimeout(fetchTimer)
    if (trailTimer) clearTimeout(trailTimer)
    map.value?.remove()
  })

  return {
    map,
    vesselData,
    visibleCount,
    isLive,
    filters,
    focusedMMSIs,
    renderMarkers,
    fetchVessels,
    loadHistorical,
    loadTrack,
    clearTrack,
    loadSTSMarkers,
    toggleSatellite,
    flyTo,
    setFocusMode,
    clearFocusMode,
  }
}

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

  // Favorites tracking
  const favoriteMMSIs = new Set<number>()

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

    // Load favorite MMSIs for popup star state
    api.getFavorites().then(data => {
      for (const f of data.favorites || []) favoriteMMSIs.add(f.mmsi)
    }).catch(() => {})

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

    // Gap markers render below vessel markers
    m.createPane('gapPane')
    m.getPane('gapPane')!.style.zIndex = '550'

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
  function mmsiToFlag(mmsi: number): string {
    const mid = String(mmsi).substring(0, 3)
    const flags: Record<string, string> = {
      '201':'Albania','202':'Andorra','203':'Austria','204':'Azores','205':'Belgium',
      '206':'Belarus','207':'Bulgaria','208':'Vatican','209':'Cyprus','210':'Cyprus',
      '211':'Germany','212':'Cyprus','213':'Georgia','214':'Moldova','215':'Malta',
      '216':'Armenia','218':'Germany','219':'Denmark','220':'Denmark','224':'Spain',
      '225':'Spain','226':'France','227':'France','228':'France','229':'Malta',
      '230':'Finland','231':'Faroe Islands','232':'UK','233':'UK','234':'UK','235':'UK',
      '236':'Gibraltar','237':'Greece','238':'Croatia','239':'Greece','240':'Greece',
      '241':'Greece','242':'Morocco','243':'Hungary','244':'Netherlands','245':'Netherlands',
      '246':'Netherlands','247':'Italy','248':'Malta','249':'Malta','250':'Ireland',
      '251':'Iceland','252':'Liechtenstein','253':'Luxembourg','254':'Madeira',
      '255':'Portugal','256':'Malta','257':'Norway','258':'Norway','259':'Norway',
      '261':'Poland','262':'Montenegro','263':'Portugal','264':'Romania','265':'Sweden',
      '266':'Sweden','267':'Slovakia','268':'San Marino','269':'Switzerland','270':'Czech Republic',
      '271':'Turkey','272':'Ukraine','273':'Russia','274':'North Macedonia','275':'Latvia',
      '276':'Estonia','277':'Lithuania','278':'Slovenia','279':'Serbia',
      '301':'Anguilla','303':'Alaska','304':'Antigua','305':'Antigua','306':'Curacao',
      '307':'Aruba','308':'Bahamas','309':'Bahamas','310':'Bermuda','311':'Bahamas',
      '312':'Belize','314':'Barbados','316':'Canada','319':'Cayman Islands',
      '321':'Costa Rica','323':'Cuba','325':'Dominica','327':'Dominican Republic',
      '329':'Guadeloupe','330':'Grenada','331':'Greenland','332':'Guatemala',
      '334':'Honduras','336':'Haiti','338':'USA','339':'Jamaica','341':'Saint Kitts',
      '343':'Saint Lucia','345':'Mexico','347':'Martinique','348':'Montserrat',
      '350':'Nicaragua','351':'Panama','352':'Panama','353':'Panama','354':'Panama',
      '355':'Panama','356':'Panama','357':'Panama','358':'Puerto Rico',
      '361':'Saint Pierre','362':'Trinidad','364':'Turks and Caicos',
      '366':'USA','367':'USA','368':'USA','369':'USA','370':'Panama',
      '371':'Panama','372':'Panama','373':'Panama','374':'Panama','375':'Saint Vincent',
      '376':'Saint Vincent','377':'Saint Vincent',
      '401':'Afghanistan','403':'Saudi Arabia','405':'Bangladesh','408':'Bahrain',
      '410':'Bhutan','412':'China','413':'China','414':'China','416':'Taiwan',
      '417':'Sri Lanka','419':'India','422':'Iran','423':'Azerbaijan','425':'Iraq',
      '428':'Israel','431':'Japan','432':'Japan','434':'Turkmenistan','436':'Kazakhstan',
      '437':'Uzbekistan','438':'Jordan','440':'South Korea','441':'South Korea',
      '443':'Palestine','445':'DPRK','447':'Kuwait','450':'Lebanon','451':'Kyrgyzstan',
      '453':'Macao','455':'Maldives','457':'Mongolia','459':'Nepal','461':'Oman',
      '463':'Pakistan','466':'Qatar','468':'Syria','470':'UAE','471':'UAE',
      '472':'Tajikistan','473':'Yemen','475':'Yemen','477':'Hong Kong',
      '478':'Bosnia','501':'Antarctica','503':'Australia','506':'Myanmar',
      '508':'Brunei','510':'Micronesia','511':'Palau','512':'New Zealand',
      '514':'Cambodia','515':'Cambodia','516':'Christmas Island','518':'Cook Islands',
      '520':'Fiji','523':'Cocos Islands','525':'Indonesia','529':'Kiribati',
      '531':'Laos','533':'Malaysia','536':'N. Mariana Islands','538':'Marshall Islands',
      '540':'New Caledonia','542':'Niue','544':'Nauru','546':'French Polynesia',
      '548':'Philippines','553':'Papua New Guinea','555':'Pitcairn','557':'Solomon Islands',
      '559':'American Samoa','561':'Samoa','563':'Singapore','564':'Singapore',
      '565':'Singapore','566':'Singapore','567':'Thailand','570':'Tonga',
      '572':'Tuvalu','574':'Vietnam','576':'Vanuatu','577':'Vanuatu',
      '578':'Wallis and Futuna',
      '601':'South Africa','603':'Angola','605':'Algeria','607':'Benin',
      '608':'Botswana','609':'Burundi','610':'Cameroon','611':'Cape Verde',
      '612':'Central African Republic','613':'Congo','614':'Ivory Coast',
      '615':'Comoros','616':'DRC','617':'Djibouti','618':'Egypt',
      '619':'Equatorial Guinea','620':'Eritrea','621':'Ethiopia',
      '622':'Gabon','624':'Ghana','625':'Gambia','626':'Guinea-Bissau',
      '627':'Guinea','629':'Kenya','630':'Lesotho','631':'Liberia',
      '632':'Liberia','633':'Liberia','634':'Libya','635':'Mauritius',
      '636':'Madagascar','637':'Mali','642':'Mozambique','644':'Mauritania',
      '645':'Namibia','647':'Niger','649':'Nigeria','650':'Rwanda',
      '654':'Sao Tome','655':'Senegal','656':'Seychelles','657':'Sierra Leone',
      '659':'Sudan','660':'Eswatini','661':'Tanzania','662':'Chad',
      '663':'Togo','664':'Tunisia','665':'Uganda','667':'Zambia','669':'Zimbabwe',
      '670':'Gambia','671':'Tanzania','672':'Comoros',
    }
    return flags[mid] || ''
  }

  function buildPopup(p: VesselProperties): string {
    const isR = p.is_russian
    const flag = mmsiToFlag(p.mmsi)
    const name = (p.name || 'Unknown') + (isR ? ' [RUS]' : '')
    const nc = isR ? 'vessel-name russian' : 'vessel-name'
    const isFav = favoriteMMSIs.has(p.mmsi)
    const starClass = isFav ? 'fav-star active' : 'fav-star'
    let h = `<div class="vessel-popup"><div class="${nc}">${name} <span class="${starClass}" onclick="window.__toggleFavorite(${p.mmsi}, '${(p.name || '').replace(/'/g, "\\'")}', '${(p.vessel_type || '').replace(/'/g, "\\'")}')" title="${isFav ? 'Remove from favorites' : 'Add to favorites'}">★</span></div>`
    if (flag) h += `<div class="vessel-flag">${flag}${isR ? ' \u{1F6A9}' : ''}</div>`
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
    if (!filters.value.showTrails || map.value.getZoom() < 7) {
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

      // Build MMSI->color and vessel info lookups
      const vesselColorMap = new Map<string, string | null>()
      const vesselInfoMap = new Map<string, VesselProperties>()
      if (vesselData.value) {
        for (const f of vesselData.value.features) {
          const p = f.properties
          vesselInfoMap.set(String(p.mmsi), p)
          if (!shouldShow(p)) {
            vesselColorMap.set(String(p.mmsi), null)
          } else {
            vesselColorMap.set(String(p.mmsi), getVesselColor(p))
          }
        }
      }

      for (const [mmsiStr, coords] of Object.entries(trails)) {
        if (coords.length < 2) continue
        const color = vesselColorMap.get(mmsiStr) ?? '#888'
        if (vesselColorMap.has(mmsiStr) && vesselColorMap.get(mmsiStr) === null) continue

        // Split into segments at time gaps (>1 hour)
        const segments: [number, number][][] = [[[coords[0]![1], coords[0]![0]]]]
        const gaps: { from: [number, number], to: [number, number], hours: number }[] = []
        for (let i = 1; i < coords.length; i++) {
          const dt = (coords[i]![2] - coords[i - 1]![2]) * 1000
          if (dt > GAP_THRESHOLD_MS) {
            const lastSeg = segments[segments.length - 1]!
            const fromPt: [number, number] = lastSeg.length > 0 ? lastSeg[lastSeg.length - 1]! : [coords[i - 1]![1], coords[i - 1]![0]]
            const toPt: [number, number] = [coords[i]![1], coords[i]![0]]
            gaps.push({ from: fromPt, to: toPt, hours: Math.round(dt / 3600000) })
            segments.push([])
          }
          segments[segments.length - 1]!.push([coords[i]![1], coords[i]![0]])
        }
        for (const seg of segments) {
          if (seg.length >= 2) {
            L.polyline(seg, { color, weight: 2, opacity: 0.6, dashArray: '4,4', interactive: false }).addTo(trailsLayer!)
          }
        }
        // Draw gap connectors and AIS-off markers
        const vInfo = vesselInfoMap.get(mmsiStr)
        const vName = vInfo?.name || 'MMSI ' + mmsiStr
        const vType = vInfo?.vessel_type || ''
        for (const gap of gaps) {
          L.polyline([gap.from, gap.to], { color: '#ff8800', weight: 3, opacity: 0.6, dashArray: '6,10', interactive: false }).addTo(trailsLayer!)
          const offIcon = L.divIcon({
            html: `<div class="ais-gap-marker ais-off">X</div>`,
            className: '', iconSize: [14, 14], iconAnchor: [-4, 7]
          })
          const onIcon = L.divIcon({
            html: `<div class="ais-gap-marker ais-on">&#9650;</div>`,
            className: '', iconSize: [14, 14], iconAnchor: [-4, 7]
          })
          const offTip = `<b>${vName}</b>${vType ? '<br>' + vType : ''}<br>AIS OFF — ~${gap.hours}h gap`
          const onTip = `<b>${vName}</b>${vType ? '<br>' + vType : ''}<br>AIS ON`
          L.marker(gap.from, { icon: offIcon, interactive: true, pane: 'gapPane' })
            .addTo(trailsLayer!).bindTooltip(offTip, { direction: 'top', offset: [0, -12] })
          L.marker(gap.to, { icon: onIcon, interactive: true, pane: 'gapPane' })
            .addTo(trailsLayer!).bindTooltip(onTip, { direction: 'top', offset: [0, -12] })
        }
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
  const GAP_THRESHOLD_MS = 60 * 60 * 1000 // 1 hour — break line if gap exceeds this

  async function loadTrack(mmsi: number) {
    clearTrack()
    if (!map.value) return
    try {
      const data = await api.getVesselTrack(mmsi, 168)
      if (!data.track?.coordinates?.length || data.track.coordinates.length < 2) return
      const coords: [number, number][] = data.track.coordinates.map(([lng, lat]: [number, number]) => [lat, lng] as [number, number])
      if (coords.length < 2) return
      activeTrack = L.layerGroup().addTo(map.value!)

      // Split into segments at time gaps
      const segments: [number, number][][] = [[coords[0]!]]
      const gaps: { from: [number, number], to: [number, number], hours: number }[] = []
      for (let i = 1; i < coords.length; i++) {
        if (data.timestamps?.[i] && data.timestamps[i - 1]) {
          const dt = new Date(data.timestamps[i]!).getTime() - new Date(data.timestamps[i - 1]!).getTime()
          if (dt > GAP_THRESHOLD_MS) {
            const lastSeg = segments[segments.length - 1]!
            const fromPt = lastSeg.length > 0 ? lastSeg[lastSeg.length - 1]! : coords[i - 1]!
            gaps.push({ from: fromPt, to: coords[i]!, hours: Math.round(dt / 3600000) })
            segments.push([])
          }
        }
        segments[segments.length - 1]!.push(coords[i]!)
      }

      // Draw each segment separately
      const allCoords: [number, number][] = []
      for (const seg of segments) {
        if (seg.length >= 2) {
          L.polyline(seg, { color: '#ff0', weight: 3, opacity: 0.85, dashArray: '8,6' }).addTo(activeTrack)
        }
        allCoords.push(...seg)
      }

      // Draw gap connectors and AIS-off/on markers
      const trackName = data.vessel?.name || 'MMSI ' + mmsi
      const trackType = data.vessel?.vessel_type_name || ''
      for (const gap of gaps) {
        L.polyline([gap.from, gap.to], { color: '#ff8800', weight: 3, opacity: 0.7, dashArray: '6,10' }).addTo(activeTrack!)
        const offIcon = L.divIcon({
          html: `<div class="ais-gap-marker ais-off ais-gap-pulse">X</div><div class="ais-gap-label ais-off-label">${trackName}<br>AIS OFF ~${gap.hours}h</div>`,
          className: '', iconSize: [18, 18], iconAnchor: [-6, 9]
        })
        const onIcon = L.divIcon({
          html: `<div class="ais-gap-marker ais-on">&#9650;</div><div class="ais-gap-label ais-on-label">${trackName}<br>AIS ON</div>`,
          className: '', iconSize: [18, 18], iconAnchor: [-6, 9]
        })
        L.marker(gap.from, { icon: offIcon, interactive: true, pane: 'gapPane' }).addTo(activeTrack!)
        L.marker(gap.to, { icon: onIcon, interactive: true, pane: 'gapPane' }).addTo(activeTrack!)
        allCoords.push(gap.from, gap.to)
      }

      L.circleMarker(coords[0]!, { radius: 5, fillColor: '#0f0', fillOpacity: 1, color: '#fff', weight: 1 })
        .addTo(activeTrack).bindTooltip(`Start (${data.point_count} pts)`)
      L.circleMarker(coords[coords.length - 1]!, { radius: 5, fillColor: '#f00', fillOpacity: 1, color: '#fff', weight: 1 })
        .addTo(activeTrack).bindTooltip('Current')
      map.value.fitBounds(L.polyline(allCoords).getBounds(), { padding: [40, 40] })
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
    ;(window as any).__toggleFavorite = async (mmsi: number, name: string, type: string) => {
      if (favoriteMMSIs.has(mmsi)) {
        await api.removeFavorite(mmsi)
        favoriteMMSIs.delete(mmsi)
      } else {
        await api.addFavorite(mmsi, name || undefined, type || undefined)
        favoriteMMSIs.add(mmsi)
      }
      // Re-render popup for the marker
      const marker = markersByMmsi.get(mmsi)
      if (marker) {
        const popup = marker.getPopup()
        if (popup && popup.isOpen()) {
          // Find the vessel properties
          const feat = vesselData.value?.features.find(f => f.properties.mmsi === mmsi)
          if (feat) popup.setContent(buildPopup(feat.properties))
        }
      }
    }
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

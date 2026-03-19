<script setup lang="ts">
import { ref, watch, computed, onMounted } from 'vue'
import { useMap } from '@/composables/useMap'
import { api } from '@/api/client'
import { formatNumber } from '@/composables/useVesselUtils'
import MapControls from '@/components/map/MapControls.vue'
import MapSearch from '@/components/map/MapSearch.vue'
import MapTimeSlider from '@/components/map/MapTimeSlider.vue'
import VesselDetailPanel from '@/components/map/VesselDetailPanel.vue'
import type { Stats } from '@/types/vessel'

const mapContainer = ref<HTMLElement | null>(null)
const timeSliderRef = ref<InstanceType<typeof MapTimeSlider> | null>(null)
const {
  map: leafletMap,
  visibleCount,
  isLive,
  filters,
  focusedMMSIs,
  selectedVessel,
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
  closePanel,
} = useMap(mapContainer)

const stats = ref<Stats | null>(null)
api.getStats().then((s) => (stats.value = s))

watch(() => filters.value.satellite, (v) => toggleSatellite(v))
watch(() => filters.value.showSTS, () => loadSTSMarkers())
watch(
  () => [
    filters.value.russian, filters.value.nonRussian,
    filters.value.cargo, filters.value.tanker, filters.value.passenger,
    filters.value.tug, filters.value.fishing, filters.value.other,
    filters.value.hideStale, filters.value.darkOnly, filters.value.darkMinHours,
    filters.value.movingOnly, filters.value.showTrails,
    filters.value.showGapMarkers,
  ],
  () => renderMarkers(),
)

function handleTimeChange(isoTime: string | null) {
  if (isoTime === null) {
    isLive.value = true
    fetchVessels()
  } else {
    isLive.value = false
    loadHistorical(isoTime)
  }
}

onMounted(() => {
  const hash = window.location.hash.replace('#', '')
  if (!hash) return
  const params = new URLSearchParams(hash)
  const goto = params.get('goto')
  const time = params.get('time')
  const mmsiA = params.get('mmsi_a')
  const mmsiB = params.get('mmsi_b')

  if (goto) {
    const parts = goto.split(',')
    if (parts.length >= 2) {
      const lat = parseFloat(parts[0]!)
      const lng = parseFloat(parts[1]!)
      const zoom = parts[2] ? parseInt(parts[2]) : 14
      // wait for map init then fly
      setTimeout(() => flyTo(lat, lng, zoom), 400)
    }
  }

  if (time) {
    // jump to historical time
    setTimeout(() => {
      isLive.value = false
      loadHistorical(time).then(() => {
        // Apply focus after historical data loads
        if (mmsiA && mmsiB) {
          setFocusMode(parseInt(mmsiA), parseInt(mmsiB))
        }
      })
      timeSliderRef.value?.jumpToTime(time)
    }, 500)
  } else if (mmsiA && mmsiB) {
    setTimeout(() => setFocusMode(parseInt(mmsiA), parseInt(mmsiB)), 800)
  }
})

const isFocused = computed(() => focusedMMSIs.value.size > 0)

async function handleToggleFavorite(mmsi: number) {
  if (typeof (window as any).__toggleFavorite === 'function') {
    const v = selectedVessel.value
    ;(window as any).__toggleFavorite(mmsi, v?.name || '', v?.vessel_type || '')
  }
}

function exitFocus() {
  clearFocusMode()
  window.history.replaceState(null, '', window.location.pathname)
}
</script>

<template>
  <div class="map-page">
    <div ref="mapContainer" class="map-container" />

    <!-- Stats panel -->
    <div class="panel stats-panel">
      <div class="panel-head">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#4fc3f7" stroke-width="2"><path d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0h4"/></svg>
        <span style="margin-left:6px">SeaTradeLab</span>
      </div>
      <div class="panel-body">
        <div class="stat-row"><span>Total</span><span class="stat-value">{{ formatNumber(stats?.total_vessels) }}</span></div>
        <div class="stat-row"><span>Russian</span><span class="stat-value russian">{{ formatNumber(stats?.russian_vessels) }}</span></div>
        <div class="stat-row"><span>Positions</span><span class="stat-value">{{ formatNumber(stats?.total_positions) }}</span></div>
        <div class="stat-row"><span>Visible</span><span class="stat-value highlight">{{ visibleCount.toLocaleString() }}</span></div>
        <router-link to="/" class="home-link">&#8592; Dashboard</router-link>
      </div>
    </div>

    <MapControls v-model:filters="filters" />

    <MapSearch @fly-to="flyTo" />

    <MapTimeSlider ref="timeSliderRef" @time-change="handleTimeChange" :is-live="isLive" />

    <VesselDetailPanel
      :vessel="selectedVessel"
      @close="closePanel"
      @load-track="loadTrack"
      @clear-track="clearTrack"
      @toggle-favorite="handleToggleFavorite"
    />

    <!-- STS Focus Mode banner -->
    <div v-if="isFocused" class="focus-banner" @click="exitFocus">
      <span class="focus-icon">&#x1F50D;</span>
      STS Focus Mode &mdash; showing 2 vessels
      <button class="focus-exit-btn">Exit Focus</button>
    </div>
  </div>
</template>

<style scoped>
.map-page {
  position: relative;
  width: 100vw;
  height: 100vh;
}
.map-container {
  width: 100%;
  height: 100%;
}

.panel {
  position: absolute;
  z-index: 1000;
  background: rgba(14, 17, 28, 0.94);
  color: #e0e0e0;
  border-radius: 10px;
  font-size: 13px;
  backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.06);
  box-shadow: 0 2px 16px rgba(0, 0, 0, 0.3);
}
.panel-head {
  padding: 10px 14px;
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  display: flex;
  align-items: center;
}
.panel-body {
  padding: 8px 14px 12px;
}

.stats-panel {
  top: 12px;
  left: 12px;
  min-width: 190px;
}
.stat-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 3px 0;
  font-size: 12px;
}
.stat-value {
  font-weight: 600;
  color: #7cb4ff;
}
.stat-value.russian {
  color: #ff6b6b;
}
.stat-value.highlight {
  color: #4fc3f7;
}
.home-link {
  display: block;
  margin-top: 6px;
  font-size: 11px;
  color: #555;
  text-decoration: none;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  padding-top: 6px;
  transition: color 0.15s;
}
.home-link:hover {
  color: #4fc3f7;
  text-decoration: none;
}

.focus-banner {
  position: absolute;
  top: 12px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 1100;
  background: rgba(0, 220, 255, 0.12);
  border: 1px solid rgba(0, 255, 255, 0.35);
  backdrop-filter: blur(12px);
  color: #fff;
  padding: 8px 18px;
  border-radius: 10px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 10px;
  white-space: nowrap;
  box-shadow: 0 2px 16px rgba(0, 200, 255, 0.15);
}
.focus-icon {
  font-size: 16px;
}
.focus-exit-btn {
  background: rgba(255, 80, 80, 0.8);
  border: none;
  color: #fff;
  padding: 3px 12px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}
.focus-exit-btn:hover {
  background: rgba(255, 60, 60, 1);
}
</style>

<style>
/* Unscoped — Leaflet focus ring animations */
.sts-focus-ring {
  animation: sts-pulse 1.5s ease-in-out infinite;
}
.sts-focus-ring-outer {
  animation: sts-pulse-outer 1.5s ease-in-out infinite;
}
@keyframes sts-pulse {
  0%, 100% { opacity: 0.9; }
  50% { opacity: 0.3; }
}
@keyframes sts-pulse-outer {
  0%, 100% { opacity: 0.5; }
  50% { opacity: 0.1; }
}

/* AIS gap markers */
.ais-gap-marker {
  width: 100%; height: 100%; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  font-weight: 900; font-size: 9px; color: #fff;
  border: 1.5px solid #fff;
  box-shadow: 0 0 6px rgba(0,0,0,0.5);
  cursor: pointer;
}
.ais-gap-marker.ais-off {
  background: #ff8800;
}
.ais-gap-marker.ais-on {
  background: #00ccff;
  font-size: 10px;
}
.ais-gap-pulse {
  animation: ais-gap-blink 1.2s ease-in-out infinite;
}
@keyframes ais-gap-blink {
  0%, 100% { box-shadow: 0 0 4px rgba(255,136,0,0.4); }
  50% { box-shadow: 0 0 16px 4px rgba(255,136,0,0.8); }
}
.ais-gap-label {
  position: absolute; top: -28px; left: 50%; transform: translateX(-50%);
  white-space: nowrap; font-size: 10px; font-weight: 700;
  padding: 2px 6px; border-radius: 3px;
  pointer-events: none; line-height: 1.3;
  text-align: center;
}
.ais-off-label {
  background: rgba(255,136,0,0.9); color: #fff;
}
.ais-on-label {
  background: rgba(0,204,255,0.9); color: #fff;
}
</style>

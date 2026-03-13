<script setup lang="ts">
import { ref, watch } from 'vue'
import { useMap } from '@/composables/useMap'
import { api } from '@/api/client'
import { formatNumber } from '@/composables/useVesselUtils'
import MapControls from '@/components/map/MapControls.vue'
import MapSearch from '@/components/map/MapSearch.vue'
import MapTimeSlider from '@/components/map/MapTimeSlider.vue'
import type { Stats } from '@/types/vessel'

const mapContainer = ref<HTMLElement | null>(null)
const {
  map: leafletMap,
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
</script>

<template>
  <div class="map-page">
    <div ref="mapContainer" class="map-container" />

    <!-- Stats panel -->
    <div class="panel stats-panel">
      <div class="panel-head">SeaTradeLab</div>
      <div class="panel-body">
        <div class="stat-row"><span>Total</span><span class="stat-value">{{ formatNumber(stats?.total_vessels) }}</span></div>
        <div class="stat-row"><span>Russian</span><span class="stat-value russian">{{ formatNumber(stats?.russian_vessels) }}</span></div>
        <div class="stat-row"><span>Positions</span><span class="stat-value">{{ formatNumber(stats?.total_positions) }}</span></div>
        <div class="stat-row"><span>Visible</span><span class="stat-value">{{ visibleCount.toLocaleString() }}</span></div>
        <router-link to="/" class="home-link">Back to Dashboard</router-link>
      </div>
    </div>

    <MapControls v-model:filters="filters" />

    <MapSearch @fly-to="flyTo" />

    <MapTimeSlider @time-change="handleTimeChange" :is-live="isLive" />
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
  background: rgba(20, 24, 33, 0.92);
  color: #e0e0e0;
  border-radius: 8px;
  font-size: 13px;
  backdrop-filter: blur(8px);
  border: 1px solid rgba(255, 255, 255, 0.08);
}
.panel-head {
  padding: 10px 14px;
  font-size: 13px;
  font-weight: 600;
  color: #fff;
}
.panel-body {
  padding: 8px 14px 12px;
}

.stats-panel {
  top: 10px;
  left: 55px;
  min-width: 200px;
}
.stat-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 2px 0;
}
.stat-value {
  font-weight: 600;
  color: #7cb4ff;
}
.stat-value.russian {
  color: #ff6b6b;
}
.home-link {
  display: block;
  margin-top: 4px;
  font-size: 11px;
  color: #7cb4ff;
  text-decoration: none;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  padding-top: 4px;
}
.home-link:hover {
  text-decoration: underline;
}
</style>

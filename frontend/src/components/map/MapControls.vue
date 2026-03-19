<script setup lang="ts">
import type { MapFilters } from '@/composables/useMap'

const filters = defineModel<MapFilters>('filters', { required: true })

const collapsed = defineModel<boolean>('collapsed', { default: false })
</script>

<template>
  <div class="panel controls-panel" :class="{ collapsed }">
    <div class="panel-head" @click="collapsed = !collapsed">
      <span>Filters</span>
      <span class="min-btn">{{ collapsed ? '+' : '-' }}</span>
    </div>
    <div v-show="!collapsed" class="panel-body">
      <div class="section-label">Flag</div>
      <label><input type="checkbox" v-model="filters.russian"> Russian</label>
      <label><input type="checkbox" v-model="filters.nonRussian"> Non-Russian</label>

      <div class="section-label">Vessel Type</div>
      <label><input type="checkbox" v-model="filters.cargo"> <div class="legend-dot" style="background:#1a73e8" /> Cargo</label>
      <label><input type="checkbox" v-model="filters.tanker"> <div class="legend-dot" style="background:#e67e22" /> Tanker</label>
      <label><input type="checkbox" v-model="filters.passenger"> <div class="legend-dot" style="background:#2ecc71" /> Passenger</label>
      <label><input type="checkbox" v-model="filters.tug"> <div class="legend-dot" style="background:#9b59b6" /> Tug/Tow</label>
      <label><input type="checkbox" v-model="filters.fishing"> <div class="legend-dot" style="background:#16a085" /> Fishing</label>
      <label><input type="checkbox" v-model="filters.other"> <div class="legend-dot" style="background:#95a5a6" /> Other</label>

      <div class="section-label">AIS Staleness</div>
      <label><input type="checkbox" v-model="filters.hideStale"> Hide stale AIS (&gt;24h)</label>
      <label><input type="checkbox" v-model="filters.darkOnly"> Dark vessels only</label>
      <div class="range-row">
        <span>Min gap:</span>
        <input type="number" v-model.number="filters.darkMinHours" min="0.5" step="0.5">
        <span>hours</span>
      </div>

      <div class="section-label">Movement</div>
      <label><input type="checkbox" v-model="filters.movingOnly"> Moving only (SOG &gt; 0.5)</label>

      <div class="section-label">Layers</div>
      <label><input type="checkbox" v-model="filters.showTrails"> Auto-trails (zoom 7+)</label>
      <label><input type="checkbox" v-model="filters.showGapMarkers"> AIS on/off markers</label>
      <label><input type="checkbox" v-model="filters.showSTS"> STS event markers</label>
      <label><input type="checkbox" v-model="filters.satellite"> Satellite imagery</label>

      <div class="legend-footer">
        <div style="display:flex;align-items:center;gap:5px;padding:1px 0">
          <div class="legend-dot" style="background:#d32f2f" /> = Russian
        </div>
        Arrow = heading/COG
      </div>
    </div>
  </div>
</template>

<style scoped>
.controls-panel {
  position: absolute;
  z-index: 1000;
  top: 12px;
  right: 12px;
  min-width: 200px;
  max-height: calc(100vh - 80px);
  background: rgba(14, 17, 28, 0.94);
  color: #e0e0e0;
  border-radius: 10px;
  font-size: 13px;
  backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.06);
  box-shadow: 0 2px 16px rgba(0, 0, 0, 0.3);
  transition: opacity 0.2s;
}
.panel-head {
  padding: 10px 14px;
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  cursor: pointer;
  display: flex;
  justify-content: space-between;
  align-items: center;
  user-select: none;
}
.panel-head:hover { background: rgba(255, 255, 255, 0.03); border-radius: 10px; }
.min-btn { font-size: 16px; color: #555; transition: color 0.15s; }
.panel-head:hover .min-btn { color: #aaa; }
.panel-body { padding: 8px 14px 12px; max-height: calc(100vh - 130px); overflow-y: auto; }
.section-label {
  font-size: 9px;
  color: #555;
  text-transform: uppercase;
  letter-spacing: 0.8px;
  margin-top: 8px;
  margin-bottom: 4px;
  padding-top: 6px;
  border-top: 1px solid rgba(255, 255, 255, 0.04);
}
.section-label:first-child { margin-top: 0; border-top: none; padding-top: 0; }
label {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  padding: 3px 0;
  font-size: 12px;
  color: #bbb;
  transition: color 0.1s;
}
label:hover { color: #fff; }
input[type='checkbox'] { accent-color: #4fc3f7; width: 13px; height: 13px; }
.legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.range-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 4px 0;
}
.range-row input[type='number'] {
  width: 55px;
  padding: 3px 6px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 6px;
  color: #fff;
  font-size: 12px;
  outline: none;
}
.range-row input:focus { border-color: #4fc3f7; }
.range-row span { font-size: 11px; color: #666; }
.legend-footer {
  margin-top: 8px;
  padding-top: 6px;
  border-top: 1px solid rgba(255, 255, 255, 0.04);
  font-size: 10px;
  color: #444;
}
</style>

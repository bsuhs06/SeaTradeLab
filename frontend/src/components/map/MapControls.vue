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
      <label><input type="checkbox" v-model="filters.showTrails"> Auto-trails (zoom 10+)</label>
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
  top: 10px;
  right: 10px;
  min-width: 200px;
  max-height: calc(100vh - 80px);
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
  cursor: pointer;
  display: flex;
  justify-content: space-between;
  align-items: center;
  user-select: none;
}
.panel-head:hover { background: rgba(255, 255, 255, 0.04); }
.min-btn { font-size: 16px; color: #888; }
.panel-body { padding: 8px 14px 12px; max-height: calc(100vh - 130px); overflow-y: auto; }
.section-label {
  font-size: 10px;
  color: #888;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-top: 8px;
  margin-bottom: 3px;
  padding-top: 6px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}
.section-label:first-child { margin-top: 0; border-top: none; padding-top: 0; }
label {
  display: flex;
  align-items: center;
  gap: 5px;
  cursor: pointer;
  padding: 2px 0;
  font-size: 12px;
}
input[type='checkbox'] { accent-color: #7cb4ff; }
.legend-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  border: 1px solid rgba(255, 255, 255, 0.25);
  flex-shrink: 0;
}
.range-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 4px 0;
}
.range-row input[type='number'] {
  width: 60px;
  padding: 3px 6px;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 4px;
  color: #fff;
  font-size: 12px;
  outline: none;
}
.range-row span { font-size: 11px; color: #999; }
.legend-footer {
  margin-top: 8px;
  padding-top: 6px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 10px;
  color: #555;
}
</style>

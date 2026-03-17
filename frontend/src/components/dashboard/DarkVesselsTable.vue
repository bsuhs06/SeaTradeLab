<script setup lang="ts">
import { ref, computed } from 'vue'
import { formatAgo } from '@/composables/useVesselUtils'
import type { VesselFeature } from '@/types/vessel'

const props = defineProps<{ vessels: VesselFeature[] }>()

const page = ref(1)
const pageSize = ref(25)
const total = computed(() => props.vessels.length)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
const rangeStart = computed(() => total.value === 0 ? 0 : (page.value - 1) * pageSize.value + 1)
const rangeEnd = computed(() => Math.min(page.value * pageSize.value, total.value))
const paginated = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return props.vessels.slice(start, start + pageSize.value)
})
</script>

<template>
  <div class="panel">
    <div class="ph"><span>Dark Vessels</span><span>AIS off 6h+</span></div>
    <div style="max-height: 400px; overflow-y: auto">
      <table>
        <thead><tr><th>Vessel</th><th>Last Seen</th><th>Gap</th></tr></thead>
        <tbody>
          <tr v-if="!vessels.length"><td colspan="3" class="empty">No dark vessels currently</td></tr>
          <tr v-for="f in paginated" :key="f.properties.mmsi">
            <td :class="{ russian: f.properties.is_russian }">
              {{ f.properties.name || f.properties.mmsi }}
              <span v-if="f.properties.is_russian"> [RUS]</span>
            </td>
            <td>{{ formatAgo(f.properties.timestamp) }}</td>
            <td>{{ f.properties.gap_hours != null ? f.properties.gap_hours + 'h' : '--' }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="pagination" v-if="total > 0">
      <button :disabled="page <= 1" @click="page--" class="pg-btn">‹ Prev</button>
      <span class="pg-info">{{ rangeStart.toLocaleString() }}–{{ rangeEnd.toLocaleString() }} of {{ total.toLocaleString() }}</span>
      <button :disabled="page >= totalPages" @click="page++" class="pg-btn">Next ›</button>
      <select v-model.number="pageSize" class="pg-size" @change="page = 1">
        <option :value="10">10/page</option>
        <option :value="25">25/page</option>
        <option :value="50">50/page</option>
        <option :value="100">100/page</option>
      </select>
    </div>
  </div>
</template>

<style scoped>
.panel { background: rgba(20, 24, 33, 0.85); border-radius: 8px; border: 1px solid rgba(255, 255, 255, 0.07); overflow: hidden; }
.ph { padding: 13px 18px; font-size: 14px; font-weight: 600; border-bottom: 1px solid rgba(255, 255, 255, 0.07); display: flex; justify-content: space-between; align-items: center; }
.ph span:last-child { font-size: 11px; color: #888; font-weight: 400; }
table { width: 100%; border-collapse: collapse; }
th { text-align: left; padding: 9px 14px; font-size: 11px; color: #888; text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid rgba(255, 255, 255, 0.08); }
td { padding: 9px 14px; font-size: 13px; border-bottom: 1px solid rgba(255, 255, 255, 0.04); }
tr:hover { background: rgba(255, 255, 255, 0.02); }
.empty { text-align: center; padding: 32px; color: #555; font-size: 13px; }
.russian { color: #ff6b6b; }
.pagination { display: flex; align-items: center; justify-content: center; gap: 12px; padding: 12px 18px; border-top: 1px solid rgba(255, 255, 255, 0.06); }
.pg-btn { background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 5px; color: #ccc; font-size: 12px; padding: 5px 14px; cursor: pointer; }
.pg-btn:disabled { opacity: 0.3; cursor: not-allowed; }
.pg-btn:hover:not(:disabled) { background: rgba(255, 255, 255, 0.1); }
.pg-info { font-size: 12px; color: #888; }
.pg-size { background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 5px; color: #ddd; font-size: 11px; padding: 4px 8px; }
</style>

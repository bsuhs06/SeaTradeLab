<script setup lang="ts">
import { ref, computed } from 'vue'
import { formatTime, formatDuration } from '@/composables/useVesselUtils'
import type { PortVisit } from '@/types/vessel'

const props = defineProps<{ visits: PortVisit[] }>()
const mode = ref<'foreign' | 'all'>('foreign')

const emit = defineEmits<{ modeChange: [nonRussian: boolean] }>()

const page = ref(1)
const pageSize = ref(25)
const total = computed(() => props.visits.length)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
const rangeStart = computed(() => total.value === 0 ? 0 : (page.value - 1) * pageSize.value + 1)
const rangeEnd = computed(() => Math.min(page.value * pageSize.value, total.value))
const paginated = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return props.visits.slice(start, start + pageSize.value)
})

function setMode(m: 'foreign' | 'all') {
  mode.value = m
  page.value = 1
  emit('modeChange', m === 'foreign')
}
</script>

<template>
  <div class="panel">
    <div class="ph"><span>Vessels Visiting Russian Ports</span><span>Last 30 days</span></div>
    <div class="tab-row">
      <button class="tab-btn" :class="{ active: mode === 'foreign' }" @click="setMode('foreign')">Non-Russian Only</button>
      <button class="tab-btn" :class="{ active: mode === 'all' }" @click="setMode('all')">All Vessels</button>
    </div>
    <div style="overflow-x: auto; max-height: 500px; overflow-y: auto">
      <table>
        <thead><tr><th>Vessel</th><th>Flag</th><th>Type</th><th>Port</th><th>Arrived</th><th>Duration</th><th>Status</th><th></th></tr></thead>
        <tbody>
          <tr v-if="!visits.length"><td colspan="8" class="empty">No Russian port visits detected yet</td></tr>
          <tr v-for="v in paginated" :key="v.mmsi + v.arrival_time">
            <td :class="v.is_russian ? 'russian' : 'foreign'">{{ v.vessel_name || v.mmsi }}</td>
            <td><span class="badge" :class="v.is_russian ? 'russian' : 'foreign'">{{ v.flag_country || 'Unknown' }}</span></td>
            <td>{{ v.vessel_type || '--' }}</td>
            <td>{{ v.port_name }}</td>
            <td>{{ formatTime(v.arrival_time) }}</td>
            <td>{{ formatDuration(v.duration_hours) }}</td>
            <td><span v-if="v.still_in_port" class="badge inport">IN PORT</span></td>
            <td>
              <router-link
                v-if="v.port_lat && v.port_lon"
                :to="{ name: 'map', hash: `#goto=${v.port_lat},${v.port_lon},13` }"
                class="link"
              >Map</router-link>
            </td>
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
.panel { background: rgba(20, 24, 33, 0.85); border-radius: 8px; border: 1px solid rgba(255, 255, 255, 0.07); overflow: hidden; margin-bottom: 20px; }
.ph { padding: 13px 18px; font-size: 14px; font-weight: 600; border-bottom: 1px solid rgba(255, 255, 255, 0.07); display: flex; justify-content: space-between; align-items: center; }
.ph span:last-child { font-size: 11px; color: #888; font-weight: 400; }
.tab-row { display: flex; gap: 0; border-bottom: 1px solid rgba(255, 255, 255, 0.07); }
.tab-btn { padding: 10px 20px; font-size: 13px; font-weight: 600; cursor: pointer; background: transparent; border: none; color: #888; border-bottom: 2px solid transparent; }
.tab-btn.active { color: #7cb4ff; border-bottom-color: #7cb4ff; }
.tab-btn:hover { color: #ccc; }
table { width: 100%; border-collapse: collapse; }
th { text-align: left; padding: 9px 14px; font-size: 11px; color: #888; text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid rgba(255, 255, 255, 0.08); }
td { padding: 9px 14px; font-size: 13px; border-bottom: 1px solid rgba(255, 255, 255, 0.04); }
tr:hover { background: rgba(255, 255, 255, 0.02); }
.empty { text-align: center; padding: 32px; color: #555; font-size: 13px; }
.russian { color: #ff6b6b; }
.foreign { color: #ffa726; }
.badge { display: inline-block; padding: 2px 8px; border-radius: 10px; font-size: 11px; font-weight: 600; }
.badge.russian { background: rgba(255, 59, 48, 0.15); color: #ff6b6b; }
.badge.foreign { background: rgba(255, 159, 0, 0.2); color: #ffa726; }
.badge.inport { background: rgba(76, 175, 80, 0.2); color: #66bb6a; }
.link { color: #7cb4ff; cursor: pointer; text-decoration: none; }
.pagination { display: flex; align-items: center; justify-content: center; gap: 12px; padding: 12px 18px; border-top: 1px solid rgba(255, 255, 255, 0.06); }
.pg-btn { background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 5px; color: #ccc; font-size: 12px; padding: 5px 14px; cursor: pointer; }
.pg-btn:disabled { opacity: 0.3; cursor: not-allowed; }
.pg-btn:hover:not(:disabled) { background: rgba(255, 255, 255, 0.1); }
.pg-info { font-size: 12px; color: #888; }
.pg-size { background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 5px; color: #ddd; font-size: 11px; padding: 4px 8px; }
</style>

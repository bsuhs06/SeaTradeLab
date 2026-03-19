<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '@/api/client'
import { formatAgo, formatTime } from '@/composables/useVesselUtils'
import type { DestinationAnomaly, DestinationChange } from '@/types/vessel'

const anomalies = ref<DestinationAnomaly[]>([])
const changes = ref<DestinationChange[]>([])
const loading = ref(false)
const tab = ref<'anomalies' | 'changes'>('anomalies')
const searchQ = ref('')
const reasonFilter = ref('')
const expandedMMSI = ref<number | null>(null)
const vesselChanges = ref<DestinationChange[]>([])
const loadingChanges = ref(false)

const page = ref(1)
const pageSize = ref(25)

const reasonLabels: Record<string, string> = {
  message_keywords: 'Message Keywords',
  long_multi_word: 'Long Message',
  multi_word_message: 'Multi-Word',
  frequent_changes: 'Frequent Changes',
  unusual_format: 'Unusual Format',
}

const reasonColors: Record<string, string> = {
  message_keywords: '#ff6b6b',
  long_multi_word: '#ffa94d',
  multi_word_message: '#ffd43b',
  frequent_changes: '#69db7c',
  unusual_format: '#74c0fc',
}

const filtered = computed(() => {
  let list = tab.value === 'anomalies' ? anomalies.value : []
  if (searchQ.value) {
    const q = searchQ.value.toLowerCase()
    list = list.filter(
      (a) =>
        a.destination.toLowerCase().includes(q) ||
        (a.name && a.name.toLowerCase().includes(q)) ||
        String(a.mmsi).includes(q),
    )
  }
  if (reasonFilter.value) {
    list = list.filter((a) => a.reason === reasonFilter.value)
  }
  return list
})

const filteredChanges = computed(() => {
  if (tab.value !== 'changes') return []
  let list = changes.value
  if (searchQ.value) {
    const q = searchQ.value.toLowerCase()
    list = list.filter(
      (c) =>
        (c.old_value && c.old_value.toLowerCase().includes(q)) ||
        (c.new_value && c.new_value.toLowerCase().includes(q)) ||
        (c.name && c.name.toLowerCase().includes(q)) ||
        String(c.mmsi).includes(q),
    )
  }
  return list
})

const currentList = computed(() => (tab.value === 'anomalies' ? filtered.value : filteredChanges.value))
const total = computed(() => currentList.value.length)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
const rangeStart = computed(() => (total.value === 0 ? 0 : (page.value - 1) * pageSize.value + 1))
const rangeEnd = computed(() => Math.min(page.value * pageSize.value, total.value))
const paginated = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return currentList.value.slice(start, start + pageSize.value)
})

async function load() {
  loading.value = true
  try {
    const data = await api.getDestinationAnomalies(168)
    anomalies.value = data.anomalies || []
    changes.value = data.changes || []
  } catch {
    /* ignore */
  } finally {
    loading.value = false
  }
}

async function toggleExpand(mmsi: number) {
  if (expandedMMSI.value === mmsi) {
    expandedMMSI.value = null
    return
  }
  expandedMMSI.value = mmsi
  loadingChanges.value = true
  try {
    const data = await api.getDestinationChanges(mmsi)
    vesselChanges.value = data.changes || []
  } catch {
    vesselChanges.value = []
  } finally {
    loadingChanges.value = false
  }
}

function resetPage() {
  page.value = 1
}

function openMap(lat: number, lon: number) {
  window.open(`https://www.google.com/maps?q=${lat},${lon}`, '_blank')
}

onMounted(load)
</script>

<template>
  <div class="panel">
    <div class="toolbar">
      <div class="tabs">
        <button :class="{ active: tab === 'anomalies' }" @click="tab = 'anomalies'; resetPage()">
          Anomalous Destinations ({{ anomalies.length }})
        </button>
        <button :class="{ active: tab === 'changes' }" @click="tab = 'changes'; resetPage()">
          Recent Changes ({{ changes.length }})
        </button>
      </div>
      <div class="filters">
        <input v-model="searchQ" placeholder="Search vessel, MMSI, or destination..." class="search" @input="resetPage" />
        <select v-if="tab === 'anomalies'" v-model="reasonFilter" class="filter-select" @change="resetPage">
          <option value="">All Reasons</option>
          <option v-for="(label, key) in reasonLabels" :key="key" :value="key">{{ label }}</option>
        </select>
        <button class="refresh-btn" @click="load" :disabled="loading">{{ loading ? 'Loading...' : 'Refresh' }}</button>
      </div>
    </div>

    <!-- Anomalies tab -->
    <div v-if="tab === 'anomalies'" style="max-height: 500px; overflow-y: auto">
      <table>
        <thead>
          <tr>
            <th>Vessel</th>
            <th>Destination</th>
            <th>Reason</th>
            <th>Changes</th>
            <th>Last Location</th>
            <th>Last Seen</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!filtered.length">
            <td colspan="6" class="empty">{{ loading ? 'Loading...' : 'No anomalous destinations detected' }}</td>
          </tr>
          <template v-for="a in (paginated as DestinationAnomaly[])" :key="a.mmsi">
            <tr class="clickable" @click="toggleExpand(a.mmsi)">
              <td>
                <div class="vessel-name">{{ a.name || a.mmsi }}</div>
                <div class="vessel-sub">{{ a.mmsi }} · {{ a.vessel_type_name || 'Unknown' }}</div>
              </td>
              <td class="dest-cell">
                <code>{{ a.destination }}</code>
              </td>
              <td>
                <span class="reason-badge" :style="{ background: reasonColors[a.reason] || '#888' }">
                  {{ reasonLabels[a.reason] || a.reason }}
                </span>
              </td>
              <td class="center">{{ a.change_count }}</td>
              <td class="loc-cell">
                <span v-if="a.latitude && a.longitude" class="loc-link" @click.stop="openMap(a.latitude, a.longitude)">
                  {{ a.latitude.toFixed(3) }}, {{ a.longitude.toFixed(3) }}
                </span>
                <span v-else class="no-loc">—</span>
              </td>
              <td>{{ formatAgo(a.last_seen_at) }}</td>
            </tr>
            <tr v-if="expandedMMSI === a.mmsi" class="expand-row">
              <td colspan="6">
                <div class="expand-content">
                  <div v-if="loadingChanges" class="expand-loading">Loading change history...</div>
                  <div v-else-if="!vesselChanges.length" class="expand-loading">No destination changes recorded yet</div>
                  <table v-else class="inner-table">
                    <thead>
                      <tr><th>From</th><th>To</th><th>When</th></tr>
                    </thead>
                    <tbody>
                      <tr v-for="c in vesselChanges" :key="c.id">
                        <td><code>{{ c.old_value || '(none)' }}</code></td>
                        <td><code>{{ c.new_value || '(none)' }}</code></td>
                        <td>{{ formatTime(c.changed_at) }}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <!-- Changes tab -->
    <div v-if="tab === 'changes'" style="max-height: 500px; overflow-y: auto">
      <table>
        <thead>
          <tr>
            <th>Vessel</th>
            <th>From</th>
            <th>To</th>
            <th>When</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!filteredChanges.length">
            <td colspan="4" class="empty">{{ loading ? 'Loading...' : 'No destination changes in the last 7 days' }}</td>
          </tr>
          <tr v-for="c in (paginated as DestinationChange[])" :key="c.id">
            <td>
              <div class="vessel-name">{{ c.name || c.mmsi }}</div>
              <div class="vessel-sub">{{ c.mmsi }}</div>
            </td>
            <td><code>{{ c.old_value || '(none)' }}</code></td>
            <td><code>{{ c.new_value || '(none)' }}</code></td>
            <td>{{ formatTime(c.changed_at) }}</td>
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
.toolbar { padding: 12px 16px; border-bottom: 1px solid rgba(255, 255, 255, 0.07); display: flex; flex-direction: column; gap: 10px; }
.tabs { display: flex; gap: 4px; }
.tabs button {
  background: rgba(255, 255, 255, 0.05); border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 5px; color: #aaa; font-size: 12px; padding: 6px 14px; cursor: pointer;
}
.tabs button.active { background: rgba(124, 180, 255, 0.15); color: #7cb4ff; border-color: rgba(124, 180, 255, 0.3); }
.tabs button:hover:not(.active) { background: rgba(255, 255, 255, 0.08); }
.filters { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.search {
  flex: 1; min-width: 180px; background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 5px; color: #ddd; font-size: 12px; padding: 6px 10px; outline: none;
}
.search:focus { border-color: rgba(124, 180, 255, 0.4); }
.search::placeholder { color: #666; }
.filter-select {
  background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 5px; color: #ddd; font-size: 11px; padding: 6px 8px;
}
.refresh-btn {
  background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 5px; color: #ccc; font-size: 12px; padding: 6px 14px; cursor: pointer;
}
.refresh-btn:hover:not(:disabled) { background: rgba(255, 255, 255, 0.1); }
.refresh-btn:disabled { opacity: 0.4; cursor: not-allowed; }
table { width: 100%; border-collapse: collapse; }
th { text-align: left; padding: 9px 14px; font-size: 11px; color: #888; text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid rgba(255, 255, 255, 0.08); }
td { padding: 9px 14px; font-size: 13px; border-bottom: 1px solid rgba(255, 255, 255, 0.04); }
tr:hover { background: rgba(255, 255, 255, 0.02); }
.clickable { cursor: pointer; }
.clickable:hover { background: rgba(124, 180, 255, 0.05); }
.empty { text-align: center; padding: 32px; color: #555; font-size: 13px; }
.vessel-name { font-size: 13px; color: #e0e0e0; }
.vessel-sub { font-size: 11px; color: #666; margin-top: 2px; }
.dest-cell code {
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace; font-size: 12px;
  background: rgba(255, 255, 255, 0.06); padding: 2px 6px; border-radius: 3px; color: #ffa94d;
  word-break: break-all;
}
.reason-badge {
  font-size: 10px; font-weight: 600; padding: 3px 8px; border-radius: 10px;
  color: #111; text-transform: uppercase; letter-spacing: 0.3px; white-space: nowrap;
}
.center { text-align: center; }
.loc-cell { font-size: 12px; white-space: nowrap; }
.loc-link { color: #74c0fc; cursor: pointer; text-decoration: underline; text-decoration-style: dotted; }
.loc-link:hover { color: #a5d8ff; }
.no-loc { color: #555; }
.expand-row { background: rgba(124, 180, 255, 0.03); }
.expand-row:hover { background: rgba(124, 180, 255, 0.03); }
.expand-content { padding: 8px 14px 14px; }
.expand-loading { font-size: 12px; color: #666; padding: 12px 0; text-align: center; }
.inner-table { background: rgba(0, 0, 0, 0.2); border-radius: 6px; overflow: hidden; }
.inner-table th { font-size: 10px; padding: 6px 12px; }
.inner-table td { font-size: 12px; padding: 6px 12px; }
.inner-table code {
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace; font-size: 11px;
  background: rgba(255, 255, 255, 0.06); padding: 1px 5px; border-radius: 3px;
}
.pagination { display: flex; align-items: center; justify-content: center; gap: 12px; padding: 12px 18px; border-top: 1px solid rgba(255, 255, 255, 0.06); }
.pg-btn { background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 5px; color: #ccc; font-size: 12px; padding: 5px 14px; cursor: pointer; }
.pg-btn:disabled { opacity: 0.3; cursor: not-allowed; }
.pg-btn:hover:not(:disabled) { background: rgba(255, 255, 255, 0.1); }
.pg-info { font-size: 12px; color: #888; }
.pg-size { background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 5px; color: #ddd; font-size: 11px; padding: 4px 8px; }
</style>

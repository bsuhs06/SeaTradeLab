<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '@/api/client'
import { mmsiToFlag } from '@/composables/useVesselUtils'
import type { VesselTaintRecord, VesselPortCall, VesselEncounter, TaintChainLink } from '@/types/vessel'

const tainted = ref<VesselTaintRecord[]>([])
const loading = ref(false)
const favMMSIs = ref(new Set<number>())

/* ---------- detail panel ---------- */
const detailMMSI = ref<number | null>(null)
const detailTaint = ref<VesselTaintRecord[]>([])
const detailPortCalls = ref<VesselPortCall[]>([])
const detailEncounters = ref<VesselEncounter[]>([])
const detailLoading = ref(false)

/* ---------- taint chain ---------- */
const chainTaintId = ref<number | null>(null)
const chainLinks = ref<TaintChainLink[]>([])
const chainLoading = ref(false)

/* ---------- filter state ---------- */
const search = ref('')
const taintTypeFilter = ref('all')
const sortBy = ref<'time' | 'type'>('time')

/* ---------- pagination ---------- */
const page = ref(1)
const pageSize = ref(25)

async function loadTainted() {
  loading.value = true
  try {
    const [taintData, favData] = await Promise.all([api.getTaintedVessels(), api.getFavorites()])
    tainted.value = taintData.tainted || []
    favMMSIs.value = new Set((favData.favorites || []).map((f: any) => f.mmsi))
  } catch { /* ignore */ }
  loading.value = false
}

async function openDetail(mmsi: number) {
  if (detailMMSI.value === mmsi) {
    detailMMSI.value = null
    return
  }
  detailMMSI.value = mmsi
  detailLoading.value = true
  try {
    const data = await api.getVesselTaintDetail(mmsi)
    detailTaint.value = data.taint || []
    detailPortCalls.value = data.port_calls || []
    detailEncounters.value = data.encounters || []
  } catch { /* ignore */ }
  detailLoading.value = false
}

async function showChain(taintId: number) {
  if (chainTaintId.value === taintId) {
    chainTaintId.value = null
    return
  }
  chainTaintId.value = taintId
  chainLoading.value = true
  try {
    const data = await api.getTaintChain(taintId)
    chainLinks.value = data.chain || []
  } catch { /* ignore */ }
  chainLoading.value = false
}

/* ---------- filtering ---------- */
const filtered = computed(() => {
  let list = tainted.value
  if (search.value) {
    const q = search.value.toLowerCase()
    list = list.filter(t =>
      (t.vessel_name || '').toLowerCase().includes(q) ||
      String(t.mmsi).includes(q) ||
      (t.source_name || '').toLowerCase().includes(q) ||
      (t.reason || '').toLowerCase().includes(q)
    )
  }
  if (taintTypeFilter.value !== 'all') list = list.filter(t => t.taint_type === taintTypeFilter.value)

  // Deduplicate by mmsi — show most recent taint per vessel
  const byMMSI = new Map<number, VesselTaintRecord>()
  for (const t of list) {
    if (!byMMSI.has(t.mmsi) || new Date(t.tainted_at) > new Date(byMMSI.get(t.mmsi)!.tainted_at)) {
      byMMSI.set(t.mmsi, t)
    }
  }
  let deduped = [...byMMSI.values()]

  if (sortBy.value === 'type') deduped.sort((a, b) => a.taint_type.localeCompare(b.taint_type))
  else deduped.sort((a, b) => new Date(b.tainted_at).getTime() - new Date(a.tainted_at).getTime())
  return deduped
})

const totalFiltered = computed(() => filtered.value.length)
const totalPages = computed(() => Math.max(1, Math.ceil(totalFiltered.value / pageSize.value)))
const rangeStart = computed(() => totalFiltered.value === 0 ? 0 : (page.value - 1) * pageSize.value + 1)
const rangeEnd = computed(() => Math.min(page.value * pageSize.value, totalFiltered.value))
const paginated = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return filtered.value.slice(start, start + pageSize.value)
})

// Reset page on filter change
computed(() => { search.value; taintTypeFilter.value; page.value = 1; return 0 })

function flagFor(mmsi: number): string {
  const [emoji] = mmsiToFlag(mmsi)
  return emoji
}

function fmtTime(ts: string): string {
  return new Date(ts).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

function taintLabel(type: string): string {
  if (type === 'russian_port') return 'Russian Port'
  if (type === 'encounter') return 'Encounter'
  if (type === 'no_subsequent_port') return 'No Clean Port'
  return type
}

function taintBadgeClass(type: string): string {
  if (type === 'russian_port') return 'badge-red'
  if (type === 'encounter') return 'badge-orange'
  if (type === 'no_subsequent_port') return 'badge-yellow'
  return ''
}

async function toggleBookmark(t: VesselTaintRecord) {
  if (favMMSIs.value.has(t.mmsi)) {
    await api.removeFavorite(t.mmsi)
    favMMSIs.value.delete(t.mmsi)
  } else {
    await api.addFavorite(t.mmsi, t.vessel_name, undefined)
    favMMSIs.value.add(t.mmsi)
  }
}

function viewOnMap(mmsi: number) {
  window.location.href = `/map#search=${mmsi}`
}

onMounted(loadTainted)
</script>

<template>
  <div class="panel">
    <div class="ph">
      <span>Tainted Petroleum Tankers</span>
      <span>{{ filtered.length }} vessels · Petroleum tankers only</span>
    </div>

    <div class="filters">
      <div class="filter-row">
        <input v-model="search" type="text" placeholder="Search vessel, MMSI, source…" class="finput search-input" />
        <select v-model="taintTypeFilter" class="finput">
          <option value="all">All Types</option>
          <option value="russian_port">Russian Port</option>
          <option value="encounter">Encounter</option>
          <option value="no_subsequent_port">No Clean Port</option>
        </select>
        <select v-model="sortBy" class="finput">
          <option value="time">Sort: Recent</option>
          <option value="type">Sort: Type</option>
        </select>
        <select v-model="pageSize" class="finput" @change="page = 1">
          <option :value="10">10/page</option>
          <option :value="25">25/page</option>
          <option :value="50">50/page</option>
          <option :value="100">100/page</option>
        </select>
        <button v-if="!loading" class="btn-sm" @click="loadTainted">Refresh</button>
      </div>
    </div>

    <div v-if="loading" class="loading">Loading tainted vessels…</div>

    <table v-else class="tbl">
      <thead>
        <tr>
          <th>Flag</th>
          <th>Vessel</th>
          <th>MMSI</th>
          <th>Taint Type</th>
          <th>Reason</th>
          <th>Source</th>
          <th>Tainted</th>
          <th>Expires</th>
          <th></th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <template v-for="t in paginated" :key="t.id">
          <tr :class="{ active: detailMMSI === t.mmsi }" @click="openDetail(t.mmsi)">
            <td>{{ flagFor(t.mmsi) }}</td>
            <td class="name">{{ t.vessel_name || '—' }}</td>
            <td class="mono">{{ t.mmsi }}</td>
            <td><span :class="['badge', taintBadgeClass(t.taint_type)]">{{ taintLabel(t.taint_type) }}</span></td>
            <td class="reason">{{ t.reason || '—' }}</td>
            <td>
              <span v-if="t.source_name">{{ flagFor(t.source_mmsi!) }} {{ t.source_name }}</span>
              <span v-else>—</span>
            </td>
            <td class="mono">{{ fmtTime(t.tainted_at) }}</td>
            <td class="mono">{{ fmtTime(t.expires_at) }}</td>
            <td>
              <button v-if="t.source_taint_id" class="btn-chain" @click.stop="showChain(t.id)" title="Trace taint chain">🔗</button>
            </td>
            <td class="action-btns">
              <button class="btn-map" @click.stop="viewOnMap(t.mmsi)" title="View on map">🗺️</button>
              <button class="btn-fav" :class="{ active: favMMSIs.has(t.mmsi) }" @click.stop="toggleBookmark(t)" :title="favMMSIs.has(t.mmsi) ? 'Remove bookmark' : 'Bookmark vessel'">★</button>
            </td>
          </tr>

          <!-- Detail panel -->
          <tr v-if="detailMMSI === t.mmsi" class="detail-row">
            <td colspan="10">
              <div v-if="detailLoading" class="loading">Loading details…</div>
              <div v-else class="detail-panel">
                <!-- All taint records for this vessel -->
                <div class="detail-section">
                  <h4>All Taint Records ({{ detailTaint.length }})</h4>
                  <table class="tbl inner-tbl">
                    <thead><tr><th>Type</th><th>Reason</th><th>Source</th><th>When</th><th>Active</th><th></th></tr></thead>
                    <tbody>
                      <tr v-for="dt in detailTaint" :key="dt.id">
                        <td><span :class="['badge', taintBadgeClass(dt.taint_type)]">{{ taintLabel(dt.taint_type) }}</span></td>
                        <td class="reason">{{ dt.reason || '—' }}</td>
                        <td>{{ dt.source_name ? `${flagFor(dt.source_mmsi!)} ${dt.source_name}` : '—' }}</td>
                        <td class="mono">{{ fmtTime(dt.tainted_at) }}</td>
                        <td>{{ dt.active ? '✓' : '✗' }}</td>
                        <td><button v-if="dt.source_taint_id" class="btn-chain" @click.stop="showChain(dt.id)" title="Trace chain">🔗</button></td>
                      </tr>
                    </tbody>
                  </table>
                </div>

                <!-- Port calls -->
                <div class="detail-section">
                  <h4>Port Calls ({{ detailPortCalls.length }})</h4>
                  <table v-if="detailPortCalls.length" class="tbl inner-tbl">
                    <thead><tr><th>Port</th><th>Country</th><th>Arrival</th><th>Departure</th><th>Duration</th><th>Status</th></tr></thead>
                    <tbody>
                      <tr v-for="pc in detailPortCalls" :key="pc.id" :class="{ 'russian-port': pc.port_country === 'Russia' }">
                        <td>{{ pc.port_name }}</td>
                        <td>{{ pc.port_country || '—' }}</td>
                        <td class="mono">{{ fmtTime(pc.arrival_time) }}</td>
                        <td class="mono">{{ pc.departure_time ? fmtTime(pc.departure_time) : '—' }}</td>
                        <td>{{ pc.duration_hours ? pc.duration_hours.toFixed(1) + 'h' : '—' }}</td>
                        <td>{{ pc.still_in_port ? '🟢 In Port' : 'Departed' }}</td>
                      </tr>
                    </tbody>
                  </table>
                  <p v-else class="empty">No port calls recorded</p>
                </div>

                <!-- Encounters -->
                <div class="detail-section">
                  <h4>Encounters ({{ detailEncounters.length }})</h4>
                  <table v-if="detailEncounters.length" class="tbl inner-tbl">
                    <thead><tr><th>Vessel A</th><th>Vessel B</th><th>Start</th><th>Duration</th><th>Min Dist</th><th>Location</th></tr></thead>
                    <tbody>
                      <tr v-for="enc in detailEncounters" :key="enc.id">
                        <td>{{ flagFor(enc.mmsi_a) }} {{ enc.name_a || enc.mmsi_a }}</td>
                        <td>{{ flagFor(enc.mmsi_b) }} {{ enc.name_b || enc.mmsi_b }}</td>
                        <td class="mono">{{ fmtTime(enc.start_time) }}</td>
                        <td>{{ enc.duration_minutes }}min</td>
                        <td>{{ enc.min_distance_m?.toFixed(0) }}m</td>
                        <td class="mono">{{ enc.avg_lat?.toFixed(3) }}°, {{ enc.avg_lon?.toFixed(3) }}°</td>
                      </tr>
                    </tbody>
                  </table>
                  <p v-else class="empty">No encounters recorded</p>
                </div>
              </div>
            </td>
          </tr>

          <!-- Chain panel -->
          <tr v-if="chainTaintId === t.id" class="chain-row">
            <td colspan="10">
              <div v-if="chainLoading" class="loading">Loading taint chain…</div>
              <div v-else class="chain-panel">
                <h4>Taint Chain ({{ chainLinks.length }} links)</h4>
                <div v-for="(link, idx) in chainLinks" :key="link.taint.id" class="chain-link">
                  <div class="chain-indicator">
                    <span class="chain-num">{{ idx + 1 }}</span>
                    <span v-if="idx < chainLinks.length - 1" class="chain-line"></span>
                  </div>
                  <div class="chain-content">
                    <div class="chain-header">
                      {{ flagFor(link.taint.mmsi) }}
                      <strong>{{ link.taint.vessel_name || link.taint.mmsi }}</strong>
                      <span :class="['badge', taintBadgeClass(link.taint.taint_type)]">{{ taintLabel(link.taint.taint_type) }}</span>
                    </div>
                    <div class="chain-reason">{{ link.taint.reason }}</div>
                    <div v-if="link.port_call" class="chain-detail">
                      Port: {{ link.port_call.port_name }} ({{ link.port_call.port_country }})
                      · Arrived {{ fmtTime(link.port_call.arrival_time) }}
                      <span v-if="link.port_call.duration_hours"> · {{ link.port_call.duration_hours.toFixed(1) }}h</span>
                    </div>
                    <div v-if="link.encounter" class="chain-detail">
                      Encounter: {{ link.encounter.name_a || link.encounter.mmsi_a }} ↔ {{ link.encounter.name_b || link.encounter.mmsi_b }}
                      · {{ link.encounter.duration_minutes }}min · {{ link.encounter.min_distance_m?.toFixed(0) }}m
                    </div>
                  </div>
                </div>
              </div>
            </td>
          </tr>
        </template>
      </tbody>
    </table>

    <div class="pager">
      <button :disabled="page <= 1" @click="page--">&laquo; Prev</button>
      <span>{{ rangeStart }}–{{ rangeEnd }} of {{ totalFiltered.toLocaleString() }}</span>
      <button :disabled="page >= totalPages" @click="page++">Next &raquo;</button>
    </div>
  </div>
</template>

<style scoped>
.panel { font-size: 13px; }
.ph { display: flex; justify-content: space-between; align-items: center; padding: 0.5rem 0; font-weight: 600; color: #ccc; }
.ph span:last-child { font-weight: 400; font-size: 12px; color: #888; }
.filters { margin-bottom: 0.5rem; }
.filter-row { display: flex; gap: 0.4rem; flex-wrap: wrap; align-items: center; }
.finput { background: #1e1e1e; border: 1px solid #333; color: #ccc; padding: 4px 8px; border-radius: 4px; font-size: 12px; }
.search-input { flex: 1; min-width: 160px; }
.loading { text-align: center; padding: 2rem; color: #888; }
.tbl { width: 100%; border-collapse: collapse; }
.tbl th { text-align: left; padding: 6px 8px; border-bottom: 1px solid #333; color: #888; font-size: 11px; text-transform: uppercase; white-space: nowrap; }
.tbl td { padding: 6px 8px; border-bottom: 1px solid #1e1e1e; white-space: nowrap; }
.tbl tbody tr:hover { background: #1e1e1e; cursor: pointer; }
.tbl tbody tr.active { background: #1a2a1a; }
.inner-tbl { margin-top: 0.3rem; font-size: 12px; }
.inner-tbl td, .inner-tbl th { padding: 4px 6px; }
.mono { font-family: 'SF Mono', 'Fira Code', monospace; font-size: 11px; }
.name { font-weight: 500; }
.reason { max-width: 300px; overflow: hidden; text-overflow: ellipsis; }
.badge { padding: 2px 6px; border-radius: 3px; font-size: 11px; font-weight: 500; }
.badge-red { background: #3a1a1a; color: #f88; }
.badge-orange { background: #3a2a1a; color: #fb5; }
.badge-yellow { background: #3a3a1a; color: #fd5; }
.russian-port { background: #2a1a1a !important; }
.btn-sm { background: #2a2a2a; border: 1px solid #444; color: #ccc; padding: 3px 10px; border-radius: 4px; cursor: pointer; font-size: 12px; }
.btn-sm:hover { background: #333; }
.btn-chain { background: none; border: none; cursor: pointer; font-size: 14px; padding: 2px 4px; }
.btn-chain:hover { transform: scale(1.2); }

.action-btns { white-space: nowrap; }
.btn-map { background: none; border: 1px solid rgba(124,180,255,0.3); border-radius: 4px; padding: 1px 5px; cursor: pointer; font-size: 13px; margin-right: 3px; }
.btn-map:hover { background: rgba(124,180,255,0.15); }
.btn-fav { background: none; border: 1px solid rgba(255,215,0,0.3); border-radius: 4px; padding: 1px 5px; cursor: pointer; font-size: 14px; color: #555; }
.btn-fav:hover { color: #ffd700; }
.btn-fav.active { color: #ffd700; border-color: rgba(255,215,0,0.5); }

.detail-row td { padding: 0.5rem 0.8rem; background: #141414; }
.detail-panel { display: flex; flex-direction: column; gap: 1rem; }
.detail-section h4 { color: #aaa; margin: 0 0 0.3rem; font-size: 12px; text-transform: uppercase; }
.empty { color: #666; font-size: 12px; margin: 0.3rem 0; }

.chain-row td { padding: 0.5rem 0.8rem; background: #0f1f0f; }
.chain-panel h4 { color: #aaa; margin: 0 0 0.6rem; font-size: 12px; text-transform: uppercase; }
.chain-link { display: flex; gap: 0.8rem; }
.chain-indicator { display: flex; flex-direction: column; align-items: center; min-width: 24px; }
.chain-num { background: #2a3a2a; color: #6f6; border-radius: 50%; width: 22px; height: 22px; display: flex; align-items: center; justify-content: center; font-size: 11px; font-weight: 600; }
.chain-line { width: 2px; flex: 1; background: #2a3a2a; min-height: 20px; }
.chain-content { flex: 1; padding-bottom: 0.6rem; }
.chain-header { font-size: 13px; }
.chain-reason { font-size: 12px; color: #999; margin-top: 2px; }
.chain-detail { font-size: 11px; color: #777; margin-top: 2px; }

.pager { display: flex; justify-content: center; align-items: center; gap: 1rem; padding: 0.5rem 0; }
.pager button { background: #2a2a2a; border: 1px solid #444; color: #ccc; padding: 4px 12px; border-radius: 4px; cursor: pointer; font-size: 12px; }
.pager button:disabled { opacity: 0.4; cursor: default; }
.pager span { font-size: 12px; color: #888; }
</style>

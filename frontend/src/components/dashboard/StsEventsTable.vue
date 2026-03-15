<script setup lang="ts">
import { ref, computed } from 'vue'
import { formatTime, mmsiToFlag, coordsToRegion } from '@/composables/useVesselUtils'
import { api } from '@/api/client'
import type { STSEvent } from '@/types/vessel'

const props = defineProps<{ events: STSEvent[] }>()
const emit = defineEmits<{ (e: 'updated'): void }>()

/* ---------- filter state ---------- */
const search = ref('')
const typeA = ref('all')
const typeB = ref('all')
const region = ref('all')
const confidence = ref('all')
const flagFilter = ref('all')
const reviewedFilter = ref('all')
const tagFilter = ref('all')
const minDuration = ref(0)
const sortBy = ref<'time' | 'duration' | 'distance'>('time')

/* ---------- pagination ---------- */
const page = ref(1)
const pageSize = ref(25)

/* ---------- triage inline editing ---------- */
const editingId = ref<number | null>(null)
const editConf = ref('')
const editTag = ref('')
const editNotes = ref('')
const editReviewed = ref(false)
const saving = ref(false)

function startEdit(e: STSEvent) {
  editingId.value = e.id
  editConf.value = e.confidence
  editTag.value = e.tag || ''
  editNotes.value = e.notes || ''
  editReviewed.value = e.reviewed
}

function cancelEdit() { editingId.value = null }

async function saveEdit() {
  if (editingId.value == null) return
  saving.value = true
  try {
    await api.updateSTSEvent(editingId.value, {
      confidence: editConf.value,
      reviewed: editReviewed.value,
      tag: editTag.value || null,
      notes: editNotes.value || null,
    })
    emit('updated')
  } catch (err) {
    console.error('Failed to save STS event', err)
  } finally {
    saving.value = false
    editingId.value = null
  }
}

async function quickToggleReviewed(e: STSEvent) {
  try {
    await api.updateSTSEvent(e.id, {
      confidence: e.confidence,
      reviewed: !e.reviewed,
      tag: e.tag || null,
      notes: e.notes || null,
    })
    emit('updated')
  } catch (err) {
    console.error('Failed to toggle reviewed', err)
  }
}

/* ---------- derived option lists ---------- */
const typesA = computed(() => {
  const s = new Set<string>()
  for (const e of props.events) s.add(shortType(e.type_a))
  s.delete('--')
  return [...s].sort()
})

const typesB = computed(() => {
  const s = new Set<string>()
  for (const e of props.events) s.add(shortType(e.type_b))
  s.delete('--')
  return [...s].sort()
})

const regions = computed(() => {
  const s = new Set<string>()
  for (const e of props.events) {
    const r = regionFor(e)
    if (r !== '--') s.add(r)
  }
  return [...s].sort()
})

const flagOptions = computed(() => {
  const m = new Map<string, string>()
  for (const e of props.events) {
    addFlag(m, e.mmsi_a)
    addFlag(m, e.mmsi_b)
  }
  return [...m.entries()].sort((a, b) => a[1].localeCompare(b[1]))
})

const tagOptions = computed(() => {
  const s = new Set<string>()
  for (const e of props.events) if (e.tag) s.add(e.tag)
  return [...s].sort()
})

function addFlag(m: Map<string, string>, mmsi: number) {
  const [emoji, code] = mmsiToFlag(mmsi)
  if (code && !m.has(code)) m.set(code, `${emoji} ${code}`)
}

/* ---------- filtering + sorting ---------- */
const filtered = computed(() => {
  let list = props.events

  if (search.value) {
    const q = search.value.toLowerCase()
    list = list.filter(e =>
      (e.name_a || '').toLowerCase().includes(q) ||
      (e.name_b || '').toLowerCase().includes(q) ||
      String(e.mmsi_a).includes(q) ||
      String(e.mmsi_b).includes(q)
    )
  }

  if (typeA.value !== 'all') list = list.filter(e => shortType(e.type_a) === typeA.value)
  if (typeB.value !== 'all') list = list.filter(e => shortType(e.type_b) === typeB.value)
  if (region.value !== 'all') { const r = region.value; list = list.filter(e => regionFor(e) === r) }
  if (confidence.value !== 'all') list = list.filter(e => e.confidence === confidence.value)

  if (reviewedFilter.value === 'yes') list = list.filter(e => e.reviewed)
  else if (reviewedFilter.value === 'no') list = list.filter(e => !e.reviewed)

  if (tagFilter.value !== 'all') list = list.filter(e => (e.tag || '') === tagFilter.value)

  if (flagFilter.value === 'russian') {
    list = list.filter(e => String(e.mmsi_a).startsWith('273') || String(e.mmsi_b).startsWith('273'))
  } else if (flagFilter.value !== 'all') {
    list = list.filter(e => {
      const [, cA] = mmsiToFlag(e.mmsi_a)
      const [, cB] = mmsiToFlag(e.mmsi_b)
      return cA === flagFilter.value || cB === flagFilter.value
    })
  }

  if (minDuration.value > 0) list = list.filter(e => e.duration_minutes >= minDuration.value)

  const sorted = [...list]
  if (sortBy.value === 'duration') sorted.sort((a, b) => b.duration_minutes - a.duration_minutes)
  else if (sortBy.value === 'distance') sorted.sort((a, b) => (a.min_distance_m ?? 9999) - (b.min_distance_m ?? 9999))
  else sorted.sort((a, b) => new Date(b.start_time).getTime() - new Date(a.start_time).getTime())
  return sorted
})

/* ---------- paginated ---------- */
const totalPages = computed(() => Math.max(1, Math.ceil(filtered.value.length / pageSize.value)))
const paginated = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return filtered.value.slice(start, start + pageSize.value)
})

// Reset page when filters change
const activeFilterCount = computed(() => {
  let n = 0
  if (search.value) n++
  if (typeA.value !== 'all') n++
  if (typeB.value !== 'all') n++
  if (region.value !== 'all') n++
  if (confidence.value !== 'all') n++
  if (flagFilter.value !== 'all') n++
  if (reviewedFilter.value !== 'all') n++
  if (tagFilter.value !== 'all') n++
  if (minDuration.value > 0) n++
  page.value = 1 // side-effect: reset page on filter change
  return n
})

function clearFilters() {
  search.value = ''
  typeA.value = 'all'
  typeB.value = 'all'
  region.value = 'all'
  confidence.value = 'all'
  flagFilter.value = 'all'
  reviewedFilter.value = 'all'
  tagFilter.value = 'all'
  minDuration.value = 0
  sortBy.value = 'time'
  page.value = 1
}

/* ---------- helpers ---------- */
function vesselLabel(name: string | undefined, mmsi: number): string {
  return name || String(mmsi)
}

function flagFor(mmsi: number): string {
  const [emoji] = mmsiToFlag(mmsi)
  return emoji
}

function regionFor(e: STSEvent): string {
  if (e.avg_lat != null && e.avg_lon != null) return coordsToRegion(e.avg_lat, e.avg_lon)
  return '--'
}

function coordStr(lat?: number, lon?: number): string {
  if (lat == null || lon == null) return '--'
  return `${lat.toFixed(3)}°${lat >= 0 ? 'N' : 'S'}, ${lon.toFixed(3)}°${lon >= 0 ? 'E' : 'W'}`
}

function shortType(t?: string): string {
  if (!t) return '--'
  const l = t.toLowerCase()
  if (l.includes('tanker')) return 'Tanker'
  if (l.includes('cargo') || l.includes('container')) return 'Cargo'
  if (l.includes('passenger')) return 'Passenger'
  if (l.includes('tug') || l.includes('tow')) return 'Tug'
  if (l.includes('fish')) return 'Fishing'
  return t.length > 14 ? t.slice(0, 14) + '…' : t
}

const TAG_OPTIONS = ['confirmed', 'suspicious', 'false-positive', 'anchorage', 'needs-review']
</script>

<template>
  <div class="panel">
    <div class="ph">
      <span>Detected Ship-to-Ship Transfers</span>
      <span>{{ filtered.length }} of {{ events.length }} events · Last 7 days</span>
    </div>

    <!-- Filter bar -->
    <div class="filters">
      <div class="filter-row">
        <input v-model="search" type="text" placeholder="Search vessel name or MMSI…" class="finput search-input" />

        <select v-model="typeA" class="finput">
          <option value="all">Type A: All</option>
          <option v-for="t in typesA" :key="t" :value="t">{{ t }}</option>
        </select>

        <select v-model="typeB" class="finput">
          <option value="all">Type B: All</option>
          <option v-for="t in typesB" :key="t" :value="t">{{ t }}</option>
        </select>

        <select v-model="region" class="finput">
          <option value="all">All Regions</option>
          <option v-for="r in regions" :key="r" :value="r">{{ r }}</option>
        </select>

        <select v-model="confidence" class="finput">
          <option value="all">All Confidence</option>
          <option value="high">High</option>
          <option value="medium">Medium</option>
          <option value="low">Low</option>
        </select>

        <select v-model="reviewedFilter" class="finput">
          <option value="all">All Status</option>
          <option value="no">Unreviewed</option>
          <option value="yes">Reviewed</option>
        </select>

        <select v-model="tagFilter" class="finput">
          <option value="all">All Tags</option>
          <option v-for="t in tagOptions" :key="t" :value="t">{{ t }}</option>
        </select>

        <select v-model="flagFilter" class="finput">
          <option value="all">All Flags</option>
          <option value="russian">🇷🇺 Russian Involved</option>
          <option v-for="[code, label] in flagOptions" :key="code" :value="code">{{ label }}</option>
        </select>

        <div class="dur-filter">
          <label>≥ {{ minDuration }}min</label>
          <input v-model.number="minDuration" type="range" min="0" max="120" step="5" class="slider" />
        </div>

        <select v-model="sortBy" class="finput">
          <option value="time">Sort: Newest</option>
          <option value="duration">Sort: Longest</option>
          <option value="distance">Sort: Closest</option>
        </select>

        <button v-if="activeFilterCount > 0" class="clear-btn" @click="clearFilters">
          Clear ({{ activeFilterCount }})
        </button>
      </div>
    </div>

    <div style="overflow-x: auto">
      <table>
        <thead>
          <tr>
            <th>✓</th>
            <th>Time</th>
            <th>Vessel A</th>
            <th>Vessel B</th>
            <th>Region</th>
            <th>Duration</th>
            <th>Min Dist</th>
            <th>Confidence</th>
            <th>Tag</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!paginated.length"><td colspan="10" class="empty">No events match current filters</td></tr>
          <tr v-for="e in paginated" :key="e.id || e.start_time + e.mmsi_a" :class="{ reviewed: e.reviewed }">
            <!-- Reviewed checkbox -->
            <td class="check-cell">
              <input type="checkbox" :checked="e.reviewed" @change="quickToggleReviewed(e)" />
            </td>
            <td class="nowrap">{{ formatTime(e.start_time) }}</td>
            <td>
              <div class="vessel-cell" :class="{ russian: String(e.mmsi_a).startsWith('273') }">
                <span class="flag">{{ flagFor(e.mmsi_a) }}</span>
                <div>
                  <div class="vname">{{ vesselLabel(e.name_a, e.mmsi_a) }}</div>
                  <div class="vmeta">{{ shortType(e.type_a) }} · {{ e.mmsi_a }}</div>
                </div>
              </div>
            </td>
            <td>
              <div class="vessel-cell" :class="{ russian: String(e.mmsi_b).startsWith('273') }">
                <span class="flag">{{ flagFor(e.mmsi_b) }}</span>
                <div>
                  <div class="vname">{{ vesselLabel(e.name_b, e.mmsi_b) }}</div>
                  <div class="vmeta">{{ shortType(e.type_b) }} · {{ e.mmsi_b }}</div>
                </div>
              </div>
            </td>
            <td class="region">{{ regionFor(e) }}</td>
            <td>{{ e.duration_minutes }} min</td>
            <td>{{ e.min_distance_m != null ? Math.round(e.min_distance_m) + 'm' : '--' }}</td>

            <!-- Confidence (inline-editable) -->
            <td>
              <template v-if="editingId === e.id">
                <select v-model="editConf" class="inline-sel">
                  <option value="high">HIGH</option>
                  <option value="medium">MEDIUM</option>
                  <option value="low">LOW</option>
                </select>
              </template>
              <span v-else class="badge" :class="e.confidence" @click="startEdit(e)" style="cursor:pointer" :title="'Click to edit'">{{ e.confidence.toUpperCase() }}</span>
            </td>

            <!-- Tag (inline-editable) -->
            <td>
              <template v-if="editingId === e.id">
                <select v-model="editTag" class="inline-sel">
                  <option value="">None</option>
                  <option v-for="t in TAG_OPTIONS" :key="t" :value="t">{{ t }}</option>
                </select>
              </template>
              <span v-else-if="e.tag" class="tag-badge" @click="startEdit(e)" style="cursor:pointer">{{ e.tag }}</span>
              <span v-else class="no-tag" @click="startEdit(e)" style="cursor:pointer" title="Click to tag">—</span>
            </td>

            <!-- Actions -->
            <td class="actions-cell">
              <template v-if="editingId === e.id">
                <input v-model="editNotes" type="text" placeholder="Notes…" class="inline-notes" />
                <button class="save-btn" @click="saveEdit" :disabled="saving">Save</button>
                <button class="cancel-btn" @click="cancelEdit">✕</button>
              </template>
              <template v-else>
                <span v-if="e.notes" class="notes-icon" :title="e.notes" @click="startEdit(e)">📝</span>
                <router-link
                  v-if="e.avg_lat && e.avg_lon"
                  :to="{ name: 'map', hash: `#goto=${e.avg_lat},${e.avg_lon},14&time=${e.start_time}&mmsi_a=${e.mmsi_a}&mmsi_b=${e.mmsi_b}` }"
                  class="link"
                >View</router-link>
              </template>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    <div class="pagination" v-if="totalPages > 1">
      <button :disabled="page <= 1" @click="page--" class="pg-btn">‹ Prev</button>
      <span class="pg-info">Page {{ page }} of {{ totalPages }}</span>
      <button :disabled="page >= totalPages" @click="page++" class="pg-btn">Next ›</button>
      <select v-model.number="pageSize" class="finput pg-size" @change="page = 1">
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

/* filter bar */
.filters { padding: 10px 14px; border-bottom: 1px solid rgba(255, 255, 255, 0.06); background: rgba(255, 255, 255, 0.02); }
.filter-row { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
.finput {
  background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 5px;
  color: #ddd; font-size: 12px; padding: 6px 10px; outline: none;
}
.finput:focus { border-color: #7cb4ff; }
.search-input { min-width: 180px; flex: 1; }
select.finput { cursor: pointer; padding-right: 20px; }
select.finput option { background: #1a2332; color: #ddd; }
.dur-filter { display: flex; align-items: center; gap: 6px; }
.dur-filter label { font-size: 11px; color: #aaa; white-space: nowrap; min-width: 52px; }
.slider { width: 80px; accent-color: #7cb4ff; cursor: pointer; }
.clear-btn {
  background: rgba(255, 59, 48, 0.15); color: #ff6b6b; border: 1px solid rgba(255, 59, 48, 0.3);
  border-radius: 5px; font-size: 11px; padding: 5px 10px; cursor: pointer; font-weight: 600;
}
.clear-btn:hover { background: rgba(255, 59, 48, 0.25); }

table { width: 100%; border-collapse: collapse; }
th { text-align: left; padding: 9px 14px; font-size: 11px; color: #888; text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid rgba(255, 255, 255, 0.08); }
td { padding: 9px 14px; font-size: 13px; border-bottom: 1px solid rgba(255, 255, 255, 0.04); }
tr:hover { background: rgba(255, 255, 255, 0.02); }
tr.reviewed { opacity: 0.6; }
.empty { text-align: center; padding: 32px; color: #555; font-size: 13px; }
.nowrap { white-space: nowrap; }
.vessel-cell { display: flex; align-items: center; gap: 6px; }
.flag { font-size: 18px; flex-shrink: 0; }
.vname { font-weight: 600; font-size: 13px; line-height: 1.3; }
.vmeta { font-size: 11px; color: #888; line-height: 1.3; }
.russian .vname { color: #ff6b6b; }
.region { color: #7cb4ff; font-size: 12px; }
.coords { font-family: 'SF Mono', Menlo, monospace; font-size: 11px; color: #aaa; }
.badge { display: inline-block; padding: 2px 8px; border-radius: 10px; font-size: 11px; font-weight: 600; }
.badge.high { background: rgba(255, 59, 48, 0.2); color: #ff6b6b; }
.badge.medium { background: rgba(255, 159, 0, 0.2); color: #ffa726; }
.badge.low { background: rgba(76, 175, 80, 0.2); color: #66bb6a; }
.link { color: #7cb4ff; cursor: pointer; }
.check-cell { text-align: center; }
.check-cell input[type="checkbox"] { accent-color: #7cb4ff; cursor: pointer; width: 15px; height: 15px; }

/* tag badge */
.tag-badge {
  display: inline-block; padding: 2px 8px; border-radius: 10px; font-size: 11px; font-weight: 600;
  background: rgba(124, 180, 255, 0.15); color: #7cb4ff;
}
.no-tag { color: #555; font-size: 12px; }

/* inline editing */
.inline-sel {
  background: rgba(255, 255, 255, 0.08); border: 1px solid rgba(124, 180, 255, 0.3); border-radius: 4px;
  color: #ddd; font-size: 11px; padding: 3px 6px; outline: none;
}
.inline-sel option { background: #1a2332; }
.inline-notes {
  background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.15); border-radius: 4px;
  color: #ddd; font-size: 11px; padding: 3px 6px; outline: none; width: 100px;
}
.actions-cell { white-space: nowrap; display: flex; align-items: center; gap: 6px; }
.save-btn {
  background: rgba(76, 175, 80, 0.2); color: #66bb6a; border: 1px solid rgba(76, 175, 80, 0.3);
  border-radius: 4px; font-size: 11px; padding: 3px 8px; cursor: pointer; font-weight: 600;
}
.save-btn:disabled { opacity: 0.5; cursor: wait; }
.cancel-btn {
  background: none; border: none; color: #888; cursor: pointer; font-size: 13px; padding: 2px;
}
.notes-icon { cursor: pointer; font-size: 14px; }

/* pagination */
.pagination {
  display: flex; align-items: center; justify-content: center; gap: 12px;
  padding: 12px 18px; border-top: 1px solid rgba(255, 255, 255, 0.06);
}
.pg-btn {
  background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 5px;
  color: #ccc; font-size: 12px; padding: 5px 14px; cursor: pointer;
}
.pg-btn:disabled { opacity: 0.3; cursor: not-allowed; }
.pg-btn:hover:not(:disabled) { background: rgba(255, 255, 255, 0.1); }
.pg-info { font-size: 12px; color: #888; }
.pg-size { font-size: 11px; padding: 4px 8px; }

@media (max-width: 860px) {
  .filter-row { flex-direction: column; }
  .search-input { min-width: 100%; }
}
</style>

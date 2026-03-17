<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '@/api/client'
import type { VesselRegistryEntry, VesselHistoryRecord, VesselNote } from '@/types/vessel'

const vessels = ref<VesselRegistryEntry[]>([])
const recentChanges = ref<VesselHistoryRecord[]>([])
const searchQuery = ref('')
const tagFilter = ref('')
const allTags = ref<string[]>([])
const loading = ref(false)

// Detail panel
const selectedMMSI = ref<number | null>(null)
const detailHistory = ref<VesselHistoryRecord[]>([])
const detailNotes = ref<VesselNote[]>([])
const detailVessel = ref<any>(null)
const showDetail = ref(false)

// Add note form
const newTag = ref('')
const newNote = ref('')

// Pagination
const page = ref(1)
const pageSize = ref(25)
const total = computed(() => vessels.value.length)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
const rangeStart = computed(() => total.value === 0 ? 0 : (page.value - 1) * pageSize.value + 1)
const rangeEnd = computed(() => Math.min(page.value * pageSize.value, total.value))
const paginatedVessels = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return vessels.value.slice(start, start + pageSize.value)
})

const PRESET_TAGS = [
  'icebreaker', 'military', 'watchlist', 'sanctions', 'shadow-fleet',
  'research', 'government', 'fishing-suspicious', 'flag-hopper', 'note',
]

let searchTimer: ReturnType<typeof setTimeout> | null = null

function midToFlag(mmsi: number): string {
  const mid = String(mmsi).substring(0, 3)
  const flags: Record<string, string> = {
    '201':'🇦🇱','203':'🇦🇹','205':'🇧🇪','207':'🇧🇬','209':'🇨🇾',
    '211':'🇩🇪','219':'🇩🇰','220':'🇩🇰','224':'🇪🇸','225':'🇪🇸',
    '226':'🇫🇷','227':'🇫🇷','228':'🇫🇷','230':'🇫🇮','232':'🇬🇧','233':'🇬🇧','234':'🇬🇧','235':'🇬🇧',
    '236':'🇬🇮','237':'🇬🇷','238':'🇭🇷','239':'🇬🇷','240':'🇬🇷','241':'🇬🇷',
    '242':'🇲🇦','243':'🇭🇺','244':'🇳🇱','245':'🇳🇱','246':'🇳🇱',
    '247':'🇮🇹','248':'🇲🇹','249':'🇲🇹','250':'🇮🇪','251':'🇮🇸',
    '255':'🇵🇹','256':'🇲🇹','257':'🇳🇴','258':'🇳🇴','259':'🇳🇴',
    '261':'🇵🇱','263':'🇵🇹','264':'🇷🇴','265':'🇸🇪','266':'🇸🇪',
    '271':'🇹🇷','272':'🇺🇦','273':'🇷🇺','275':'🇱🇻','276':'🇪🇪','277':'🇱🇹',
    '305':'🇦🇬','308':'🇧🇸','309':'🇧🇸','310':'🇧🇲','311':'🇧🇸',
    '312':'🇧🇿','314':'🇧🇧','316':'🇨🇦','319':'🇰🇾',
    '338':'🇺🇸','345':'🇲🇽',
    '351':'🇵🇦','352':'🇵🇦','353':'🇵🇦','354':'🇵🇦','355':'🇵🇦',
    '356':'🇵🇦','357':'🇵🇦','370':'🇵🇦','371':'🇵🇦','372':'🇵🇦','373':'🇵🇦',
    '366':'🇺🇸','367':'🇺🇸','368':'🇺🇸','369':'🇺🇸',
    '375':'🇻🇨','376':'🇻🇨',
    '412':'🇨🇳','413':'🇨🇳','414':'🇨🇳','416':'🇹🇼',
    '419':'🇮🇳','422':'🇮🇷','425':'🇮🇶','428':'🇮🇱',
    '431':'🇯🇵','432':'🇯🇵','440':'🇰🇷','441':'🇰🇷',
    '445':'🇰🇵','447':'🇰🇼','461':'🇴🇲','470':'🇦🇪','471':'🇦🇪',
    '477':'🇭🇰',
    '503':'🇦🇺','512':'🇳🇿','525':'🇮🇩','533':'🇲🇾',
    '538':'🇲🇭','548':'🇵🇭','563':'🇸🇬','564':'🇸🇬','565':'🇸🇬','566':'🇸🇬',
    '567':'🇹🇭','574':'🇻🇳',
    '601':'🇿🇦','605':'🇩🇿','618':'🇪🇬','631':'🇱🇷','632':'🇱🇷','633':'🇱🇷',
    '636':'🇲🇬','649':'🇳🇬',
  }
  return flags[mid] || ''
}

async function loadVessels() {
  loading.value = true
  try {
    const data = await api.getVesselRegistry(searchQuery.value, tagFilter.value)
    vessels.value = data.vessels || []
    page.value = 1
  } catch { /* ignore */ }
  loading.value = false
}

async function loadChanges() {
  try {
    const data = await api.getVesselChanges(100)
    recentChanges.value = data.changes || []
  } catch { /* ignore */ }
}

async function loadTags() {
  try {
    const data = await api.getVesselTags()
    allTags.value = data.tags || []
  } catch { /* ignore */ }
}

function onSearch() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(loadVessels, 300)
}

function filterByTag(tag: string) {
  tagFilter.value = tagFilter.value === tag ? '' : tag
  loadVessels()
}

async function openDetail(mmsi: number) {
  selectedMMSI.value = mmsi
  showDetail.value = true
  try {
    const data = await api.getVesselRegistryDetail(mmsi)
    detailVessel.value = data.vessel
    detailHistory.value = data.history || []
    detailNotes.value = data.notes || []
  } catch { /* ignore */ }
}

function closeDetail() {
  showDetail.value = false
  selectedMMSI.value = null
}

async function addNote() {
  if (!selectedMMSI.value || !newTag.value) return
  await api.addVesselNote(selectedMMSI.value, newTag.value, newNote.value || undefined)
  newTag.value = ''
  newNote.value = ''
  openDetail(selectedMMSI.value)
  loadTags()
  loadVessels()
}

async function removeNote(tag: string) {
  if (!selectedMMSI.value) return
  await api.deleteVesselNote(selectedMMSI.value, tag)
  openDetail(selectedMMSI.value)
  loadTags()
  loadVessels()
}

function formatTime(ts: string) {
  return new Date(ts).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

function formatDate(ts: string) {
  return new Date(ts).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

function isRussian(mmsi: number) {
  return String(mmsi).startsWith('273')
}

onMounted(() => {
  loadVessels()
  loadChanges()
  loadTags()
})
</script>

<template>
  <div class="registry-panel">
    <!-- Search & Filter Bar -->
    <div class="toolbar">
      <input
        v-model="searchQuery"
        @input="onSearch"
        placeholder="Search by name, MMSI, call sign, or IMO..."
        class="search-input"
      />
      <div class="tag-filters" v-if="allTags.length">
        <span class="tag-label">Tags:</span>
        <button
          v-for="t in allTags" :key="t"
          class="tag-btn"
          :class="{ active: tagFilter === t }"
          @click="filterByTag(t)"
        >{{ t }}</button>
      </div>
    </div>

    <!-- Two-column layout -->
    <div class="columns">
      <!-- Vessel list -->
      <div class="vessel-list">
        <div class="list-header">
          <span>{{ vessels.length }} vessels</span>
          <span v-if="loading" class="loading-dot">Loading...</span>
        </div>
        <div class="scroll-area">
          <table>
            <thead>
              <tr>
                <th></th>
                <th>Vessel</th>
                <th>Type</th>
                <th>MMSI</th>
                <th>Tags</th>
                <th>Changes</th>
                <th>Last Seen</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!vessels.length"><td colspan="7" class="empty">No vessels found</td></tr>
              <tr
                v-for="v in paginatedVessels" :key="v.mmsi"
                @click="openDetail(v.mmsi)"
                :class="{ selected: selectedMMSI === v.mmsi, 'has-changes': v.change_count > 0 }"
              >
                <td class="flag-col">{{ midToFlag(v.mmsi) }}</td>
                <td :class="{ russian: isRussian(v.mmsi) }">
                  {{ v.name || '—' }}
                  <span v-if="isRussian(v.mmsi)" class="rus-badge">RUS</span>
                </td>
                <td class="dim">{{ v.vessel_type_name || '—' }}</td>
                <td class="mono">{{ v.mmsi }}</td>
                <td>
                  <span v-if="v.tags" class="tags-cell">
                    <span v-for="t in v.tags.split(',')" :key="t" class="tag-pill">{{ t }}</span>
                  </span>
                </td>
                <td>
                  <span v-if="v.change_count" class="change-badge">{{ v.change_count }}</span>
                </td>
                <td class="dim">{{ formatDate(v.last_seen_at) }}</td>
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

      <!-- Recent Changes -->
      <div class="changes-panel">
        <div class="ch-header">Recent Identity Changes</div>
        <div class="scroll-area-sm">
          <div v-if="!recentChanges.length" class="empty-sm">No changes recorded yet — the trigger will catch future name/type/callsign changes automatically.</div>
          <div v-for="c in recentChanges" :key="c.id" class="change-row" @click="openDetail(c.mmsi)">
            <span class="ch-field">{{ c.field_name }}</span>
            <span class="ch-mmsi">{{ c.mmsi }}</span>
            <div class="ch-vals">
              <span class="ch-old">{{ c.old_value || '(empty)' }}</span>
              <span class="ch-arrow">→</span>
              <span class="ch-new">{{ c.new_value || '(empty)' }}</span>
            </div>
            <span class="ch-time">{{ formatTime(c.changed_at) }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Detail modal -->
    <div v-if="showDetail" class="detail-overlay" @click.self="closeDetail">
      <div class="detail-modal">
        <button class="close-btn" @click="closeDetail">&times;</button>
        <template v-if="detailVessel">
          <h3>
            {{ midToFlag(detailVessel.mmsi) }}
            {{ detailVessel.name || 'Unknown' }}
            <span v-if="isRussian(detailVessel.mmsi)" class="rus-badge">RUS</span>
          </h3>
          <div class="detail-grid">
            <div><span class="lbl">MMSI</span><span class="val mono">{{ detailVessel.mmsi }}</span></div>
            <div><span class="lbl">IMO</span><span class="val">{{ detailVessel.imo_number || '—' }}</span></div>
            <div><span class="lbl">Call Sign</span><span class="val">{{ detailVessel.call_sign || '—' }}</span></div>
            <div><span class="lbl">Type</span><span class="val">{{ detailVessel.vessel_type_name || '—' }}</span></div>
            <div><span class="lbl">Draught</span><span class="val">{{ detailVessel.draught ? detailVessel.draught + 'm' : '—' }}</span></div>
            <div><span class="lbl">Destination</span><span class="val">{{ detailVessel.destination || '—' }}</span></div>
            <div><span class="lbl">First Seen</span><span class="val">{{ formatDate(detailVessel.first_seen_at) }}</span></div>
            <div><span class="lbl">Last Seen</span><span class="val">{{ formatDate(detailVessel.last_seen_at) }}</span></div>
          </div>

          <!-- Tags / Notes -->
          <div class="section-label">Tags &amp; Notes</div>
          <div class="notes-list">
            <div v-for="n in detailNotes" :key="n.id" class="note-item">
              <span class="tag-pill">{{ n.tag }}</span>
              <span v-if="n.note" class="note-text">{{ n.note }}</span>
              <button class="rm-btn" @click="removeNote(n.tag)">&times;</button>
            </div>
            <div v-if="!detailNotes.length" class="empty-sm">No tags assigned</div>
          </div>
          <div class="add-note">
            <select v-model="newTag">
              <option value="">— select tag —</option>
              <option v-for="t in PRESET_TAGS" :key="t" :value="t">{{ t }}</option>
            </select>
            <input v-model="newNote" placeholder="Optional note..." class="note-input" />
            <button @click="addNote" :disabled="!newTag" class="add-btn">Add</button>
          </div>

          <!-- Change History -->
          <div class="section-label">Identity Change History</div>
          <div class="history-list">
            <div v-if="!detailHistory.length" class="empty-sm">No identity changes recorded for this vessel</div>
            <div v-for="h in detailHistory" :key="h.id" class="hist-row">
              <span class="ch-field">{{ h.field_name }}</span>
              <span class="ch-old">{{ h.old_value || '(empty)' }}</span>
              <span class="ch-arrow">→</span>
              <span class="ch-new">{{ h.new_value || '(empty)' }}</span>
              <span class="ch-time">{{ formatTime(h.changed_at) }}</span>
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.registry-panel { background: rgba(20, 24, 33, 0.85); border-radius: 8px; border: 1px solid rgba(255, 255, 255, 0.07); padding: 16px; }

.toolbar { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; margin-bottom: 14px; }
.search-input { flex: 1; min-width: 200px; padding: 8px 14px; background: rgba(0,0,0,0.3); border: 1px solid rgba(255,255,255,0.1); border-radius: 6px; color: #e0e0e0; font-size: 13px; outline: none; }
.search-input:focus { border-color: #4fc3f7; }
.tag-filters { display: flex; gap: 6px; align-items: center; flex-wrap: wrap; }
.tag-label { font-size: 11px; color: #888; text-transform: uppercase; }
.tag-btn { padding: 3px 10px; border-radius: 12px; border: 1px solid rgba(255,255,255,0.12); background: transparent; color: #aaa; font-size: 11px; cursor: pointer; }
.tag-btn.active { background: #4fc3f7; color: #0f0f23; border-color: #4fc3f7; }

.columns { display: flex; gap: 16px; }
.vessel-list { flex: 2; }
.changes-panel { flex: 1; min-width: 260px; }

.list-header { font-size: 11px; color: #888; padding: 4px 0 8px; display: flex; justify-content: space-between; }
.loading-dot { color: #4fc3f7; }

.scroll-area { max-height: 500px; overflow-y: auto; }
.scroll-area-sm { max-height: 500px; overflow-y: auto; }

table { width: 100%; border-collapse: collapse; }
th { text-align: left; padding: 8px 10px; font-size: 10px; color: #666; text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid rgba(255,255,255,0.08); position: sticky; top: 0; background: rgba(20,24,33,0.98); }
td { padding: 7px 10px; font-size: 12px; border-bottom: 1px solid rgba(255,255,255,0.03); cursor: pointer; }
tr:hover { background: rgba(255,255,255,0.03); }
tr.selected { background: rgba(79,195,247,0.08); }
tr.has-changes td:first-child { border-left: 2px solid #ff8800; }

.flag-col { font-size: 16px; width: 28px; text-align: center; }
.dim { color: #777; }
.mono { font-family: monospace; font-size: 11px; }
.russian { color: #ff6b6b; }
.rus-badge { font-size: 9px; background: #d32f2f; color: white; padding: 1px 5px; border-radius: 3px; margin-left: 4px; vertical-align: middle; }

.tags-cell { display: flex; gap: 3px; flex-wrap: wrap; }
.tag-pill { font-size: 10px; padding: 1px 7px; border-radius: 10px; background: rgba(79,195,247,0.15); color: #7cb4ff; white-space: nowrap; }
.change-badge { font-size: 10px; background: #ff8800; color: #fff; padding: 1px 6px; border-radius: 8px; font-weight: 700; }

.empty, .empty-sm { text-align: center; padding: 24px; color: #555; font-size: 12px; }

/* Changes panel */
.ch-header { font-size: 13px; font-weight: 600; padding: 0 0 10px; border-bottom: 1px solid rgba(255,255,255,0.07); margin-bottom: 8px; }
.change-row { padding: 8px 0; border-bottom: 1px solid rgba(255,255,255,0.04); cursor: pointer; font-size: 12px; }
.change-row:hover { background: rgba(255,255,255,0.02); }
.ch-field { font-weight: 600; color: #4fc3f7; margin-right: 6px; font-size: 10px; text-transform: uppercase; }
.ch-mmsi { font-family: monospace; font-size: 10px; color: #777; float: right; }
.ch-vals { margin-top: 3px; }
.ch-old { color: #ff6b6b; text-decoration: line-through; }
.ch-arrow { color: #555; margin: 0 6px; }
.ch-new { color: #66bb6a; }
.ch-time { font-size: 10px; color: #555; display: block; margin-top: 2px; }

/* Detail modal */
.detail-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.7); z-index: 1000; display: flex; align-items: center; justify-content: center; }
.detail-modal { background: #141821; border: 1px solid rgba(255,255,255,0.1); border-radius: 10px; padding: 24px; width: 520px; max-height: 80vh; overflow-y: auto; position: relative; }
.close-btn { position: absolute; top: 12px; right: 14px; background: none; border: none; color: #888; font-size: 22px; cursor: pointer; }
.detail-modal h3 { font-size: 18px; margin: 0 0 14px; color: #e0e0e0; }

.detail-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 16px; margin-bottom: 18px; }
.detail-grid .lbl { font-size: 10px; color: #666; text-transform: uppercase; display: block; }
.detail-grid .val { font-size: 13px; color: #ccc; }

.section-label { font-size: 12px; font-weight: 600; color: #4fc3f7; margin: 14px 0 8px; text-transform: uppercase; letter-spacing: 0.5px; border-top: 1px solid rgba(255,255,255,0.06); padding-top: 12px; }

.notes-list { margin-bottom: 10px; }
.note-item { display: flex; align-items: center; gap: 8px; padding: 5px 0; }
.note-text { font-size: 12px; color: #aaa; }
.rm-btn { background: none; border: none; color: #d32f2f; font-size: 16px; cursor: pointer; margin-left: auto; }

.add-note { display: flex; gap: 6px; align-items: center; }
.add-note select { padding: 5px 8px; background: rgba(0,0,0,0.3); border: 1px solid rgba(255,255,255,0.1); border-radius: 4px; color: #ccc; font-size: 12px; }
.note-input { flex: 1; padding: 5px 8px; background: rgba(0,0,0,0.3); border: 1px solid rgba(255,255,255,0.1); border-radius: 4px; color: #ccc; font-size: 12px; }
.add-btn { padding: 5px 14px; background: #4fc3f7; color: #0f0f23; border: none; border-radius: 4px; cursor: pointer; font-weight: 600; font-size: 12px; }
.add-btn:disabled { opacity: 0.4; cursor: not-allowed; }

.history-list { max-height: 200px; overflow-y: auto; }
.hist-row { padding: 6px 0; border-bottom: 1px solid rgba(255,255,255,0.04); font-size: 12px; display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }

.pagination { display: flex; align-items: center; justify-content: center; gap: 12px; padding: 12px 0; border-top: 1px solid rgba(255,255,255,0.06); }
.pg-btn { background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); border-radius: 5px; color: #ccc; font-size: 12px; padding: 5px 14px; cursor: pointer; }
.pg-btn:disabled { opacity: 0.3; cursor: not-allowed; }
.pg-btn:hover:not(:disabled) { background: rgba(255,255,255,0.1); }
.pg-info { font-size: 12px; color: #888; }
.pg-size { background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); border-radius: 5px; color: #ddd; font-size: 11px; padding: 4px 8px; }

@media (max-width: 900px) {
  .columns { flex-direction: column; }
}
</style>

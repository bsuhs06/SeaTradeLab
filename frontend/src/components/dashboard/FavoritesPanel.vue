<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { api } from '@/api/client'
import { formatAgo } from '@/composables/useVesselUtils'
import type { VesselFavorite } from '@/types/vessel'

const favorites = ref<VesselFavorite[]>([])
const loading = ref(false)

/* ---------- search & filter ---------- */
const search = ref('')
const typeFilter = ref('all')
const sortBy = ref<'name' | 'recent' | 'speed'>('recent')

/* ---------- notes editing ---------- */
const editingMMSI = ref<number | null>(null)
const editNotes = ref('')
const savingNotes = ref(false)

/* ---------- pagination ---------- */
const page = ref(1)
const pageSize = ref(25)

async function loadFavorites() {
  loading.value = true
  try {
    const data = await api.getFavorites()
    favorites.value = data.favorites || []
  } catch { /* ignore */ }
  loading.value = false
}

async function removeFavorite(mmsi: number) {
  try {
    await api.removeFavorite(mmsi)
    favorites.value = favorites.value.filter(f => f.mmsi !== mmsi)
    if (editingMMSI.value === mmsi) editingMMSI.value = null
  } catch { /* ignore */ }
}

function goToVessel(mmsi: number, lat?: number, lng?: number) {
  if (lat && lng) {
    window.location.href = `/map#search=${mmsi}&goto=${lat},${lng},14`
  } else {
    window.location.href = `/map#search=${mmsi}`
  }
}

function startEditNotes(f: VesselFavorite) {
  if (editingMMSI.value === f.mmsi) {
    editingMMSI.value = null
    return
  }
  editingMMSI.value = f.mmsi
  editNotes.value = f.notes || ''
}

async function saveNotes(mmsi: number) {
  savingNotes.value = true
  try {
    await api.updateFavoriteNotes(mmsi, editNotes.value)
    const fav = favorites.value.find(f => f.mmsi === mmsi)
    if (fav) fav.notes = editNotes.value || undefined
    editingMMSI.value = null
  } catch { /* ignore */ }
  savingNotes.value = false
}

/* ---------- filtering & sorting ---------- */
const vesselTypes = computed(() => {
  const types = new Set<string>()
  for (const f of favorites.value) {
    if (f.vessel_type) types.add(f.vessel_type)
  }
  return [...types].sort()
})

const filtered = computed(() => {
  let list = favorites.value
  if (search.value) {
    const q = search.value.toLowerCase()
    list = list.filter(f =>
      (f.vessel_name || '').toLowerCase().includes(q) ||
      String(f.mmsi).includes(q) ||
      (f.vessel_type || '').toLowerCase().includes(q) ||
      (f.destination || '').toLowerCase().includes(q) ||
      (f.notes || '').toLowerCase().includes(q)
    )
  }
  if (typeFilter.value !== 'all') {
    list = list.filter(f => f.vessel_type === typeFilter.value)
  }
  if (sortBy.value === 'name') list = [...list].sort((a, b) => (a.vessel_name || '').localeCompare(b.vessel_name || ''))
  else if (sortBy.value === 'speed') list = [...list].sort((a, b) => (b.speed_over_ground ?? 0) - (a.speed_over_ground ?? 0))
  else list = [...list].sort((a, b) => new Date(b.last_seen || 0).getTime() - new Date(a.last_seen || 0).getTime())
  return list
})

const totalFiltered = computed(() => filtered.value.length)
const totalPages = computed(() => Math.max(1, Math.ceil(totalFiltered.value / pageSize.value)))
const rangeStart = computed(() => totalFiltered.value === 0 ? 0 : (page.value - 1) * pageSize.value + 1)
const rangeEnd = computed(() => Math.min(page.value * pageSize.value, totalFiltered.value))
const paginated = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return filtered.value.slice(start, start + pageSize.value)
})

watch([search, typeFilter], () => { page.value = 1 })

onMounted(loadFavorites)
</script>

<template>
  <div class="panel">
    <div class="ph">
      <span>Bookmarked Vessels</span>
      <span class="sub">{{ totalFiltered }} of {{ favorites.length }} vessel{{ favorites.length !== 1 ? 's' : '' }}</span>
    </div>

    <div class="filters">
      <div class="filter-row">
        <input v-model="search" type="text" placeholder="Search name, MMSI, type, destination, notes…" class="finput search-input" />
        <select v-model="typeFilter" class="finput">
          <option value="all">All Types</option>
          <option v-for="t in vesselTypes" :key="t" :value="t">{{ t }}</option>
        </select>
        <select v-model="sortBy" class="finput">
          <option value="recent">Sort: Last Seen</option>
          <option value="name">Sort: Name</option>
          <option value="speed">Sort: Speed</option>
        </select>
        <select v-model.number="pageSize" class="finput" @change="page = 1">
          <option :value="10">10/page</option>
          <option :value="25">25/page</option>
          <option :value="50">50/page</option>
          <option :value="100">100/page</option>
        </select>
        <button class="btn-sm" @click="loadFavorites">↻ Refresh</button>
      </div>
    </div>

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="!favorites.length" class="empty-msg">
      No favorites yet. Star a vessel from the map popup or search to add it here.
    </div>
    <div v-else style="max-height: 560px; overflow-y: auto">
      <table>
        <thead>
          <tr>
            <th>Vessel</th>
            <th>Type</th>
            <th>Position</th>
            <th>Speed</th>
            <th>Destination</th>
            <th>Last Seen</th>
            <th>Notes</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <template v-for="f in paginated" :key="f.mmsi">
            <tr class="fav-row">
              <td>
                <div class="vessel-name">{{ f.vessel_name || 'Unknown' }}</div>
                <div class="vessel-mmsi">{{ f.mmsi }}</div>
              </td>
              <td class="type-col">{{ f.vessel_type || '—' }}</td>
              <td class="pos-col">
                <template v-if="f.latitude && f.longitude">
                  {{ f.latitude.toFixed(3) }}, {{ f.longitude.toFixed(3) }}
                </template>
                <template v-else>—</template>
              </td>
              <td>{{ f.speed_over_ground != null ? f.speed_over_ground.toFixed(1) + ' kn' : '—' }}</td>
              <td>{{ f.destination || '—' }}</td>
              <td>{{ f.last_seen ? formatAgo(f.last_seen) : '—' }}</td>
              <td class="notes-col" @click.stop="startEditNotes(f)" :title="f.notes || 'Click to add notes'">
                <span v-if="f.notes" class="notes-preview">{{ f.notes }}</span>
                <span v-else class="notes-empty">+ note</span>
              </td>
              <td class="actions-col" @click.stop>
                <button class="view-btn" @click="goToVessel(f.mmsi, f.latitude, f.longitude)" title="View on map">🗺️</button>
                <button class="remove-btn" @click="removeFavorite(f.mmsi)" title="Remove from favorites">✕</button>
              </td>
            </tr>
            <!-- Notes editor row -->
            <tr v-if="editingMMSI === f.mmsi" class="notes-row">
              <td colspan="8">
                <div class="notes-editor">
                  <textarea v-model="editNotes" placeholder="Add notes about this vessel…" rows="2" class="notes-textarea"></textarea>
                  <div class="notes-actions">
                    <button class="btn-save" @click="saveNotes(f.mmsi)" :disabled="savingNotes">{{ savingNotes ? 'Saving…' : 'Save' }}</button>
                    <button class="btn-cancel" @click="editingMMSI = null">Cancel</button>
                  </div>
                </div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
    <div class="pager" v-if="totalFiltered > pageSize">
      <button :disabled="page <= 1" @click="page--" class="pg-btn">‹ Prev</button>
      <span class="pg-info">{{ rangeStart }}–{{ rangeEnd }} of {{ totalFiltered }}</span>
      <button :disabled="page >= totalPages" @click="page++" class="pg-btn">Next ›</button>
    </div>
  </div>
</template>

<style scoped>
.panel { padding: 16px; font-size: 13px; }
.ph { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; font-weight: 600; color: #ccc; }
.sub { font-size: 12px; color: #888; font-weight: 400; }
.filters { margin-bottom: 0.5rem; }
.filter-row { display: flex; gap: 0.4rem; flex-wrap: wrap; align-items: center; }
.finput { background: #1e1e1e; border: 1px solid #333; color: #ccc; padding: 4px 8px; border-radius: 4px; font-size: 12px; }
.search-input { flex: 1; min-width: 160px; }
.btn-sm { background: #2a2a2a; border: 1px solid #444; color: #ccc; padding: 3px 10px; border-radius: 4px; cursor: pointer; font-size: 12px; }
.btn-sm:hover { background: #333; }
.loading { text-align: center; padding: 24px; color: #888; }
.empty-msg { text-align: center; padding: 32px; color: #666; font-size: 13px; }
table { width: 100%; border-collapse: collapse; }
thead th {
  text-align: left;
  padding: 6px 8px;
  color: #888;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  border-bottom: 1px solid rgba(255,255,255,0.08);
  white-space: nowrap;
}
tbody tr {
  border-bottom: 1px solid rgba(255,255,255,0.04);
  transition: background 0.15s;
}
tbody tr.fav-row:hover { background: rgba(124,180,255,0.08); }
td { padding: 6px 8px; color: #ddd; }
.vessel-name { font-weight: 600; color: #7cb4ff; }
.vessel-mmsi { font-size: 11px; color: #666; }
.type-col { max-width: 140px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pos-col { font-size: 11px; font-family: monospace; color: #aaa; white-space: nowrap; }
.notes-col { cursor: pointer; max-width: 160px; }
.notes-preview { font-size: 11px; color: #aaa; display: -webkit-box; -webkit-line-clamp: 1; -webkit-box-orient: vertical; overflow: hidden; }
.notes-empty { font-size: 11px; color: #555; }
.notes-empty:hover { color: #7cb4ff; }
.actions-col { white-space: nowrap; }
.view-btn {
  background: none;
  border: 1px solid rgba(124,180,255,0.3);
  border-radius: 4px;
  padding: 2px 6px;
  cursor: pointer;
  font-size: 13px;
  margin-right: 4px;
}
.view-btn:hover { background: rgba(124,180,255,0.15); }
.remove-btn {
  background: none;
  border: 1px solid rgba(255,80,80,0.3);
  color: #ff5050;
  border-radius: 4px;
  padding: 2px 8px;
  cursor: pointer;
  font-size: 12px;
}
.remove-btn:hover { background: rgba(255,80,80,0.15); }
.notes-row td { padding: 0 8px 8px; background: #141414; border-bottom: 1px solid rgba(255,255,255,0.06); }
.notes-editor { display: flex; flex-direction: column; gap: 6px; }
.notes-textarea {
  width: 100%;
  background: #1e1e1e;
  border: 1px solid #333;
  color: #ddd;
  border-radius: 4px;
  padding: 6px 8px;
  font-size: 12px;
  font-family: inherit;
  resize: vertical;
}
.notes-textarea:focus { border-color: #7cb4ff; outline: none; }
.notes-actions { display: flex; gap: 6px; }
.btn-save { background: #1a73e8; color: #fff; border: none; border-radius: 4px; padding: 4px 14px; cursor: pointer; font-size: 12px; }
.btn-save:hover { background: #1557b0; }
.btn-save:disabled { opacity: 0.5; }
.btn-cancel { background: #333; color: #ccc; border: 1px solid #444; border-radius: 4px; padding: 4px 14px; cursor: pointer; font-size: 12px; }
.btn-cancel:hover { background: #444; }
.pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 10px;
  border-top: 1px solid rgba(255,255,255,0.06);
}
.pg-btn {
  background: rgba(255,255,255,0.06);
  border: 1px solid rgba(255,255,255,0.1);
  color: #ccc;
  padding: 4px 12px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
}
.pg-btn:disabled { opacity: 0.3; cursor: default; }
.pg-info { font-size: 12px; color: #888; }
.refresh-btn {
  background: rgba(124,180,255,0.1);
  border: 1px solid rgba(124,180,255,0.2);
  color: #7cb4ff;
  padding: 4px 14px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
}
.refresh-btn:hover { background: rgba(124,180,255,0.2); }
</style>

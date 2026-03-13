<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '@/api/client'
import type { Port } from '@/types/vessel'

const ports = ref<Port[]>([])
const search = ref('')
const countryFilter = ref('')
const typeFilter = ref('')

const name = ref('')
const lat = ref<number | null>(null)
const lon = ref<number | null>(null)
const radius = ref(4.0)
const country = ref('')
const portType = ref('commercial')

const countries = computed(() => {
  const set = new Set<string>()
  for (const p of ports.value) { if (p.country) set.add(p.country) }
  return [...set].sort()
})

const filtered = computed(() =>
  ports.value.filter((p) => {
    if (search.value && !p.name.toLowerCase().includes(search.value.toLowerCase()) && !(p.country || '').toLowerCase().includes(search.value.toLowerCase())) return false
    if (countryFilter.value && p.country !== countryFilter.value) return false
    if (typeFilter.value && p.port_type !== typeFilter.value) return false
    return true
  }),
)

async function load() {
  try { ports.value = await api.getPorts() } catch { /* ignore */ }
}

async function addPort() {
  if (!name.value || lat.value == null || lon.value == null) { alert('Name, latitude, and longitude required'); return }
  try {
    await api.addPort({ name: name.value, latitude: lat.value, longitude: lon.value, radius_km: radius.value, country: country.value, port_type: portType.value })
    name.value = ''; lat.value = null; lon.value = null; radius.value = 4.0; country.value = ''
    load()
  } catch (e: any) { alert('Error: ' + e.message) }
}

async function removeOverride(id: number) {
  if (!confirm('Remove this custom port?')) return
  try { await api.removePort(id); load() } catch (e: any) { alert('Error: ' + e.message) }
}

async function excludeBuiltin(portName: string) {
  if (!confirm(`Exclude built-in port "${portName}" from detection?`)) return
  try { await api.excludeBuiltinPort(portName); load() } catch (e: any) { alert('Error: ' + e.message) }
}

onMounted(load)
</script>

<template>
  <div class="panel">
    <div class="ph"><span>Port Definitions</span><span>{{ ports.length }} ports</span></div>
    <div class="port-filter">
      <input type="text" v-model="search" placeholder="Filter ports..." />
      <select v-model="countryFilter">
        <option value="">All Countries</option>
        <option v-for="c in countries" :key="c" :value="c">{{ c }}</option>
      </select>
      <select v-model="typeFilter">
        <option value="">All Types</option>
        <option value="oil">Oil</option>
        <option value="lng">LNG</option>
        <option value="commercial">Commercial</option>
        <option value="cargo">Cargo</option>
        <option value="ferry">Ferry</option>
        <option value="naval">Naval</option>
      </select>
    </div>
    <div class="port-table">
      <table>
        <thead><tr><th>Port</th><th>Country</th><th>Type</th><th>Lat</th><th>Lon</th><th>Radius</th><th>Source</th><th></th></tr></thead>
        <tbody>
          <tr v-if="!filtered.length"><td colspan="8" class="empty">No ports match filters</td></tr>
          <tr v-for="p in filtered" :key="p.name + p.source">
            <td>{{ p.name }}</td>
            <td>{{ p.country }}</td>
            <td><span class="port-type-tag" :class="p.port_type">{{ p.port_type }}</span></td>
            <td>{{ p.latitude.toFixed(3) }}</td>
            <td>{{ p.longitude.toFixed(3) }}</td>
            <td>{{ p.radius_km }} km</td>
            <td :style="p.source === 'custom' ? 'color:#4fc3f7' : 'color:#888'">{{ p.source === 'builtin' ? 'Built-in' : 'Custom' }}</td>
            <td>
              <button v-if="p.source === 'custom' && p.override_id" class="rm-btn" @click="removeOverride(p.override_id!)">Remove</button>
              <button v-else class="rm-btn" @click="excludeBuiltin(p.name)">Exclude</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="port-controls">
      <div class="fg"><label>Port Name</label><input type="text" v-model="name" placeholder="Port name" style="width:140px" /></div>
      <div class="fg"><label>Latitude</label><input type="number" v-model.number="lat" step="0.001" placeholder="60.000" style="width:90px" /></div>
      <div class="fg"><label>Longitude</label><input type="number" v-model.number="lon" step="0.001" placeholder="25.000" style="width:90px" /></div>
      <div class="fg"><label>Radius (km)</label><input type="number" v-model.number="radius" step="0.5" min="1" max="20" style="width:70px" /></div>
      <div class="fg"><label>Country</label><input type="text" v-model="country" placeholder="Finland" style="width:100px" /></div>
      <div class="fg"><label>Type</label>
        <select v-model="portType" style="width:110px">
          <option value="commercial">Commercial</option>
          <option value="oil">Oil</option>
          <option value="lng">LNG</option>
          <option value="cargo">Cargo</option>
          <option value="ferry">Ferry</option>
          <option value="naval">Naval</option>
          <option value="fishing">Fishing</option>
        </select>
      </div>
      <button class="add-btn" @click="addPort">Add Port</button>
    </div>
  </div>
</template>

<style scoped>
.panel { background: rgba(20, 24, 33, 0.85); border-radius: 8px; border: 1px solid rgba(255, 255, 255, 0.07); overflow: hidden; margin-bottom: 20px; }
.ph { padding: 13px 18px; font-size: 14px; font-weight: 600; border-bottom: 1px solid rgba(255, 255, 255, 0.07); display: flex; justify-content: space-between; align-items: center; }
.ph span:last-child { font-size: 11px; color: #888; font-weight: 400; }
.port-filter { padding: 8px 16px; display: flex; gap: 8px; flex-wrap: wrap; }
.port-filter input, .port-filter select { padding: 6px 10px; background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.12); border-radius: 4px; color: #fff; font-size: 13px; outline: none; }
.port-table { max-height: 400px; overflow-y: auto; }
table { width: 100%; border-collapse: collapse; }
th { text-align: left; padding: 9px 14px; font-size: 11px; color: #888; text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid rgba(255, 255, 255, 0.08); }
td { padding: 9px 14px; font-size: 13px; border-bottom: 1px solid rgba(255, 255, 255, 0.04); }
tr:hover { background: rgba(255, 255, 255, 0.02); }
.empty { text-align: center; padding: 32px; color: #555; font-size: 13px; }
.port-type-tag { display: inline-block; padding: 1px 6px; border-radius: 8px; font-size: 10px; font-weight: 600; text-transform: uppercase; }
.port-type-tag.oil { background: rgba(255, 159, 0, 0.2); color: #ffa726; }
.port-type-tag.lng { background: rgba(156, 39, 176, 0.2); color: #ba68c8; }
.port-type-tag.commercial { background: rgba(33, 150, 243, 0.2); color: #42a5f5; }
.port-type-tag.cargo { background: rgba(121, 85, 72, 0.2); color: #a1887f; }
.port-type-tag.ferry { background: rgba(0, 150, 136, 0.2); color: #4db6ac; }
.port-type-tag.naval { background: rgba(255, 59, 48, 0.15); color: #ff6b6b; }
.port-type-tag.fishing { background: rgba(76, 175, 80, 0.2); color: #66bb6a; }
.rm-btn { cursor: pointer; color: #ff6b6b; font-size: 12px; border: none; background: transparent; padding: 2px 8px; }
.rm-btn:hover { color: #ff3b3b; text-decoration: underline; }
.port-controls { display: flex; gap: 8px; padding: 12px 16px; flex-wrap: wrap; align-items: flex-end; }
.port-controls input, .port-controls select { padding: 7px 10px; background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.12); border-radius: 4px; color: #fff; font-size: 13px; outline: none; }
.fg { display: flex; flex-direction: column; }
.fg label { font-size: 11px; color: #888; text-transform: uppercase; letter-spacing: 0.5px; display: block; margin-bottom: 2px; }
.add-btn { padding: 7px 16px; border-radius: 4px; font-size: 13px; font-weight: 600; border: none; cursor: pointer; color: #fff; background: #2ecc71; align-self: flex-end; }
.add-btn:hover { background: #27ae60; }
</style>

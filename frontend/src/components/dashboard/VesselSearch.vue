<script setup lang="ts">
import { ref } from 'vue'
import { api } from '@/api/client'
import { formatAgo } from '@/composables/useVesselUtils'

const query = ref('')
const results = ref<any[]>([])
let timer: ReturnType<typeof setTimeout> | null = null

function onInput() {
  if (query.value.trim().length < 2) { results.value = []; return }
  if (timer) clearTimeout(timer)
  timer = setTimeout(async () => {
    try {
      const data = await api.searchVessels(query.value.trim(), 15)
      results.value = data.features || []
    } catch { results.value = [] }
  }, 300)
}

function goToVessel(lat: number, lng: number, mmsi: number) {
  window.location.href = `/map#search=${mmsi}&goto=${lat},${lng},14`
}

function clearResults() {
  setTimeout(() => { results.value = [] }, 200)
}
</script>

<template>
  <div class="search-box">
    <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24"><path fill="#555" d="M15.5 14h-.79l-.28-.27A6.47 6.47 0 0016 9.5 6.5 6.5 0 109.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z" /></svg>
    <input
      type="text"
      v-model="query"
      @input="onInput"
      @blur="clearResults"
      placeholder="Search vessels by name, MMSI, IMO, or call sign..."
      autocomplete="off"
    />
    <div v-if="results.length" class="sr">
      <div
        v-for="f in results"
        :key="f.properties.mmsi"
        class="sr-item"
        @click="goToVessel(f.geometry.coordinates[1], f.geometry.coordinates[0], f.properties.mmsi)"
      >
        <div>
          <span class="nm" :class="{ ru: f.properties.is_russian }">
            {{ f.properties.name || 'Unknown' }}
          </span>
          <span v-if="f.properties.is_russian" class="badge-rus">RUS</span>
          <br />
          <span class="mt">MMSI: {{ f.properties.mmsi }}
            <template v-if="f.properties.vessel_type"> | {{ f.properties.vessel_type }}</template>
          </span>
        </div>
        <span class="mt">{{ formatAgo(f.properties.timestamp) }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.search-box { position: relative; margin-bottom: 24px; }
.search-box svg { position: absolute; left: 14px; top: 14px; }
.search-box input {
  width: 100%;
  padding: 14px 18px 14px 44px;
  font-size: 15px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 8px;
  color: #fff;
  outline: none;
}
.search-box input:focus { border-color: #7cb4ff; }
.search-box input::placeholder { color: #555; }
.sr {
  max-height: 320px;
  overflow-y: auto;
  background: rgba(20, 24, 33, 0.95);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 0 0 8px 8px;
  position: absolute;
  left: 0;
  right: 0;
  z-index: 10;
}
.sr-item {
  padding: 10px 18px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  cursor: pointer;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.sr-item:hover { background: rgba(255, 255, 255, 0.05); }
.nm { font-weight: 600; }
.mt { font-size: 11px; color: #888; }
.ru { color: #ff6b6b; }
.badge-rus { display: inline-block; padding: 1px 6px; border-radius: 10px; font-size: 10px; font-weight: 600; background: rgba(255, 59, 48, 0.2); color: #ff6b6b; margin-left: 4px; }
</style>

<script setup lang="ts">
import { ref } from 'vue'
import { api } from '@/api/client'

const emit = defineEmits<{
  flyTo: [lat: number, lng: number, zoom?: number]
}>()

const query = ref('')
const results = ref<any[]>([])
const focused = ref(false)
let timer: ReturnType<typeof setTimeout> | null = null

function onInput() {
  if (query.value.trim().length < 2) {
    results.value = []
    return
  }
  if (timer) clearTimeout(timer)
  timer = setTimeout(async () => {
    try {
      const data = await api.searchVessels(query.value.trim(), 10)
      results.value = data.features || []
    } catch { results.value = [] }
  }, 300)
}

function select(f: any) {
  const [lng, lat] = f.geometry.coordinates
  emit('flyTo', lat, lng, 14)
  query.value = ''
  results.value = []
  focused.value = false
}

function clearSearch() {
  query.value = ''
  results.value = []
}

function onBlur() {
  setTimeout(() => { focused.value = false }, 200)
}
</script>

<template>
  <div class="search-bar" :class="{ expanded: focused || results.length > 0 }">
    <div class="search-icon">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#888" stroke-width="2.5" stroke-linecap="round">
        <circle cx="11" cy="11" r="7" /><line x1="16.5" y1="16.5" x2="21" y2="21" />
      </svg>
    </div>
    <input
      type="text"
      v-model="query"
      @input="onInput"
      @focus="focused = true"
      @blur="onBlur"
      placeholder="Search vessels by name, MMSI, or IMO..."
      autocomplete="off"
    />
    <button v-if="query" class="clear-btn" @click="clearSearch">&times;</button>
    <div v-if="results.length" class="search-results">
      <div v-for="f in results" :key="f.properties.mmsi" class="result-item" @click="select(f)">
        <div class="result-left">
          <span class="result-name" :class="{ ru: f.properties.is_russian }">
            {{ f.properties.name || 'Unknown' }}
          </span>
          <span class="result-type">{{ f.properties.vessel_type || '' }}</span>
        </div>
        <div class="result-right">
          <span class="result-mmsi">{{ f.properties.mmsi }}</span>
          <span v-if="f.properties.is_russian" class="ru-badge">RUS</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.search-bar {
  position: absolute;
  z-index: 1000;
  top: 12px;
  left: 50%;
  transform: translateX(-50%);
  width: 420px;
  background: rgba(14, 17, 28, 0.94);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 10px;
  backdrop-filter: blur(16px);
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  transition: width 0.2s, box-shadow 0.2s;
  box-shadow: 0 2px 16px rgba(0, 0, 0, 0.3);
}
.search-bar.expanded {
  width: 500px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.5);
}
.search-icon {
  padding: 0 6px 0 14px;
  display: flex;
  align-items: center;
  flex-shrink: 0;
}
input {
  flex: 1;
  padding: 11px 8px;
  background: transparent;
  border: none;
  color: #e0e0e0;
  font-size: 13px;
  outline: none;
  min-width: 0;
}
input::placeholder { color: #555; }
.clear-btn {
  background: none;
  border: none;
  color: #666;
  font-size: 18px;
  padding: 0 12px;
  cursor: pointer;
  line-height: 1;
}
.clear-btn:hover { color: #aaa; }
.search-results {
  width: 100%;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  max-height: 260px;
  overflow-y: auto;
}
.result-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 16px;
  cursor: pointer;
  transition: background 0.1s;
}
.result-item:hover { background: rgba(255, 255, 255, 0.05); }
.result-left { display: flex; flex-direction: column; gap: 1px; }
.result-name { font-size: 13px; font-weight: 600; color: #e0e0e0; }
.result-name.ru { color: #ff6b6b; }
.result-type { font-size: 10px; color: #666; }
.result-right { display: flex; align-items: center; gap: 8px; }
.result-mmsi { font-size: 11px; color: #555; font-family: monospace; }
.ru-badge {
  font-size: 9px;
  font-weight: 700;
  padding: 1px 5px;
  border-radius: 3px;
  background: rgba(255, 60, 60, 0.2);
  color: #ff6b6b;
}
</style>

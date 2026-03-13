<script setup lang="ts">
import { ref } from 'vue'
import { api } from '@/api/client'

const emit = defineEmits<{
  flyTo: [lat: number, lng: number, zoom?: number]
}>()

const query = ref('')
const results = ref<any[]>([])
const collapsed = ref(false)
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
}
</script>

<template>
  <div class="panel search-panel" :class="{ collapsed }">
    <div class="panel-head" @click="collapsed = !collapsed">
      <span>Vessel Search</span>
      <span class="min-btn">{{ collapsed ? '+' : '-' }}</span>
    </div>
    <div v-show="!collapsed" class="panel-body">
      <input
        type="text"
        v-model="query"
        @input="onInput"
        placeholder="Name, MMSI, IMO..."
        autocomplete="off"
      />
      <div v-if="results.length" class="search-results">
        <div
          v-for="f in results"
          :key="f.properties.mmsi"
          class="sres"
          @click="select(f)"
        >
          <span class="sn" :class="{ ru: f.properties.is_russian }">
            {{ f.properties.name || 'Unknown' }}
            <span v-if="f.properties.is_russian"> [RUS]</span>
          </span>
          <span class="sm">{{ f.properties.mmsi }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.search-panel {
  position: absolute;
  z-index: 1000;
  top: 10px;
  left: 50%;
  transform: translateX(-50%);
  width: 360px;
  background: rgba(20, 24, 33, 0.92);
  color: #e0e0e0;
  border-radius: 8px;
  font-size: 13px;
  backdrop-filter: blur(8px);
  border: 1px solid rgba(255, 255, 255, 0.08);
}
.panel-head {
  padding: 10px 14px;
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  cursor: pointer;
  display: flex;
  justify-content: space-between;
  align-items: center;
  user-select: none;
}
.min-btn { font-size: 16px; color: #888; }
.panel-body { padding: 8px 14px 12px; }
input[type='text'] {
  width: 100%;
  padding: 8px 12px;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 4px;
  color: #fff;
  font-size: 13px;
  outline: none;
}
input::placeholder { color: #666; }
input:focus { border-color: #7cb4ff; }
.search-results { max-height: 200px; overflow-y: auto; margin-top: 4px; }
.sres {
  padding: 6px 8px;
  cursor: pointer;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  font-size: 12px;
  display: flex;
  justify-content: space-between;
}
.sres:hover { background: rgba(255, 255, 255, 0.06); }
.sn { font-weight: 600; }
.sm { color: #888; font-size: 11px; }
.ru { color: #ff6b6b; }
</style>

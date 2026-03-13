<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/api/client'

const props = defineProps<{ isLive: boolean }>()
const emit = defineEmits<{ timeChange: [isoTime: string | null] }>()

const collapsed = ref(false)
const sliderValue = ref(100)
const sliderMax = ref(100)
const timeLabel = ref('')
const playing = ref(false)

let timeMin = 0
let timeMax = 0
let playInterval: ReturnType<typeof setInterval> | null = null

onMounted(async () => {
  try {
    const data = await api.getTimeRange()
    if (!data.min || !data.max) return
    timeMax = new Date(data.max).getTime()
    const sevenDays = 7 * 24 * 3600000
    timeMin = timeMax - sevenDays
    sliderMax.value = Math.floor((timeMax - timeMin) / (15 * 60000))
    sliderValue.value = sliderMax.value
  } catch { /* no data yet */ }
})

function onSliderInput() {
  const ms = timeMin + sliderValue.value * 15 * 60000
  const d = new Date(ms)
  timeLabel.value = d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }) +
    ' ' + d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })
  emit('timeChange', d.toISOString())
}

function goLive() {
  stopPlay()
  sliderValue.value = sliderMax.value
  timeLabel.value = ''
  emit('timeChange', null)
}

function togglePlay() {
  if (playing.value) {
    stopPlay()
  } else {
    playing.value = true
    playInterval = setInterval(() => {
      if (sliderValue.value >= sliderMax.value) {
        stopPlay()
        return
      }
      sliderValue.value++
      onSliderInput()
    }, 500)
  }
}

function stopPlay() {
  playing.value = false
  if (playInterval) { clearInterval(playInterval); playInterval = null }
}
</script>

<template>
  <div class="panel time-panel" :class="{ collapsed }">
    <div class="panel-head" @click="collapsed = !collapsed">
      <span>Time Slider</span>
      <span class="min-btn">{{ collapsed ? '+' : '-' }}</span>
    </div>
    <div v-show="!collapsed" class="panel-body">
      <div class="time-controls">
        <button @click="togglePlay" :class="{ active: playing }">{{ playing ? 'Pause' : 'Play' }}</button>
        <input type="range" v-model.number="sliderValue" :min="0" :max="sliderMax" @input="onSliderInput" />
        <span class="time-label">
          <span v-if="props.isLive" class="live-badge">LIVE</span>
          <template v-else>{{ timeLabel }}</template>
        </span>
        <button @click="goLive" :class="{ active: props.isLive }">Live</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.time-panel {
  position: absolute;
  z-index: 1000;
  bottom: 10px;
  left: 50%;
  transform: translateX(-50%);
  width: calc(100vw - 80px);
  max-width: 900px;
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
.panel-body { padding: 8px 14px; }
.time-controls {
  display: flex;
  align-items: center;
  gap: 10px;
}
.time-controls button {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.2);
  color: #fff;
  padding: 4px 12px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
}
.time-controls button:hover { background: rgba(255, 255, 255, 0.2); }
.time-controls button.active { background: #1a73e8; border-color: #1a73e8; }
.time-controls input[type='range'] { flex: 1; accent-color: #7cb4ff; }
.time-label {
  font-size: 12px;
  color: #7cb4ff;
  min-width: 150px;
  text-align: center;
  font-weight: 600;
}
.live-badge {
  background: #2ecc71;
  color: #000;
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 10px;
  font-weight: 700;
}
</style>

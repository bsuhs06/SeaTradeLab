<script setup lang="ts">
import { ref } from 'vue'
import { api } from '@/api/client'
import type { AnalyticsStatus } from '@/types/vessel'

const hours = ref(24)
const distance = ref(500)
const speed = ref(3)
const minDuration = ref(15)
const gapHours = ref(2)
const status = ref<AnalyticsStatus | null>(null)
const output = ref('')
const running = ref(false)
let pollTimer: ReturnType<typeof setInterval> | null = null

const emit = defineEmits<{ completed: [] }>()

async function run(task: string) {
  running.value = true
  output.value = 'Starting...'
  try {
    const data = await api.runAnalytics({
      task,
      hours: hours.value,
      distance: distance.value,
      speed: speed.value,
      min_duration: minDuration.value,
      gap_hours: gapHours.value,
    })
    status.value = data.run
    if (pollTimer) clearInterval(pollTimer)
    pollTimer = setInterval(pollStatus, 2000)
  } catch (e: any) {
    output.value = 'Error: ' + e.message
    running.value = false
  }
}

async function pollStatus() {
  try {
    const s = await api.getAnalyticsStatus()
    status.value = s
    if (s.output) output.value = s.output
    if (s.status !== 'running') {
      running.value = false
      if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
      if (s.status === 'completed') emit('completed')
    }
  } catch { /* ignore */ }
}
</script>

<template>
  <div class="panel">
    <div class="ph"><span>Run Analytics</span><span>Configure and execute detection scripts</span></div>
    <div class="controls">
      <div class="pg"><label>Lookback Hours</label><input type="number" v-model.number="hours" min="1" max="720" /></div>
      <div class="pg"><label>STS Distance (m)</label><input type="number" v-model.number="distance" min="100" max="5000" step="100" /></div>
      <div class="pg"><label>Max Speed (kn)</label><input type="number" v-model.number="speed" min="0.5" max="10" step="0.5" /></div>
      <div class="pg"><label>Min Duration (min)</label><input type="number" v-model.number="minDuration" min="5" max="120" step="5" /></div>
      <div class="pg"><label>AIS Gap Hours</label><input type="number" v-model.number="gapHours" min="0.5" max="48" step="0.5" /></div>
    </div>
    <div class="btn-row">
      <button class="run-btn sts" @click="run('sts')" :disabled="running">STS Detection</button>
      <button class="run-btn gaps" @click="run('gaps')" :disabled="running">AIS Gaps</button>
      <button class="run-btn ports" @click="run('ports')" :disabled="running">Port Tracking</button>
      <button class="run-btn rports" @click="run('russian-ports')" :disabled="running">Russian Ports</button>
      <button class="run-btn detect" @click="run('detect')" :disabled="running">Fleet Summary</button>
      <button class="run-btn all" @click="run('all')" :disabled="running">Run All</button>
    </div>
    <div v-if="status" class="a-status">
      <span :class="status.status">
        {{ status.status === 'running' ? 'Running...' : status.status === 'completed' ? 'Completed' : 'Failed' }}
      </span>
    </div>
    <pre v-if="output" class="a-output">{{ output }}</pre>
  </div>
</template>

<style scoped>
.panel { background: rgba(20, 24, 33, 0.85); border-radius: 8px; border: 1px solid rgba(255, 255, 255, 0.07); overflow: hidden; margin-bottom: 20px; }
.ph { padding: 13px 18px; font-size: 14px; font-weight: 600; border-bottom: 1px solid rgba(255, 255, 255, 0.07); display: flex; justify-content: space-between; align-items: center; }
.ph span:last-child { font-size: 11px; color: #888; font-weight: 400; }
.controls { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 12px; padding: 16px; }
.pg { display: flex; flex-direction: column; gap: 4px; }
.pg label { font-size: 11px; color: #888; text-transform: uppercase; letter-spacing: 0.5px; }
.pg input { padding: 8px 10px; background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.12); border-radius: 4px; color: #fff; font-size: 13px; outline: none; }
.pg input:focus { border-color: #7cb4ff; }
.btn-row { display: flex; gap: 8px; padding: 0 16px 16px; flex-wrap: wrap; }
.run-btn { padding: 8px 20px; border-radius: 6px; font-size: 13px; font-weight: 600; border: none; cursor: pointer; color: #fff; }
.run-btn.sts { background: #e67e22; } .run-btn.sts:hover { background: #d35400; }
.run-btn.gaps { background: #9b59b6; } .run-btn.gaps:hover { background: #8e44ad; }
.run-btn.ports { background: #2ecc71; } .run-btn.ports:hover { background: #27ae60; }
.run-btn.rports { background: #e74c3c; } .run-btn.rports:hover { background: #c0392b; }
.run-btn.detect { background: #3498db; } .run-btn.detect:hover { background: #2980b9; }
.run-btn.all { background: #1a73e8; } .run-btn.all:hover { background: #1557b0; }
.run-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.a-status { padding: 0 16px 8px; font-size: 12px; }
.running { color: #ffa726; }
.completed { color: #66bb6a; }
.failed { color: #ff6b6b; }
.a-output { margin: 0 16px 16px; padding: 12px; background: rgba(0, 0, 0, 0.3); border-radius: 6px; font-family: 'SF Mono', monospace; font-size: 12px; line-height: 1.6; color: #aaa; max-height: 300px; overflow-y: auto; white-space: pre-wrap; }
</style>

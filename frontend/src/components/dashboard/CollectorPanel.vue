<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { api } from '@/api/client'
import type { CollectorStatus } from '@/types/vessel'
import { formatAgo } from '@/composables/useVesselUtils'

const status = ref<CollectorStatus | null>(null)
let pollInterval: ReturnType<typeof setInterval> | null = null

async function poll() {
  try { status.value = await api.getCollectorStatus() } catch { /* ignore */ }
}

async function start() {
  try {
    await api.startCollector()
    setTimeout(poll, 1000)
  } catch (e: any) { alert('Error: ' + e.message) }
}

async function stop() {
  try {
    await api.stopCollector()
    setTimeout(poll, 1000)
  } catch (e: any) { alert('Error: ' + e.message) }
}

onMounted(() => { poll(); pollInterval = setInterval(poll, 10000) })
onUnmounted(() => { if (pollInterval) clearInterval(pollInterval) })

function isRunning(): boolean {
  if (!status.value) return false
  return status.value.managed_running || (status.value.external_pids?.length ?? 0) > 0
}
</script>

<template>
  <div class="panel">
    <div class="ph"><span>AIS Collector</span><span>Process control</span></div>
    <div class="status-row">
      <span class="dot" :class="{ on: isRunning(), off: !isRunning() && status?.binary_found, ext: !status?.managed_running && (status?.external_pids?.length ?? 0) > 0 }" />
      <span class="status-text">
        <template v-if="!status">Checking...</template>
        <template v-else-if="!status.binary_found">Binary not found</template>
        <template v-else-if="status.managed_running">Running</template>
        <template v-else-if="(status.external_pids?.length ?? 0) > 0">Running (external)</template>
        <template v-else>Stopped</template>
      </span>
      <span class="status-detail">
        <template v-if="status?.managed_running">PID: {{ status.managed_pid }}</template>
        <template v-if="status?.last_collected"> | Last poll: {{ formatAgo(status.last_collected) }}</template>
      </span>
    </div>
    <div class="ctl-btns">
      <button class="ctl-btn start" @click="start" :disabled="isRunning() || !status?.binary_found">Start Collector</button>
      <button class="ctl-btn stop" @click="stop" :disabled="!isRunning()">Stop Collector</button>
    </div>
    <pre v-if="status?.log" class="collector-log">{{ status.log }}</pre>
  </div>
</template>

<style scoped>
.panel { background: rgba(20, 24, 33, 0.85); border-radius: 8px; border: 1px solid rgba(255, 255, 255, 0.07); overflow: hidden; margin-bottom: 20px; }
.ph { padding: 13px 18px; font-size: 14px; font-weight: 600; border-bottom: 1px solid rgba(255, 255, 255, 0.07); display: flex; justify-content: space-between; align-items: center; }
.ph span:last-child { font-size: 11px; color: #888; font-weight: 400; }
.status-row { display: flex; align-items: center; gap: 14px; padding: 16px; }
.dot { width: 12px; height: 12px; border-radius: 50%; flex-shrink: 0; }
.dot.on { background: #66bb6a; box-shadow: 0 0 8px rgba(102, 187, 106, 0.5); }
.dot.off { background: #ff6b6b; box-shadow: 0 0 8px rgba(255, 107, 107, 0.5); }
.dot.ext { background: #ffa726; box-shadow: 0 0 8px rgba(255, 167, 38, 0.5); }
.status-text { font-size: 14px; font-weight: 600; flex: 1; }
.status-detail { font-size: 12px; color: #888; }
.ctl-btns { display: flex; gap: 8px; padding: 0 16px 16px; }
.ctl-btn { padding: 8px 24px; border-radius: 6px; font-size: 13px; font-weight: 600; border: none; cursor: pointer; color: #fff; }
.ctl-btn.start { background: #2ecc71; }
.ctl-btn.start:hover { background: #27ae60; }
.ctl-btn.stop { background: #e74c3c; }
.ctl-btn.stop:hover { background: #c0392b; }
.ctl-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.collector-log { margin: 0 16px 16px; padding: 12px; background: rgba(0, 0, 0, 0.3); border-radius: 6px; font-family: 'SF Mono', monospace; font-size: 12px; line-height: 1.6; color: #aaa; max-height: 200px; overflow-y: auto; white-space: pre-wrap; }
</style>

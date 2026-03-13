<script setup lang="ts">
import { formatNumber } from '@/composables/useVesselUtils'
import type { Stats } from '@/types/vessel'

defineProps<{
  stats: Stats | null
  stsCount: number
  darkCount: number
  portVisitCount: number
}>()
</script>

<template>
  <div class="stats-row">
    <div class="sc"><div class="v">{{ formatNumber(stats?.total_vessels) }}</div><div class="l">Total Vessels Tracked</div></div>
    <div class="sc alert"><div class="v">{{ formatNumber(stats?.russian_vessels) }}</div><div class="l">Russian-Flagged Vessels</div></div>
    <div class="sc"><div class="v">{{ formatNumber(stats?.total_positions) }}</div><div class="l">Position Reports</div></div>
    <div class="sc alert"><div class="v">{{ stsCount }}</div><div class="l">STS Alerts (7d)</div></div>
    <div class="sc"><div class="v">{{ darkCount }}</div><div class="l">Dark Vessels (6h+)</div></div>
    <div class="sc warn"><div class="v">{{ portVisitCount }}</div><div class="l">Foreign Vessels in RUS Ports</div></div>
  </div>
</template>

<style scoped>
.stats-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 14px; margin-bottom: 28px; }
.sc { background: rgba(20, 24, 33, 0.85); padding: 18px; border-radius: 8px; border: 1px solid rgba(255, 255, 255, 0.07); }
.sc .v { font-size: 28px; font-weight: 700; color: #7cb4ff; }
.sc .l { font-size: 12px; color: #888; margin-top: 4px; }
.sc.alert .v { color: #ff6b6b; }
.sc.warn .v { color: #ffa726; }
</style>

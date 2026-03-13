<script setup lang="ts">
import { formatTime } from '@/composables/useVesselUtils'
import type { STSEvent } from '@/types/vessel'

defineProps<{ events: STSEvent[] }>()
</script>

<template>
  <div class="panel">
    <div class="ph"><span>Detected Ship-to-Ship Transfers</span><span>Last 7 days</span></div>
    <div style="overflow-x: auto">
      <table>
        <thead><tr><th>Time</th><th>Vessel A</th><th>Vessel B</th><th>Duration</th><th>Min Dist</th><th>Confidence</th><th></th></tr></thead>
        <tbody>
          <tr v-if="!events.length"><td colspan="7" class="empty">No STS events detected yet</td></tr>
          <tr v-for="e in events" :key="e.start_time + e.mmsi_a">
            <td>{{ formatTime(e.start_time) }}</td>
            <td :class="{ russian: String(e.mmsi_a).startsWith('273') }">{{ e.name_a || e.mmsi_a }}</td>
            <td :class="{ russian: String(e.mmsi_b).startsWith('273') }">{{ e.name_b || e.mmsi_b }}</td>
            <td>{{ e.duration_minutes }} min</td>
            <td>{{ e.min_distance_m != null ? Math.round(e.min_distance_m) + 'm' : '--' }}</td>
            <td><span class="badge" :class="e.confidence">{{ e.confidence.toUpperCase() }}</span></td>
            <td>
              <router-link
                v-if="e.avg_lat && e.avg_lon"
                :to="{ name: 'map', hash: `#goto=${e.avg_lat},${e.avg_lon},14` }"
                class="link"
              >View</router-link>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.panel { background: rgba(20, 24, 33, 0.85); border-radius: 8px; border: 1px solid rgba(255, 255, 255, 0.07); overflow: hidden; }
.ph { padding: 13px 18px; font-size: 14px; font-weight: 600; border-bottom: 1px solid rgba(255, 255, 255, 0.07); display: flex; justify-content: space-between; align-items: center; }
.ph span:last-child { font-size: 11px; color: #888; font-weight: 400; }
table { width: 100%; border-collapse: collapse; }
th { text-align: left; padding: 9px 14px; font-size: 11px; color: #888; text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid rgba(255, 255, 255, 0.08); }
td { padding: 9px 14px; font-size: 13px; border-bottom: 1px solid rgba(255, 255, 255, 0.04); }
tr:hover { background: rgba(255, 255, 255, 0.02); }
.empty { text-align: center; padding: 32px; color: #555; font-size: 13px; }
.russian { color: #ff6b6b; }
.badge { display: inline-block; padding: 2px 8px; border-radius: 10px; font-size: 11px; font-weight: 600; }
.badge.high { background: rgba(255, 59, 48, 0.2); color: #ff6b6b; }
.badge.medium { background: rgba(255, 159, 0, 0.2); color: #ffa726; }
.badge.low { background: rgba(76, 175, 80, 0.2); color: #66bb6a; }
.link { color: #7cb4ff; cursor: pointer; }
</style>

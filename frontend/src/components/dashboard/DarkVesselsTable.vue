<script setup lang="ts">
import { formatAgo } from '@/composables/useVesselUtils'
import type { VesselFeature } from '@/types/vessel'

defineProps<{ vessels: VesselFeature[] }>()
</script>

<template>
  <div class="panel">
    <div class="ph"><span>Dark Vessels</span><span>AIS off 6h+</span></div>
    <div style="max-height: 400px; overflow-y: auto">
      <table>
        <thead><tr><th>Vessel</th><th>Last Seen</th><th>Gap</th></tr></thead>
        <tbody>
          <tr v-if="!vessels.length"><td colspan="3" class="empty">No dark vessels currently</td></tr>
          <tr v-for="f in vessels.slice(0, 50)" :key="f.properties.mmsi">
            <td :class="{ russian: f.properties.is_russian }">
              {{ f.properties.name || f.properties.mmsi }}
              <span v-if="f.properties.is_russian"> [RUS]</span>
            </td>
            <td>{{ formatAgo(f.properties.timestamp) }}</td>
            <td>{{ f.properties.gap_hours != null ? f.properties.gap_hours + 'h' : '--' }}</td>
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
</style>

<script setup lang="ts">
import { computed } from 'vue'
import type { VesselProperties } from '@/types/vessel'
import { classifyVessel, getVesselColor, formatAgo } from '@/composables/useVesselUtils'

const props = defineProps<{
  vessel: VesselProperties | null
}>()

const emit = defineEmits<{
  close: []
  loadTrack: [mmsi: number]
  clearTrack: []
  toggleFavorite: [mmsi: number]
}>()

const flag = computed(() => {
  if (!props.vessel) return ''
  const mid = String(props.vessel.mmsi).substring(0, 3)
  const flags: Record<string, string> = {
    '201':'Albania','205':'Belgium','209':'Cyprus','210':'Cyprus','211':'Germany',
    '212':'Cyprus','215':'Malta','219':'Denmark','220':'Denmark','224':'Spain',
    '225':'Spain','226':'France','227':'France','228':'France','229':'Malta',
    '230':'Finland','232':'UK','233':'UK','234':'UK','235':'UK',
    '237':'Greece','238':'Croatia','239':'Greece','240':'Greece','241':'Greece',
    '244':'Netherlands','245':'Netherlands','246':'Netherlands','247':'Italy',
    '248':'Malta','249':'Malta','250':'Ireland','255':'Portugal','256':'Malta',
    '257':'Norway','258':'Norway','259':'Norway','261':'Poland','265':'Sweden',
    '266':'Sweden','271':'Turkey','272':'Ukraine','273':'Russia',
    '308':'Bahamas','309':'Bahamas','311':'Bahamas','312':'Belize',
    '314':'Barbados','316':'Canada','319':'Cayman Islands','338':'USA',
    '345':'Mexico','351':'Panama','352':'Panama','353':'Panama','354':'Panama',
    '355':'Panama','356':'Panama','357':'Panama','366':'USA','367':'USA',
    '368':'USA','369':'USA','370':'Panama','371':'Panama','372':'Panama',
    '403':'Saudi Arabia','405':'Bangladesh','412':'China','413':'China',
    '416':'Taiwan','419':'India','422':'Iran','428':'Israel',
    '431':'Japan','432':'Japan','440':'South Korea','441':'South Korea',
    '445':'DPRK','447':'Kuwait','461':'Oman','463':'Pakistan','470':'UAE',
    '471':'UAE','477':'Hong Kong','503':'Australia','525':'Indonesia',
    '533':'Malaysia','538':'Marshall Islands','548':'Philippines',
    '563':'Singapore','564':'Singapore','566':'Singapore','567':'Thailand',
    '574':'Vietnam','601':'South Africa','605':'Algeria','618':'Egypt',
    '620':'Eritrea','622':'Gabon','624':'Ghana','631':'Liberia',
    '632':'Liberia','633':'Liberia','636':'Madagascar','649':'Nigeria',
    '657':'Sierra Leone','664':'Tunisia',
  }
  return flags[mid] || ''
})

const category = computed(() => {
  if (!props.vessel) return 'other'
  return classifyVessel(props.vessel)
})

const vesselColor = computed(() => {
  if (!props.vessel) return '#888'
  return getVesselColor(props.vessel)
})

const lastAIS = computed(() => {
  if (!props.vessel) return ''
  return formatAgo(props.vessel.timestamp)
})

const agoMinutes = computed(() => {
  if (!props.vessel) return 0
  return Math.round((Date.now() - new Date(props.vessel.timestamp).getTime()) / 60000)
})

const statusLabel = computed(() => {
  if (!props.vessel) return ''
  const min = agoMinutes.value
  if (min < 10) return 'Live'
  if (min < 60) return 'Recent'
  if (min < 360) return 'Delayed'
  return 'Dark'
})

const statusClass = computed(() => {
  const min = agoMinutes.value
  if (min < 10) return 'status-live'
  if (min < 60) return 'status-recent'
  if (min < 360) return 'status-delayed'
  return 'status-dark'
})

const speedFormatted = computed(() => {
  if (!props.vessel || props.vessel.sog === undefined) return '--'
  return `${props.vessel.sog.toFixed(1)} kn`
})

const courseFormatted = computed(() => {
  if (!props.vessel || props.vessel.cog === undefined) return '--'
  return `${props.vessel.cog.toFixed(0)}°`
})

const headingFormatted = computed(() => {
  if (!props.vessel || props.vessel.heading === undefined) return '--'
  return `${props.vessel.heading}°`
})
</script>

<template>
  <Transition name="slide">
    <div v-if="vessel" class="vessel-panel">
      <div class="panel-header">
        <div class="header-top">
          <button class="close-btn" @click="emit('close')" title="Close">&times;</button>
        </div>
        <div class="vessel-identity">
          <div class="vessel-dot" :style="{ background: vesselColor }" />
          <div>
            <div class="vessel-name" :class="{ russian: vessel.is_russian }">
              {{ vessel.name || 'Unknown' }}
              <span v-if="vessel.is_russian" class="rus-tag">RUS</span>
            </div>
            <div class="vessel-subtitle">
              {{ vessel.vessel_type || 'Unknown type' }}
              <span v-if="flag" class="flag-text">· {{ flag }}</span>
            </div>
          </div>
        </div>
        <div class="status-row">
          <span class="status-badge" :class="statusClass">{{ statusLabel }}</span>
          <span class="ais-ago">{{ lastAIS }}</span>
        </div>
      </div>

      <div class="panel-body">
        <div class="info-section">
          <div class="section-title">Navigation</div>
          <div class="info-grid">
            <div class="info-item">
              <span class="info-label">Speed</span>
              <span class="info-value">{{ speedFormatted }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">Course</span>
              <span class="info-value">{{ courseFormatted }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">Heading</span>
              <span class="info-value">{{ headingFormatted }}</span>
            </div>
            <div class="info-item" v-if="vessel.nav_status">
              <span class="info-label">Status</span>
              <span class="info-value">{{ vessel.nav_status }}</span>
            </div>
          </div>
        </div>

        <div class="info-section" v-if="vessel.destination">
          <div class="section-title">Voyage</div>
          <div class="info-grid">
            <div class="info-item wide">
              <span class="info-label">Destination</span>
              <span class="info-value dest">{{ vessel.destination }}</span>
            </div>
            <div class="info-item" v-if="vessel.draught">
              <span class="info-label">Draught</span>
              <span class="info-value">{{ vessel.draught }}m</span>
            </div>
          </div>
        </div>

        <div class="info-section">
          <div class="section-title">Identification</div>
          <div class="info-grid">
            <div class="info-item">
              <span class="info-label">MMSI</span>
              <span class="info-value mono">{{ vessel.mmsi }}</span>
            </div>
            <div class="info-item" v-if="vessel.imo">
              <span class="info-label">IMO</span>
              <span class="info-value mono">{{ vessel.imo }}</span>
            </div>
            <div class="info-item" v-if="vessel.call_sign">
              <span class="info-label">Call Sign</span>
              <span class="info-value mono">{{ vessel.call_sign }}</span>
            </div>
            <div class="info-item" v-if="vessel.sources">
              <span class="info-label">Sources</span>
              <span class="info-value">{{ vessel.sources }}</span>
            </div>
          </div>
        </div>

        <div class="action-buttons">
          <button class="action-btn primary" @click="emit('loadTrack', vessel.mmsi)">
            <span class="btn-icon">&#x1F5FA;</span> Show 7-Day Track
          </button>
          <button class="action-btn" @click="emit('clearTrack')">
            Clear Track
          </button>
          <button class="action-btn fav-btn" @click="emit('toggleFavorite', vessel.mmsi)">
            &#9733; Favorite
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.vessel-panel {
  position: absolute;
  top: 0;
  right: 0;
  z-index: 1100;
  width: 340px;
  height: 100%;
  background: rgba(14, 17, 28, 0.96);
  backdrop-filter: blur(16px);
  border-left: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  flex-direction: column;
  overflow-y: auto;
  box-shadow: -4px 0 24px rgba(0, 0, 0, 0.5);
}

.slide-enter-active,
.slide-leave-active {
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}
.slide-enter-from,
.slide-leave-to {
  transform: translateX(100%);
}

.panel-header {
  padding: 16px 18px 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.header-top {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 8px;
}

.close-btn {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #aaa;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  font-size: 18px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
}
.close-btn:hover { background: rgba(255, 80, 80, 0.3); color: #fff; }

.vessel-identity {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.vessel-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  margin-top: 5px;
  flex-shrink: 0;
  box-shadow: 0 0 6px currentColor;
}

.vessel-name {
  font-size: 16px;
  font-weight: 700;
  color: #fff;
  letter-spacing: 0.3px;
}
.vessel-name.russian { color: #ff6b6b; }

.rus-tag {
  background: rgba(255, 60, 60, 0.25);
  color: #ff6b6b;
  font-size: 9px;
  padding: 1px 5px;
  border-radius: 3px;
  margin-left: 6px;
  font-weight: 600;
  vertical-align: middle;
}

.vessel-subtitle {
  font-size: 12px;
  color: #888;
  margin-top: 2px;
}
.flag-text { color: #666; }

.status-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 10px;
}

.status-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 10px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.status-live { background: rgba(46, 204, 113, 0.2); color: #2ecc71; }
.status-recent { background: rgba(124, 180, 255, 0.2); color: #7cb4ff; }
.status-delayed { background: rgba(230, 126, 34, 0.2); color: #e67e22; }
.status-dark { background: rgba(255, 68, 68, 0.2); color: #ff4444; }

.ais-ago { font-size: 11px; color: #666; }

.panel-body {
  padding: 12px 18px 24px;
  flex: 1;
}

.info-section {
  margin-bottom: 16px;
}

.section-title {
  font-size: 10px;
  font-weight: 600;
  color: #555;
  text-transform: uppercase;
  letter-spacing: 0.8px;
  margin-bottom: 8px;
  padding-bottom: 4px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}

.info-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.info-item.wide { grid-column: 1 / -1; }

.info-label {
  font-size: 10px;
  color: #555;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.info-value {
  font-size: 14px;
  color: #e0e0e0;
  font-weight: 500;
}
.info-value.mono { font-family: 'SF Mono', monospace; font-size: 13px; }
.info-value.dest { color: #7cb4ff; }

.action-buttons {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 20px;
}

.action-btn {
  width: 100%;
  padding: 10px 14px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.04);
  color: #ccc;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  transition: background 0.15s;
}
.action-btn:hover { background: rgba(255, 255, 255, 0.1); color: #fff; }
.action-btn.primary {
  background: rgba(79, 195, 247, 0.15);
  border-color: rgba(79, 195, 247, 0.3);
  color: #4fc3f7;
}
.action-btn.primary:hover { background: rgba(79, 195, 247, 0.25); }
.btn-icon { font-size: 15px; }
.fav-btn { color: #ffd700; }
</style>

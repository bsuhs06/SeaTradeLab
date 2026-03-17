<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { api } from '@/api/client'
import type { Stats, STSEvent, VesselFeature, PortVisit } from '@/types/vessel'
import StatsBar from '@/components/dashboard/StatsBar.vue'
import VesselSearch from '@/components/dashboard/VesselSearch.vue'
import PortVisitsTable from '@/components/dashboard/PortVisitsTable.vue'
import StsEventsTable from '@/components/dashboard/StsEventsTable.vue'
import DarkVesselsTable from '@/components/dashboard/DarkVesselsTable.vue'
import AnalyticsPanel from '@/components/dashboard/AnalyticsPanel.vue'
import CollectorPanel from '@/components/dashboard/CollectorPanel.vue'
import PortManager from '@/components/dashboard/PortManager.vue'
import VesselRegistry from '@/components/dashboard/VesselRegistry.vue'
import TaintTracker from '@/components/dashboard/TaintTracker.vue'
import FavoritesPanel from '@/components/dashboard/FavoritesPanel.vue'
import FaqSection from '@/components/dashboard/FaqSection.vue'

const stats = ref<Stats | null>(null)
const stsEvents = ref<STSEvent[]>([])
const darkVessels = ref<VesselFeature[]>([])
const portVisits = ref<PortVisit[]>([])

const collapsed = reactive<Record<string, boolean>>({
  search: false,
  favorites: false,
  registry: false,
  taint: false,
  ports: false,
  sts: false,
  dark: false,
  analytics: true,
  collector: true,
  portMgmt: true,
  faq: true,
})

function toggle(key: string) {
  collapsed[key] = !collapsed[key]
}

async function loadStats() {
  try { stats.value = await api.getStats() } catch { /* ignore */ }
}

async function loadSTS() {
  try {
    const data = await api.getSTSEvents(168)
    stsEvents.value = data.events || []
  } catch { /* ignore */ }
}

async function loadDark() {
  try {
    const data = await api.getDarkVessels(6)
    darkVessels.value = data.features || []
  } catch { /* ignore */ }
}

async function loadPortVisits(nonRussian = true) {
  try {
    const data = await api.getPortVisits(720, nonRussian)
    portVisits.value = data.visits || []
  } catch { /* ignore */ }
}

function refreshAll() {
  loadStats()
  loadSTS()
  loadDark()
  loadPortVisits()
}

onMounted(refreshAll)
</script>

<template>
  <div class="dashboard">
    <div class="header">
      <h1>Sea-Trade Lab</h1>
      <div class="sub">Real-time AIS monitoring, ship-to-ship transfer, and shadow fleet detection</div>
      <div class="nav">
        <router-link to="/map" class="pri">Open Live Map</router-link>
        <a href="#favorites" class="sec">Favorites</a>
        <a href="#registry" class="sec">Vessel Registry</a>
        <a href="#taint" class="sec">Taint Tracker</a>
        <a href="#events" class="sec">STS Events</a>
        <a href="#port-visitors" class="sec">Port Visitors</a>
        <a href="#analytics" class="sec">Run Analytics</a>
        <a href="#collector" class="sec">Collector</a>
        <a href="#port-mgmt" class="sec">Ports</a>
        <a href="#faq" class="sec">How It Works</a>
      </div>
    </div>

    <div class="container">
      <StatsBar
        :stats="stats"
        :sts-count="stsEvents.length"
        :dark-count="darkVessels.length"
        :port-visit-count="portVisits.length"
      />

      <div class="section">
        <div class="section-bar" @click="toggle('search')">
          <span class="section-title">Vessel Search</span>
          <span class="chevron" :class="{ open: !collapsed.search }">&#9650;</span>
        </div>
        <div v-show="!collapsed.search" class="section-body">
          <VesselSearch />
        </div>
      </div>

      <div id="favorites" class="section">
        <div class="section-bar" @click="toggle('favorites')">
          <span class="section-title">⭐ Bookmarked Vessels</span>
          <span class="chevron" :class="{ open: !collapsed.favorites }">&#9650;</span>
        </div>
        <div v-show="!collapsed.favorites" class="section-body">
          <FavoritesPanel />
        </div>
      </div>

      <div id="registry" class="section">
        <div class="section-bar" @click="toggle('registry')">
          <span class="section-title">Vessel Registry</span>
          <span class="chevron" :class="{ open: !collapsed.registry }">&#9650;</span>
        </div>
        <div v-show="!collapsed.registry" class="section-body">
          <VesselRegistry />
        </div>
      </div>

      <div id="taint" class="section">
        <div class="section-bar" @click="toggle('taint')">
          <span class="section-title">Taint Tracker — Petroleum Tankers</span>
          <span class="chevron" :class="{ open: !collapsed.taint }">&#9650;</span>
        </div>
        <div v-show="!collapsed.taint" class="section-body">
          <TaintTracker />
        </div>
      </div>

      <div id="port-visitors" class="section">
        <div class="section-bar" @click="toggle('ports')">
          <span class="section-title">Vessels Visiting Russian Ports</span>
          <span class="chevron" :class="{ open: !collapsed.ports }">&#9650;</span>
        </div>
        <div v-show="!collapsed.ports" class="section-body">
          <PortVisitsTable :visits="portVisits" @mode-change="loadPortVisits" />
        </div>
      </div>

      <div id="events" class="section">
        <div class="section-bar" @click="toggle('sts')">
          <span class="section-title">STS Events</span>
          <span class="chevron" :class="{ open: !collapsed.sts }">&#9650;</span>
        </div>
        <div v-show="!collapsed.sts" class="section-body">
          <StsEventsTable :events="stsEvents" @updated="loadSTS" />
        </div>
      </div>

      <div class="section">
        <div class="section-bar" @click="toggle('dark')">
          <span class="section-title">Dark Vessels</span>
          <span class="chevron" :class="{ open: !collapsed.dark }">&#9650;</span>
        </div>
        <div v-show="!collapsed.dark" class="section-body">
          <DarkVesselsTable :vessels="darkVessels" />
        </div>
      </div>

      <div id="analytics" class="section">
        <div class="section-bar" @click="toggle('analytics')">
          <span class="section-title">Run Analytics</span>
          <span class="chevron" :class="{ open: !collapsed.analytics }">&#9650;</span>
        </div>
        <div v-show="!collapsed.analytics" class="section-body">
          <AnalyticsPanel @completed="refreshAll" />
        </div>
      </div>

      <div id="collector" class="section">
        <div class="section-bar" @click="toggle('collector')">
          <span class="section-title">AIS Collector</span>
          <span class="chevron" :class="{ open: !collapsed.collector }">&#9650;</span>
        </div>
        <div v-show="!collapsed.collector" class="section-body">
          <CollectorPanel />
        </div>
      </div>

      <div id="port-mgmt" class="section">
        <div class="section-bar" @click="toggle('portMgmt')">
          <span class="section-title">Port Definitions</span>
          <span class="chevron" :class="{ open: !collapsed.portMgmt }">&#9650;</span>
        </div>
        <div v-show="!collapsed.portMgmt" class="section-body">
          <PortManager />
        </div>
      </div>

      <div id="faq" class="section">
        <div class="section-bar" @click="toggle('faq')">
          <span class="section-title">How It Works</span>
          <span class="chevron" :class="{ open: !collapsed.faq }">&#9650;</span>
        </div>
        <div v-show="!collapsed.faq" class="section-body">
          <FaqSection />
        </div>
      </div>
    </div>

    <div class="footer">SeaTradeLab | Data: Finnish Digitraffic Maritime API, aisstream.io</div>
  </div>
</template>

<style scoped>
.dashboard {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif;
  background: #0d1117;
  color: #e0e0e0;
  min-height: 100vh;
}
a { color: #7cb4ff; text-decoration: none; }
a:hover { text-decoration: underline; }
.header {
  background: linear-gradient(135deg, #0d1117 0%, #1a2332 100%);
  padding: 48px 24px;
  text-align: center;
  border-bottom: 1px solid rgba(124, 180, 255, 0.15);
}
.header h1 { font-size: 52px; color: #7cb4ff; letter-spacing: 10px; font-weight: 800; }
.sub { font-size: 13px; color: #888; letter-spacing: 2px; margin-top: 6px; text-transform: uppercase; }
.tag { font-size: 17px; color: #ccc; margin-top: 18px; }
.nav { display: flex; justify-content: center; gap: 12px; margin-top: 28px; flex-wrap: wrap; }
.nav a { padding: 10px 28px; border-radius: 6px; font-size: 14px; font-weight: 600; text-decoration: none; }
.pri { background: #1a73e8; color: #fff; }
.pri:hover { background: #1557b0; text-decoration: none; }
.sec { background: rgba(255, 255, 255, 0.07); color: #ccc; border: 1px solid rgba(255, 255, 255, 0.12); }
.sec:hover { background: rgba(255, 255, 255, 0.12); text-decoration: none; }
.container { max-width: 1200px; margin: 0 auto; padding: 24px; }
.section { margin-bottom: 8px; border-radius: 8px; overflow: hidden; border: 1px solid rgba(255, 255, 255, 0.08); }
.section-bar {
  display: flex; justify-content: space-between; align-items: center;
  padding: 12px 16px; cursor: pointer; user-select: none;
  background: rgba(255, 255, 255, 0.04);
  transition: background 0.15s;
}
.section-bar:hover { background: rgba(255, 255, 255, 0.08); }
.section-title { font-size: 14px; font-weight: 600; color: #ccc; letter-spacing: 0.5px; }
.chevron {
  font-size: 10px; color: #666; transition: transform 0.2s;
  transform: rotate(180deg);
}
.chevron.open { transform: rotate(0deg); }
.section-body { border-top: 1px solid rgba(255, 255, 255, 0.06); }
.two-col { display: grid; grid-template-columns: 2fr 1fr; gap: 20px; margin-bottom: 28px; }
@media (max-width: 860px) { .two-col { grid-template-columns: 1fr; } }
.footer { text-align: center; padding: 28px; font-size: 12px; color: #444; border-top: 1px solid rgba(255, 255, 255, 0.06); margin-top: 48px; }
</style>

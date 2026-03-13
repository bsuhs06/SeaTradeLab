<script setup lang="ts">
import { ref, onMounted } from 'vue'
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
import FaqSection from '@/components/dashboard/FaqSection.vue'

const stats = ref<Stats | null>(null)
const stsEvents = ref<STSEvent[]>([])
const darkVessels = ref<VesselFeature[]>([])
const portVisits = ref<PortVisit[]>([])
const portVisitCount = ref(0)

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
    if (nonRussian) portVisitCount.value = data.count
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
      <h1>SeaTradeLab</h1>
      <div class="sub">Baltic AIS monitoring and shadow fleet detection</div>
      <div class="tag">Real-time maritime surveillance and ship-to-ship transfer detection</div>
      <div class="nav">
        <router-link to="/map" class="pri">Open Live Map</router-link>
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
        :port-visit-count="portVisitCount"
      />

      <VesselSearch />

      <div id="port-visitors">
        <PortVisitsTable :visits="portVisits" @mode-change="loadPortVisits" />
      </div>

      <div class="two-col" id="events">
        <StsEventsTable :events="stsEvents" />
        <DarkVesselsTable :vessels="darkVessels" />
      </div>

      <div id="analytics">
        <AnalyticsPanel @completed="refreshAll" />
      </div>

      <div id="collector">
        <CollectorPanel />
      </div>

      <div id="port-mgmt">
        <PortManager />
      </div>

      <div id="faq">
        <FaqSection />
      </div>
    </div>

    <div class="footer">SeaTradeLab | Data: Finnish Digitraffic Maritime API</div>
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
.two-col { display: grid; grid-template-columns: 2fr 1fr; gap: 20px; margin-bottom: 28px; }
@media (max-width: 860px) { .two-col { grid-template-columns: 1fr; } }
.footer { text-align: center; padding: 28px; font-size: 12px; color: #444; border-top: 1px solid rgba(255, 255, 255, 0.06); margin-top: 48px; }
</style>

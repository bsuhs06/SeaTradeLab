import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@/api/client'
import type {
  VesselFeatureCollection,
  Stats,
  STSResponse,
  DarkVesselResponse,
  PortVisitResponse,
  Port,
  CollectorStatus,
  AnalyticsStatus,
  TimeRange,
} from '@/types/vessel'

export const useVesselStore = defineStore('vessels', () => {
  const vessels = ref<VesselFeatureCollection | null>(null)
  const stats = ref<Stats | null>(null)
  const stsEvents = ref<STSResponse | null>(null)
  const darkVessels = ref<DarkVesselResponse | null>(null)
  const portVisits = ref<PortVisitResponse | null>(null)
  const ports = ref<Port[]>([])
  const collectorStatus = ref<CollectorStatus | null>(null)
  const analyticsStatus = ref<AnalyticsStatus | null>(null)
  const timeRange = ref<TimeRange | null>(null)
  const loading = ref(false)

  async function fetchVessels() {
    vessels.value = await api.getVessels()
  }

  async function fetchStats() {
    stats.value = await api.getStats()
  }

  async function fetchSTSEvents(hours = 168) {
    stsEvents.value = await api.getSTSEvents(hours)
  }

  async function fetchDarkVessels(minHours = 6) {
    darkVessels.value = await api.getDarkVessels(minHours)
  }

  async function fetchPortVisits(hours = 720, nonRussian = false) {
    portVisits.value = await api.getPortVisits(hours, nonRussian)
  }

  async function fetchPorts() {
    ports.value = await api.getPorts()
  }

  async function fetchCollectorStatus() {
    collectorStatus.value = await api.getCollectorStatus()
  }

  async function fetchAnalyticsStatus() {
    analyticsStatus.value = await api.getAnalyticsStatus()
  }

  async function fetchTimeRange() {
    timeRange.value = await api.getTimeRange()
  }

  async function fetchAll() {
    loading.value = true
    try {
      await Promise.all([fetchVessels(), fetchStats(), fetchSTSEvents(), fetchDarkVessels()])
    } finally {
      loading.value = false
    }
  }

  return {
    vessels,
    stats,
    stsEvents,
    darkVessels,
    portVisits,
    ports,
    collectorStatus,
    analyticsStatus,
    timeRange,
    loading,
    fetchVessels,
    fetchStats,
    fetchSTSEvents,
    fetchDarkVessels,
    fetchPortVisits,
    fetchPorts,
    fetchCollectorStatus,
    fetchAnalyticsStatus,
    fetchTimeRange,
    fetchAll,
  }
})

import type { VesselProperties, VesselCategory } from '@/types/vessel'

const VESSEL_COLORS: Record<VesselCategory, string> = {
  tanker: '#e67e22',
  cargo: '#1a73e8',
  passenger: '#2ecc71',
  tug: '#9b59b6',
  fishing: '#16a085',
  other: '#95a5a6',
}

export function classifyVessel(p: VesselProperties): VesselCategory {
  const t = (p.vessel_type || '').toLowerCase()
  if (t.includes('tanker')) return 'tanker'
  if (t.includes('cargo') || t.includes('container')) return 'cargo'
  if (t.includes('passenger')) return 'passenger'
  if (t.includes('tug') || t.includes('tow') || t.includes('pilot')) return 'tug'
  if (t.includes('fish')) return 'fishing'
  return 'other'
}

export function getVesselColor(p: VesselProperties): string {
  if (p.is_russian) return '#d32f2f'
  return VESSEL_COLORS[classifyVessel(p)]
}

export function isStaleAIS(p: VesselProperties): boolean {
  if (!p.timestamp) return false
  return (Date.now() - new Date(p.timestamp).getTime()) / 3600000 >= 6
}

export function formatAgo(ts: string): string {
  const h = (Date.now() - new Date(ts).getTime()) / 3600000
  if (h < 1) return `${Math.round(h * 60)}m ago`
  if (h < 24) return `${Math.round(h)}h ago`
  return `${Math.round(h / 24)}d ago`
}

export function formatTime(ts: string): string {
  const d = new Date(ts)
  return (
    d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }) +
    ' ' +
    d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })
  )
}

export function formatDuration(h: number | null | undefined): string {
  if (h == null) return '--'
  if (h < 1) return `${Math.round(h * 60)}m`
  if (h < 24) return `${h.toFixed(1)}h`
  return `${(h / 24).toFixed(1)}d`
}

export function formatNumber(n: number | null | undefined): string {
  if (n == null) return '--'
  return Number(n).toLocaleString()
}

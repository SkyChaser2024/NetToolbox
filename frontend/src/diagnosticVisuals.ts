export const featureIcons = {
  brand: 'activity',
  overview: 'dashboard',
  auth: 'shield',
  nat: 'network',
  ipv6: 'globe',
  ping: 'radar',
  trace: 'route',
  settings: 'settings'
} as const

export const diagnosticActionIcons = {
  overviewAction: 'refresh',
  pingAction: 'radar'
} as const

export type TraceConnectorEdges = {
  incoming: boolean
  outgoing: boolean
}

export function traceConnectorEdges(
  index: number,
  total: number,
  awaitingNext: boolean
): TraceConnectorEdges {
  return {
    incoming: index > 0,
    outgoing: index < total - 1 || awaitingNext
  }
}

export type LatencyTone = 'good' | 'fair' | 'slow' | 'muted'

export function latencyTone(value?: number): LatencyTone {
  if (typeof value !== 'number' || !Number.isFinite(value)) return 'muted'
  if (value <= 80) return 'good'
  if (value <= 180) return 'fair'
  return 'slow'
}

export function pingLossPresentation(sent: number, lost: number) {
  if (sent === 0) return { tone: 'muted', detail: '等待首个结果' } as const
  if (lost > 0) return { tone: 'warning', detail: `${lost} 个请求未响应` } as const
  return { tone: 'good', detail: '连接稳定' } as const
}

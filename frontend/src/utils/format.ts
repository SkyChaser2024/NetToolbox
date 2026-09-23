export function safeHostname(value: string) {
  try {
    return new URL(value).hostname
  } catch {
    return value
  }
}

export function stripPort(value: string) {
  return value.replace(/:\d+$/, '')
}

export function formatBytes(value: number) {
  return value >= 1024 ? `${(value / 1024).toFixed(0)} KiB` : `${value} B`
}

export function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error)
}

export function createSessionID(prefix: string) {
  return typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `${prefix}-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

export function formatTraceLatency(value?: number) {
  if (typeof value !== 'number') return '—'
  if (value < 1) return '<1 ms'
  return `${value < 10 ? value.toFixed(1) : Math.round(value)} ms`
}

export function formatTraceDuration(value: number) {
  return value < 1000 ? `${value} ms` : `${(value / 1000).toFixed(value < 10000 ? 1 : 0)} 秒`
}

export function formatPercentage(value: number) {
  return `${value < 10 && value > 0 ? value.toFixed(1) : Math.round(value)}%`
}

export function splitLines(value: string) {
  return value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean)
}

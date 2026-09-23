import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, defaultDiagnostics } from '../../api'
import type { BootstrapData, LatencyTarget, LatencyProbe, OverviewResult } from '../../api'
import { safeHostname, errorMessage } from '../../utils/format'
import { startLatencyPolling } from './latencyPolling'

const latencySampleCount = 16
const pendingLatencyProbe = (target: LatencyTarget): LatencyProbe => ({
  ...target,
  host: safeHostname(target.url),
  status: 'pending'
})
const pendingLatencyProbes: LatencyProbe[] =
  defaultDiagnostics.latencyTargets.map(pendingLatencyProbe)
export function useOverview(
  bootstrap: BootstrapData | null,
  loaded: boolean,
  active: boolean,
  onError: (message: string) => void
) {
  const [overviewResult, setOverviewResult] = useState<OverviewResult | null>(null)
  const [overviewRunning, setOverviewRunning] = useState(true)
  const [publicRunning, setPublicRunning] = useState<Array<'ipv4' | 'ipv6'>>([])
  const [latencyPolling, setLatencyPolling] = useState(false)
  const latencySamples = useRef<Record<string, number[]>>({})
  const latencySampleKey = useRef('')
  const configuredLatencyTargets = bootstrap?.diagnostics.latencyTargets
  const latencyTargetKey = useMemo(
    () =>
      configuredLatencyTargets
        ?.map(
          (target) => `${target.id}\u0000${target.url}\u0000${target.name}\u0000${target.region}`
        )
        .join('\u0001') || '',
    [configuredLatencyTargets]
  )
  const configuredPendingProbes = useMemo(
    () => configuredLatencyTargets?.map(pendingLatencyProbe) || pendingLatencyProbes,
    [configuredLatencyTargets]
  )
  const pollingTargetsRef = useRef<LatencyTarget[]>([])
  useEffect(() => {
    pollingTargetsRef.current = (configuredLatencyTargets || []).slice(0, 16)
  }, [configuredLatencyTargets])
  const refreshOverview = useCallback(async () => {
    setOverviewRunning(true)
    try {
      const refreshed = await api().CheckOverview()
      setOverviewResult((current) => ({
        ...refreshed,
        probes: current?.probes?.length ? current.probes : refreshed.probes
      }))
    } catch (error) {
      onError(errorMessage(error))
    } finally {
      setOverviewRunning(false)
    }
  }, [onError])

  const refreshPublicNetwork = async (version: 'ipv4' | 'ipv6') => {
    if (overviewRunning || publicRunning.includes(version)) return
    setPublicRunning((current) => [...current, version])
    try {
      const info =
        version === 'ipv4' ? await api().CheckPublicIPv4() : await api().CheckPublicIPv6()
      setOverviewResult((current) =>
        current
          ? {
              ...current,
              [version]: info,
              checkedAt: new Date().toLocaleTimeString('zh-CN', { hour12: false })
            }
          : current
      )
    } catch (error) {
      onError(errorMessage(error))
    } finally {
      setPublicRunning((current) => current.filter((item) => item !== version))
    }
  }
  const initialized = useRef(false)
  useEffect(() => {
    if (initialized.current || !loaded) return
    initialized.current = true
    const probes =
      bootstrap?.diagnostics.latencyTargets.map(pendingLatencyProbe) || pendingLatencyProbes
    if (bootstrap?.cachedOverview) {
      setOverviewResult({ ...bootstrap.cachedOverview, probes })
      setOverviewRunning(false)
    } else {
      setOverviewResult({
        ipv4: { available: false },
        ipv6: { available: false },
        probes,
        checkedAt: ''
      })
      void refreshOverview()
    }
  }, [bootstrap, loaded, refreshOverview])

  useEffect(() => {
    if (!latencyTargetKey || !active) return
    // Settings saves replace the bootstrap object. Only restart polling when
    // target content changes; take its current snapshot at that boundary.
    const targets = pollingTargetsRef.current
    if (latencySampleKey.current !== latencyTargetKey) {
      latencySampleKey.current = latencyTargetKey
      latencySamples.current = Object.fromEntries(targets.map((target) => [target.id, []]))
      setOverviewResult((current) =>
        current ? { ...current, probes: targets.map(pendingLatencyProbe) } : current
      )
    }
    let stopPolling: (() => void) | undefined
    const pause = () => {
      stopPolling?.()
      stopPolling = undefined
      setLatencyPolling(false)
      void api().CancelLatencyChecks()
    }
    const recordProbe = (target: LatencyTarget, probe: LatencyProbe) => {
      const samples = latencySamples.current[target.id] || []
      const sample =
        probe.status === 'ok' &&
        typeof probe.latencyMs === 'number' &&
        Number.isFinite(probe.latencyMs)
          ? probe.latencyMs
          : -1
      samples.push(sample)
      if (samples.length > latencySampleCount) samples.shift()
      latencySamples.current[target.id] = samples

      let validCount = 0
      let total = 0
      let minimum = Number.POSITIVE_INFINITY
      let maximum = Number.NEGATIVE_INFINITY
      for (const value of samples) {
        if (value < 0) continue
        validCount += 1
        total += value
        minimum = Math.min(minimum, value)
        maximum = Math.max(maximum, value)
      }
      if (validCount > 5) {
        validCount -= 2
        total -= minimum + maximum
      }
      const latencyMs = validCount ? Math.round(total / validCount) : undefined
      const displayed: LatencyProbe =
        latencyMs === undefined ? probe : { ...probe, status: 'ok', latencyMs, error: undefined }

      setOverviewResult((current) => {
        if (!current) return current
        const probesAligned =
          current.probes.length === targets.length &&
          targets.every((item, index) => current.probes[index]?.id === item.id)
        if (!probesAligned) {
          const existing = new Map(current.probes.map((item) => [item.id, item]))
          return {
            ...current,
            probes: targets.map((item) =>
              item.id === target.id ? displayed : existing.get(item.id) || pendingLatencyProbe(item)
            )
          }
        }
        const probeIndex = current.probes.findIndex((item) => item.id === target.id)
        if (probeIndex < 0) return current
        const probes = current.probes.slice()
        probes[probeIndex] = displayed
        return {
          ...current,
          probes
        }
      })
    }
    const start = () => {
      if (document.hidden || stopPolling) return
      setLatencyPolling(true)
      stopPolling = startLatencyPolling({
        targets,
        probe: (target) => api().CheckLatency(target.id),
        onResult: recordProbe,
        onError: (target) =>
          recordProbe(target, {
            ...pendingLatencyProbe(target),
            status: 'failed',
            error: '连接测试失败'
          })
      })
    }
    const handleVisibility = () => {
      if (document.hidden) pause()
      else start()
    }

    document.addEventListener('visibilitychange', handleVisibility)
    start()
    return () => {
      document.removeEventListener('visibilitychange', handleVisibility)
      pause()
    }
  }, [latencyTargetKey, active])
  return {
    overviewResult,
    overviewRunning,
    publicRunning,
    latencyPolling,
    configuredPendingProbes,
    refreshOverview,
    refreshPublicNetwork
  }
}

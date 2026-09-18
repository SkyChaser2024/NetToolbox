import { useCallback, useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { CSSProperties, FormEvent, ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { siBilibili, siGithub, siTaobao, siTelegram, siTiktok, siWechat, siX, siYoutube } from 'simple-icons'
import { EventsOn, WindowSetDarkTheme, WindowSetLightTheme, WindowSetSystemDefaultTheme } from '../wailsjs/runtime/runtime'
import { api, appVersion, defaultDiagnostics, defaultProfile } from './api'
import type {
  Adapter,
  AuthEvent,
  AuthRequest,
  AuthState,
  BootstrapData,
  DiagnosticSettings,
  IPv6Result,
  LatencyTarget,
  LatencyProbe,
  NATResult,
  NetworkInterface,
  OverviewResult,
  PingEvent,
  PingReply,
  PingSummary,
  Profile,
  PublicNetworkInfo,
  SettingsRequest,
  TraceEvent,
  TraceHop,
  TraceProtocol,
} from './api'
import './App.css'

type Page = 'home' | 'auth' | 'nat' | 'ipv6' | 'trace' | 'settings'
type Theme = 'system' | 'light' | 'dark'
type SettingsSaver = (update: (current: BootstrapData) => SettingsRequest) => Promise<void>

const pageMeta: Record<Page, { title: string }> = {
  home: { title: '网络概览' },
  auth: { title: '锐捷认证' },
  nat: { title: 'NAT 类型检测' },
  ipv6: { title: 'IPv6 连接测试' },
  trace: { title: 'Ping 与路由追踪' },
  settings: { title: '设置' },
}

const serviceIcons = { douyin: siTiktok, bilibili: siBilibili, wechat: siWechat, taobao: siTaobao, github: siGithub, telegram: siTelegram, x: siX, youtube: siYoutube }
const toolIcons = { home: 'activity', nat: 'network', ipv6: 'globe', ping: 'activity', trace: 'route' } as const

const latencySampleCount = 16
const latencyPollIntervalMs = 618 * 3
const pendingLatencyProbe = (target: LatencyTarget): LatencyProbe => ({...target, host: safeHostname(target.url), status: 'pending'})
const pendingLatencyProbes: LatencyProbe[] = defaultDiagnostics.latencyTargets.map(pendingLatencyProbe)

type TracePhase = 'idle' | 'starting' | 'resolving' | 'running' | 'stopping' | 'completed' | 'cancelled' | 'error'
type PingPhase = TracePhase

interface PingViewState {
  target: string
  submittedTarget: string
  protocol: TraceProtocol
  count: number
  timeoutMs: number
  intervalMs: number
  advancedOpen: boolean
  sessionId: string
  phase: PingPhase
  resolvedTarget: string
  actualProtocol: 'ipv4' | 'ipv6' | ''
  replies: PingReply[]
  summary: PingSummary | null
  error: string
}

interface TraceViewState {
  target: string
  submittedTarget: string
  protocol: TraceProtocol
  maxHops: number
  timeoutMs: number
  resolveHostnames: boolean
  advancedOpen: boolean
  sessionId: string
  phase: TracePhase
  resolvedTarget: string
  actualProtocol: 'ipv4' | 'ipv6' | ''
  hops: TraceHop[]
  durationMs: number
  resultStatus: TraceEvent['status'] | ''
  error: string
}

const initialTraceState: TraceViewState = {
  target: '', protocol: 'auto', maxHops: 30, timeoutMs: 1000, resolveHostnames: true, advancedOpen: false,
  submittedTarget: '', sessionId: '', phase: 'idle', resolvedTarget: '', actualProtocol: '', hops: [], durationMs: 0, resultStatus: '', error: '',
}

const initialPingState: PingViewState = {
  target: '', protocol: 'auto', count: 4, timeoutMs: 1000, intervalMs: 1000, advancedOpen: false,
  submittedTarget: '', sessionId: '', phase: 'idle', resolvedTarget: '', actualProtocol: '', replies: [], summary: null, error: '',
}

function Icon({ name, size = 20 }: { name: string; size?: number }) {
  const common = { width: size, height: size, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 1.8, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const, 'aria-hidden': true }
  switch (name) {
    case 'home': return <svg {...common}><path d="m3 11 9-8 9 8"/><path d="M5 10v10h14V10M9 20v-6h6v6"/></svg>
    case 'shield': return <svg {...common}><path d="M12 3 5 6v5c0 4.7 2.8 8.1 7 10 4.2-1.9 7-5.3 7-10V6l-7-3Z"/><path d="m9 12 2 2 4-4"/></svg>
    case 'network': return <svg {...common}><circle cx="6" cy="6" r="2"/><circle cx="18" cy="6" r="2"/><circle cx="12" cy="18" r="2"/><path d="m7.7 7.1 3.1 8.8M16.3 7.1l-3.1 8.8M8 6h8"/></svg>
    case 'globe': return <svg {...common}><circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18"/></svg>
    case 'info': return <svg {...common}><circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/></svg>
    case 'refresh': return <svg {...common}><path d="M20 7v5h-5M4 17v-5h5"/><path d="M18.5 10a7 7 0 0 0-12-2.5L4 10m16 4-2.5 2.5A7 7 0 0 1 5.5 14"/></svg>
    case 'plug': return <svg {...common}><path d="M9 3v5M15 3v5M7 8h10v2a5 5 0 0 1-5 5 5 5 0 0 1-5-5V8ZM12 15v6"/></svg>
    case 'moon': return <svg {...common}><path d="M20 15.5A8 8 0 0 1 8.5 4 8.5 8.5 0 1 0 20 15.5Z"/></svg>
    case 'chevron': return <svg {...common}><path d="m9 6 6 6-6 6"/></svg>
    case 'lock': return <svg {...common}><rect x="5" y="10" width="14" height="10" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/></svg>
    case 'activity': return <svg {...common}><path d="M3 12h4l2-7 4 14 2-7h6"/></svg>
    case 'route': return <svg {...common}><circle cx="6" cy="5" r="2"/><circle cx="18" cy="19" r="2"/><path d="M8 5h3a3 3 0 0 1 3 3v0a3 3 0 0 1-3 3H9a3 3 0 0 0-3 3v0a3 3 0 0 0 3 3h7"/></svg>
    case 'external': return <svg {...common}><path d="M14 5h5v5M19 5l-8 8"/><path d="M18 13v5a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5"/></svg>
    case 'settings': return <svg {...common}><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.8 2.8-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.6v.2h-4V21a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1L4.2 17l.1-.1a1.7 1.7 0 0 0 .3-1.9A1.7 1.7 0 0 0 3 14H2.8v-4H3a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9L4.2 7 7 4.2l.1.1a1.7 1.7 0 0 0 1.9.3A1.7 1.7 0 0 0 10 3V2.8h4V3a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1L19.8 7l-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.6 1h.2v4H21a1.7 1.7 0 0 0-1.6 1Z"/></svg>
    default: return <svg {...common}><circle cx="12" cy="12" r="8"/></svg>
  }
}

function App() {
  const [page, setPage] = useState<Page>('home')
  const [theme, setTheme] = useState<Theme>(loadTheme)
  const [bootstrap, setBootstrap] = useState<BootstrapData | null>(null)
  const [authState, setAuthState] = useState<AuthState>('idle')
  const [logs, setLogs] = useState<AuthEvent[]>([])
  const [overviewResult, setOverviewResult] = useState<OverviewResult | null>(null)
  const [overviewRunning, setOverviewRunning] = useState(true)
  const [publicRunning, setPublicRunning] = useState<Array<'ipv4' | 'ipv6'>>([])
  const [latencyPolling, setLatencyPolling] = useState(false)
  const [natResult, setNatResult] = useState<NATResult | null>(null)
  const [natRunning, setNatRunning] = useState(false)
  const [ipv6Result, setIPv6Result] = useState<IPv6Result | null>(null)
  const [ipv6Running, setIPv6Running] = useState(false)
  const [ping, setPing] = useState<PingViewState>(initialPingState)
  const [trace, setTrace] = useState<TraceViewState>(initialTraceState)
  const [toast, setToast] = useState('')
  const latencySamples = useRef<Record<string, number[]>>({})
  const latencySampleKey = useRef('')
  const bootstrapRef = useRef<BootstrapData | null>(null)
  const settingsQueue = useRef<Promise<void>>(Promise.resolve())
  const configuredLatencyTargets = bootstrap?.diagnostics.latencyTargets
  const latencyTargetKey = useMemo(() => configuredLatencyTargets?.map(target => `${target.id}\u0000${target.url}\u0000${target.name}\u0000${target.region}`).join('\u0001') || '', [configuredLatencyTargets])
  const configuredPendingProbes = useMemo(() => configuredLatencyTargets?.map(pendingLatencyProbe) || pendingLatencyProbes, [configuredLatencyTargets])

  bootstrapRef.current = bootstrap

  const saveSettings = useCallback((update: (current: BootstrapData) => SettingsRequest) => {
    const persist = async () => {
      const current = bootstrapRef.current
      if (!current) return
      const stored = await api().SaveSettings(update(current))
      setBootstrap(previous => previous ? {...previous, ...stored} : previous)
    }
    const operation = settingsQueue.current.then(persist, persist)
    settingsQueue.current = operation.catch(() => undefined)
    return operation
  }, [])

  const refreshOverview = async () => {
    setOverviewRunning(true)
    try {
      const refreshed = await api().CheckOverview()
      setOverviewResult(current => ({...refreshed, probes: current?.probes?.length ? current.probes : refreshed.probes}))
    }
    catch (error) { setToast(errorMessage(error)) }
    finally { setOverviewRunning(false) }
  }

  const refreshPublicNetwork = async (version: 'ipv4' | 'ipv6') => {
    if (overviewRunning || publicRunning.includes(version)) return
    setPublicRunning(current => [...current, version])
    try {
      const info = version === 'ipv4' ? await api().CheckPublicIPv4() : await api().CheckPublicIPv6()
      setOverviewResult(current => current ? {...current, [version]: info, checkedAt: new Date().toLocaleTimeString('zh-CN', {hour12: false})} : current)
    } catch (error) { setToast(errorMessage(error)) }
    finally { setPublicRunning(current => current.filter(item => item !== version)) }
  }

  useEffect(() => {
    api().Bootstrap().then(data => {
      setBootstrap(data)
      setAuthState(data.authState)
      if (data.configurationError) setToast(data.configurationError)
      const pending = data.diagnostics.latencyTargets.map(pendingLatencyProbe)
      if (data.cachedOverview) {
        setOverviewResult({...data.cachedOverview, probes: pending})
        setOverviewRunning(false)
      } else {
        setOverviewResult({ipv4: {available: false}, ipv6: {available: false}, probes: pending, checkedAt: ''})
        void refreshOverview()
      }
    }).catch(error => { setToast(errorMessage(error)); void refreshOverview() })
  }, [])

  useEffect(() => {
    if (!bootstrap || !latencyTargetKey || page !== 'home') {
      setLatencyPolling(false)
      void api().CancelLatencyChecks()
      return
    }
    const targets = (configuredLatencyTargets || []).slice(0, 16)
    if (latencySampleKey.current !== latencyTargetKey) {
      latencySampleKey.current = latencyTargetKey
      latencySamples.current = Object.fromEntries(targets.map(target => [target.id, []]))
    }
    let session = 0
    let intervals: number[] = []

    const clearIntervals = () => {
      intervals.forEach(timer => window.clearInterval(timer))
      intervals = []
    }

    const pause = () => {
      session += 1
      clearIntervals()
      setLatencyPolling(false)
      void api().CancelLatencyChecks()
    }

    const recordProbe = (target: LatencyTarget, probe: LatencyProbe) => {
      const samples = latencySamples.current[target.id] || []
      const sample = probe.status === 'ok' && typeof probe.latencyMs === 'number' && Number.isFinite(probe.latencyMs) ? probe.latencyMs : -1
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
      const displayed: LatencyProbe = latencyMs === undefined ? probe : {...probe, status: 'ok', latencyMs, error: undefined}

      setOverviewResult(current => {
        if (!current) return current
        const probesAligned = current.probes.length === targets.length && targets.every((item, index) => current.probes[index]?.id === item.id)
        if (!probesAligned) {
          const existing = new Map(current.probes.map(item => [item.id, item]))
          return {
            ...current,
            probes: targets.map(item => item.id === target.id ? displayed : existing.get(item.id) || pendingLatencyProbe(item)),
          }
        }
        const probeIndex = current.probes.findIndex(item => item.id === target.id)
        if (probeIndex < 0) return current
        const probes = current.probes.slice()
        probes[probeIndex] = displayed
        return {
          ...current,
          probes,
        }
      })
    }

    const pollSite = async (target: LatencyTarget, activeSession: number) => {
      try {
        const probe = await api().CheckLatency(target.id)
        if (session !== activeSession || document.hidden) return
        recordProbe(target, probe)
      } catch {
        if (session !== activeSession || document.hidden) return
        recordProbe(target, {...pendingLatencyProbe(target), status: 'failed', error: '连接测试失败'})
      }
    }

    const runSite = async (target: LatencyTarget, activeSession: number) => {
      while (session === activeSession && !document.hidden && (latencySamples.current[target.id]?.length || 0) < latencySampleCount) {
        await pollSite(target, activeSession)
        await Promise.resolve()
      }
      if (session !== activeSession || document.hidden) return

      let inProgress = false
      const timer = window.setInterval(async () => {
        if (session !== activeSession || document.hidden || inProgress) return
        inProgress = true
        try { await pollSite(target, activeSession) }
        finally { inProgress = false }
      }, latencyPollIntervalMs)
      intervals.push(timer)
    }

    const start = () => {
      if (document.hidden) return
      clearIntervals()
      const activeSession = ++session
      setLatencyPolling(true)
      targets.forEach(target => { void runSite(target, activeSession) })
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
  }, [latencyTargetKey, page])

  useEffect(() => {
    document.documentElement.dataset.theme = theme
    localStorage.setItem('theme', theme)
    if (!window.go?.main?.App) return
    const frame = window.requestAnimationFrame(() => {
      if (theme === 'dark') WindowSetDarkTheme()
      else if (theme === 'light') WindowSetLightTheme()
      else WindowSetSystemDefaultTheme()
    })
    return () => window.cancelAnimationFrame(frame)
  }, [theme])

  useEffect(() => {
    if (!window.go?.main?.App) return
    return EventsOn('auth:event', (event: AuthEvent) => {
      setAuthState(event.state)
      setLogs(current => [...current.slice(-79), event])
    })
  }, [])

  useEffect(() => {
    if (!window.go?.main?.App) return
    return EventsOn('ping:event', (event: PingEvent) => {
      setPing(current => {
        if (!current.sessionId || event.sessionId !== current.sessionId) return current
        if (current.phase === 'cancelled' && event.type !== 'cancelled') return current
        if (current.phase === 'stopping' && (event.type === 'started' || event.type === 'reply')) return current
        if (event.type === 'started') {
          return {...current, phase: 'running', submittedTarget: event.target || current.submittedTarget, resolvedTarget: event.address || '', actualProtocol: event.protocol || '', error: ''}
        }
        if (event.type === 'reply' && event.reply) {
          const replies = current.replies.filter(reply => reply.sequence !== event.reply?.sequence)
          replies.push(event.reply)
          replies.sort((left, right) => left.sequence - right.sequence)
          return {...current, phase: 'running', replies}
        }
        if (event.type === 'completed') {
          return {...current, phase: 'completed', resolvedTarget: event.summary?.address || current.resolvedTarget, actualProtocol: event.summary?.protocol || current.actualProtocol, summary: event.summary || current.summary, error: ''}
        }
        if (event.type === 'cancelled') {
          return {...current, phase: 'cancelled', resolvedTarget: event.summary?.address || current.resolvedTarget, actualProtocol: event.summary?.protocol || current.actualProtocol, summary: event.summary || current.summary}
        }
        if (event.type === 'error') {
          return {...current, phase: 'error', resolvedTarget: event.summary?.address || current.resolvedTarget, actualProtocol: event.summary?.protocol || current.actualProtocol, summary: event.summary || current.summary, error: event.error || 'Ping 测试失败'}
        }
        return current
      })
    })
  }, [])

  useEffect(() => {
    if (!window.go?.main?.App) return
    return EventsOn('traceroute:event', (event: TraceEvent) => {
      setTrace(current => {
        if (!current.sessionId || event.sessionId !== current.sessionId) return current
        if (current.phase === 'cancelled' && event.type !== 'cancelled') return current
        if (current.phase === 'stopping' && (event.type === 'started' || event.type === 'hop')) return current
        if (event.type === 'started') {
          return {...current, phase: 'running', submittedTarget: event.target || current.submittedTarget, resolvedTarget: event.address || '', actualProtocol: event.protocol || '', error: ''}
        }
        if (event.type === 'hop' && event.hop) {
          const hops = current.hops.filter(hop => hop.number !== event.hop?.number)
          hops.push(event.hop)
          hops.sort((left, right) => left.number - right.number)
          return {...current, phase: 'running', hops}
        }
        if (event.type === 'completed') {
          return {...current, phase: 'completed', resolvedTarget: event.address || current.resolvedTarget, actualProtocol: event.protocol || current.actualProtocol, durationMs: event.durationMs || 0, resultStatus: event.status || '', error: ''}
        }
        if (event.type === 'cancelled') {
          return {...current, phase: 'cancelled', durationMs: event.durationMs || current.durationMs, resultStatus: 'cancelled'}
        }
        if (event.type === 'error') {
          return {...current, phase: 'error', durationMs: event.durationMs || current.durationMs, resultStatus: 'error', error: event.error || '路由追踪失败'}
        }
        return current
      })
    })
  }, [])

  useEffect(() => {
    if (page === 'trace' || (ping.phase !== 'resolving' && ping.phase !== 'running') || !ping.sessionId) return
    const sessionId = ping.sessionId
    setPing(current => current.sessionId === sessionId ? {...current, phase: 'cancelled'} : current)
    void api().CancelPing(sessionId)
  }, [page, ping.phase, ping.sessionId])

  useEffect(() => {
    if (page === 'trace' || (trace.phase !== 'resolving' && trace.phase !== 'running') || !trace.sessionId) return
    const sessionId = trace.sessionId
    setTrace(current => current.sessionId === sessionId ? {...current, phase: 'cancelled', resultStatus: 'cancelled'} : current)
    void api().CancelTraceroute(sessionId)
  }, [page, trace.phase, trace.sessionId])

  useEffect(() => {
    if (!toast) return
    const timer = window.setTimeout(() => setToast(''), 5000)
    return () => window.clearTimeout(timer)
  }, [toast])

  const updatePing = (values: Partial<PingViewState>) => setPing(current => ({...current, ...values}))

  const startPing = async () => {
    const target = ping.target.trim() || 'baidu.com'
    const sessionId = createSessionID('ping')
    const request = {sessionId, target, protocol: ping.protocol, count: ping.count, timeoutMs: ping.timeoutMs, intervalMs: ping.intervalMs}
    setPing(current => ({...current, target, submittedTarget: target, sessionId, phase: 'starting', resolvedTarget: '', actualProtocol: '', replies: [], summary: null, error: ''}))
    try {
      await api().StartPing(request)
      setPing(current => current.sessionId === sessionId && current.phase === 'starting' ? {...current, phase: 'resolving'} : current)
    } catch (error) {
      setPing(current => current.sessionId === sessionId ? {...current, sessionId: '', phase: 'idle', error: errorMessage(error)} : current)
    }
  }

  const stopPing = async () => {
    const sessionId = ping.sessionId
    if (!sessionId) return
    setPing(current => current.sessionId === sessionId ? {...current, phase: 'stopping'} : current)
    try { await api().CancelPing(sessionId) }
    catch (error) { setPing(current => current.sessionId === sessionId ? {...current, phase: 'error', error: errorMessage(error)} : current) }
  }

  const updateTrace = (values: Partial<TraceViewState>) => setTrace(current => ({...current, ...values}))

  const startTraceroute = async () => {
    const target = trace.target.trim() || 'baidu.com'
    const sessionId = createSessionID('trace')
    const request = {sessionId, target, protocol: trace.protocol, maxHops: trace.maxHops, timeoutMs: trace.timeoutMs, resolveHostnames: trace.resolveHostnames}
    setTrace(current => ({...current, target, submittedTarget: target, sessionId, phase: 'starting', resolvedTarget: '', actualProtocol: '', hops: [], durationMs: 0, resultStatus: '', error: ''}))
    try {
      await api().StartTraceroute(request)
      setTrace(current => current.sessionId === sessionId && current.phase === 'starting' ? {...current, phase: 'resolving'} : current)
    } catch (error) {
      setTrace(current => current.sessionId === sessionId ? {...current, sessionId: '', phase: 'idle', resultStatus: '', error: errorMessage(error)} : current)
    }
  }

  const stopTraceroute = async () => {
    const sessionId = trace.sessionId
    if (!sessionId) return
    setTrace(current => current.sessionId === sessionId ? {...current, phase: 'stopping'} : current)
    try { await api().CancelTraceroute(sessionId) }
    catch (error) { setTrace(current => current.sessionId === sessionId ? {...current, phase: 'error', resultStatus: 'error', error: errorMessage(error)} : current) }
  }

  const cycleTheme = () => setTheme(current => current === 'dark' ? 'light' : current === 'light' ? 'dark' : window.matchMedia('(prefers-color-scheme: dark)').matches ? 'light' : 'dark')
  const meta = pageMeta[page]

  return <div className="app-shell">
    <aside className="sidebar">
      <div className="brand"><div className="brand-mark"><Icon name="activity" size={24}/></div><div><strong>网络工具箱</strong><span>Network Toolbox</span></div></div>
      <nav className="nav-list" aria-label="主要功能">
        <NavItem active={page === 'home'} icon="home" label="网络概览" onClick={() => setPage('home')}/>
        <NavItem active={page === 'auth'} icon="shield" label="锐捷认证" onClick={() => setPage('auth')}/>
        <NavItem active={page === 'nat'} icon="network" label="NAT 检测" onClick={() => setPage('nat')}/>
        <NavItem active={page === 'ipv6'} icon="globe" label="IPv6 测试" onClick={() => setPage('ipv6')}/>
        <NavItem active={page === 'trace'} icon="route" label="Ping / 路由" onClick={() => setPage('trace')}/>
        <NavItem active={page === 'settings'} icon="settings" label="设置" onClick={() => setPage('settings')}/>
      </nav>
      <div className="sidebar-spacer"/>
      <div className="version-note" title={`网络工具箱 ${bootstrap?.version || appVersion}`}><span className="version-note-mark"><Icon name="activity" size={13}/></span><span className="version-note-copy"><small>VERSION</small><strong>v{(bootstrap?.version || appVersion).replace(/^v/i, '')}</strong></span></div>
    </aside>
    <main className="main-area">
      <header className="topbar"><h1>{meta.title}</h1><div className="top-actions">{page === 'auth' && <StatusPill state={authState}/>}<button className="icon-button" onClick={cycleTheme} title={`主题：${theme}`} aria-label="切换界面主题"><Icon name="moon"/></button></div></header>
      <div className="content-scroll">{page === 'home' ? <HomePage result={overviewResult} interfaces={bootstrap?.networkInterfaces || []} pendingProbes={configuredPendingProbes} running={overviewRunning} publicRunning={publicRunning} latencyPolling={latencyPolling} diagnostics={bootstrap?.diagnostics} onRefresh={refreshOverview} onRefreshPublic={refreshPublicNetwork} onSaveSettings={saveSettings} onError={setToast}/> : !bootstrap ? <LoadingCard text="正在读取网络环境…"/> : page === 'auth' ? <AuthPage data={bootstrap} state={authState} logs={logs} onAdapters={adapters => setBootstrap(current => current ? {...current, adapters} : current)} onProfile={profile => setBootstrap(current => current ? {...current, profile} : current)} onError={setToast}/> : page === 'nat' ? <NATPage data={bootstrap} result={natResult} setResult={setNatResult} running={natRunning} setRunning={setNatRunning} onSaveSettings={saveSettings} onError={setToast}/> : page === 'ipv6' ? <IPv6Page data={bootstrap} result={ipv6Result} setResult={setIPv6Result} running={ipv6Running} setRunning={setIPv6Running} onSaveSettings={saveSettings} onError={setToast}/> : page === 'trace' ? <NetworkPathPage ping={ping} trace={trace} onPingChange={updatePing} onPingStart={startPing} onPingStop={stopPing} onTraceChange={updateTrace} onTraceStart={startTraceroute} onTraceStop={stopTraceroute}/> : <SettingsPage value={bootstrap} onSaveSettings={saveSettings} onError={setToast}/>}</div>
    </main>
    {toast && <div className="toast" role="alert"><span>{toast}</span><button onClick={() => setToast('')} aria-label="关闭提示">×</button></div>}
  </div>
}

function NavItem({ active, icon, label, onClick }: { active: boolean; icon: string; label: string; onClick: () => void }) {
  return <button className={`nav-item ${active ? 'active' : ''}`} aria-current={active ? 'page' : undefined} onClick={onClick}><Icon name={icon}/><span>{label}</span>{active && <i/>}</button>
}

function StatusPill({ state }: { state: AuthState }) {
  const labels: Record<AuthState, string> = { idle: '未认证', starting: '正在启动', waiting_identity: '等待身份请求', waiting_challenge: '正在校验', authenticated: '认证成功', failed: '认证失败', stopping: '正在断开', error: '发生错误' }
  return <div className={`status-pill ${state}`}><span className="status-dot"/>{labels[state]}</div>
}

function HomePage({ result, interfaces, pendingProbes, running, publicRunning, latencyPolling, diagnostics, onRefresh, onRefreshPublic, onSaveSettings, onError }: { result: OverviewResult | null; interfaces: NetworkInterface[]; pendingProbes: LatencyProbe[]; running: boolean; publicRunning: Array<'ipv4' | 'ipv6'>; latencyPolling: boolean; diagnostics?: DiagnosticSettings; onRefresh: () => void; onRefreshPublic: (version: 'ipv4' | 'ipv6') => void; onSaveSettings: SettingsSaver; onError: (message: string) => void }) {
  const probes = result?.probes?.length ? result.probes : pendingProbes
  const domestic = probes.filter(item => item.region === '国内')
  const international = probes.filter(item => item.region === '国际')
  const online = Boolean(result?.ipv4.available || result?.ipv6.available || result?.probes?.some(item => item.status === 'ok'))
  return <div className="home-page">
    <section className="card overview-hero">
      <div className={`overview-orb ${online ? 'online' : ''}`}><Icon name={toolIcons.home} size={28}/></div>
      <div><span className="eyebrow">NETWORK OVERVIEW</span><h2>{running && !result ? '正在识别网络…' : online ? '网络连接正常' : '尚未获取公网信息'}</h2></div>
      <div className="overview-meta"><span><i className={online ? 'online' : ''}/>{online ? '公网可达' : '等待检测'}</span>{result?.checkedAt && <small>更新于 {result.checkedAt}</small>}</div>
      <button className="button primary" onClick={onRefresh} disabled={running}>{running ? <><span className="spinner"/>刷新中…</> : <><Icon name={toolIcons.home} size={17}/>刷新信息</>}</button>
    </section>

    <div className="public-network-grid"><PublicNetworkCard version="IPv4" info={result?.ipv4} loading={running && !result} refreshing={running || publicRunning.includes('ipv4')} endpoints={diagnostics?.ipv4Endpoints} onRefresh={() => onRefreshPublic('ipv4')} onSaveSettings={onSaveSettings} onError={onError}/><PublicNetworkCard version="IPv6" info={result?.ipv6} loading={running && !result} refreshing={running || publicRunning.includes('ipv6')} endpoints={diagnostics?.ipv6Endpoints} onRefresh={() => onRefreshPublic('ipv6')} onSaveSettings={onSaveSettings} onError={onError}/></div>

    <section className="card latency-section">
      <div className="section-title"><h3>网站延迟</h3></div>
      <LatencyGroup probes={domestic} polling={latencyPolling}/>
      <LatencyGroup probes={international} polling={latencyPolling}/>
      {diagnostics && <LatencySettings value={diagnostics.latencyTargets} onSaveSettings={onSaveSettings} onError={onError}/>}
    </section>

    <NetworkInterfaces items={interfaces}/>
  </div>
}

function PublicNetworkCard({ version, info, loading, refreshing, endpoints, onRefresh, onSaveSettings, onError }: { version: 'IPv4' | 'IPv6'; info?: PublicNetworkInfo; loading: boolean; refreshing: boolean; endpoints?: string[]; onRefresh: () => void; onSaveSettings: SettingsSaver; onError: (message: string) => void }) {
  const location = [info?.country, info?.region, info?.city].filter(Boolean).join(' · ')
  const source = sourceHostname(info?.source)
  return <section className={`card public-network-card ${info?.available ? 'available' : ''}`}>
    <div className="public-card-head"><span className={`protocol-mark ${version.toLowerCase()}`}>{version}</span><div className="public-card-actions"><span className={`availability ${info?.available ? 'yes' : 'no'}`}>{loading ? '检测中' : info?.available ? '已连接' : '未获取'}</span><button type="button" className="public-refresh-button" onClick={onRefresh} disabled={refreshing} title={`刷新 ${version} 信息`} aria-label={`刷新 ${version} 信息`}>{refreshing ? <span className="spinner dark"/> : <Icon name="refresh" size={15}/>}</button></div></div>
    <strong className="network-address" title={info?.address}><NetworkAddress value={loading ? '' : info?.address} fallback={loading ? '正在获取…' : '未获取到公网地址'}/></strong>
    <div className="public-facts"><div><span>ISP</span><b>{info?.isp || '—'}</b></div><div><span>ASN</span><b>{info?.asn ? `AS${info.asn}` : '—'}</b></div><div><span>网络</span><b>{info?.asnOrganization || '—'}</b></div><div><span>位置</span><b>{location || '—'}</b></div></div>
    {!loading && info?.error && <small className={`public-error ${info.available ? 'partial' : ''}`} title={info.error}>{info.error}</small>}
    {!loading && source && <div className="public-source" title={info?.source}><Icon name="globe" size={13}/><span>数据来源</span><b>{source}</b></div>}
    {endpoints && <PublicEndpointSettings version={version} value={endpoints} onSaveSettings={onSaveSettings} onError={onError}/>}
  </section>
}

function sourceHostname(value?: string) {
  if (!value) return ''
  try { return new URL(value).hostname.replace(/^www\./i, '') }
  catch { return value }
}

function LatencyGroup({ probes, polling }: { probes: LatencyProbe[]; polling: boolean }) {
  return <div className="latency-group"><div className="latency-grid">{probes.map(probe => <LatencyCard key={probe.id} probe={probe} running={polling && probe.status === 'pending'}/>)}</div></div>
}

function LatencyCard({ probe, running }: { probe: LatencyProbe; running: boolean }) {
  const latencyClass = probe.status === 'pending' ? 'pending' : probe.status !== 'ok' ? 'failed' : (probe.latencyMs || 0) < 100 ? 'fast' : (probe.latencyMs || 0) < 250 ? 'medium' : 'slow'
  const url = probe.url || probe.host
  const httpStatus = probe.statusCode ? `HTTP ${probe.statusCode}` : running ? 'HTTP 检测中' : probe.status === 'timeout' ? 'HTTP TIMEOUT' : probe.status === 'failed' ? 'HTTP FAILED' : 'HTTP —'
  const serverAddress = probe.address || '服务器 IP 暂未获取'
  return <div className={`latency-card ${latencyClass} ${running ? 'running' : ''}`} aria-label={`${probe.name}，${probe.region}，${running ? '正在测试' : '轮询结果'}`}><LatencyTooltip label={`${url}，${httpStatus}，服务器 IP ${probe.address || '暂未获取'}`} content={<><span className="latency-tooltip-url" title={url}>{url}</span><strong>{httpStatus}</strong><code className={probe.address ? '' : 'empty'}><NetworkAddress value={probe.address} fallback={serverAddress}/></code></>}><ServiceIcon id={probe.id}/></LatencyTooltip><div className="service-copy"><strong>{probe.name}</strong><span className={`region-label ${probe.region === '国内' ? 'domestic' : 'international'}`}>{probe.region}</span></div><div className="latency-value">{running ? <span className="latency-pending"><span className="spinner dark"/><span>测试中</span></span> : probe.status === 'pending' ? <span className="latency-waiting">已暂停</span> : probe.status === 'ok' ? <><b>{probe.latencyMs}</b><small>ms</small></> : <b>{probe.status === 'timeout' ? 'TIMEOUT' : 'FAILED'}</b>}</div></div>
}

function LatencyTooltip({ children, label, content }: { children: ReactNode; label: string; content: ReactNode }) {
  const triggerRef = useRef<HTMLSpanElement>(null)
  const tooltipRef = useRef<HTMLDivElement>(null)
  const tooltipId = useId()
  const [open, setOpen] = useState(false)
  const [position, setPosition] = useState({left: 12, top: 12, placement: 'bottom' as 'top' | 'bottom'})

  const updatePosition = useCallback(() => {
    const trigger = triggerRef.current
    if (!trigger) return
    const triggerRect = trigger.getBoundingClientRect()
    const tooltipRect = tooltipRef.current?.getBoundingClientRect()
    const width = tooltipRect?.width || 250
    const height = tooltipRect?.height || 104
    const viewportPadding = 12
    const gap = 8
    const below = triggerRect.bottom + gap
    const placement = below + height <= window.innerHeight - viewportPadding || triggerRect.top < height + gap + viewportPadding ? 'bottom' : 'top'
    const unclampedLeft = triggerRect.left + triggerRect.width / 2 - width / 2
    const maxLeft = Math.max(viewportPadding, window.innerWidth - width - viewportPadding)
    setPosition({
      left: Math.min(maxLeft, Math.max(viewportPadding, unclampedLeft)),
      top: placement === 'bottom' ? below : Math.max(viewportPadding, triggerRect.top - height - gap),
      placement,
    })
  }, [])

  useLayoutEffect(() => {
    if (!open) return
    updatePosition()
    const refresh = () => updatePosition()
    window.addEventListener('resize', refresh)
    window.addEventListener('scroll', refresh, true)
    return () => {
      window.removeEventListener('resize', refresh)
      window.removeEventListener('scroll', refresh, true)
    }
  }, [open, updatePosition])

  const show = () => { updatePosition(); setOpen(true) }
  return <>
    <span ref={triggerRef} className="service-tooltip-trigger" tabIndex={0} aria-label={label} aria-describedby={open ? tooltipId : undefined} onMouseEnter={show} onMouseLeave={() => setOpen(false)} onFocus={show} onBlur={() => setOpen(false)}>{children}</span>
    {open && createPortal(<div ref={tooltipRef} id={tooltipId} className="latency-tooltip" role="tooltip" data-placement={position.placement} style={{left: position.left, top: position.top}}>{content}</div>, document.body)}
  </>
}

function ServiceIcon({ id }: { id: string }) {
  const icon = serviceIcons[id as keyof typeof serviceIcons]
  return <span className={`service-mark ${icon ? id : 'custom'}`}>{icon ? <svg role="img" viewBox="0 0 24 24" aria-label={icon.title}><path d={icon.path}/></svg> : <Icon name="globe" size={19}/>}</span>
}

function NetworkInterfaces({ items }: { items: NetworkInterface[] }) {
  type Filter = 'physical' | 'active' | 'all' | 'ethernet' | 'wifi' | 'virtual' | 'other'
  const [filter, setFilter] = useState<Filter>('physical')
  const visible = useMemo(() => items.filter(item => {
    if (filter === 'all') return true
    if (filter === 'physical') return item.physical
    if (filter === 'active') return item.up
    if (filter === 'ethernet' || filter === 'wifi') return item.kind === filter && item.physical
    if (filter === 'virtual') return !item.physical && ['virtual', 'loopback', 'tunnel'].includes(item.kind)
    return !item.physical && !['virtual', 'loopback', 'tunnel'].includes(item.kind)
  }), [filter, items])
  return <section className="card adapter-section">
    <div className="section-title"><h3>网卡</h3><div className="adapter-section-actions"><select className="adapter-filter" aria-label="选择网卡类型" value={filter} onChange={event => setFilter(event.target.value as Filter)}><option value="physical">物理网卡</option><option value="active">已连接</option><option value="ethernet">有线网卡</option><option value="wifi">Wi-Fi</option><option value="virtual">虚拟与隧道</option><option value="other">其他类型</option><option value="all">全部网卡</option></select><span className="section-badge">{visible.length} / {items.length}</span></div></div>
    <div className="adapter-grid">{visible.length ? visible.map(item => {
      const address = item.ipv4?.[0] || item.ipv6?.[0] || item.mac
      return <div className={`adapter-card ${item.up ? 'up' : ''}`} key={`${item.index}-${item.name}`} title={item.description}><span className={`adapter-kind ${item.kind}`}><Icon name={item.kind === 'ethernet' ? 'network' : 'globe'} size={18}/></span><div><div className="adapter-title-row"><strong>{item.name}</strong>{item.up && <span className="adapter-online"><i/>在线</span>}</div><small>{networkInterfaceLabel(item)} · {item.up ? '已连接' : '未连接'}{item.linkSpeedMbps ? ` · ${item.linkSpeedMbps} Mbps` : ''}</small><code className="network-address" title={address}><NetworkAddress value={address} fallback="暂无地址"/></code></div><div className="adapter-metrics"><span>MAC {item.mac || '—'}</span><span>Metric {item.ipv4Metric || item.ipv6Metric || '自动'}</span></div></div>
    }) : <div className="adapter-empty">当前筛选条件下没有网卡</div>}</div>
  </section>
}

function networkInterfaceLabel(item: NetworkInterface) {
  const labels: Record<NetworkInterface['kind'], string> = { ethernet: '有线以太网', wifi: '无线 Wi-Fi', virtual: '虚拟网卡', loopback: '回环接口', tunnel: '隧道接口', ppp: 'PPP 接口', other: '其他接口' }
  return labels[item.kind] || '其他接口'
}

function AuthPage({ data, state, logs, onAdapters, onProfile, onError }: { data: BootstrapData; state: AuthState; logs: AuthEvent[]; onAdapters: (items: Adapter[]) => void; onProfile: (profile: Profile) => void; onError: (message: string) => void }) {
  const [profile, setProfile] = useState<Profile>(data.profile || defaultProfile)
  const [password, setPassword] = useState('')
  const [advanced, setAdvanced] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [refreshMotion, setRefreshMotion] = useState(0)
  const authenticating = state === 'starting' || state === 'waiting_identity' || state === 'waiting_challenge' || state === 'stopping'
  const formDisabled = authenticating || state === 'authenticated'
  const ethernetAdapters = useMemo(() => data.adapters.filter(item => item.kind === 'ethernet' && item.recommended), [data.adapters])
  const selectedAdapter = useMemo(() => ethernetAdapters.find(item => item.deviceName === profile.deviceName), [ethernetAdapters, profile.deviceName])

  useEffect(() => {
    if ((!profile.deviceName || !ethernetAdapters.some(item => item.deviceName === profile.deviceName)) && ethernetAdapters.length) {
      const first = ethernetAdapters.find(item => item.up) || ethernetAdapters[0]
      setProfile(current => ({...current, deviceName: first.deviceName, adapterLabel: first.name, localMac: first.mac || current.localMac}))
    }
  }, [ethernetAdapters, profile.deviceName])

  const update = <K extends keyof Profile>(key: K, value: Profile[K]) => setProfile(current => ({...current, [key]: value}))
  const chooseAdapter = (deviceName: string) => {
    const adapter = ethernetAdapters.find(item => item.deviceName === deviceName)
    setProfile(current => ({...current, deviceName, adapterLabel: adapter?.name || '', localMac: adapter?.mac || current.localMac}))
  }
  const refresh = async () => {
    setRefreshMotion(current => current + 1)
    setRefreshing(true)
    try { const result = await api().RefreshAdapters(); onAdapters(result.adapters || []); if (result.error) onError(result.error) }
    catch (error) { onError(errorMessage(error)) } finally { setRefreshing(false) }
  }
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    try { const request: AuthRequest = {...profile, password}; await api().Connect(request); onProfile({...profile, passwordSet: profile.rememberPassword && (password !== '' || profile.passwordSet)}); setPassword('') }
    catch (error) { onError(errorMessage(error)) }
  }
  const logout = async () => { try { await api().Logout() } catch (error) { onError(errorMessage(error)) } }
  const cancel = async () => { try { await api().CancelAuthentication() } catch (error) { onError(errorMessage(error)) } }

  return <div className="page-grid auth-grid">
    <section className="card connect-card">
      <div className="card-heading"><div><span className="eyebrow">802.1X ACCESS</span><h2>{state === 'authenticated' ? '网络已认证' : '连接校园网'}</h2></div><div className={`signal-orb ${state}`}><span/><span/><span/></div></div>
      {!data.npcapAvailable && <div className="notice warning"><Icon name="info"/><div><strong>需要安装 Npcap</strong><p>{data.adapterError || '认证功能需要 Npcap 驱动来收发 EAPOL 报文。'}</p><button className="link-button" onClick={() => api().OpenLink('https://npcap.com/#download')}>打开下载页 <Icon name="external" size={14}/></button></div></div>}
      <form onSubmit={submit} className="auth-form">
        <label className="field field-wide"><span>有线网卡</span><div className="input-row"><select value={profile.deviceName} onChange={event => chooseAdapter(event.target.value)} disabled={formDisabled}><option value="">请选择认证网卡</option>{ethernetAdapters.map(adapter => <option key={adapter.deviceName} value={adapter.deviceName}>{adapter.name}{adapter.mac ? ` · ${adapter.mac}` : ''}</option>)}</select><button type="button" className="square-button adapter-refresh-button" onClick={refresh} disabled={refreshing || formDisabled} title={refreshing ? '正在刷新网卡' : '刷新网卡'} aria-label={refreshing ? '正在刷新网卡' : '刷新网卡'} aria-busy={refreshing}><span className={`adapter-refresh-glyph ${refreshMotion ? 'animate' : ''}`} key={refreshMotion}><Icon name="refresh" size={18}/></span></button></div>{selectedAdapter && <small>{selectedAdapter.description || selectedAdapter.deviceName}{selectedAdapter.ipv4?.length ? ` · IPv4 ${selectedAdapter.ipv4[0]}` : ''}</small>}</label>
        <label className="field"><span>校园网账号</span><input value={profile.username} onChange={event => update('username', event.target.value)} autoComplete="username" placeholder="学号 / 用户名" disabled={formDisabled}/></label>
        <label className="field"><span>密码</span><input type="password" value={password} onChange={event => setPassword(event.target.value)} autoComplete="current-password" placeholder={profile.passwordSet ? '已安全保存，留空继续使用' : '请输入密码'} disabled={formDisabled}/></label>
        <label className="field field-wide"><span>网卡 MAC 地址</span><input value={profile.localMac} onChange={event => update('localMac', event.target.value)} placeholder="例如 12:34:56:78:9A:BC" disabled={formDisabled}/></label>
        <label className="check-row field-wide"><input type="checkbox" checked={profile.rememberPassword} onChange={event => update('rememberPassword', event.target.checked)} disabled={formDisabled}/><span>为当前 Windows 用户安全保存密码</span></label>
        <details className="auth-advanced field-wide" open={advanced} onToggle={event => setAdvanced(event.currentTarget.open)}>
          <summary><span>高级适配选项</span><Icon name="chevron" size={14}/></summary>
        {advanced && <div className="advanced-panel">
          <label className="field"><span>EAP Identity</span><input value={profile.identity} onChange={event => update('identity', event.target.value)} placeholder="留空时与账号一致" disabled={formDisabled}/></label>
          <label className="field"><span>Identity 后缀（Hex）</span><input value={profile.identitySuffix} onChange={event => update('identitySuffix', event.target.value)} placeholder="例如 00 00 13 11 00" disabled={formDisabled}/></label>
          <label className="field"><span>启动延迟</span><div className="unit-input"><input type="number" min="0" max="30000" value={profile.startDelayMs} onChange={event => update('startDelayMs', Number(event.target.value))} disabled={formDisabled}/><em>ms</em></div></label>
          <label className="field"><span>失败重试</span><div className="unit-input"><input type="number" min="0" max="60000" value={profile.retryDelayMs} onChange={event => update('retryDelayMs', Number(event.target.value))} disabled={formDisabled}/><em>ms</em></div></label>
          <label className="check-row compact"><input type="checkbox" checked={profile.debug} onChange={event => update('debug', event.target.checked)} disabled={formDisabled}/><span>记录原始报文调试信息</span></label>
        </div>}
        </details>
        <div className="form-actions field-wide">{state === 'authenticated' ? <button type="button" className="button danger" onClick={logout}><Icon name="plug"/>注销</button> : authenticating ? <button type="button" className="button danger" onClick={cancel}><span className="trace-stop-square"/>取消认证</button> : <button type="submit" className="button primary" disabled={!data.npcapAvailable || !ethernetAdapters.length}><Icon name="shield"/>开始认证</button>}{state === 'authenticated' && <span>认证会话已结束，不会持续抓包或发送保活报文</span>}</div>
      </form>
    </section>
    <section className="card session-card"><div className="section-title"><div><h3>认证会话</h3><p>最近的协议事件</p></div>{authenticating && <span className="live-indicator"><i/>LIVE</span>}</div><div className="timeline">{logs.length === 0 ? <div className="empty-state"><div><Icon name="activity" size={28}/></div><strong>{state === 'authenticated' ? '已检测到有线网络认证' : '等待开始认证'}</strong><p>{state === 'authenticated' ? '本次启动无需重复认证，需要断开时点击“注销”。' : '交换机的 Identity、Challenge 和结果会显示在这里。'}</p></div> : [...logs].reverse().map((log, index) => <div className={`log-row ${log.level}`} key={`${log.timestamp}-${index}`}><span className="log-node"/><div><p>{log.message}</p><time>{log.timestamp || '--:--:--'}</time></div></div>)}</div><div className="protocol-strip"><span>EAPOL</span><i/><span>Identity</span><i/><span>MD5</span><i/><span>Success</span></div></section>
  </div>
}

function NATPage({ data, result, setResult, running, setRunning, onSaveSettings, onError }: { data: BootstrapData; result: NATResult | null; setResult: (value: NATResult | null) => void; running: boolean; setRunning: (value: boolean) => void; onSaveSettings: SettingsSaver; onError: (message: string) => void }) {
  const check = async () => { setRunning(true); setResult(null); try { setResult(await api().CheckNAT()) } catch (error) { onError(errorMessage(error)) } finally { setRunning(false) } }
  return <div className="tool-page"><section className="card tool-hero integrated-tool-hero nat-hero"><div className="integrated-tool-heading"><div className="tool-icon"><Icon name={toolIcons.nat} size={30}/></div><div><span className="eyebrow">STUN BEHAVIOUR DISCOVERY</span><h2>{result?.type || '你的网络属于哪种 NAT？'}</h2></div><button className="button primary" onClick={check} disabled={running}>{running ? <><span className="spinner"/>检测中…</> : <><Icon name={toolIcons.nat}/>开始检测</>}</button></div><NATSettings value={data.diagnostics} onSaveSettings={onSaveSettings} onError={onError}/></section>
    {running && <ProgressSteps steps={[
      {title: '连接节点', detail: '探测可用 STUN 服务'},
      {title: '比较映射', detail: '分析公网端点变化'},
      {title: '验证类型', detail: '汇总映射与过滤行为'},
    ]}/>}
    {result && <>
      {result.status === 'error' && <div className="notice error"><Icon name="info"/><div><strong>检测未完成</strong><p>{result.error || result.summary}</p></div></div>}
      <div className="metrics-grid"><Metric label="NAT 类型" value={result.type || '未知'} accent/><Metric label="公网端点" value={result.publicIp ? `${result.publicIp}:${result.publicPort}` : '未获取'}/><Metric label="本地 IPv4" value={result.localIp || '未获取'}/><Metric label="首包延迟" value={result.latencyMs ? `${result.latencyMs} ms` : '—'}/></div>
      <section className="card detail-card"><div className="section-title"><div><h3>行为分析</h3><p>{result.rfc5780 ? '已完成 RFC 5780 映射与过滤测试' : '已使用多节点映射对比回退'}</p></div><span className={`quality-badge ${result.rfc5780 ? 'good' : 'partial'}`}>{result.rfc5780 ? '完整结果' : '保守结果'}</span></div><div className="behavior-list"><Behavior name="映射行为" value={result.mappingBehavior}/><Behavior name="过滤行为" value={result.filteringBehavior}/><Behavior name="响应节点" value={`${result.serverCount} / ${result.probes?.length || 0}`}/></div></section>
      <section className="card detail-card"><div className="section-title"><div><h3>STUN 节点明细</h3><p>保留全部节点的解析、映射、时延和失败原因</p></div><span className="section-badge">{result.probes?.length || 0} NODES</span></div><div className="result-table-wrap"><div className="result-table nat-results"><div className="result-table-head"><span>服务器</span><span>服务器 IP</span><span>外部映射</span><span>时延</span><span>状态</span></div>{result.probes?.map(probe => <div className={`result-table-row ${probe.error ? 'failed' : ''}`} key={probe.server}><code title={probe.server}>{stripPort(probe.server)}</code><code>{probe.serverIp || '—'}</code><code>{probe.endpoint || '—'}</code><time>{probe.latencyMs ? `${probe.latencyMs} ms` : '—'}</time><div className="result-status"><span className={`probe-status ${probe.error ? 'bad' : 'good'}`}>{probe.error ? '超时/失败' : '成功'}</span>{probe.error && <small title={probe.error}>{probe.error}</small>}</div></div>)}</div></div></section>
    </>}
  </div>
}

function IPv6Page({ data, result, setResult, running, setRunning, onSaveSettings, onError }: { data: BootstrapData; result: IPv6Result | null; setResult: (value: IPv6Result | null) => void; running: boolean; setRunning: (value: boolean) => void; onSaveSettings: SettingsSaver; onError: (message: string) => void }) {
  const check = async () => { setRunning(true); setResult(null); try { setResult(await api().CheckIPv6()) } catch (error) { onError(errorMessage(error)) } finally { setRunning(false) } }
  return <div className="tool-page"><section className="card tool-hero integrated-tool-hero ipv6-hero"><div className="integrated-tool-heading"><div className="tool-icon"><Icon name={toolIcons.ipv6} size={30}/></div><div><span className="eyebrow">DUAL STACK CONNECTIVITY</span><h2>{result?.connectionType || 'IPv6 是否真正可用？'}</h2></div><button className="button primary" onClick={check} disabled={running}>{running ? <><span className="spinner"/>检测中…</> : <><Icon name={toolIcons.ipv6}/>开始测试</>}</button></div><IPv6Settings value={data.diagnostics} onSaveSettings={onSaveSettings} onError={onError}/></section>
    {running && <ProgressSteps steps={[
      {title: '扫描本机', detail: '识别 IPv6 地址环境'},
      {title: '测试连通', detail: '并行访问双栈与网站'},
      {title: '汇总结果', detail: '判断公网连接类型'},
    ]}/>}
    {result && <><div className="ip-cards"><IPCard version="IPv4" result={result.ipv4}/><IPCard version="IPv6" result={result.ipv6}/></div><section className="card detail-card"><div className="section-title"><div><h3>本机 IPv6 环境</h3><p>拥有地址不代表公网出口一定可用</p></div><span className={`quality-badge ${result.ipv6.available ? 'good' : 'partial'}`}>{result.ipv6.available ? '公网可达' : '需要排查'}</span></div><div className="behavior-list"><Behavior name="全局单播地址" value={result.localGlobal.length ? result.localGlobal.join(' · ') : '未发现'}/><Behavior name="链路本地地址" value={result.localLink.length ? `${result.localLink.length} 个` : '未发现'}/><Behavior name="AAAA DNS 解析" value={result.dnsAAAA ? '正常' : '未通过'}/>{result.largePacket && <Behavior name="IPv6 数据端点" value={result.largePacket.available ? `正常 · ${result.largePacket.latencyMs} ms · ${formatBytes(result.largePacket.bytesRead || 0)}` : result.largePacket.error || '未通过'}/>}</div></section><section className="card detail-card"><div className="section-title"><div><h3>常用网站 IPv6</h3><p>强制通过 IPv6 访问，不受 IPv4 回退影响</p></div><span className="section-badge">LIVE WEB</span></div><div className="website-results">{result.sites?.map(site => <div className={`website-result ${site.available ? 'available' : 'failed'}`} key={site.url}><div className="website-name"><span className="probe-dot"/><strong>{site.host}</strong><small className="network-address" title={site.address}><NetworkAddress value={site.address} fallback="未建立 IPv6 连接"/></small></div><time>{site.latencyMs ? `${site.latencyMs} ms` : '—'}</time><span>{site.statusCode ? `HTTP ${site.statusCode}` : '—'}</span><span className={`probe-status ${site.available ? 'good' : 'bad'}`}>{site.available ? '可访问' : '失败'}</span>{site.error && <small className="website-error" title={site.error}>{site.error}</small>}</div>)}</div></section></>}
  </div>
}

function NetworkPathPage({ ping, trace, onPingChange, onPingStart, onPingStop, onTraceChange, onTraceStart, onTraceStop }: {
  ping: PingViewState
  trace: TraceViewState
  onPingChange: (values: Partial<PingViewState>) => void
  onPingStart: () => void
  onPingStop: () => void
  onTraceChange: (values: Partial<TraceViewState>) => void
  onTraceStart: () => void
  onTraceStop: () => void
}) {
  return <div className="tool-page trace-page">
    <PingTool value={ping} onChange={onPingChange} onStart={onPingStart} onStop={onPingStop}/>
    <TracerouteTool value={trace} onChange={onTraceChange} onStart={onTraceStart} onStop={onTraceStop}/>
  </div>
}

function PingTool({ value, onChange, onStart, onStop }: { value: PingViewState; onChange: (values: Partial<PingViewState>) => void; onStart: () => void; onStop: () => void }) {
  const starting = value.phase === 'starting'
  const probing = value.phase === 'resolving' || value.phase === 'running'
  const stopping = value.phase === 'stopping'
  const active = starting || probing || stopping
  const submit = (event: FormEvent) => { event.preventDefault(); if (!active) onStart() }
  const received = value.summary?.received ?? value.replies.filter(reply => reply.status === 'reply').length
  const sent = value.summary?.sent ?? value.replies.length
  const lost = value.summary?.lost ?? Math.max(0, sent - received)
  const lossPercent = value.summary?.lossPercent ?? (sent ? lost / sent * 100 : 0)
  const latencies = value.replies.flatMap(reply => reply.status === 'reply' && typeof reply.latencyMs === 'number' ? [reply.latencyMs] : [])
  const minimum = received ? value.summary?.minMs ?? Math.min(...latencies) : undefined
  const average = received ? value.summary?.averageMs ?? latencies.reduce((total, item) => total + item, 0) / latencies.length : undefined
  const maximum = received ? value.summary?.maxMs ?? Math.max(...latencies) : undefined
  const statusLabel = starting ? '正在启动 Ping' : value.phase === 'resolving' ? '正在解析目标' : value.phase === 'running' ? `正在探测 ${value.replies.length}/${value.count}` : stopping ? '正在停止 Ping' : value.phase === 'cancelled' ? '已停止' : value.phase === 'error' ? 'Ping 失败' : value.phase === 'completed' && lossPercent >= 100 ? '目标未响应' : value.phase === 'completed' && lost ? '测试完成，有丢包' : value.phase === 'completed' ? '测试完成' : '准备就绪'

  return <div className="diagnostic-tool ping-tool">
    <section className="card trace-hero ping-hero">
      <div className="trace-heading"><div className="tool-icon ping-icon"><Icon name={toolIcons.ping} size={30}/></div><div><span className="eyebrow">ICMP PING</span><h2>检测连通性与延迟</h2></div></div>
      <form className="trace-form" onSubmit={submit}>
        <label className={`trace-target ${value.error && value.phase === 'idle' ? 'invalid' : ''}`}><span className="sr-only">Ping 目标</span><input value={value.target} onChange={event => onChange({target: event.target.value, error: ''})} placeholder="baidu.com（默认）或 8.8.8.8" disabled={active} aria-invalid={Boolean(value.error && value.phase === 'idle')}/></label>
        <select className="trace-protocol" aria-label="选择 Ping 协议" value={value.protocol} onChange={event => onChange({protocol: event.target.value as TraceProtocol})} disabled={active}><option value="auto">自动</option><option value="ipv4">IPv4</option><option value="ipv6">IPv6</option></select>
        {starting ? <button type="button" className="button primary" disabled><span className="spinner"/>正在启动…</button> : stopping ? <button type="button" className="button trace-stop" disabled><span className="spinner"/>正在停止…</button> : probing ? <button type="button" className="button trace-stop" onClick={onStop}><span className="trace-stop-square"/>停止 Ping</button> : <button type="submit" className="button primary"><Icon name={toolIcons.ping} size={17}/>{value.replies.length || value.summary ? '再次 Ping' : '开始 Ping'}</button>}
      </form>
      {value.error && value.phase === 'idle' && <small className="trace-input-error">{value.error}</small>}
      <details className="trace-advanced" open={value.advancedOpen} onToggle={event => onChange({advancedOpen: event.currentTarget.open})}>
        <summary><span>高级设置</span><Icon name="chevron" size={14}/></summary>
        <div className="trace-advanced-grid ping-advanced-grid">
          <label><span>探测次数</span><input type="number" min="1" max="100" value={value.count} onChange={event => onChange({count: Number(event.target.value)})} onBlur={() => onChange({count: Math.min(100, Math.max(1, value.count || 1))})} disabled={active}/></label>
          <label><span>单次超时</span><div className="unit-input"><input type="number" min="200" max="5000" step="100" value={value.timeoutMs} onChange={event => onChange({timeoutMs: Number(event.target.value)})} onBlur={() => onChange({timeoutMs: Math.min(5000, Math.max(200, value.timeoutMs || 200))})} disabled={active}/><em>ms</em></div></label>
          <label><span>发送间隔</span><div className="unit-input"><input type="number" min="100" max="10000" step="100" value={value.intervalMs} onChange={event => onChange({intervalMs: Number(event.target.value)})} onBlur={() => onChange({intervalMs: Math.min(10000, Math.max(100, value.intervalMs || 100))})} disabled={active}/><em>ms</em></div></label>
        </div>
      </details>
    </section>

    {value.phase !== 'idle' && <section className="card trace-summary ping-summary">
      <div className="trace-summary-target"><span>目标</span><strong>{value.submittedTarget || '—'}</strong>{value.resolvedTarget && <code><NetworkAddress value={value.resolvedTarget}/></code>}</div>
      <div className="trace-summary-facts"><span>{value.actualProtocol ? value.actualProtocol.toUpperCase() : '解析中'}</span><span>{sent} 已发送</span><span>{received} 已接收</span></div>
      <span className={`trace-run-status ${value.phase} ${value.phase === 'completed' ? lossPercent >= 100 ? 'unreachable' : lossPercent > 0 ? 'partial' : 'reached' : ''}`}>{active && <span className="spinner dark"/>}{statusLabel}</span>
    </section>}

    {value.error && value.phase !== 'idle' && <div className="notice error trace-error"><Icon name="info"/><div><strong>Ping 测试未完成</strong><p>{value.error}</p></div></div>}

    {(value.replies.length > 0 || value.summary || probing || stopping) && <section className="card ping-results">
      <div className="section-title trace-results-title"><div><h3>Ping 结果</h3><p>ICMP 超时可能由防火墙过滤引起，不一定代表网站无法访问</p></div><span className="section-badge">{sent}/{value.count} PACKETS</span></div>
      <div className="ping-metrics" aria-label="Ping 统计">
        <div><span>丢包率</span><strong className={lossPercent ? 'warning' : ''}>{formatPercentage(lossPercent)}</strong></div>
        <div><span>最小延迟</span><strong>{formatTraceLatency(minimum)}</strong></div>
        <div><span>平均延迟</span><strong>{formatTraceLatency(average)}</strong></div>
        <div><span>最大延迟</span><strong>{formatTraceLatency(maximum)}</strong></div>
      </div>
      <div className="ping-table" role="table" aria-label="Ping 逐包结果">
        <div className="ping-table-head" role="row"><span>序号</span><span>结果</span><span>响应地址</span><span>延迟</span></div>
        {value.replies.map(reply => <PingReplyRow reply={reply} key={reply.sequence}/>) }
        {probing && <div className="trace-waiting"><span className="spinner dark"/><span>{value.phase === 'resolving' ? '正在解析目标地址…' : `正在等待第 ${value.replies.length + 1} 次响应…`}</span></div>}
      </div>
    </section>}
  </div>
}

function PingReplyRow({ reply }: { reply: PingReply }) {
  const label = reply.status === 'reply' ? '已响应' : reply.status === 'unreachable' ? '不可达' : '请求超时'
  return <div className={`ping-row ${reply.status}`} role="row">
    <b>#{reply.sequence}</b>
    <span className={`ping-reply-status ${reply.status}`}>{label}</span>
    <code title={reply.address}>{reply.address ? <NetworkAddress value={reply.address}/> : '—'}</code>
    <strong>{reply.status === 'reply' ? formatTraceLatency(reply.latencyMs) : '*'}</strong>
  </div>
}

function TracerouteTool({ value, onChange, onStart, onStop }: { value: TraceViewState; onChange: (values: Partial<TraceViewState>) => void; onStart: () => void; onStop: () => void }) {
  const starting = value.phase === 'starting'
  const probing = value.phase === 'resolving' || value.phase === 'running'
  const stopping = value.phase === 'stopping'
  const active = starting || probing || stopping
  const submit = (event: FormEvent) => { event.preventDefault(); if (!active) onStart() }
  const statusLabel = starting ? '正在启动追踪' : value.phase === 'resolving' ? '正在解析目标' : value.phase === 'running' ? `正在追踪第 ${value.hops.length + 1} 跳` : stopping ? '正在停止追踪' : value.phase === 'cancelled' ? '已停止' : value.phase === 'error' ? '追踪失败' : value.resultStatus === 'reached' ? '已到达目标' : value.resultStatus === 'unreachable' ? '目标不可达' : value.resultStatus === 'max_hops' ? '已达到最大跳数' : '准备就绪'
  return <div className="diagnostic-tool trace-tool">
    <section className="card trace-hero">
      <div className="trace-heading"><div className="tool-icon trace-icon"><Icon name={toolIcons.trace} size={30}/></div><div><span className="eyebrow">ROUTE TRACE</span><h2>逐跳定位网络路径</h2></div></div>
      <form className="trace-form" onSubmit={submit}>
        <label className={`trace-target ${value.error && value.phase === 'idle' ? 'invalid' : ''}`}><span className="sr-only">追踪目标</span><input value={value.target} onChange={event => onChange({target: event.target.value, error: ''})} placeholder="baidu.com（默认）或 8.8.8.8" disabled={active} aria-invalid={Boolean(value.error && value.phase === 'idle')}/></label>
        <select className="trace-protocol" aria-label="选择路由追踪协议" value={value.protocol} onChange={event => onChange({protocol: event.target.value as TraceProtocol})} disabled={active}><option value="auto">自动</option><option value="ipv4">IPv4</option><option value="ipv6">IPv6</option></select>
        {starting ? <button type="button" className="button primary" disabled><span className="spinner"/>正在启动…</button> : stopping ? <button type="button" className="button trace-stop" disabled><span className="spinner"/>正在停止…</button> : probing ? <button type="button" className="button trace-stop" onClick={onStop}><span className="trace-stop-square"/>停止追踪</button> : <button type="submit" className="button primary"><Icon name={toolIcons.trace} size={17}/>{value.hops.length ? '再次追踪' : '开始追踪'}</button>}
      </form>
      {value.error && value.phase === 'idle' && <small className="trace-input-error">{value.error}</small>}
      <details className="trace-advanced" open={value.advancedOpen} onToggle={event => onChange({advancedOpen: event.currentTarget.open})}>
        <summary><span>高级设置</span><Icon name="chevron" size={14}/></summary>
        <div className="trace-advanced-grid">
          <label><span>最大跳数</span><input type="number" min="1" max="64" value={value.maxHops} onChange={event => onChange({maxHops: Number(event.target.value)})} onBlur={() => onChange({maxHops: Math.min(64, Math.max(1, value.maxHops || 1))})} disabled={active}/></label>
          <label><span>每跳超时</span><div className="unit-input"><input type="number" min="200" max="5000" step="100" value={value.timeoutMs} onChange={event => onChange({timeoutMs: Number(event.target.value)})} onBlur={() => onChange({timeoutMs: Math.min(5000, Math.max(200, value.timeoutMs || 200))})} disabled={active}/><em>ms</em></div></label>
          <label className="trace-dns-toggle"><input type="checkbox" checked={value.resolveHostnames} onChange={event => onChange({resolveHostnames: event.target.checked})} disabled={active}/><span><strong>解析主机名</strong><small>为响应节点执行反向 DNS 查询</small></span></label>
        </div>
      </details>
    </section>

    {value.phase !== 'idle' && <section className="card trace-summary">
      <div className="trace-summary-target"><span>目标</span><strong>{value.submittedTarget || '—'}</strong>{value.resolvedTarget && <code><NetworkAddress value={value.resolvedTarget}/></code>}</div>
      <div className="trace-summary-facts"><span>{value.actualProtocol ? value.actualProtocol.toUpperCase() : '解析中'}</span><span>{value.hops.length} 跳</span><span>{value.durationMs ? formatTraceDuration(value.durationMs) : '实时'}</span></div>
      <span className={`trace-run-status ${value.phase} ${value.resultStatus || ''}`}>{active && <span className="spinner dark"/>}{statusLabel}</span>
    </section>}

    {value.error && value.phase !== 'idle' && <div className="notice error trace-error"><Icon name="info"/><div><strong>路由追踪未完成</strong><p>{value.error}</p></div></div>}

    {(value.hops.length > 0 || probing || stopping) && <section className="card trace-results">
      <div className="section-title trace-results-title"><div><h3>路由节点</h3><p>每跳发送 3 次 ICMP 探测；单跳超时不代表链路一定中断</p></div><span className="section-badge">{value.hops.length} HOPS</span></div>
      <div className="trace-table" role="table" aria-label="路由追踪结果">
        <div className="trace-table-head" role="row"><span>跳数</span><span>路由节点</span><span>探测 1</span><span>探测 2</span><span>探测 3</span><span>平均</span><span>状态</span></div>
        {value.hops.map((hop, index) => <TraceHopRow hop={hop} last={!active && index === value.hops.length - 1} key={hop.number}/>)}
        {probing && <div className="trace-waiting"><span className="spinner dark"/><span>{value.phase === 'resolving' ? '正在解析目标地址…' : `正在等待第 ${value.hops.length + 1} 跳响应…`}</span></div>}
      </div>
    </section>}

  </div>
}

function TraceHopRow({ hop, last }: { hop: TraceHop; last: boolean }) {
  const nodes = Array.from(new Map(hop.probes.filter(probe => probe.address).map(probe => [probe.address, {address: probe.address || '', hostname: probe.hostname || ''}])).values())
  const statusText = hop.status === 'destination' ? '目标节点' : hop.status === 'unreachable' ? '不可达' : hop.status === 'timeout' ? '请求超时' : '正常'
  return <div className={`trace-hop-row ${hop.status} ${last ? 'last' : ''}`} role="row">
    <div className="trace-hop-number"><i/><b>{hop.number}</b></div>
    <div className="trace-hop-nodes">{nodes.length ? nodes.map(node => <div className="trace-node" key={node.address}>{node.hostname && <strong title={node.hostname}>{node.hostname}</strong>}<code title={node.address}><NetworkAddress value={node.address}/></code></div>) : <span>未响应</span>}</div>
    {Array.from({length: 3}, (_, index) => { const probe = hop.probes[index]; return <span className={`trace-probe-time ${probe?.status || 'timeout'}`} title={probe?.address || '请求超时'} key={index}>{probe?.status !== 'timeout' ? formatTraceLatency(probe?.latencyMs) : '*'}</span> })}
    <strong className="trace-average">{typeof hop.averageMs === 'number' ? formatTraceLatency(hop.averageMs) : '—'}</strong>
    <span className={`trace-hop-status ${hop.status}`}>{statusText}</span>
  </div>
}

function useDiagnosticAutosave(draftKey: string, build: (current: DiagnosticSettings) => DiagnosticSettings, onSaveSettings: SettingsSaver, onError: (message: string) => void) {
  const baseline = useRef(draftKey)
  const buildRef = useRef(build)
  buildRef.current = build

  useEffect(() => {
    if (draftKey === baseline.current) return
    const timer = window.setTimeout(() => {
      void onSaveSettings(current => ({profile: current.profile, system: current.system, diagnostics: buildRef.current(current.diagnostics)})).then(() => {
        baseline.current = draftKey
      }).catch(error => onError(errorMessage(error)))
    }, 700)
    return () => window.clearTimeout(timer)
  }, [draftKey, onSaveSettings, onError])
}

function LatencySettings({ value, onSaveSettings, onError }: { value: DiagnosticSettings['latencyTargets']; onSaveSettings: SettingsSaver; onError: (message: string) => void }) {
  const [targets, setTargets] = useState(formatLatencyTargets(value))
  useDiagnosticAutosave(targets, current => ({...current, latencyTargets: parseLatencyTargets(targets)}), onSaveSettings, onError)
  return <details className="module-settings latency-settings">
    <summary><span>延迟目标</span><Icon name="chevron" size={14}/></summary>
    <div className="module-settings-body"><div className="settings-heading compact-heading"><p>每行格式：名称 | URL | 国内或国际。</p><button type="button" className="reset-button" onClick={() => setTargets(formatLatencyTargets(defaultDiagnostics.latencyTargets))}>恢复默认</button></div><textarea className="settings-textarea compact" spellCheck={false} value={targets} onChange={event => setTargets(event.target.value)} aria-label="网站响应测试列表"/></div>
  </details>
}

function PublicEndpointSettings({ version, value, onSaveSettings, onError }: { version: 'IPv4' | 'IPv6'; value: string[]; onSaveSettings: SettingsSaver; onError: (message: string) => void }) {
  const [endpoints, setEndpoints] = useState(value.join('\n'))
  const defaults = version === 'IPv4' ? defaultDiagnostics.ipv4Endpoints : defaultDiagnostics.ipv6Endpoints
  useDiagnosticAutosave(endpoints, current => version === 'IPv4' ? {...current, ipv4Endpoints: splitLines(endpoints)} : {...current, ipv6Endpoints: splitLines(endpoints)}, onSaveSettings, onError)
  return <details className="module-settings public-endpoint-settings">
    <summary><span>查询源</span><Icon name="chevron" size={14}/></summary>
    <div className="module-settings-body"><div className="settings-heading compact-heading"><p>刷新时会按顺序尝试，每行一个 URL。</p><button type="button" className="reset-button" onClick={() => setEndpoints(defaults.join('\n'))}>恢复默认</button></div><textarea className="settings-textarea compact" spellCheck={false} value={endpoints} onChange={event => setEndpoints(event.target.value)} aria-label={`${version} 公网查询源`}/></div>
  </details>
}

function NATSettings({ value, onSaveSettings, onError }: { value: DiagnosticSettings; onSaveSettings: SettingsSaver; onError: (message: string) => void }) {
  const [natServers, setNatServers] = useState(value.natServers.join('\n'))
  useDiagnosticAutosave(natServers, current => ({...current, natServers: splitLines(natServers)}), onSaveSettings, onError)
  return <details className="inline-tool-settings">
    <summary><span>检测节点</span><Icon name="chevron" size={14}/></summary>
    <div className="inline-settings-body"><div className="settings-heading compact-heading"><p>每行一个 host:port；至少保留两个节点，以便比较公网映射。</p><button type="button" className="reset-button" onClick={() => setNatServers(defaultDiagnostics.natServers.join('\n'))}>恢复默认</button></div><textarea className="settings-textarea compact" spellCheck={false} value={natServers} onChange={event => setNatServers(event.target.value)} aria-label="STUN 服务器列表"/></div>
  </details>
}

function IPv6Settings({ value, onSaveSettings, onError }: { value: DiagnosticSettings; onSaveSettings: SettingsSaver; onError: (message: string) => void }) {
  const [ipv6Sites, setIPv6Sites] = useState(value.ipv6Sites.join('\n'))
  const [aaaaDomain, setAAAADomain] = useState(value.aaaaDomain)
  const [largeUrl, setLargeURL] = useState(value.ipv6LargeUrl)
  const draftKey = JSON.stringify([ipv6Sites, aaaaDomain, largeUrl])
  useDiagnosticAutosave(draftKey, current => ({...current, ipv6Sites: splitLines(ipv6Sites), aaaaDomain, ipv6LargeUrl: largeUrl}), onSaveSettings, onError)
  return <details className="inline-tool-settings">
    <summary><span>高级设置</span><Icon name="chevron" size={14}/></summary>
    <div className="inline-settings-body"><div className="settings-heading compact-heading"><p>网站访问会强制使用 IPv6，不会回退到 IPv4。</p><button type="button" className="reset-button" onClick={() => { setIPv6Sites(defaultDiagnostics.ipv6Sites.join('\n')); setAAAADomain(defaultDiagnostics.aaaaDomain); setLargeURL(defaultDiagnostics.ipv6LargeUrl) }}>恢复默认</button></div><div className="settings-grid"><label className="settings-field field-wide"><span>常用网站 <small>每行一个 URL</small></span><textarea className="settings-textarea compact" spellCheck={false} value={ipv6Sites} onChange={event => setIPv6Sites(event.target.value)}/></label><label className="settings-field"><span>AAAA 测试域名</span><input value={aaaaDomain} onChange={event => setAAAADomain(event.target.value)} placeholder="www.qq.com"/></label><label className="settings-field"><span>数据端点 URL <small>可选</small></span><input value={largeUrl} onChange={event => setLargeURL(event.target.value)} placeholder="https://…"/></label></div></div>
  </details>
}

function SettingsPage({ value, onSaveSettings, onError }: { value: BootstrapData; onSaveSettings: SettingsSaver; onError: (message: string) => void }) {
  const [system, setSystem] = useState(value.system)
  const initialRender = useRef(true)
  const updateSystem = (update: (current: typeof system) => typeof system) => setSystem(update)

  useEffect(() => {
    if (initialRender.current) {
      initialRender.current = false
      return
    }
    const timer = window.setTimeout(() => {
      void onSaveSettings(current => ({profile: current.profile, diagnostics: current.diagnostics, system})).catch(error => onError(errorMessage(error)))
    }, 0)
    return () => window.clearTimeout(timer)
  }, [system, onSaveSettings, onError])

  return <div className="settings-page">
    <section className="card settings-overview"><div className="brand-mark large"><Icon name="activity" size={31}/></div><div className="about-copy"><span className="eyebrow">NETWORK TOOLBOX</span><h2>网络工具箱</h2><div className="overview-facts"><span>Go + Wails</span><span>802.1X / EAP-MD5</span><span>Windows DPAPI</span></div></div><div className="settings-save-meta"><span className="version">Version {value.version}</span></div></section>

    <details className="card settings-disclosure">
      <summary><div><span className="disclosure-icon blue"><Icon name="activity" size={19}/></span><span><strong>网络与启动</strong><small>{system.priorityMode === 'ethernet' ? '以太网优先' : system.priorityMode === 'wifi' ? 'Wi-Fi 优先' : '系统自动选择'} · 自动认证{system.autoAuthenticate ? '已开启' : '已关闭'}</small></span></div><span className="summary-side"><Icon name="chevron" size={16}/></span></summary>
      <div className="disclosure-body"><div className="priority-options">
        {([['automatic', '系统自动', '由 Windows 自动选择'], ['ethernet', '以太网优先', '优先使用有线网络'], ['wifi', 'Wi-Fi 优先', '优先使用无线网络']] as const).map(([mode, title, description]) => <label className={`priority-option ${system.priorityMode === mode ? 'selected' : ''}`} key={mode}><input type="radio" name="priority" value={mode} checked={system.priorityMode === mode} onChange={() => updateSystem(current => ({...current, priorityMode: mode}))}/><span><strong>{title}</strong><small>{description}</small></span></label>)}
      </div>
      <label className="toggle-setting"><div><strong>登录后自动认证并监测断线</strong><p>此功能依赖托盘后台；开启时同时开启托盘驻留，每 30 秒检测一次，断线后最多认证 3 次。</p>{(!value.profile.passwordSet || !value.profile.username) && <small>请先在锐捷认证页保存账号和密码</small>}</div><input type="checkbox" checked={system.autoAuthenticate} disabled={!value.profile.passwordSet || !value.profile.username} onChange={event => updateSystem(current => ({...current, autoAuthenticate: event.target.checked, closeToTray: event.target.checked ? true : current.closeToTray}))}/><i/></label>
      <label className="toggle-setting"><div><strong>关闭窗口后驻留系统托盘</strong><p>关闭此项会同步关闭自动认证后台；关闭窗口将彻底退出程序。</p></div><input type="checkbox" checked={system.closeToTray} onChange={event => updateSystem(current => ({...current, closeToTray: event.target.checked, autoAuthenticate: event.target.checked ? current.autoAuthenticate : false}))}/><i/></label></div>
    </details>

  </div>
}

function LoadingCard({ text }: { text: string }) { return <div className="loading-card"><span className="spinner dark"/><p>{text}</p></div> }
function Metric({ label, value, accent = false }: { label: string; value: string; accent?: boolean }) { return <div className={`card metric ${accent ? 'accent' : ''}`}><span>{label}</span><strong>{value}</strong></div> }
function Behavior({ name, value }: { name: string; value: string }) { return <div><span>{name}</span><strong>{value || '无法判断'}</strong></div> }
function ProgressSteps({ steps }: { steps: Array<{title: string; detail: string}> }) {
  return <section className="card diagnostic-progress" aria-live="polite" aria-label="检测正在进行">
    <div className="progress-heading"><span className="progress-spinner"><i/><i/><i/></span><div><strong>正在执行检测</strong><small>各步骤会根据网络响应时间依次完成</small></div></div>
    <div className="progress-track">{steps.map((step, index) => <div className="progress-step" style={{'--step-index': index} as CSSProperties} key={step.title}><span><b>{index + 1}</b></span><div className="progress-step-copy"><strong>{step.title}</strong><small>{step.detail}</small></div>{index < steps.length - 1 && <i/>}</div>)}</div>
  </section>
}
function IPCard({ version, result }: { version: string; result: { available: boolean; address?: string; latencyMs?: number; error?: string } }) { return <section className={`card ip-card ${result.available ? 'available' : ''}`}><div><span className="ip-version">{version}</span><span className={`availability ${result.available ? 'yes' : 'no'}`}>{result.available ? '可用' : '不可用'}</span></div><strong className="network-address" title={result.address}><NetworkAddress value={result.address} fallback="未检测到公网地址"/></strong><p>{result.available ? `HTTPS 直连 · ${result.latencyMs} ms` : result.error || '连接失败'}</p></section> }
function NetworkAddress({ value, fallback = '' }: { value?: string; fallback?: string }) {
  if (!value) return <>{fallback}</>
  if (!value.includes(':')) return <>{value}</>
  return <>{value.split(':').map((segment, index) => <span key={`${index}-${segment}`}>{index > 0 && <>:<wbr/></>}{segment}</span>)}</>
}
function createSessionID(prefix: string) { return typeof crypto.randomUUID === 'function' ? crypto.randomUUID() : `${prefix}-${Date.now()}-${Math.random().toString(16).slice(2)}` }
function formatTraceLatency(value?: number) { if (typeof value !== 'number') return '—'; if (value < 1) return '<1 ms'; return `${value < 10 ? value.toFixed(1) : Math.round(value)} ms` }
function formatTraceDuration(value: number) { return value < 1000 ? `${value} ms` : `${(value / 1000).toFixed(value < 10000 ? 1 : 0)} 秒` }
function formatPercentage(value: number) { return `${value < 10 && value > 0 ? value.toFixed(1) : Math.round(value)}%` }
function splitLines(value: string) { return value.split(/\r?\n/).map(item => item.trim()).filter(Boolean) }
function formatLatencyTargets(values: LatencyTarget[]) { return values.map(item => `${item.name} | ${item.url} | ${item.region}`).join('\n') }
function parseLatencyTargets(value: string): LatencyTarget[] {
  return splitLines(value).map((line, index) => {
    const parts = line.split('|').map(item => item.trim())
    if (parts.length !== 3 || !parts[0] || !parts[1] || (parts[2] !== '国内' && parts[2] !== '国际')) {
      throw new Error(`网站响应测试第 ${index + 1} 行格式无效，应为：名称 | URL | 国内或国际`)
    }
    return {id: '', name: parts[0], url: parts[1], region: parts[2]}
  })
}
function safeHostname(value: string) { try { return new URL(value).hostname } catch { return value } }
function stripPort(value: string) { return value.replace(/:\d+$/, '') }
function formatBytes(value: number) { return value >= 1024 ? `${(value / 1024).toFixed(0)} KiB` : `${value} B` }
function errorMessage(error: unknown) { return error instanceof Error ? error.message : String(error) }

function loadTheme(): Theme {
  const stored = localStorage.getItem('theme')
  return stored === 'light' || stored === 'dark' ? stored : 'system'
}

export default App

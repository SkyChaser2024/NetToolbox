import { useEffect, useMemo, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { siBilibili, siGithub, siTaobao, siTelegram, siTiktok, siWechat, siX, siYoutube } from 'simple-icons'
import { EventsOn, WindowSetDarkTheme, WindowSetLightTheme, WindowSetSystemDefaultTheme } from '../wailsjs/runtime/runtime'
import { api, appVersion, defaultDiagnostics, defaultProfile } from './api'
import type {
  Adapter,
  AuthEvent,
  AuthRequest,
  AuthState,
  BootstrapData,
  IPv6Result,
  LatencyTarget,
  LatencyProbe,
  NATResult,
  NetworkInterface,
  OverviewResult,
  Profile,
  PublicNetworkInfo,
  SettingsResult,
} from './api'
import './App.css'

type Page = 'home' | 'auth' | 'nat' | 'ipv6' | 'settings'
type Theme = 'system' | 'light' | 'dark'

const pageMeta: Record<Page, { title: string }> = {
  home: { title: '网络概览' },
  auth: { title: '锐捷认证' },
  nat: { title: 'NAT 类型检测' },
  ipv6: { title: 'IPv6 连接测试' },
  settings: { title: '设置' },
}

const serviceIcons = { douyin: siTiktok, bilibili: siBilibili, wechat: siWechat, taobao: siTaobao, github: siGithub, telegram: siTelegram, x: siX, youtube: siYoutube }

const pendingLatencyProbes: LatencyProbe[] = defaultDiagnostics.latencyTargets.map(target => ({...target, host: new URL(target.url).hostname, status: 'pending'}))

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
  const [latencyRunning, setLatencyRunning] = useState<string[]>([])
  const [natResult, setNatResult] = useState<NATResult | null>(null)
  const [natRunning, setNatRunning] = useState(false)
  const [ipv6Result, setIPv6Result] = useState<IPv6Result | null>(null)
  const [ipv6Running, setIPv6Running] = useState(false)
  const [toast, setToast] = useState('')

  const refreshOverview = async () => {
    setOverviewRunning(true)
    try { setOverviewResult(await api().CheckOverview()) }
    catch (error) { setToast(errorMessage(error)) }
    finally { setOverviewRunning(false) }
  }

  const refreshLatency = async (id: string) => {
    if (latencyRunning.includes(id)) return
    setLatencyRunning(current => [...current, id])
    try {
      const probe = await api().CheckLatency(id)
      setOverviewResult(current => current ? {...current, probes: current.probes.map(item => item.id === id ? probe : item), checkedAt: new Date().toLocaleTimeString('zh-CN', {hour12: false})} : current)
    } catch (error) { setToast(errorMessage(error)) }
    finally { setLatencyRunning(current => current.filter(item => item !== id)) }
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
      if (data.cachedOverview) {
        setOverviewResult(data.cachedOverview)
        setOverviewRunning(false)
      } else {
        void refreshOverview()
      }
    }).catch(error => { setToast(errorMessage(error)); void refreshOverview() })
  }, [])

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
    if (!toast) return
    const timer = window.setTimeout(() => setToast(''), 5000)
    return () => window.clearTimeout(timer)
  }, [toast])

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
        <NavItem active={page === 'settings'} icon="settings" label="设置" onClick={() => setPage('settings')}/>
      </nav>
      <div className="sidebar-spacer"/>
      <div className="version-note">{bootstrap?.version || appVersion}</div>
    </aside>
    <main className="main-area">
      <header className="topbar"><h1>{meta.title}</h1><div className="top-actions">{page === 'auth' && <StatusPill state={authState}/>}<button className="icon-button" onClick={cycleTheme} title={`主题：${theme}`} aria-label="切换界面主题"><Icon name="moon"/></button></div></header>
      <div className="content-scroll">{page === 'home' ? <HomePage result={overviewResult} interfaces={bootstrap?.networkInterfaces || []} pendingProbes={bootstrap?.diagnostics.latencyTargets.map(target => ({...target, host: safeHostname(target.url), status: 'pending' as const})) || pendingLatencyProbes} running={overviewRunning} publicRunning={publicRunning} latencyRunning={latencyRunning} onRefresh={refreshOverview} onRefreshPublic={refreshPublicNetwork} onRefreshLatency={refreshLatency}/> : !bootstrap ? <LoadingCard text="正在读取网络环境…"/> : page === 'auth' ? <AuthPage data={bootstrap} state={authState} logs={logs} onAdapters={adapters => setBootstrap(current => current ? {...current, adapters} : current)} onProfile={profile => setBootstrap(current => current ? {...current, profile} : current)} onError={setToast}/> : page === 'nat' ? <NATPage result={natResult} setResult={setNatResult} running={natRunning} setRunning={setNatRunning} onError={setToast}/> : page === 'ipv6' ? <IPv6Page result={ipv6Result} setResult={setIPv6Result} running={ipv6Running} setRunning={setIPv6Running} onError={setToast}/> : <SettingsPage value={bootstrap} onSaved={result => setBootstrap(current => current ? {...current, ...result} : current)} onError={setToast}/>}</div>
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

function HomePage({ result, interfaces, pendingProbes, running, publicRunning, latencyRunning, onRefresh, onRefreshPublic, onRefreshLatency }: { result: OverviewResult | null; interfaces: NetworkInterface[]; pendingProbes: LatencyProbe[]; running: boolean; publicRunning: Array<'ipv4' | 'ipv6'>; latencyRunning: string[]; onRefresh: () => void; onRefreshPublic: (version: 'ipv4' | 'ipv6') => void; onRefreshLatency: (id: string) => void }) {
  const probes = result?.probes?.length ? result.probes : pendingProbes
  const domestic = probes.filter(item => item.region === '国内')
  const international = probes.filter(item => item.region === '国际')
  const initialLatencyRunning = running && !result
  const online = Boolean(result?.ipv4.available || result?.ipv6.available || result?.probes?.some(item => item.status === 'ok'))
  return <div className="home-page">
    <section className="card overview-hero">
      <div className={`overview-orb ${online ? 'online' : ''}`}><Icon name="activity" size={28}/></div>
      <div><span className="eyebrow">NETWORK OVERVIEW</span><h2>{running && !result ? '正在识别网络…' : online ? '网络连接正常' : '尚未获取公网信息'}</h2></div>
      <div className="overview-meta"><span><i className={online ? 'online' : ''}/>{online ? '公网可达' : '等待检测'}</span>{result?.checkedAt && <small>更新于 {result.checkedAt}</small>}</div>
      <button className="button primary" onClick={onRefresh} disabled={running}>{running ? <><span className="spinner"/>刷新中…</> : <><Icon name="refresh" size={17}/>刷新信息</>}</button>
    </section>

    <div className="public-network-grid"><PublicNetworkCard version="IPv4" info={result?.ipv4} loading={running && !result} refreshing={running || publicRunning.includes('ipv4')} onRefresh={() => onRefreshPublic('ipv4')}/><PublicNetworkCard version="IPv6" info={result?.ipv6} loading={running && !result} refreshing={running || publicRunning.includes('ipv6')} onRefresh={() => onRefreshPublic('ipv6')}/></div>

    <section className="card latency-section">
      <div className="section-title"><h3>网站延迟</h3></div>
      <LatencyGroup probes={domestic} running={latencyRunning} initialRunning={initialLatencyRunning} onRefresh={onRefreshLatency}/>
      <LatencyGroup probes={international} running={latencyRunning} initialRunning={initialLatencyRunning} onRefresh={onRefreshLatency}/>
    </section>

    <NetworkInterfaces items={interfaces}/>
  </div>
}

function PublicNetworkCard({ version, info, loading, refreshing, onRefresh }: { version: 'IPv4' | 'IPv6'; info?: PublicNetworkInfo; loading: boolean; refreshing: boolean; onRefresh: () => void }) {
  const location = [info?.country, info?.region, info?.city].filter(Boolean).join(' · ')
  const source = sourceHostname(info?.source)
  return <section className={`card public-network-card ${info?.available ? 'available' : ''}`}>
    <div className="public-card-head"><span className={`protocol-mark ${version.toLowerCase()}`}>{version}</span><div className="public-card-actions"><span className={`availability ${info?.available ? 'yes' : 'no'}`}>{loading ? '检测中' : info?.available ? '已连接' : '未获取'}</span><button type="button" className="public-refresh-button" onClick={onRefresh} disabled={refreshing} title={`刷新 ${version} 信息`} aria-label={`刷新 ${version} 信息`}>{refreshing ? <span className="spinner dark"/> : <Icon name="refresh" size={15}/>}</button></div></div>
    <strong>{loading ? '正在获取…' : info?.address || '未获取到公网地址'}</strong>
    <div className="public-facts"><div><span>ISP</span><b>{info?.isp || '—'}</b></div><div><span>ASN</span><b>{info?.asn ? `AS${info.asn}` : '—'}</b></div><div><span>网络</span><b>{info?.asnOrganization || '—'}</b></div><div><span>位置</span><b>{location || '—'}</b></div></div>
    {!loading && info?.error && <small className={`public-error ${info.available ? 'partial' : ''}`} title={info.error}>{info.error}</small>}
    {!loading && source && <div className="public-source" title={info?.source}><Icon name="globe" size={13}/><span>数据来源</span><b>{source}</b></div>}
  </section>
}

function sourceHostname(value?: string) {
  if (!value) return ''
  try { return new URL(value).hostname.replace(/^www\./i, '') }
  catch { return value }
}

function LatencyGroup({ probes, running, initialRunning, onRefresh }: { probes: LatencyProbe[]; running: string[]; initialRunning: boolean; onRefresh: (id: string) => void }) {
  return <div className="latency-group"><div className="latency-grid">{probes.map(probe => <LatencyCard key={probe.id} probe={probe} running={initialRunning || running.includes(probe.id)} onRefresh={() => onRefresh(probe.id)}/>)}</div></div>
}

function LatencyCard({ probe, running, onRefresh }: { probe: LatencyProbe; running: boolean; onRefresh: () => void }) {
  const latencyClass = probe.status === 'pending' ? 'pending' : probe.status !== 'ok' ? 'failed' : (probe.latencyMs || 0) < 100 ? 'fast' : (probe.latencyMs || 0) < 250 ? 'medium' : 'slow'
  return <button type="button" className={`latency-card ${latencyClass} ${running ? 'running' : ''}`} title={`${probe.url || probe.host}${probe.statusCode ? ` · HTTP ${probe.statusCode}` : ''}`} aria-label={`${probe.name}，${probe.region}，${running ? '正在测试' : '点击重新测试'}`} onClick={onRefresh} disabled={running}><ServiceIcon id={probe.id}/><div className="service-copy"><strong>{probe.name}</strong><span className={`region-label ${probe.region === '国内' ? 'domestic' : 'international'}`}>{probe.region}</span></div><div className="latency-value">{running ? <span className="latency-pending"><span className="spinner dark"/><span>测试中</span></span> : probe.status === 'pending' ? <span className="latency-waiting">待测试</span> : probe.status === 'ok' ? <><b>{probe.latencyMs}</b><small>ms</small></> : <b>{probe.status === 'timeout' ? 'TIMEOUT' : 'FAILED'}</b>}</div></button>
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
    <div className="adapter-grid">{visible.length ? visible.map(item => <div className={`adapter-card ${item.up ? 'up' : ''}`} key={`${item.index}-${item.name}`} title={item.description}><span className={`adapter-kind ${item.kind}`}><Icon name={item.kind === 'ethernet' ? 'network' : 'globe'} size={18}/></span><div><div className="adapter-title-row"><strong>{item.name}</strong>{item.up && <span className="adapter-online"><i/>在线</span>}</div><small>{networkInterfaceLabel(item)} · {item.up ? '已连接' : '未连接'}{item.linkSpeedMbps ? ` · ${item.linkSpeedMbps} Mbps` : ''}</small><code>{item.ipv4?.[0] || item.ipv6?.[0] || item.mac || '暂无地址'}</code></div><div className="adapter-metrics"><span>MAC {item.mac || '—'}</span><span>Metric {item.ipv4Metric || item.ipv6Metric || '自动'}</span></div></div>) : <div className="adapter-empty">当前筛选条件下没有网卡</div>}</div>
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
        <label className="field field-wide"><span>有线网卡</span><div className="input-row"><select value={profile.deviceName} onChange={event => chooseAdapter(event.target.value)} disabled={formDisabled}><option value="">请选择认证网卡</option>{ethernetAdapters.map(adapter => <option key={adapter.deviceName} value={adapter.deviceName}>{adapter.name}{adapter.mac ? ` · ${adapter.mac}` : ''}</option>)}</select><button type="button" className="square-button" onClick={refresh} disabled={refreshing || formDisabled} title="刷新网卡"><Icon name="refresh" size={18}/></button></div>{selectedAdapter && <small>{selectedAdapter.description || selectedAdapter.deviceName}{selectedAdapter.ipv4?.length ? ` · IPv4 ${selectedAdapter.ipv4[0]}` : ''}</small>}</label>
        <label className="field"><span>校园网账号</span><input value={profile.username} onChange={event => update('username', event.target.value)} autoComplete="username" placeholder="学号 / 用户名" disabled={formDisabled}/></label>
        <label className="field"><span>密码</span><input type="password" value={password} onChange={event => setPassword(event.target.value)} autoComplete="current-password" placeholder={profile.passwordSet ? '已安全保存，留空继续使用' : '请输入密码'} disabled={formDisabled}/></label>
        <label className="field field-wide"><span>网卡 MAC 地址</span><input value={profile.localMac} onChange={event => update('localMac', event.target.value)} placeholder="例如 12:34:56:78:9A:BC" disabled={formDisabled}/></label>
        <label className="check-row field-wide"><input type="checkbox" checked={profile.rememberPassword} onChange={event => update('rememberPassword', event.target.checked)} disabled={formDisabled}/><span>为当前 Windows 用户安全保存密码</span></label>
        <button type="button" className={`advanced-toggle field-wide ${advanced ? 'open' : ''}`} onClick={() => setAdvanced(value => !value)}><Icon name="chevron" size={17}/><span>高级适配选项</span><small>不同学校可能需要</small></button>
        {advanced && <div className="advanced-panel field-wide">
          <label className="field"><span>EAP Identity</span><input value={profile.identity} onChange={event => update('identity', event.target.value)} placeholder="留空时与账号一致" disabled={formDisabled}/></label>
          <label className="field"><span>Identity 后缀（Hex）</span><input value={profile.identitySuffix} onChange={event => update('identitySuffix', event.target.value)} placeholder="例如 00 00 13 11 00" disabled={formDisabled}/></label>
          <label className="field"><span>启动延迟</span><div className="unit-input"><input type="number" min="0" max="30000" value={profile.startDelayMs} onChange={event => update('startDelayMs', Number(event.target.value))} disabled={formDisabled}/><em>ms</em></div></label>
          <label className="field"><span>失败重试</span><div className="unit-input"><input type="number" min="0" max="60000" value={profile.retryDelayMs} onChange={event => update('retryDelayMs', Number(event.target.value))} disabled={formDisabled}/><em>ms</em></div></label>
          <label className="check-row compact"><input type="checkbox" checked={profile.debug} onChange={event => update('debug', event.target.checked)} disabled={formDisabled}/><span>记录原始报文调试信息</span></label>
        </div>}
        <div className="form-actions field-wide">{state === 'authenticated' ? <button type="button" className="button danger" onClick={logout}><Icon name="plug"/>注销</button> : authenticating ? <button type="button" className="button secondary" onClick={cancel}>取消认证</button> : <button type="submit" className="button primary" disabled={!data.npcapAvailable || !ethernetAdapters.length}><Icon name="shield"/>开始认证</button>}<span>{state === 'authenticated' ? '认证会话已结束，不会持续抓包或发送保活报文' : '应用将以管理员权限访问有线网卡'}</span></div>
      </form>
    </section>
    <section className="card session-card"><div className="section-title"><div><h3>认证会话</h3><p>最近的协议事件</p></div>{authenticating && <span className="live-indicator"><i/>LIVE</span>}</div><div className="timeline">{logs.length === 0 ? <div className="empty-state"><div><Icon name="activity" size={28}/></div><strong>{state === 'authenticated' ? '已检测到有线网络认证' : '等待开始认证'}</strong><p>{state === 'authenticated' ? '本次启动无需重复认证，需要断开时点击“注销”。' : '交换机的 Identity、Challenge 和结果会显示在这里。'}</p></div> : [...logs].reverse().map((log, index) => <div className={`log-row ${log.level}`} key={`${log.timestamp}-${index}`}><span className="log-node"/><div><p>{log.message}</p><time>{log.timestamp || '--:--:--'}</time></div></div>)}</div><div className="protocol-strip"><span>EAPOL</span><i/><span>Identity</span><i/><span>MD5</span><i/><span>Success</span></div></section>
  </div>
}

function NATPage({ result, setResult, running, setRunning, onError }: { result: NATResult | null; setResult: (value: NATResult | null) => void; running: boolean; setRunning: (value: boolean) => void; onError: (message: string) => void }) {
  const check = async () => { setRunning(true); setResult(null); try { setResult(await api().CheckNAT()) } catch (error) { onError(errorMessage(error)) } finally { setRunning(false) } }
  return <div className="tool-page"><section className="card tool-hero nat-hero"><div className="tool-icon"><Icon name="network" size={30}/></div><div><span className="eyebrow">STUN BEHAVIOUR DISCOVERY</span><h2>{result?.type || '你的网络属于哪种 NAT？'}</h2></div><button className="button primary" onClick={check} disabled={running}>{running ? <><span className="spinner"/>检测中…</> : <><Icon name="activity"/>开始检测</>}</button></section>
    {running && <ProgressSteps labels={['连接 STUN 节点', '比较公网映射', '验证过滤行为']}/>}
    {result && <>
      {result.status === 'error' && <div className="notice error"><Icon name="info"/><div><strong>检测未完成</strong><p>{result.error || result.summary}</p></div></div>}
      <div className="metrics-grid"><Metric label="NAT 类型" value={result.type || '未知'} accent/><Metric label="公网端点" value={result.publicIp ? `${result.publicIp}:${result.publicPort}` : '未获取'}/><Metric label="本地 IPv4" value={result.localIp || '未获取'}/><Metric label="首包延迟" value={result.latencyMs ? `${result.latencyMs} ms` : '—'}/></div>
      <section className="card detail-card"><div className="section-title"><div><h3>行为分析</h3><p>{result.rfc5780 ? '已完成 RFC 5780 映射与过滤测试' : '已使用多节点映射对比回退'}</p></div><span className={`quality-badge ${result.rfc5780 ? 'good' : 'partial'}`}>{result.rfc5780 ? '完整结果' : '保守结果'}</span></div><div className="behavior-list"><Behavior name="映射行为" value={result.mappingBehavior}/><Behavior name="过滤行为" value={result.filteringBehavior}/><Behavior name="响应节点" value={`${result.serverCount} / ${result.probes?.length || 0}`}/></div></section>
      <section className="card detail-card"><div className="section-title"><div><h3>STUN 节点明细</h3><p>保留全部节点的解析、映射、时延和失败原因</p></div><span className="section-badge">{result.probes?.length || 0} NODES</span></div><div className="result-table-wrap"><div className="result-table nat-results"><div className="result-table-head"><span>服务器</span><span>服务器 IP</span><span>外部映射</span><span>时延</span><span>状态</span></div>{result.probes?.map(probe => <div className={`result-table-row ${probe.error ? 'failed' : ''}`} key={probe.server}><code title={probe.server}>{stripPort(probe.server)}</code><code>{probe.serverIp || '—'}</code><code>{probe.endpoint || '—'}</code><time>{probe.latencyMs ? `${probe.latencyMs} ms` : '—'}</time><div className="result-status"><span className={`probe-status ${probe.error ? 'bad' : 'good'}`}>{probe.error ? '超时/失败' : '成功'}</span>{probe.error && <small title={probe.error}>{probe.error}</small>}</div></div>)}</div></div></section>
    </>}
    {!result && !running && <InfoTiles items={[['NAT1', '映射开放，P2P 兼容性最好'], ['NAT2 / NAT3', '锥形 NAT，有不同程度过滤'], ['NAT4', '对称映射，直连通常较困难']]}/>}
  </div>
}

function IPv6Page({ result, setResult, running, setRunning, onError }: { result: IPv6Result | null; setResult: (value: IPv6Result | null) => void; running: boolean; setRunning: (value: boolean) => void; onError: (message: string) => void }) {
  const check = async () => { setRunning(true); setResult(null); try { setResult(await api().CheckIPv6()) } catch (error) { onError(errorMessage(error)) } finally { setRunning(false) } }
  return <div className="tool-page"><section className="card tool-hero ipv6-hero"><div className="tool-icon"><Icon name="globe" size={30}/></div><div><span className="eyebrow">DUAL STACK CONNECTIVITY</span><h2>{result?.connectionType || 'IPv6 是否真正可用？'}</h2></div><button className="button primary" onClick={check} disabled={running}>{running ? <><span className="spinner"/>检测中…</> : <><Icon name="activity"/>开始测试</>}</button></section>
    {running && <ProgressSteps labels={['扫描本机地址', '并行测试双栈与常用网站', '汇总连接类型']}/>}
    {result && <><div className="ip-cards"><IPCard version="IPv4" result={result.ipv4}/><IPCard version="IPv6" result={result.ipv6}/></div><section className="card detail-card"><div className="section-title"><div><h3>本机 IPv6 环境</h3><p>拥有地址不代表公网出口一定可用</p></div><span className={`quality-badge ${result.ipv6.available ? 'good' : 'partial'}`}>{result.ipv6.available ? '公网可达' : '需要排查'}</span></div><div className="behavior-list"><Behavior name="全局单播地址" value={result.localGlobal.length ? result.localGlobal.join(' · ') : '未发现'}/><Behavior name="链路本地地址" value={result.localLink.length ? `${result.localLink.length} 个` : '未发现'}/><Behavior name="AAAA DNS 解析" value={result.dnsAAAA ? '正常' : '未通过'}/>{result.largePacket && <Behavior name="IPv6 数据端点" value={result.largePacket.available ? `正常 · ${result.largePacket.latencyMs} ms · ${formatBytes(result.largePacket.bytesRead || 0)}` : result.largePacket.error || '未通过'}/>}</div></section><section className="card detail-card"><div className="section-title"><div><h3>常用网站 IPv6</h3><p>强制通过 IPv6 访问，不受 IPv4 回退影响</p></div><span className="section-badge">LIVE WEB</span></div><div className="website-results">{result.sites?.map(site => <div className={`website-result ${site.available ? 'available' : 'failed'}`} key={site.url}><div className="website-name"><span className="probe-dot"/><strong>{site.host}</strong><small>{site.address || '未建立 IPv6 连接'}</small></div><time>{site.latencyMs ? `${site.latencyMs} ms` : '—'}</time><span>{site.statusCode ? `HTTP ${site.statusCode}` : '—'}</span><span className={`probe-status ${site.available ? 'good' : 'bad'}`}>{site.available ? '可访问' : '失败'}</span>{site.error && <small className="website-error" title={site.error}>{site.error}</small>}</div>)}</div></section></>}
    {!result && !running && <InfoTiles items={[['双栈', 'IPv4 和 IPv6 均可访问公网'], ['仅 IPv4', '常见配置，IPv6 出口未开通'], ['本地有 IPv6', '仍需公网直连成功才算可用']]}/>}
  </div>
}

function SettingsPage({ value, onSaved, onError }: { value: BootstrapData; onSaved: (value: SettingsResult) => void; onError: (message: string) => void }) {
  const [profile, setProfile] = useState(value.profile)
  const [system, setSystem] = useState(value.system)
  const [latencyTargets, setLatencyTargets] = useState(formatLatencyTargets(value.diagnostics.latencyTargets))
  const [natServers, setNatServers] = useState(value.diagnostics.natServers.join('\n'))
  const [ipv4Endpoints, setIPv4Endpoints] = useState(value.diagnostics.ipv4Endpoints.join('\n'))
  const [ipv6Endpoints, setIPv6Endpoints] = useState(value.diagnostics.ipv6Endpoints.join('\n'))
  const [ipv6Sites, setIPv6Sites] = useState(value.diagnostics.ipv6Sites.join('\n'))
  const [aaaaDomain, setAAAADomain] = useState(value.diagnostics.aaaaDomain)
  const [largeUrl, setLargeURL] = useState(value.diagnostics.ipv6LargeUrl)
  const [saveState, setSaveState] = useState<'idle' | 'pending' | 'saving' | 'saved' | 'error'>('idle')
  const [saveMessage, setSaveMessage] = useState('')
  const initialRender = useRef(true)
  const saveRevision = useRef(0)
  const saveQueue = useRef<Promise<void>>(Promise.resolve())
  const immediateSave = useRef(false)
  const onSavedRef = useRef(onSaved)
  const onErrorRef = useRef(onError)
  onSavedRef.current = onSaved
  onErrorRef.current = onError
  const updateProfile = <K extends keyof Profile>(key: K, fieldValue: Profile[K]) => setProfile(current => ({...current, [key]: fieldValue}))
  const updateSystem = (update: (current: typeof system) => typeof system) => { immediateSave.current = true; setSystem(update) }
  const ethernetAdapters = value.adapters.filter(item => item.kind === 'ethernet' && item.recommended)

  useEffect(() => {
    if (initialRender.current) {
      initialRender.current = false
      return
    }
    const revision = ++saveRevision.current
    const delay = immediateSave.current ? 0 : 700
    immediateSave.current = false
    setSaveState('pending')
    setSaveMessage('')
    const timer = window.setTimeout(() => {
      let request
      try {
        request = {
          profile,
          system,
          diagnostics: {
            latencyTargets: parseLatencyTargets(latencyTargets),
            natServers: splitLines(natServers),
            ipv4Endpoints: splitLines(ipv4Endpoints),
            ipv6Endpoints: splitLines(ipv6Endpoints),
            ipv6Sites: splitLines(ipv6Sites),
            aaaaDomain,
            ipv6LargeUrl: largeUrl,
          },
        }
      } catch (error) {
        if (revision === saveRevision.current) {
          setSaveState('error')
          setSaveMessage(errorMessage(error))
        }
        return
      }
      const persist = async () => {
        if (revision === saveRevision.current) setSaveState('saving')
        try {
          const stored = await api().SaveSettings(request)
          onSavedRef.current(stored)
          if (revision === saveRevision.current) {
            setSaveState('saved')
            setSaveMessage('')
          }
        } catch (error) {
          if (revision === saveRevision.current) {
            const message = errorMessage(error)
            setSaveState('error')
            setSaveMessage(message)
            onErrorRef.current(message)
          }
        }
      }
      saveQueue.current = saveQueue.current.then(persist, persist)
    }, delay)
    return () => window.clearTimeout(timer)
  }, [profile, system, latencyTargets, natServers, ipv4Endpoints, ipv6Endpoints, ipv6Sites, aaaaDomain, largeUrl])

  const saveLabel = saveState === 'saving' ? '正在自动保存…' : saveState === 'pending' ? '等待自动保存…' : saveState === 'saved' ? '已自动保存' : saveState === 'error' ? '自动保存失败' : '修改后自动保存'

  return <div className="settings-page">
    <section className="card settings-overview"><div className="brand-mark large"><Icon name="activity" size={31}/></div><div className="about-copy"><span className="eyebrow">NETWORK TOOLBOX</span><h2>网络工具箱</h2><div className="overview-facts"><span>Go + Wails</span><span>802.1X / EAP-MD5</span><span>Windows DPAPI</span></div></div><div className="settings-save-meta"><span className="version">Version {value.version}</span><span className={`autosave-status ${saveState}`} title={saveMessage}><i/>{saveLabel}</span></div></section>

    <details className="card settings-disclosure">
      <summary><div><span className="disclosure-icon blue"><Icon name="activity" size={19}/></span><span><strong>网络与启动</strong><small>{system.priorityMode === 'ethernet' ? '以太网优先' : system.priorityMode === 'wifi' ? 'Wi-Fi 优先' : '系统自动选择'} · 自动认证{system.autoAuthenticate ? '已开启' : '已关闭'}</small></span></div><span className="summary-side">配置 <Icon name="chevron" size={16}/></span></summary>
      <div className="disclosure-body"><div className="priority-options">
        {([['automatic', '系统自动', '由 Windows 自动选择'], ['ethernet', '以太网优先', '优先使用有线网络'], ['wifi', 'Wi-Fi 优先', '优先使用无线网络']] as const).map(([mode, title, description]) => <label className={`priority-option ${system.priorityMode === mode ? 'selected' : ''}`} key={mode}><input type="radio" name="priority" value={mode} checked={system.priorityMode === mode} onChange={() => updateSystem(current => ({...current, priorityMode: mode}))}/><span><strong>{title}</strong><small>{description}</small></span></label>)}
      </div>
      <label className="toggle-setting"><div><strong>登录后自动认证并监测断线</strong><p>此功能依赖托盘后台；开启时同时开启托盘驻留，每 30 秒检测一次，断线后最多认证 3 次。</p>{(!profile.passwordSet || !profile.username) && <small>请先在锐捷认证页保存账号和密码</small>}</div><input type="checkbox" checked={system.autoAuthenticate} disabled={!profile.passwordSet || !profile.username} onChange={event => updateSystem(current => ({...current, autoAuthenticate: event.target.checked, closeToTray: event.target.checked ? true : current.closeToTray}))}/><i/></label>
      <label className="toggle-setting"><div><strong>关闭窗口后驻留系统托盘</strong><p>关闭此项会同步关闭自动认证后台；关闭窗口将彻底退出程序。</p></div><input type="checkbox" checked={system.closeToTray} onChange={event => updateSystem(current => ({...current, closeToTray: event.target.checked, autoAuthenticate: event.target.checked ? current.autoAuthenticate : false}))}/><i/></label></div>
    </details>

    <details className="card settings-disclosure">
      <summary><div><span className="disclosure-icon blue"><Icon name="globe" size={19}/></span><span><strong>网站响应测试</strong><small>主页延迟卡片与公网 IPv4 / IPv6 查询网站</small></span></div><span className="summary-side">{splitLines(latencyTargets).length} 个网站 <Icon name="chevron" size={16}/></span></summary>
      <div className="disclosure-body">
        <div className="settings-heading compact-heading"><p>延迟网站每行格式：名称 | URL | 国内或国际。IPv4、IPv6 查询网站每行一个 URL，并强制使用对应协议连接。</p><button type="button" className="reset-button" onClick={() => { setLatencyTargets(formatLatencyTargets(defaultDiagnostics.latencyTargets)); setIPv4Endpoints(defaultDiagnostics.ipv4Endpoints.join('\n')); setIPv6Endpoints(defaultDiagnostics.ipv6Endpoints.join('\n')) }}>恢复默认</button></div>
        <label className="settings-field"><span>主页网站响应测试</span><textarea className="settings-textarea tall" spellCheck={false} value={latencyTargets} onChange={event => setLatencyTargets(event.target.value)} aria-label="网站响应测试列表"/></label>
        <div className="settings-grid protocol-endpoint-grid"><label className="settings-field"><span>IPv4 公网信息测试网站</span><textarea className="settings-textarea" spellCheck={false} value={ipv4Endpoints} onChange={event => setIPv4Endpoints(event.target.value)} aria-label="IPv4 公网信息测试网站"/></label><label className="settings-field"><span>IPv6 公网信息测试网站</span><textarea className="settings-textarea" spellCheck={false} value={ipv6Endpoints} onChange={event => setIPv6Endpoints(event.target.value)} aria-label="IPv6 公网信息测试网站"/></label></div>
      </div>
    </details>

    <details className="card settings-disclosure">
      <summary><div><span className="disclosure-icon blue"><Icon name="shield" size={19}/></span><span><strong>锐捷认证参数</strong><small>有线网卡、Identity 与最多三次重试</small></span></div><span className="summary-side">高级配置 <Icon name="chevron" size={16}/></span></summary>
      <div className="disclosure-body"><div className="settings-grid auth-settings-grid">
        <label className="settings-field"><span>默认以太网卡</span><select value={profile.deviceName} onChange={event => { const item = value.adapters.find(adapter => adapter.deviceName === event.target.value); setProfile(current => ({...current, deviceName: event.target.value, adapterLabel: item?.name || '', localMac: item?.mac || current.localMac})) }}><option value="">自动选择首个可用网卡</option>{ethernetAdapters.map(item => <option value={item.deviceName} key={item.deviceName}>{item.name} · {item.mac}</option>)}</select></label>
        <label className="settings-field"><span>EAP Identity</span><input value={profile.identity} onChange={event => updateProfile('identity', event.target.value)} placeholder="留空时与账号一致"/></label>
        <label className="settings-field"><span>Identity 后缀（Hex）</span><input value={profile.identitySuffix} onChange={event => updateProfile('identitySuffix', event.target.value)} placeholder="例如 00 00 13 11 00"/></label>
        <label className="settings-field"><span>启动延迟 <small>ms</small></span><input type="number" min="0" max="30000" value={profile.startDelayMs} onChange={event => updateProfile('startDelayMs', Number(event.target.value))}/></label>
        <label className="settings-field"><span>失败重试 <small>ms</small></span><input type="number" min="0" max="60000" value={profile.retryDelayMs} onChange={event => updateProfile('retryDelayMs', Number(event.target.value))}/></label>
      </div>
      <label className="plain-check"><input type="checkbox" checked={profile.debug} onChange={event => updateProfile('debug', event.target.checked)}/><span>记录协议调试信息（自动隐藏凭据载荷）</span></label></div>
    </details>

    <details className="card settings-disclosure">
      <summary><div><span className="disclosure-icon blue"><Icon name="network" size={19}/></span><span><strong>NAT 检测节点</strong><small>中国大陆优先的 STUN 服务列表</small></span></div><span className="summary-side">{splitLines(natServers).length} 个节点 <Icon name="chevron" size={16}/></span></summary>
      <div className="disclosure-body"><div className="settings-heading compact-heading"><p>每行一个 host:port；检测会依次尝试 RFC 5780，再使用多节点映射对比。</p><button type="button" className="reset-button" onClick={() => setNatServers(defaultDiagnostics.natServers.join('\n'))}>恢复默认</button></div><textarea className="settings-textarea tall" spellCheck={false} value={natServers} onChange={event => setNatServers(event.target.value)} aria-label="STUN 服务器列表"/></div>
    </details>

    <details className="card settings-disclosure">
      <summary><div><span className="disclosure-icon violet"><Icon name="globe" size={19}/></span><span><strong>IPv6 深度检测</strong><small>常用网站、AAAA 解析与数据端点</small></span></div><span className="summary-side">高级配置 <Icon name="chevron" size={16}/></span></summary>
      <div className="disclosure-body"><div className="settings-heading compact-heading"><p>常用网站测试会强制使用 IPv6，不会回退到 IPv4。</p><button type="button" className="reset-button" onClick={() => { setIPv6Sites(defaultDiagnostics.ipv6Sites.join('\n')); setAAAADomain(defaultDiagnostics.aaaaDomain); setLargeURL(defaultDiagnostics.ipv6LargeUrl) }}>恢复默认</button></div><div className="settings-grid"><label className="settings-field field-wide"><span>常用网站 IPv6 探测 <small>每行一个 URL</small></span><textarea className="settings-textarea" spellCheck={false} value={ipv6Sites} onChange={event => setIPv6Sites(event.target.value)}/></label><label className="settings-field"><span>AAAA 解析测试域名</span><input value={aaaaDomain} onChange={event => setAAAADomain(event.target.value)} placeholder="www.qq.com"/></label><label className="settings-field"><span>IPv6 数据端点 URL <small>可选</small></span><input value={largeUrl} onChange={event => setLargeURL(event.target.value)} placeholder="https://…"/></label></div></div>
    </details>

  </div>
}

function LoadingCard({ text }: { text: string }) { return <div className="loading-card"><span className="spinner dark"/><p>{text}</p></div> }
function Metric({ label, value, accent = false }: { label: string; value: string; accent?: boolean }) { return <div className={`card metric ${accent ? 'accent' : ''}`}><span>{label}</span><strong>{value}</strong></div> }
function Behavior({ name, value }: { name: string; value: string }) { return <div><span>{name}</span><strong>{value || '无法判断'}</strong></div> }
function ProgressSteps({ labels }: { labels: string[] }) { return <div className="card progress-steps">{labels.map((label, index) => <div key={label}><span>{index + 1}</span><p>{label}</p>{index < labels.length - 1 && <i/>}</div>)}</div> }
function InfoTiles({ items }: { items: string[][] }) { return <div className="info-tiles">{items.map(([title, text]) => <div className="card" key={title}><strong>{title}</strong><p>{text}</p></div>)}</div> }
function IPCard({ version, result }: { version: string; result: { available: boolean; address?: string; latencyMs?: number; error?: string } }) { return <section className={`card ip-card ${result.available ? 'available' : ''}`}><div><span className="ip-version">{version}</span><span className={`availability ${result.available ? 'yes' : 'no'}`}>{result.available ? '可用' : '不可用'}</span></div><strong>{result.address || '未检测到公网地址'}</strong><p>{result.available ? `HTTPS 直连 · ${result.latencyMs} ms` : result.error || '连接失败'}</p></section> }
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

import { useState } from 'react'
import { api, appVersion } from './api'
import { featureIcons } from './diagnosticVisuals'
import type { IPv6Result, NATResult } from './api'
import type { Page } from './appTypes'
import { Icon } from './components/Icon'
import { NavItem, StatusPill } from './components/Navigation'
import { LoadingCard } from './components/DiagnosticUI'
import { HomePage } from './features/overview/HomePage'
import { AuthPage } from './features/auth/AuthPage'
import { NATPage } from './features/nat/NATPage'
import { IPv6Page } from './features/ipv6/IPv6Page'
import { NetworkPathPage } from './features/path/NetworkPathPage'
import { SettingsPage } from './features/settings/SettingsPage'
import { useOverview } from './features/overview/useOverview'
import { usePing } from './features/path/usePing'
import { useTraceroute } from './features/path/useTraceroute'
import { useTheme } from './hooks/useTheme'
import { useAuthEvents } from './features/auth/useAuthEvents'
import { useToast } from './hooks/useToast'
import { useBootstrapSettings } from './hooks/useBootstrapSettings'
import { DiagnosticDraftProvider } from './hooks/DiagnosticDraftProvider'
import './App.css'

const pageMeta: Record<Page, { title: string }> = {
  home: { title: '网络概览' },
  auth: { title: '锐捷认证' },
  nat: { title: 'NAT 类型检测' },
  ipv6: { title: 'IPv6 连接测试' },
  trace: { title: 'Ping 与路由追踪' },
  settings: { title: '设置' }
}
function App() {
  const [page, setPage] = useState<Page>('home')
  const { theme, cycleTheme } = useTheme()
  const { toast, setToast } = useToast()
  const { bootstrap, loaded, updateBootstrap, saveSettings } = useBootstrapSettings(setToast)
  const { authState, logs } = useAuthEvents(bootstrap?.authState)
  const {
    overviewResult,
    overviewRunning,
    publicRunning,
    latencyPolling,
    configuredPendingProbes,
    refreshOverview,
    refreshPublicNetwork
  } = useOverview(bootstrap, loaded, page === 'home', setToast, updateBootstrap)
  const { ping, updatePing, startPing, stopPing } = usePing(page === 'trace')
  const { trace, updateTrace, startTraceroute, stopTraceroute } = useTraceroute(page === 'trace')
  const [natResult, setNatResult] = useState<NATResult | null>(null)
  const [natRunning, setNatRunning] = useState(false)
  const [ipv6Result, setIPv6Result] = useState<IPv6Result | null>(null)
  const [ipv6Running, setIPv6Running] = useState(false)
  const meta = pageMeta[page]

  const renderPage = () => {
    if (page === 'home')
      return (
        <HomePage
          result={overviewResult}
          interfaces={bootstrap?.networkInterfaces || []}
          pendingProbes={configuredPendingProbes}
          running={overviewRunning}
          publicRunning={publicRunning}
          latencyPolling={latencyPolling}
          diagnostics={bootstrap?.diagnostics}
          onRefresh={refreshOverview}
          onRefreshPublic={refreshPublicNetwork}
          onSaveSettings={saveSettings}
          onError={setToast}
        />
      )
    if (!bootstrap) return <LoadingCard text="正在读取网络环境…" />
    switch (page) {
      case 'auth':
        return (
          <AuthPage
            data={bootstrap}
            state={authState}
            logs={logs}
            onAdapters={(adapters) =>
              updateBootstrap((current) => (current ? { ...current, adapters } : current))
            }
            onProfile={(profile) =>
              updateBootstrap((current) => (current ? { ...current, profile } : current))
            }
            onError={setToast}
          />
        )
      case 'nat':
        return (
          <NATPage
            data={bootstrap}
            result={natResult}
            setResult={setNatResult}
            running={natRunning}
            setRunning={setNatRunning}
            onSaveSettings={saveSettings}
            onError={setToast}
          />
        )
      case 'ipv6':
        return (
          <IPv6Page
            data={bootstrap}
            result={ipv6Result}
            setResult={setIPv6Result}
            running={ipv6Running}
            setRunning={setIPv6Running}
            onSaveSettings={saveSettings}
            onError={setToast}
          />
        )
      case 'trace':
        return (
          <NetworkPathPage
            ping={ping}
            trace={trace}
            onPingChange={updatePing}
            onPingStart={startPing}
            onPingStop={stopPing}
            onTraceChange={updateTrace}
            onTraceStart={startTraceroute}
            onTraceStop={stopTraceroute}
          />
        )
      case 'settings':
        return (
          <SettingsPage
            value={bootstrap}
            onSaveSettings={saveSettings}
            onClearLocalData={() => api().ClearLocalData()}
            onError={setToast}
          />
        )
    }
  }

  return (
    <DiagnosticDraftProvider>
      <div className="app-shell">
        <aside className="sidebar">
          <div className="brand">
            <div className="brand-mark">
              <Icon name={featureIcons.brand} size={24} />
            </div>
            <div>
              <strong>网络工具箱</strong>
              <span>Network Toolbox</span>
            </div>
          </div>
          <nav className="nav-list" aria-label="主要功能">
            <NavItem
              active={page === 'home'}
              icon={featureIcons.overview}
              label="网络概览"
              onClick={() => setPage('home')}
            />
            <NavItem
              active={page === 'auth'}
              icon={featureIcons.auth}
              label="锐捷认证"
              onClick={() => setPage('auth')}
            />
            <NavItem
              active={page === 'nat'}
              icon={featureIcons.nat}
              label="NAT 检测"
              onClick={() => setPage('nat')}
            />
            <NavItem
              active={page === 'ipv6'}
              icon={featureIcons.ipv6}
              label="IPv6 测试"
              onClick={() => setPage('ipv6')}
            />
            <NavItem
              active={page === 'trace'}
              icon={featureIcons.trace}
              label="Ping / 路由"
              onClick={() => setPage('trace')}
            />
            <NavItem
              active={page === 'settings'}
              icon={featureIcons.settings}
              label="设置"
              onClick={() => setPage('settings')}
            />
          </nav>
          <div className="sidebar-spacer" />
          <div className="version-note" title={`网络工具箱 ${bootstrap?.version || appVersion}`}>
            <span className="version-note-mark">
              <Icon name="activity" size={13} />
            </span>
            <span className="version-note-copy">
              <small>VERSION</small>
              <strong>v{(bootstrap?.version || appVersion).replace(/^v/i, '')}</strong>
            </span>
          </div>
        </aside>
        <main className="main-area">
          <header className="topbar">
            <h1>{meta.title}</h1>
            <div className="top-actions">
              {page === 'auth' && <StatusPill state={authState} />}
              <button
                className="icon-button"
                onClick={cycleTheme}
                title={`主题：${theme}`}
                aria-label="切换界面主题"
              >
                <Icon name="moon" />
              </button>
            </div>
          </header>
          <div className="content-scroll">{renderPage()}</div>
          {toast && (
            <div className="toast-region">
              <div className="toast" role="alert">
                <Icon name="info" size={18} />
                <span>{toast}</span>
                <button onClick={() => setToast('')} aria-label="关闭提示">
                  ×
                </button>
              </div>
            </div>
          )}
        </main>
      </div>
    </DiagnosticDraftProvider>
  )
}
export default App

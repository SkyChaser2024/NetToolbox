import { diagnosticActionIcons } from '../../diagnosticVisuals'
import type { DiagnosticSettings, LatencyProbe, NetworkInterface, OverviewResult } from '../../api'
import type { SettingsSaver } from '../../appTypes'
import { Icon } from '../../components/Icon'
import { toolIcons } from '../../diagnosticIcons'
import { PublicNetworkCard } from './PublicNetworkCard'
import { LatencyGroup } from './LatencyCards'
import { NetworkInterfaces } from './NetworkInterfaces'
import { LatencySettings } from './OverviewSettings'

export function HomePage({
  result,
  interfaces,
  pendingProbes,
  running,
  publicRunning,
  latencyPolling,
  diagnostics,
  onRefresh,
  onRefreshPublic,
  onSaveSettings,
  onError
}: {
  result: OverviewResult | null
  interfaces: NetworkInterface[]
  pendingProbes: LatencyProbe[]
  running: boolean
  publicRunning: Array<'ipv4' | 'ipv6'>
  latencyPolling: boolean
  diagnostics?: DiagnosticSettings
  onRefresh: () => void
  onRefreshPublic: (version: 'ipv4' | 'ipv6') => void
  onSaveSettings: SettingsSaver
  onError: (message: string) => void
}) {
  const probes = result?.probes?.length ? result.probes : pendingProbes
  const domestic = probes.filter((item) => item.region === '国内')
  const international = probes.filter((item) => item.region === '国际')
  const online = Boolean(
    result?.ipv4.available ||
    result?.ipv6.available ||
    result?.probes?.some((item) => item.status === 'ok')
  )
  return (
    <div className="home-page">
      <section className="card overview-hero">
        <div className={`overview-orb ${online ? 'online' : ''}`}>
          <Icon name={toolIcons.home} size={28} />
        </div>
        <div>
          <span className="eyebrow">NETWORK OVERVIEW</span>
          <h2>
            {running && !result ? '正在识别网络…' : online ? '网络连接正常' : '尚未获取公网信息'}
          </h2>
        </div>
        <div className="overview-meta">
          <span>
            <i className={online ? 'online' : ''} />
            {online ? '公网可达' : '等待检测'}
          </span>
          {result?.checkedAt && <small>更新于 {result.checkedAt}</small>}
        </div>
        <button className="button primary" onClick={onRefresh} disabled={running}>
          {running ? (
            <>
              <span className="spinner" />
              刷新中…
            </>
          ) : (
            <>
              <Icon name={diagnosticActionIcons.overviewAction} size={17} />
              刷新信息
            </>
          )}
        </button>
      </section>

      <div className="public-network-grid">
        <PublicNetworkCard
          version="IPv4"
          info={result?.ipv4}
          loading={running && !result}
          refreshing={running || publicRunning.includes('ipv4')}
          endpoints={diagnostics?.ipv4Endpoints}
          onRefresh={() => onRefreshPublic('ipv4')}
          onSaveSettings={onSaveSettings}
          onError={onError}
        />
        <PublicNetworkCard
          version="IPv6"
          info={result?.ipv6}
          loading={running && !result}
          refreshing={running || publicRunning.includes('ipv6')}
          endpoints={diagnostics?.ipv6Endpoints}
          onRefresh={() => onRefreshPublic('ipv6')}
          onSaveSettings={onSaveSettings}
          onError={onError}
        />
      </div>

      <section className="card latency-section">
        <div className="section-title">
          <h3>网站延迟</h3>
        </div>
        <LatencyGroup probes={domestic} polling={latencyPolling} />
        <LatencyGroup probes={international} polling={latencyPolling} />
        {diagnostics && (
          <LatencySettings
            value={diagnostics.latencyTargets}
            onSaveSettings={onSaveSettings}
            onError={onError}
          />
        )}
      </section>

      <NetworkInterfaces items={interfaces} />
    </div>
  )
}

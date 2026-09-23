import type { PublicNetworkInfo } from '../../api'
import type { SettingsSaver } from '../../appTypes'
import { Icon } from '../../components/Icon'
import { NetworkAddress } from '../../components/DiagnosticUI'
import { PublicEndpointSettings } from './OverviewSettings'

export function PublicNetworkCard({
  version,
  info,
  loading,
  refreshing,
  endpoints,
  onRefresh,
  onSaveSettings,
  onError
}: {
  version: 'IPv4' | 'IPv6'
  info?: PublicNetworkInfo
  loading: boolean
  refreshing: boolean
  endpoints?: string[]
  onRefresh: () => void
  onSaveSettings: SettingsSaver
  onError: (message: string) => void
}) {
  const location = [info?.country, info?.region, info?.city].filter(Boolean).join(' · ')
  const source = sourceHostname(info?.source)
  return (
    <section className={`card public-network-card ${info?.available ? 'available' : ''}`}>
      <div className="public-card-head">
        <span className={`protocol-mark ${version.toLowerCase()}`}>{version}</span>
        <div className="public-card-actions">
          <span className={`availability ${info?.available ? 'yes' : 'no'}`}>
            {loading ? '检测中' : info?.available ? '已连接' : '未获取'}
          </span>
          <button
            type="button"
            className="public-refresh-button"
            onClick={onRefresh}
            disabled={refreshing}
            title={`刷新 ${version} 信息`}
            aria-label={`刷新 ${version} 信息`}
          >
            {refreshing ? <span className="spinner dark" /> : <Icon name="refresh" size={15} />}
          </button>
        </div>
      </div>
      <strong className="network-address" title={info?.address}>
        <NetworkAddress
          value={loading ? '' : info?.address}
          fallback={loading ? '正在获取…' : '未获取到公网地址'}
        />
      </strong>
      <div className="public-facts">
        <div>
          <span>ISP</span>
          <b>{info?.isp || '—'}</b>
        </div>
        <div>
          <span>ASN</span>
          <b>{info?.asn ? `AS${info.asn}` : '—'}</b>
        </div>
        <div>
          <span>网络</span>
          <b>{info?.asnOrganization || '—'}</b>
        </div>
        <div>
          <span>位置</span>
          <b>{location || '—'}</b>
        </div>
      </div>
      {!loading && info?.error && (
        <small className={`public-error ${info.available ? 'partial' : ''}`} title={info.error}>
          {info.error}
        </small>
      )}
      {!loading && source && (
        <div className="public-source" title={info?.source}>
          <Icon name="globe" size={13} />
          <span>数据来源</span>
          <b>{source}</b>
        </div>
      )}
      {endpoints && (
        <PublicEndpointSettings
          version={version}
          value={endpoints}
          onSaveSettings={onSaveSettings}
          onError={onError}
        />
      )}
    </section>
  )
}

function sourceHostname(value?: string) {
  if (!value) return ''
  try {
    return new URL(value).hostname.replace(/^www\./i, '')
  } catch {
    return value
  }
}

import { api, defaultDiagnostics } from '../../api'
import type { BootstrapData, DiagnosticSettings, IPv6Result } from '../../api'
import type { SettingsSaver } from '../../appTypes'
import { Icon } from '../../components/Icon'
import { Behavior, ProgressSteps, IPCard, NetworkAddress } from '../../components/DiagnosticUI'
import { useDiagnosticAutosave } from '../../hooks/useDiagnosticAutosave'
import { useDiagnosticDraft } from '../../hooks/DiagnosticDraftProvider'
import { formatBytes, errorMessage, splitLines } from '../../utils/format'
import { toolIcons } from '../../diagnosticIcons'

export function IPv6Page({
  data,
  result,
  setResult,
  running,
  setRunning,
  onSaveSettings,
  onError
}: {
  data: BootstrapData
  result: IPv6Result | null
  setResult: (value: IPv6Result | null) => void
  running: boolean
  setRunning: (value: boolean) => void
  onSaveSettings: SettingsSaver
  onError: (message: string) => void
}) {
  const check = async () => {
    setRunning(true)
    setResult(null)
    try {
      setResult(await api().CheckIPv6())
    } catch (error) {
      onError(errorMessage(error))
    } finally {
      setRunning(false)
    }
  }
  return (
    <div className="tool-page">
      <section className="card tool-hero integrated-tool-hero ipv6-hero">
        <div className="integrated-tool-heading">
          <div className="tool-icon">
            <Icon name={toolIcons.ipv6} size={30} />
          </div>
          <div>
            <span className="eyebrow">DUAL STACK CONNECTIVITY</span>
            <h2>{result?.connectionType || 'IPv6 是否真正可用？'}</h2>
          </div>
          <button className="button primary" onClick={check} disabled={running}>
            {running ? (
              <>
                <span className="spinner" />
                检测中…
              </>
            ) : (
              <>
                <Icon name={toolIcons.ipv6} />
                {result ? '再次测试' : '开始测试'}
              </>
            )}
          </button>
        </div>
        <IPv6Settings value={data.diagnostics} onSaveSettings={onSaveSettings} onError={onError} />
      </section>
      {running && (
        <ProgressSteps
          steps={[
            { title: '扫描本机', detail: '识别 IPv6 地址环境' },
            { title: '测试连通', detail: '并行访问双栈与网站' },
            { title: '汇总结果', detail: '判断公网连接类型' }
          ]}
        />
      )}
      {result && (
        <>
          <div className="ip-cards">
            <IPCard version="IPv4" result={result.ipv4} />
            <IPCard version="IPv6" result={result.ipv6} />
          </div>
          <section className="card detail-card">
            <div className="section-title">
              <div>
                <h3>本机 IPv6 环境</h3>
                <p>拥有地址不代表公网出口一定可用</p>
              </div>
              <span className={`quality-badge ${result.ipv6.available ? 'good' : 'partial'}`}>
                {result.ipv6.available ? '公网可达' : '需要排查'}
              </span>
            </div>
            <div className="behavior-list">
              <Behavior
                name="全局单播地址"
                value={result.localGlobal.length ? result.localGlobal.join(' · ') : '未发现'}
              />
              <Behavior
                name="链路本地地址"
                value={result.localLink.length ? `${result.localLink.length} 个` : '未发现'}
              />
              <Behavior name="AAAA DNS 解析" value={result.dnsAAAA ? '正常' : '未通过'} />
              {result.largePacket && (
                <Behavior
                  name="IPv6 数据端点"
                  value={
                    result.largePacket.available
                      ? `正常 · ${result.largePacket.latencyMs} ms · ${formatBytes(result.largePacket.bytesRead || 0)}`
                      : result.largePacket.error || '未通过'
                  }
                />
              )}
            </div>
          </section>
          <section className="card detail-card">
            <div className="section-title">
              <div>
                <h3>常用网站 IPv6</h3>
                <p>强制通过 IPv6 访问，不受 IPv4 回退影响</p>
              </div>
              <span className="section-badge">LIVE WEB</span>
            </div>
            <div className="website-results">
              {result.sites?.map((site) => (
                <div
                  className={`website-result ${site.available ? 'available' : 'failed'}`}
                  key={site.url}
                >
                  <div className="website-name">
                    <span className="probe-dot" />
                    <strong>{site.host}</strong>
                    <small className="network-address" title={site.address}>
                      <NetworkAddress value={site.address} fallback="未建立 IPv6 连接" />
                    </small>
                  </div>
                  <time>{site.latencyMs ? `${site.latencyMs} ms` : '—'}</time>
                  <span>{site.statusCode ? `HTTP ${site.statusCode}` : '—'}</span>
                  <span className={`probe-status ${site.available ? 'good' : 'bad'}`}>
                    {site.available ? '可访问' : '失败'}
                  </span>
                  {site.error && (
                    <small className="website-error" title={site.error}>
                      {site.error}
                    </small>
                  )}
                </div>
              ))}
            </div>
          </section>
        </>
      )}
    </div>
  )
}

export function IPv6Settings({
  value,
  onSaveSettings,
  onError
}: {
  value: DiagnosticSettings
  onSaveSettings: SettingsSaver
  onError: (message: string) => void
}) {
  const [ipv6Sites, setIPv6Sites] = useDiagnosticDraft('ipv6Sites', value.ipv6Sites.join('\n'))
  const [aaaaDomain, setAAAADomain] = useDiagnosticDraft('aaaaDomain', value.aaaaDomain)
  const [largeUrl, setLargeURL] = useDiagnosticDraft('ipv6LargeUrl', value.ipv6LargeUrl)
  const draftKey = JSON.stringify([ipv6Sites, aaaaDomain, largeUrl])
  useDiagnosticAutosave(
    draftKey,
    (current) => ({
      ...current,
      ipv6Sites: splitLines(ipv6Sites),
      aaaaDomain,
      ipv6LargeUrl: largeUrl
    }),
    onSaveSettings,
    onError,
    'ipv6Settings'
  )
  return (
    <details className="inline-tool-settings">
      <summary>
        <span>高级设置</span>
        <Icon name="chevron" size={14} />
      </summary>
      <div className="inline-settings-body">
        <div className="settings-heading compact-heading">
          <p>网站访问会强制使用 IPv6，不会回退到 IPv4。</p>
          <button
            type="button"
            className="reset-button"
            onClick={() => {
              setIPv6Sites(defaultDiagnostics.ipv6Sites.join('\n'))
              setAAAADomain(defaultDiagnostics.aaaaDomain)
              setLargeURL(defaultDiagnostics.ipv6LargeUrl)
            }}
          >
            恢复默认
          </button>
        </div>
        <div className="settings-grid">
          <label className="settings-field field-wide">
            <span>
              常用网站 <small>每行一个 URL</small>
            </span>
            <textarea
              className="settings-textarea compact"
              spellCheck={false}
              value={ipv6Sites}
              onChange={(event) => setIPv6Sites(event.target.value)}
            />
          </label>
          <label className="settings-field">
            <span>AAAA 测试域名</span>
            <input
              value={aaaaDomain}
              onChange={(event) => setAAAADomain(event.target.value)}
              placeholder="www.qq.com"
            />
          </label>
          <label className="settings-field">
            <span>
              数据端点 URL <small>可选</small>
            </span>
            <input
              value={largeUrl}
              onChange={(event) => setLargeURL(event.target.value)}
              placeholder="https://…"
            />
          </label>
        </div>
      </div>
    </details>
  )
}

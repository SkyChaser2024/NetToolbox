import { api, defaultDiagnostics } from '../../api'
import type { BootstrapData, DiagnosticSettings, NATResult } from '../../api'
import type { SettingsSaver } from '../../appTypes'
import { Icon } from '../../components/Icon'
import { Metric, Behavior, ProgressSteps } from '../../components/DiagnosticUI'
import { useDiagnosticAutosave } from '../../hooks/useDiagnosticAutosave'
import { useDiagnosticDraft } from '../../hooks/DiagnosticDraftProvider'
import { stripPort, errorMessage, splitLines } from '../../utils/format'
import { toolIcons } from '../../diagnosticIcons'

export function NATPage({
  data,
  result,
  setResult,
  running,
  setRunning,
  onSaveSettings,
  onError
}: {
  data: BootstrapData
  result: NATResult | null
  setResult: (value: NATResult | null) => void
  running: boolean
  setRunning: (value: boolean) => void
  onSaveSettings: SettingsSaver
  onError: (message: string) => void
}) {
  const check = async () => {
    setRunning(true)
    setResult(null)
    try {
      setResult(await api().CheckNAT())
    } catch (error) {
      onError(errorMessage(error))
    } finally {
      setRunning(false)
    }
  }
  return (
    <div className="tool-page">
      <section className="card tool-hero integrated-tool-hero nat-hero">
        <div className="integrated-tool-heading">
          <div className="tool-icon">
            <Icon name={toolIcons.nat} size={30} />
          </div>
          <div>
            <span className="eyebrow">STUN BEHAVIOUR DISCOVERY</span>
            <h2>{result?.type || '你的网络属于哪种 NAT？'}</h2>
          </div>
          <button className="button primary" onClick={check} disabled={running}>
            {running ? (
              <>
                <span className="spinner" />
                检测中…
              </>
            ) : (
              <>
                <Icon name={toolIcons.nat} />
                开始检测
              </>
            )}
          </button>
        </div>
        <NATSettings value={data.diagnostics} onSaveSettings={onSaveSettings} onError={onError} />
      </section>
      {running && (
        <ProgressSteps
          steps={[
            { title: '连接节点', detail: '探测可用 STUN 服务' },
            { title: '比较映射', detail: '分析公网端点变化' },
            { title: '验证类型', detail: '汇总映射与过滤行为' }
          ]}
        />
      )}
      {result && (
        <>
          {result.status === 'error' && (
            <div className="notice error">
              <Icon name="info" />
              <div>
                <strong>检测未完成</strong>
                <p>{result.error || result.summary}</p>
              </div>
            </div>
          )}
          <div className="metrics-grid">
            <Metric label="NAT 类型" value={result.type || '未知'} accent />
            <Metric
              label="公网端点"
              value={result.publicIp ? `${result.publicIp}:${result.publicPort}` : '未获取'}
            />
            <Metric label="本地 IPv4" value={result.localIp || '未获取'} />
            <Metric label="首包延迟" value={result.latencyMs ? `${result.latencyMs} ms` : '—'} />
          </div>
          <section className="card detail-card">
            <div className="section-title">
              <div>
                <h3>行为分析</h3>
                <p>
                  {result.rfc5780 ? '已完成 RFC 5780 映射与过滤测试' : '已使用多节点映射对比回退'}
                </p>
              </div>
              <span className={`quality-badge ${result.rfc5780 ? 'good' : 'partial'}`}>
                {result.rfc5780 ? '完整结果' : '保守结果'}
              </span>
            </div>
            <div className="behavior-list">
              <Behavior name="映射行为" value={result.mappingBehavior} />
              <Behavior name="过滤行为" value={result.filteringBehavior} />
              <Behavior
                name="响应节点"
                value={`${result.serverCount} / ${result.probes?.length || 0}`}
              />
            </div>
          </section>
          <section className="card detail-card">
            <div className="section-title">
              <div>
                <h3>STUN 节点明细</h3>
                <p>保留全部节点的解析、映射、时延和失败原因</p>
              </div>
              <span className="section-badge">{result.probes?.length || 0} NODES</span>
            </div>
            <div className="result-table-wrap">
              <div className="result-table nat-results" role="table" aria-label="STUN 节点检测结果">
                <div className="result-table-head" role="row">
                  <span role="columnheader">服务器</span>
                  <span role="columnheader">服务器 IP</span>
                  <span role="columnheader">外部映射</span>
                  <span role="columnheader">时延</span>
                  <span role="columnheader">状态</span>
                </div>
                {result.probes?.map((probe) => (
                  <div
                    className={`result-table-row ${probe.error ? 'failed' : ''}`}
                    key={probe.server}
                    role="row"
                  >
                    <code title={probe.server} role="cell">
                      {stripPort(probe.server)}
                    </code>
                    <code role="cell">{probe.serverIp || '—'}</code>
                    <code role="cell">{probe.endpoint || '—'}</code>
                    <time role="cell">{probe.latencyMs ? `${probe.latencyMs} ms` : '—'}</time>
                    <div className="result-status" role="cell">
                      <span className={`probe-status ${probe.error ? 'bad' : 'good'}`}>
                        {probe.error ? '超时/失败' : '成功'}
                      </span>
                      {probe.error && <small title={probe.error}>{probe.error}</small>}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </section>
        </>
      )}
    </div>
  )
}

export function NATSettings({
  value,
  onSaveSettings,
  onError
}: {
  value: DiagnosticSettings
  onSaveSettings: SettingsSaver
  onError: (message: string) => void
}) {
  const [natServers, setNatServers] = useDiagnosticDraft('natServers', value.natServers.join('\n'))
  useDiagnosticAutosave(
    natServers,
    (current) => ({ ...current, natServers: splitLines(natServers) }),
    onSaveSettings,
    onError,
    'natServers'
  )
  return (
    <details className="inline-tool-settings">
      <summary>
        <span>检测节点</span>
        <Icon name="chevron" size={14} />
      </summary>
      <div className="inline-settings-body">
        <div className="settings-heading compact-heading">
          <p>每行一个 host:port；至少保留两个节点，以便比较公网映射。</p>
          <button
            type="button"
            className="reset-button"
            onClick={() => setNatServers(defaultDiagnostics.natServers.join('\n'))}
          >
            恢复默认
          </button>
        </div>
        <textarea
          className="settings-textarea compact"
          spellCheck={false}
          value={natServers}
          onChange={(event) => setNatServers(event.target.value)}
          aria-label="STUN 服务器列表"
        />
      </div>
    </details>
  )
}

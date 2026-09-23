import type { FormEvent } from 'react'
import { latencyTone, traceConnectorEdges } from '../../diagnosticVisuals'
import type { TraceHop, TraceProtocol } from '../../api'
import { Icon } from '../../components/Icon'
import { NetworkAddress } from '../../components/DiagnosticUI'
import type { TraceViewState } from './types'
import { formatTraceLatency, formatTraceDuration } from '../../utils/format'
import { toolIcons } from '../../diagnosticIcons'

export function TracerouteTool({
  value,
  onChange,
  onStart,
  onStop
}: {
  value: TraceViewState
  onChange: (values: Partial<TraceViewState>) => void
  onStart: () => void
  onStop: () => void
}) {
  const starting = value.phase === 'starting'
  const probing = value.phase === 'resolving' || value.phase === 'running'
  const stopping = value.phase === 'stopping'
  const active = starting || probing || stopping
  const submit = (event: FormEvent) => {
    event.preventDefault()
    if (!active) onStart()
  }
  const statusLabel = starting
    ? '正在启动追踪'
    : value.phase === 'resolving'
      ? '正在解析目标'
      : value.phase === 'running'
        ? `正在追踪第 ${value.hops.length + 1} 跳`
        : stopping
          ? '正在停止追踪'
          : value.phase === 'cancelled'
            ? '已停止'
            : value.phase === 'error'
              ? '追踪失败'
              : value.resultStatus === 'reached'
                ? '已到达目标'
                : value.resultStatus === 'unreachable'
                  ? '目标不可达'
                  : value.resultStatus === 'max_hops'
                    ? '已达到最大跳数'
                    : '准备就绪'
  return (
    <div className="diagnostic-tool trace-tool">
      <section className="card trace-hero">
        <div className="trace-heading">
          <div className="tool-icon trace-icon">
            <Icon name={toolIcons.trace} size={30} />
          </div>
          <div>
            <span className="eyebrow">ROUTE TRACE</span>
            <h2>逐跳定位网络路径</h2>
          </div>
        </div>
        <form className="trace-form" onSubmit={submit}>
          <label
            className={`trace-target ${value.error && value.phase === 'idle' ? 'invalid' : ''}`}
          >
            <span className="sr-only">追踪目标</span>
            <input
              value={value.target}
              onChange={(event) => onChange({ target: event.target.value, error: '' })}
              placeholder="baidu.com（默认）或 8.8.8.8"
              disabled={active}
              aria-invalid={Boolean(value.error && value.phase === 'idle')}
            />
          </label>
          <select
            className="trace-protocol"
            aria-label="选择路由追踪协议"
            value={value.protocol}
            onChange={(event) => onChange({ protocol: event.target.value as TraceProtocol })}
            disabled={active}
          >
            <option value="auto">自动</option>
            <option value="ipv4">IPv4</option>
            <option value="ipv6">IPv6</option>
          </select>
          {starting ? (
            <button type="button" className="button primary" disabled>
              <span className="spinner" />
              正在启动…
            </button>
          ) : stopping ? (
            <button type="button" className="button trace-stop" disabled>
              <span className="spinner" />
              正在停止…
            </button>
          ) : probing ? (
            <button type="button" className="button trace-stop" onClick={onStop}>
              <span className="trace-stop-square" />
              停止追踪
            </button>
          ) : (
            <button type="submit" className="button primary">
              <Icon name={toolIcons.trace} size={17} />
              {value.hops.length ? '再次追踪' : '开始追踪'}
            </button>
          )}
        </form>
        {value.error && value.phase === 'idle' && (
          <small className="trace-input-error">{value.error}</small>
        )}
        <details
          className="trace-advanced"
          open={value.advancedOpen}
          onToggle={(event) => onChange({ advancedOpen: event.currentTarget.open })}
        >
          <summary>
            <span>高级设置</span>
            <Icon name="chevron" size={14} />
          </summary>
          <div className="trace-advanced-grid">
            <label>
              <span>最大跳数</span>
              <input
                type="number"
                min="1"
                max="64"
                value={value.maxHops}
                onChange={(event) => onChange({ maxHops: Number(event.target.value) })}
                onBlur={() => onChange({ maxHops: Math.min(64, Math.max(1, value.maxHops || 1)) })}
                disabled={active}
              />
            </label>
            <label>
              <span>每跳超时</span>
              <div className="unit-input">
                <input
                  type="number"
                  min="200"
                  max="5000"
                  step="100"
                  value={value.timeoutMs}
                  onChange={(event) => onChange({ timeoutMs: Number(event.target.value) })}
                  onBlur={() =>
                    onChange({ timeoutMs: Math.min(5000, Math.max(200, value.timeoutMs || 200)) })
                  }
                  disabled={active}
                />
                <em>ms</em>
              </div>
            </label>
            <label className="trace-dns-toggle">
              <input
                type="checkbox"
                checked={value.resolveHostnames}
                onChange={(event) => onChange({ resolveHostnames: event.target.checked })}
                disabled={active}
              />
              <span>
                <strong>解析主机名</strong>
                <small>为响应节点执行反向 DNS 查询</small>
              </span>
            </label>
          </div>
        </details>
      </section>

      {value.phase !== 'idle' && (
        <section className="card trace-summary">
          <div className="trace-summary-target">
            <span>目标</span>
            <strong>{value.submittedTarget || '—'}</strong>
            {value.resolvedTarget && (
              <code>
                <NetworkAddress value={value.resolvedTarget} />
              </code>
            )}
          </div>
          <div className="trace-summary-facts">
            <span>{value.actualProtocol ? value.actualProtocol.toUpperCase() : '解析中'}</span>
            <span>{value.hops.length} 跳</span>
            <span>{value.durationMs ? formatTraceDuration(value.durationMs) : '实时'}</span>
          </div>
          <span className={`trace-run-status ${value.phase} ${value.resultStatus || ''}`}>
            {active && <span className="spinner dark" />}
            {statusLabel}
          </span>
        </section>
      )}

      {value.error && value.phase !== 'idle' && (
        <div className="notice error trace-error">
          <Icon name="info" />
          <div>
            <strong>路由追踪未完成</strong>
            <p>{value.error}</p>
          </div>
        </div>
      )}

      {(value.hops.length > 0 || probing || stopping) && (
        <section className="card trace-results">
          <div className="section-title trace-results-title">
            <div>
              <h3>路由节点</h3>
              <p>每跳发送 3 次 ICMP 探测；单跳超时不代表链路一定中断</p>
            </div>
            <span className="section-badge">{value.hops.length} HOPS</span>
          </div>
          <div
            className="diagnostic-table-scroll"
            role="region"
            aria-label="路由追踪结果区域"
            tabIndex={0}
          >
            <div className="trace-table" role="table" aria-label="路由追踪结果">
              <div className="trace-table-head" role="row">
                <span role="columnheader">跳数</span>
                <span role="columnheader">路由节点</span>
                <span role="columnheader">探测 1</span>
                <span role="columnheader">探测 2</span>
                <span role="columnheader">探测 3</span>
                <span role="columnheader">平均</span>
                <span role="columnheader">状态</span>
              </div>
              {value.hops.map((hop, index) => (
                <TraceHopRow
                  hop={hop}
                  connector={traceConnectorEdges(index, value.hops.length, active)}
                  key={hop.number}
                />
              ))}
            </div>
            {probing && (
              <div className="trace-waiting" role="status" aria-live="polite">
                <span className="spinner dark" />
                <span>
                  {value.phase === 'resolving'
                    ? '正在解析目标地址…'
                    : `正在等待第 ${value.hops.length + 1} 跳响应…`}
                </span>
              </div>
            )}
          </div>
        </section>
      )}
    </div>
  )
}

export function TraceHopRow({
  hop,
  connector
}: {
  hop: TraceHop
  connector: { incoming: boolean; outgoing: boolean }
}) {
  const nodes = Array.from(
    new Map(
      hop.probes
        .filter((probe) => probe.address)
        .map((probe) => [
          probe.address,
          { address: probe.address || '', hostname: probe.hostname || '' }
        ])
    ).values()
  )
  const statusText =
    hop.status === 'destination'
      ? '目标节点'
      : hop.status === 'unreachable'
        ? '不可达'
        : hop.status === 'timeout'
          ? '请求超时'
          : '正常'
  return (
    <div
      className={`trace-hop-row ${hop.status} ${connector.incoming ? 'has-incoming' : ''} ${connector.outgoing ? 'has-outgoing' : ''}`}
      role="row"
    >
      <div className="trace-hop-number" role="cell">
        <i />
        <b>{hop.number}</b>
      </div>
      <div className="trace-hop-nodes" role="cell">
        {nodes.length ? (
          nodes.map((node) => (
            <div className="trace-node" key={node.address}>
              {node.hostname && <strong title={node.hostname}>{node.hostname}</strong>}
              <code title={node.address}>
                <NetworkAddress value={node.address} />
              </code>
            </div>
          ))
        ) : (
          <span>未响应</span>
        )}
      </div>
      {Array.from({ length: 3 }, (_, index) => {
        const probe = hop.probes[index]
        return (
          <span
            className={`trace-probe-time ${probe?.status || 'timeout'} ${latencyTone(probe?.latencyMs)}`}
            title={probe?.address || '请求超时'}
            role="cell"
            key={index}
          >
            {probe?.status !== 'timeout' ? formatTraceLatency(probe?.latencyMs) : '*'}
          </span>
        )
      })}
      <strong className={`trace-average ${latencyTone(hop.averageMs)}`} role="cell">
        {typeof hop.averageMs === 'number' ? formatTraceLatency(hop.averageMs) : '—'}
      </strong>
      <span className={`trace-hop-status ${hop.status}`} role="cell">
        {statusText}
      </span>
    </div>
  )
}

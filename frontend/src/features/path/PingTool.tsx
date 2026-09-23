import type { FormEvent } from 'react'
import { diagnosticActionIcons, latencyTone, pingLossPresentation } from '../../diagnosticVisuals'
import type { PingReply, TraceProtocol } from '../../api'
import { Icon } from '../../components/Icon'
import { NetworkAddress } from '../../components/DiagnosticUI'
import type { PingViewState } from './types'
import { formatTraceLatency, formatPercentage } from '../../utils/format'
import { toolIcons } from '../../diagnosticIcons'

export function PingTool({
  value,
  onChange,
  onStart,
  onStop
}: {
  value: PingViewState
  onChange: (values: Partial<PingViewState>) => void
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
  const received =
    value.summary?.received ?? value.replies.filter((reply) => reply.status === 'reply').length
  const sent = value.summary?.sent ?? value.replies.length
  const lost = value.summary?.lost ?? Math.max(0, sent - received)
  const lossPercent = value.summary?.lossPercent ?? (sent ? (lost / sent) * 100 : 0)
  const lossPresentation = pingLossPresentation(sent, lost)
  const latencies = value.replies.flatMap((reply) =>
    reply.status === 'reply' && typeof reply.latencyMs === 'number' ? [reply.latencyMs] : []
  )
  const minimum = received ? (value.summary?.minMs ?? Math.min(...latencies)) : undefined
  const average = received
    ? (value.summary?.averageMs ??
      latencies.reduce((total, item) => total + item, 0) / latencies.length)
    : undefined
  const maximum = received ? (value.summary?.maxMs ?? Math.max(...latencies)) : undefined
  const statusLabel = starting
    ? '正在启动 Ping'
    : value.phase === 'resolving'
      ? '正在解析目标'
      : value.phase === 'running'
        ? `正在探测 ${value.replies.length}/${value.count}`
        : stopping
          ? '正在停止 Ping'
          : value.phase === 'cancelled'
            ? '已停止'
            : value.phase === 'error'
              ? 'Ping 失败'
              : value.phase === 'completed' && lossPercent >= 100
                ? '目标未响应'
                : value.phase === 'completed' && lost
                  ? '测试完成，有丢包'
                  : value.phase === 'completed'
                    ? '测试完成'
                    : '准备就绪'

  return (
    <div className="diagnostic-tool ping-tool">
      <section className="card trace-hero ping-hero">
        <div className="trace-heading">
          <div className="tool-icon ping-icon">
            <Icon name={toolIcons.ping} size={30} />
          </div>
          <div>
            <span className="eyebrow">ICMP PING</span>
            <h2>检测连通性与延迟</h2>
          </div>
        </div>
        <form className="trace-form" onSubmit={submit}>
          <label
            className={`trace-target ${value.error && value.phase === 'idle' ? 'invalid' : ''}`}
          >
            <span className="sr-only">Ping 目标</span>
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
            aria-label="选择 Ping 协议"
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
              停止 Ping
            </button>
          ) : (
            <button type="submit" className="button primary">
              <Icon name={diagnosticActionIcons.pingAction} size={17} />
              {value.replies.length || value.summary ? '再次 Ping' : '开始 Ping'}
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
          <div className="trace-advanced-grid ping-advanced-grid">
            <label>
              <span>探测次数</span>
              <input
                type="number"
                min="1"
                max="100"
                value={value.count}
                onChange={(event) => onChange({ count: Number(event.target.value) })}
                onBlur={() => onChange({ count: Math.min(100, Math.max(1, value.count || 1)) })}
                disabled={active}
              />
            </label>
            <label>
              <span>单次超时</span>
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
            <label>
              <span>发送间隔</span>
              <div className="unit-input">
                <input
                  type="number"
                  min="100"
                  max="10000"
                  step="100"
                  value={value.intervalMs}
                  onChange={(event) => onChange({ intervalMs: Number(event.target.value) })}
                  onBlur={() =>
                    onChange({
                      intervalMs: Math.min(10000, Math.max(100, value.intervalMs || 100))
                    })
                  }
                  disabled={active}
                />
                <em>ms</em>
              </div>
            </label>
          </div>
        </details>
      </section>

      {value.phase !== 'idle' && (
        <section className="card trace-summary ping-summary">
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
            <span>{sent} 已发送</span>
            <span>{received} 已接收</span>
          </div>
          <span
            className={`trace-run-status ${value.phase} ${value.phase === 'completed' ? (lossPercent >= 100 ? 'unreachable' : lossPercent > 0 ? 'partial' : 'reached') : ''}`}
          >
            {active && <span className="spinner dark" />}
            {statusLabel}
          </span>
        </section>
      )}

      {value.error && value.phase !== 'idle' && (
        <div className="notice error trace-error">
          <Icon name="info" />
          <div>
            <strong>Ping 测试未完成</strong>
            <p>{value.error}</p>
          </div>
        </div>
      )}

      {(value.replies.length > 0 || value.summary || probing || stopping) && (
        <section className="card ping-results">
          <div className="section-title trace-results-title">
            <div>
              <h3>Ping 结果</h3>
              <p>ICMP 超时可能由防火墙过滤引起，不一定代表网站无法访问</p>
            </div>
            <span className="section-badge">
              {sent}/{value.count} PACKETS
            </span>
          </div>
          <div className="ping-metrics" aria-label="Ping 统计">
            <div className={`ping-metric ${lossPresentation.tone}`}>
              <span>
                丢包率
                <i />
              </span>
              <strong>{formatPercentage(lossPercent)}</strong>
              <small>{lossPresentation.detail}</small>
            </div>
            <div className={`ping-metric ${latencyTone(minimum)}`}>
              <span>
                最小延迟
                <i />
              </span>
              <strong>{formatTraceLatency(minimum)}</strong>
              <small>最快响应</small>
            </div>
            <div className={`ping-metric ${latencyTone(average)}`}>
              <span>
                平均延迟
                <i />
              </span>
              <strong>{formatTraceLatency(average)}</strong>
              <small>整体水平</small>
            </div>
            <div className={`ping-metric ${latencyTone(maximum)}`}>
              <span>
                最大延迟
                <i />
              </span>
              <strong>{formatTraceLatency(maximum)}</strong>
              <small>最慢响应</small>
            </div>
          </div>
          <div
            className="diagnostic-table-scroll"
            role="region"
            aria-label="Ping 逐包结果区域"
            tabIndex={0}
          >
            <div className="ping-table" role="table" aria-label="Ping 逐包结果">
              <div className="ping-table-head" role="row">
                <span role="columnheader">序号</span>
                <span role="columnheader">结果</span>
                <span role="columnheader">响应地址</span>
                <span role="columnheader">延迟</span>
              </div>
              {value.replies.map((reply) => (
                <PingReplyRow reply={reply} key={reply.sequence} />
              ))}
            </div>
            {probing && (
              <div className="trace-waiting" role="status" aria-live="polite">
                <span className="spinner dark" />
                <span>
                  {value.phase === 'resolving'
                    ? '正在解析目标地址…'
                    : `正在等待第 ${value.replies.length + 1} 次响应…`}
                </span>
              </div>
            )}
          </div>
        </section>
      )}
    </div>
  )
}

export function PingReplyRow({ reply }: { reply: PingReply }) {
  const label =
    reply.status === 'reply' ? '已响应' : reply.status === 'unreachable' ? '不可达' : '请求超时'
  const tone = reply.status === 'reply' ? latencyTone(reply.latencyMs) : 'muted'
  return (
    <div className={`ping-row ${reply.status}`} role="row">
      <span className="ping-sequence" role="cell">
        <i />
        <b>#{reply.sequence}</b>
      </span>
      <span className={`ping-reply-status ${reply.status}`} role="cell">
        {label}
      </span>
      <code title={reply.address} role="cell">
        {reply.address ? <NetworkAddress value={reply.address} /> : '—'}
      </code>
      <strong className={`ping-latency-chip ${tone}`} role="cell">
        {reply.status === 'reply' ? formatTraceLatency(reply.latencyMs) : '*'}
      </strong>
    </div>
  )
}

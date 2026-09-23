import type { CSSProperties } from 'react'

export function LoadingCard({ text }: { text: string }) {
  return (
    <div className="loading-card">
      <span className="spinner dark" />
      <p>{text}</p>
    </div>
  )
}

export function Metric({
  label,
  value,
  accent = false
}: {
  label: string
  value: string
  accent?: boolean
}) {
  return (
    <div className={`card metric ${accent ? 'accent' : ''}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  )
}

export function Behavior({ name, value }: { name: string; value: string }) {
  return (
    <div>
      <span>{name}</span>
      <strong>{value || '无法判断'}</strong>
    </div>
  )
}

export function ProgressSteps({ steps }: { steps: Array<{ title: string; detail: string }> }) {
  return (
    <section className="card diagnostic-progress" aria-live="polite" aria-label="检测正在进行">
      <div className="progress-heading">
        <span className="progress-spinner">
          <i />
          <i />
          <i />
        </span>
        <div>
          <strong>正在执行检测</strong>
          <small>各步骤会根据网络响应时间依次完成</small>
        </div>
      </div>
      <div className="progress-track">
        {steps.map((step, index) => (
          <div
            className="progress-step"
            style={{ '--step-index': index } as CSSProperties}
            key={step.title}
          >
            <span>
              <b>{index + 1}</b>
            </span>
            <div className="progress-step-copy">
              <strong>{step.title}</strong>
              <small>{step.detail}</small>
            </div>
            {index < steps.length - 1 && <i />}
          </div>
        ))}
      </div>
    </section>
  )
}

export function IPCard({
  version,
  result
}: {
  version: string
  result: { available: boolean; address?: string; latencyMs?: number; error?: string }
}) {
  return (
    <section className={`card ip-card ${result.available ? 'available' : ''}`}>
      <div>
        <span className="ip-version">{version}</span>
        <span className={`availability ${result.available ? 'yes' : 'no'}`}>
          {result.available ? '可用' : '不可用'}
        </span>
      </div>
      <strong className="network-address" title={result.address}>
        <NetworkAddress value={result.address} fallback="未检测到公网地址" />
      </strong>
      <p>{result.available ? `HTTPS 直连 · ${result.latencyMs} ms` : result.error || '连接失败'}</p>
    </section>
  )
}

export function NetworkAddress({ value, fallback = '' }: { value?: string; fallback?: string }) {
  if (!value) return <>{fallback}</>
  if (!value.includes(':')) return <>{value}</>
  return (
    <>
      {value.split(':').map((segment, index) => (
        <span key={`${index}-${segment}`}>
          {index > 0 && (
            <>
              :<wbr />
            </>
          )}
          {segment}
        </span>
      ))}
    </>
  )
}

import { memo } from 'react'
import {
  siBilibili,
  siGithub,
  siTaobao,
  siTelegram,
  siTiktok,
  siWechat,
  siX,
  siYoutube
} from 'simple-icons'
import type { LatencyProbe } from '../../api'
import { Icon } from '../../components/Icon'
import { NetworkAddress } from '../../components/DiagnosticUI'
import { LatencyTooltip } from './LatencyTooltip'

export function LatencyGroup({ probes, polling }: { probes: LatencyProbe[]; polling: boolean }) {
  return (
    <div className="latency-group">
      <div className="latency-grid">
        {probes.map((probe) => (
          <LatencyCard
            key={probe.id}
            probe={probe}
            running={polling && probe.status === 'pending'}
          />
        ))}
      </div>
    </div>
  )
}

export const LatencyCard = memo(function LatencyCard({
  probe,
  running
}: {
  probe: LatencyProbe
  running: boolean
}) {
  const latencyClass =
    probe.status === 'pending'
      ? 'pending'
      : probe.status !== 'ok'
        ? 'failed'
        : (probe.latencyMs || 0) < 100
          ? 'fast'
          : (probe.latencyMs || 0) < 250
            ? 'medium'
            : 'slow'
  const url = probe.url || probe.host
  const httpStatus = probe.statusCode
    ? `HTTP ${probe.statusCode}`
    : running
      ? 'HTTP 检测中'
      : probe.status === 'timeout'
        ? 'HTTP TIMEOUT'
        : probe.status === 'failed'
          ? 'HTTP FAILED'
          : 'HTTP —'
  const serverAddress = probe.address || '服务器 IP 暂未获取'
  return (
    <div
      className={`latency-card ${latencyClass} ${running ? 'running' : ''}`}
      aria-label={`${probe.name}，${probe.region}，${running ? '正在测试' : '轮询结果'}`}
    >
      <LatencyTooltip
        label={`${url}，${httpStatus}，服务器 IP ${probe.address || '暂未获取'}`}
        content={
          <>
            <span className="latency-tooltip-url" title={url}>
              {url}
            </span>
            <strong>{httpStatus}</strong>
            <code className={probe.address ? '' : 'empty'}>
              <NetworkAddress value={probe.address} fallback={serverAddress} />
            </code>
          </>
        }
      >
        <ServiceIcon id={probe.id} />
      </LatencyTooltip>
      <div className="service-copy">
        <strong>{probe.name}</strong>
        <span className={`region-label ${probe.region === '国内' ? 'domestic' : 'international'}`}>
          {probe.region}
        </span>
      </div>
      <div className="latency-value">
        {running ? (
          <span className="latency-pending">
            <span className="spinner dark" />
            <span>测试中</span>
          </span>
        ) : probe.status === 'pending' ? (
          <span className="latency-waiting">已暂停</span>
        ) : probe.status === 'ok' ? (
          <>
            <b>{probe.latencyMs}</b>
            <small>ms</small>
          </>
        ) : (
          <b>{probe.status === 'timeout' ? 'TIMEOUT' : 'FAILED'}</b>
        )}
      </div>
    </div>
  )
})

export function ServiceIcon({ id }: { id: string }) {
  const icon = serviceIcons[id as keyof typeof serviceIcons]
  return (
    <span className={`service-mark ${icon ? id : 'custom'}`}>
      {icon ? (
        <svg role="img" viewBox="0 0 24 24" aria-label={icon.title}>
          <path d={icon.path} />
        </svg>
      ) : (
        <Icon name="globe" size={19} />
      )}
    </span>
  )
}

const serviceIcons = {
  douyin: siTiktok,
  bilibili: siBilibili,
  wechat: siWechat,
  taobao: siTaobao,
  github: siGithub,
  telegram: siTelegram,
  x: siX,
  youtube: siYoutube
}

import { memo, useMemo, useState } from 'react'
import type { NetworkInterface } from '../../api'
import { Icon } from '../../components/Icon'
import { NetworkAddress } from '../../components/DiagnosticUI'

export const NetworkInterfaces = memo(function NetworkInterfaces({
  items
}: {
  items: NetworkInterface[]
}) {
  type Filter = 'physical' | 'active' | 'all' | 'ethernet' | 'wifi' | 'virtual' | 'other'
  const [filter, setFilter] = useState<Filter>('physical')
  const visible = useMemo(
    () =>
      items.filter((item) => {
        if (filter === 'all') return true
        if (filter === 'physical') return item.physical
        if (filter === 'active') return item.up
        if (filter === 'ethernet' || filter === 'wifi') return item.kind === filter && item.physical
        if (filter === 'virtual')
          return !item.physical && ['virtual', 'loopback', 'tunnel'].includes(item.kind)
        return !item.physical && !['virtual', 'loopback', 'tunnel'].includes(item.kind)
      }),
    [filter, items]
  )
  return (
    <section className="card adapter-section">
      <div className="section-title">
        <h3>网卡</h3>
        <div className="adapter-section-actions">
          <select
            className="adapter-filter"
            aria-label="选择网卡类型"
            value={filter}
            onChange={(event) => setFilter(event.target.value as Filter)}
          >
            <option value="physical">物理网卡</option>
            <option value="active">已连接</option>
            <option value="ethernet">有线网卡</option>
            <option value="wifi">Wi-Fi</option>
            <option value="virtual">虚拟与隧道</option>
            <option value="other">其他类型</option>
            <option value="all">全部网卡</option>
          </select>
          <span className="section-badge">
            {visible.length} / {items.length}
          </span>
        </div>
      </div>
      <div className="adapter-grid">
        {visible.length ? (
          visible.map((item) => {
            const address = item.ipv4?.[0] || item.ipv6?.[0] || item.mac
            return (
              <div
                className={`adapter-card ${item.up ? 'up' : ''}`}
                key={`${item.index}-${item.name}`}
                title={item.description}
              >
                <span className={`adapter-kind ${item.kind}`}>
                  <Icon name={item.kind === 'ethernet' ? 'network' : 'globe'} size={18} />
                </span>
                <div>
                  <div className="adapter-title-row">
                    <strong>{item.name}</strong>
                    {item.up && (
                      <span className="adapter-online">
                        <i />
                        在线
                      </span>
                    )}
                  </div>
                  <small>
                    {networkInterfaceLabel(item)} · {item.up ? '已连接' : '未连接'}
                    {item.linkSpeedMbps ? ` · ${item.linkSpeedMbps} Mbps` : ''}
                  </small>
                  <code className="network-address" title={address}>
                    <NetworkAddress value={address} fallback="暂无地址" />
                  </code>
                </div>
                <div className="adapter-metrics">
                  <span>MAC {item.mac || '—'}</span>
                  <span>Metric {item.ipv4Metric || item.ipv6Metric || '自动'}</span>
                </div>
              </div>
            )
          })
        ) : (
          <div className="adapter-empty">当前筛选条件下没有网卡</div>
        )}
      </div>
    </section>
  )
})

function networkInterfaceLabel(item: NetworkInterface) {
  const labels: Record<NetworkInterface['kind'], string> = {
    ethernet: '有线以太网',
    wifi: '无线 Wi-Fi',
    virtual: '虚拟网卡',
    loopback: '回环接口',
    tunnel: '隧道接口',
    ppp: 'PPP 接口',
    other: '其他接口'
  }
  return labels[item.kind] || '其他接口'
}

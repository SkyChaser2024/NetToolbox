import type { AuthState } from '../api'
import { Icon } from './Icon'

export function NavItem({
  active,
  icon,
  label,
  onClick
}: {
  active: boolean
  icon: string
  label: string
  onClick: () => void
}) {
  return (
    <button
      className={`nav-item ${active ? 'active' : ''}`}
      aria-current={active ? 'page' : undefined}
      onClick={onClick}
    >
      <Icon name={icon} />
      <span>{label}</span>
      {active && <i />}
    </button>
  )
}

export function StatusPill({ state }: { state: AuthState }) {
  const labels: Record<AuthState, string> = {
    idle: '未认证',
    starting: '正在启动',
    waiting_identity: '等待身份请求',
    waiting_challenge: '正在校验',
    authenticated: '认证成功',
    failed: '认证失败',
    stopping: '正在断开',
    error: '发生错误'
  }
  return (
    <div className={`status-pill ${state}`}>
      <span className="status-dot" />
      {labels[state]}
    </div>
  )
}

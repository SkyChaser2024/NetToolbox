import { useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { api, defaultProfile } from '../../api'
import type { Adapter, AuthEvent, AuthRequest, AuthState, BootstrapData, Profile } from '../../api'
import { Icon } from '../../components/Icon'
import { errorMessage } from '../../utils/format'

function FieldError({ field, message }: { field: string; message?: string }) {
  if (!message) return null
  return (
    <small className="field-error" id={`auth-${field}-error`} role="alert">
      <Icon name="info" size={14} />
      <span>{message}</span>
    </small>
  )
}

export function AuthPage({
  data,
  state,
  logs,
  onAdapters,
  onProfile,
  onError
}: {
  data: BootstrapData
  state: AuthState
  logs: AuthEvent[]
  onAdapters: (items: Adapter[]) => void
  onProfile: (profile: Profile) => void
  onError: (message: string) => void
}) {
  const [profile, setProfile] = useState<Profile>(data.profile || defaultProfile)
  const [password, setPassword] = useState('')
  const [advanced, setAdvanced] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [refreshMotion, setRefreshMotion] = useState(0)
  const [pendingLogs, setPendingLogs] = useState<AuthEvent[] | null>(null)
  const [validationRequested, setValidationRequested] = useState(false)
  const fieldErrors: Partial<Record<'deviceName' | 'username' | 'password' | 'localMac', string>> =
    {}
  if (!profile.deviceName) fieldErrors.deviceName = '请选择用于认证的网卡'
  if (!profile.username.trim()) fieldErrors.username = '请输入校园网账号'
  if (!password && !(profile.rememberPassword && profile.passwordSet)) {
    fieldErrors.password = '请输入校园网密码'
  }
  if (!/^(?:[0-9a-f]{2}[:-]){5}[0-9a-f]{2}$/i.test(profile.localMac)) {
    fieldErrors.localMac = '请输入有效的网卡 MAC 地址'
  }
  const visibleErrors = validationRequested ? fieldErrors : {}
  const startingRequest = pendingLogs === logs
  const authenticating =
    state === 'starting' ||
    state === 'waiting_identity' ||
    state === 'waiting_challenge' ||
    state === 'stopping'
  const formDisabled = startingRequest || authenticating || state === 'authenticated'
  const ethernetAdapters = useMemo(
    () => data.adapters.filter((item) => item.kind === 'ethernet' && item.recommended),
    [data.adapters]
  )
  const selectedAdapter = useMemo(
    () => ethernetAdapters.find((item) => item.deviceName === profile.deviceName),
    [ethernetAdapters, profile.deviceName]
  )

  useEffect(() => {
    if (
      (!profile.deviceName ||
        !ethernetAdapters.some((item) => item.deviceName === profile.deviceName)) &&
      ethernetAdapters.length
    ) {
      const first = ethernetAdapters.find((item) => item.up) || ethernetAdapters[0]
      setProfile((current) => ({
        ...current,
        deviceName: first.deviceName,
        adapterLabel: first.name,
        localMac: first.mac || current.localMac
      }))
    }
  }, [ethernetAdapters, profile.deviceName])

  const update = <K extends keyof Profile>(key: K, value: Profile[K]) =>
    setProfile((current) => ({ ...current, [key]: value }))
  const chooseAdapter = (deviceName: string) => {
    const adapter = ethernetAdapters.find((item) => item.deviceName === deviceName)
    setProfile((current) => ({
      ...current,
      deviceName,
      adapterLabel: adapter?.name || '',
      localMac: adapter?.mac || current.localMac
    }))
  }
  const refresh = async () => {
    setRefreshMotion((current) => current + 1)
    setRefreshing(true)
    try {
      const result = await api().RefreshAdapters()
      onAdapters(result.adapters || [])
      if (result.error) onError(result.error)
    } catch (error) {
      onError(errorMessage(error))
    } finally {
      setRefreshing(false)
    }
  }
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setValidationRequested(true)
    const firstInvalid = Object.keys(fieldErrors)[0]
    if (firstInvalid) {
      const control = (event.currentTarget as HTMLFormElement).elements.namedItem(firstInvalid)
      if (control instanceof HTMLElement) control.focus()
      return
    }
    setValidationRequested(false)
    setPendingLogs(logs)
    try {
      const request: AuthRequest = { ...profile, password }
      await api().Connect(request)
      onProfile({
        ...profile,
        passwordSet: profile.rememberPassword && (password !== '' || profile.passwordSet)
      })
      setPassword('')
    } catch (error) {
      setPendingLogs(null)
      onError(errorMessage(error))
    }
  }
  const logout = async () => {
    try {
      await api().Logout()
    } catch (error) {
      onError(errorMessage(error))
    }
  }
  const cancel = async () => {
    try {
      await api().CancelAuthentication()
    } catch (error) {
      onError(errorMessage(error))
    }
  }

  return (
    <div className="page-grid auth-grid">
      <section className="card connect-card">
        <div className="card-heading">
          <div>
            <span className="eyebrow">802.1X ACCESS</span>
            <h2>{state === 'authenticated' ? '网络已认证' : '连接校园网'}</h2>
          </div>
          <div className={`signal-orb ${state}`}>
            <span />
            <span />
            <span />
          </div>
        </div>
        {!data.npcapAvailable && (
          <div className="notice warning">
            <Icon name="info" />
            <div>
              <strong>需要安装 Npcap</strong>
              <p>{data.adapterError || '认证功能需要 Npcap 驱动来收发 EAPOL 报文。'}</p>
              <button
                className="link-button"
                onClick={() => api().OpenLink('https://npcap.com/#download')}
              >
                打开下载页 <Icon name="external" size={14} />
              </button>
            </div>
          </div>
        )}
        <form onSubmit={submit} className="auth-form">
          <label className="field field-wide">
            <span>有线网卡</span>
            <div className="input-row">
              <select
                name="deviceName"
                value={profile.deviceName}
                aria-invalid={Boolean(visibleErrors.deviceName)}
                aria-describedby={visibleErrors.deviceName ? 'auth-deviceName-error' : undefined}
                onChange={(event) => chooseAdapter(event.target.value)}
                disabled={formDisabled}
              >
                <option value="">请选择认证网卡</option>
                {ethernetAdapters.map((adapter) => (
                  <option key={adapter.deviceName} value={adapter.deviceName}>
                    {adapter.name}
                    {adapter.mac ? ` · ${adapter.mac}` : ''}
                  </option>
                ))}
              </select>
              <button
                type="button"
                className="square-button adapter-refresh-button"
                onClick={refresh}
                disabled={refreshing || formDisabled}
                title={refreshing ? '正在刷新网卡' : '刷新网卡'}
                aria-label={refreshing ? '正在刷新网卡' : '刷新网卡'}
                aria-busy={refreshing}
              >
                <span
                  className={`adapter-refresh-glyph ${refreshMotion ? 'animate' : ''}`}
                  key={refreshMotion}
                >
                  <Icon name="refresh" size={18} />
                </span>
              </button>
            </div>
            <FieldError field="deviceName" message={visibleErrors.deviceName} />
            {selectedAdapter && (
              <small>
                {selectedAdapter.description || selectedAdapter.deviceName}
                {selectedAdapter.ipv4?.length ? ` · IPv4 ${selectedAdapter.ipv4[0]}` : ''}
              </small>
            )}
          </label>
          <label className="field">
            <span>校园网账号</span>
            <input
              name="username"
              value={profile.username}
              aria-invalid={Boolean(visibleErrors.username)}
              aria-describedby={visibleErrors.username ? 'auth-username-error' : undefined}
              onChange={(event) => update('username', event.target.value)}
              autoComplete="username"
              placeholder="学号 / 用户名"
              disabled={formDisabled}
            />
            <FieldError field="username" message={visibleErrors.username} />
          </label>
          <label className="field">
            <span>密码</span>
            <input
              type="password"
              name="password"
              value={password}
              aria-invalid={Boolean(visibleErrors.password)}
              aria-describedby={visibleErrors.password ? 'auth-password-error' : undefined}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
              placeholder={
                profile.rememberPassword && profile.passwordSet
                  ? '已安全保存，留空继续使用'
                  : '请输入密码'
              }
              disabled={formDisabled}
            />
            <FieldError field="password" message={visibleErrors.password} />
          </label>
          <label className="field field-wide">
            <span>网卡 MAC 地址</span>
            <input
              name="localMac"
              value={profile.localMac}
              aria-invalid={Boolean(visibleErrors.localMac)}
              aria-describedby={visibleErrors.localMac ? 'auth-localMac-error' : undefined}
              onChange={(event) => update('localMac', event.target.value)}
              placeholder="例如 12:34:56:78:9A:BC"
              disabled={formDisabled}
            />
            <FieldError field="localMac" message={visibleErrors.localMac} />
          </label>
          <label className="check-row field-wide">
            <input
              type="checkbox"
              checked={profile.rememberPassword}
              onChange={(event) => update('rememberPassword', event.target.checked)}
              disabled={formDisabled}
            />
            <span>为当前 Windows 用户安全保存密码</span>
          </label>
          <details
            className="auth-advanced field-wide"
            open={advanced}
            onToggle={(event) => setAdvanced(event.currentTarget.open)}
          >
            <summary>
              <span>高级适配选项</span>
              <Icon name="chevron" size={14} />
            </summary>
            {advanced && (
              <div className="advanced-panel">
                <label className="field">
                  <span>EAP Identity</span>
                  <input
                    value={profile.identity}
                    onChange={(event) => update('identity', event.target.value)}
                    placeholder="留空时与账号一致"
                    disabled={formDisabled}
                  />
                </label>
                <label className="field">
                  <span>Identity 后缀（Hex）</span>
                  <input
                    value={profile.identitySuffix}
                    onChange={(event) => update('identitySuffix', event.target.value)}
                    placeholder="例如 00 00 13 11 00"
                    disabled={formDisabled}
                  />
                </label>
                <label className="field">
                  <span>启动延迟</span>
                  <div className="unit-input">
                    <input
                      type="number"
                      min="0"
                      max="30000"
                      value={profile.startDelayMs}
                      onChange={(event) => update('startDelayMs', Number(event.target.value))}
                      disabled={formDisabled}
                    />
                    <em>ms</em>
                  </div>
                </label>
                <label className="field">
                  <span>失败重试</span>
                  <div className="unit-input">
                    <input
                      type="number"
                      min="0"
                      max="60000"
                      value={profile.retryDelayMs}
                      onChange={(event) => update('retryDelayMs', Number(event.target.value))}
                      disabled={formDisabled}
                    />
                    <em>ms</em>
                  </div>
                </label>
                <label className="check-row compact">
                  <input
                    type="checkbox"
                    checked={profile.debug}
                    onChange={(event) => update('debug', event.target.checked)}
                    disabled={formDisabled}
                  />
                  <span>记录原始报文调试信息</span>
                </label>
              </div>
            )}
          </details>
          <div className="form-actions field-wide">
            {state === 'authenticated' ? (
              <button type="button" className="button danger" onClick={logout}>
                <Icon name="plug" />
                注销
              </button>
            ) : startingRequest ? (
              <button type="button" className="button primary" disabled aria-busy="true">
                <span className="spinner" />
                正在启动…
              </button>
            ) : authenticating ? (
              <button type="button" className="button danger" onClick={cancel}>
                <span className="trace-stop-square" />
                取消认证
              </button>
            ) : (
              <button
                type="submit"
                className="button primary"
                disabled={!data.npcapAvailable || !ethernetAdapters.length}
              >
                <Icon name="shield" />
                开始认证
              </button>
            )}
            {state === 'authenticated' && <span>认证会话已结束，不会持续抓包或发送保活报文</span>}
          </div>
        </form>
      </section>
      <section className="card session-card">
        <div className="section-title">
          <div>
            <h3>认证会话</h3>
            <p>最近的协议事件</p>
          </div>
          {authenticating && (
            <span className="live-indicator">
              <i />
              LIVE
            </span>
          )}
        </div>
        <div className="timeline">
          {startingRequest && (
            <div className="auth-pending" role="status">
              <span className="spinner dark" />
              正在启动认证，等待协议事件…
            </div>
          )}
          {logs.length === 0 ? (
            <div className="empty-state">
              <div>
                <Icon name="activity" size={28} />
              </div>
              <strong>
                {startingRequest
                  ? '正在启动认证'
                  : state === 'authenticated'
                    ? '已检测到有线网络认证'
                    : '等待开始认证'}
              </strong>
              <p>
                {startingRequest
                  ? '正在检查网卡并准备认证会话。'
                  : state === 'authenticated'
                    ? '本次启动无需重复认证，需要断开时点击“注销”。'
                    : '交换机的 Identity、Challenge 和结果会显示在这里。'}
              </p>
            </div>
          ) : (
            [...logs].reverse().map((log, index) => (
              <div className={`log-row ${log.level}`} key={`${log.timestamp}-${index}`}>
                <span className="log-node" />
                <div>
                  <p>{log.message}</p>
                  <time>{log.timestamp || '--:--:--'}</time>
                </div>
              </div>
            ))
          )}
        </div>
        <div className="protocol-strip">
          <span>EAPOL</span>
          <i />
          <span>Identity</span>
          <i />
          <span>MD5</span>
          <i />
          <span>Success</span>
        </div>
      </section>
    </div>
  )
}

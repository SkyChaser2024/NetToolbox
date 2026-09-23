import { useState } from 'react'
import type { BootstrapData, SystemPreferences } from '../../api'
import type { SettingsSaver } from '../../appTypes'
import { Icon } from '../../components/Icon'
import { errorMessage } from '../../utils/format'

type SystemUpdate = (current: SystemPreferences) => SystemPreferences

export function SettingsPage({
  value,
  onSaveSettings,
  onError
}: {
  value: BootstrapData
  onSaveSettings: SettingsSaver
  onError: (message: string) => void
}) {
  const [pendingUpdates, setPendingUpdates] = useState<SystemUpdate[]>([])
  // Replay pending field changes over the latest committed snapshot so an
  // earlier response cannot overwrite a later click. The shared saver serializes writes.
  const system = pendingUpdates.reduce((current, update) => update(current), value.system)
  const updateSystem = (update: SystemUpdate) => {
    setPendingUpdates((current) => [...current, update])
    void onSaveSettings((current) => ({
      profile: current.profile,
      diagnostics: current.diagnostics,
      system: update(current.system)
    }))
      .catch((error) => onError(errorMessage(error)))
      .finally(() => {
        setPendingUpdates((current) => current.filter((pending) => pending !== update))
      })
  }

  return (
    <div className="settings-page">
      <section className="card settings-overview">
        <div className="brand-mark large">
          <Icon name="activity" size={31} />
        </div>
        <div className="about-copy">
          <span className="eyebrow">NETWORK TOOLBOX</span>
          <h2>网络工具箱</h2>
          <div className="overview-facts">
            <span>Go + Wails</span>
            <span>802.1X / EAP-MD5</span>
            <span>Windows DPAPI</span>
          </div>
        </div>
        <div className="settings-save-meta">
          <span className="version">Version {value.version}</span>
        </div>
      </section>

      <details className="card settings-disclosure">
        <summary>
          <div>
            <span className="disclosure-icon blue">
              <Icon name="activity" size={19} />
            </span>
            <span>
              <strong>网络与启动</strong>
            </span>
          </div>
          <span className="summary-side">
            <Icon name="chevron" size={16} />
          </span>
        </summary>
        <div className="disclosure-body">
          <div className="priority-options">
            {(
              [
                ['automatic', '系统自动', '由 Windows 自动选择'],
                ['ethernet', '以太网优先', '优先使用有线网络'],
                ['wifi', 'Wi-Fi 优先', '优先使用无线网络']
              ] as const
            ).map(([mode, title, description]) => (
              <label
                className={`priority-option ${system.priorityMode === mode ? 'selected' : ''}`}
                key={mode}
              >
                <input
                  type="radio"
                  name="priority"
                  value={mode}
                  checked={system.priorityMode === mode}
                  onChange={() => updateSystem((current) => ({ ...current, priorityMode: mode }))}
                />
                <span>
                  <strong>{title}</strong>
                  <small>{description}</small>
                </span>
              </label>
            ))}
          </div>
          {(
            [
              ['closeToTray', '退出时最小化', '关闭窗口时最小化到系统托盘'],
              ['startAtLogin', '自启动', '登录 Windows 后自动启动'],
              ['silentStart', '静默启动', '启动时隐藏主窗口，下次启动生效'],
              ['autoAuthenticate', '自动认证', '自动认证并在断线后重连']
            ] as const
          ).map(([key, title, description]) => (
            <label className="toggle-setting" key={key}>
              <div>
                <strong>{title}</strong>
                <p>{description}</p>
                {key === 'autoAuthenticate' &&
                  (!value.profile.passwordSet || !value.profile.username) && (
                    <small>请先在锐捷认证页保存账号和密码</small>
                  )}
              </div>
              <input
                type="checkbox"
                checked={system[key]}
                disabled={
                  key === 'autoAuthenticate' &&
                  !system.autoAuthenticate &&
                  (!value.profile.passwordSet || !value.profile.username)
                }
                onChange={(event) => {
                  const checked = event.target.checked
                  updateSystem((current) => ({ ...current, [key]: checked }))
                }}
              />
              <i />
            </label>
          ))}
        </div>
      </details>
    </div>
  )
}

import { defaultDiagnostics } from '../../api'
import type { DiagnosticSettings, LatencyTarget } from '../../api'
import type { SettingsSaver } from '../../appTypes'
import { Icon } from '../../components/Icon'
import { useDiagnosticAutosave } from '../../hooks/useDiagnosticAutosave'
import { useDiagnosticDraft } from '../../hooks/DiagnosticDraftProvider'
import { splitLines } from '../../utils/format'

export function LatencySettings({
  value,
  onSaveSettings,
  onError
}: {
  value: DiagnosticSettings['latencyTargets']
  onSaveSettings: SettingsSaver
  onError: (message: string) => void
}) {
  const [targets, setTargets] = useDiagnosticDraft('latencyTargets', formatLatencyTargets(value))
  useDiagnosticAutosave(
    targets,
    (current) => ({ ...current, latencyTargets: parseLatencyTargets(targets) }),
    onSaveSettings,
    onError,
    'latencyTargets'
  )
  return (
    <details className="module-settings latency-settings">
      <summary>
        <span>延迟目标</span>
        <Icon name="chevron" size={14} />
      </summary>
      <div className="module-settings-body">
        <div className="settings-heading compact-heading">
          <p>每行格式：名称 | URL | 国内或国际。</p>
          <button
            type="button"
            className="reset-button"
            onClick={() => setTargets(formatLatencyTargets(defaultDiagnostics.latencyTargets))}
          >
            恢复默认
          </button>
        </div>
        <textarea
          className="settings-textarea compact"
          spellCheck={false}
          value={targets}
          onChange={(event) => setTargets(event.target.value)}
          aria-label="网站响应测试列表"
        />
      </div>
    </details>
  )
}

export function PublicEndpointSettings({
  version,
  value,
  onSaveSettings,
  onError
}: {
  version: 'IPv4' | 'IPv6'
  value: string[]
  onSaveSettings: SettingsSaver
  onError: (message: string) => void
}) {
  const editorId = `${version}Endpoints`
  const [endpoints, setEndpoints] = useDiagnosticDraft(editorId, value.join('\n'))
  const defaults =
    version === 'IPv4' ? defaultDiagnostics.ipv4Endpoints : defaultDiagnostics.ipv6Endpoints
  useDiagnosticAutosave(
    endpoints,
    (current) =>
      version === 'IPv4'
        ? { ...current, ipv4Endpoints: splitLines(endpoints) }
        : { ...current, ipv6Endpoints: splitLines(endpoints) },
    onSaveSettings,
    onError,
    editorId
  )
  return (
    <details className="module-settings public-endpoint-settings">
      <summary>
        <span>查询源</span>
        <Icon name="chevron" size={14} />
      </summary>
      <div className="module-settings-body">
        <div className="settings-heading compact-heading">
          <p>刷新时会按顺序尝试，每行一个 URL。</p>
          <button
            type="button"
            className="reset-button"
            onClick={() => setEndpoints(defaults.join('\n'))}
          >
            恢复默认
          </button>
        </div>
        <textarea
          className="settings-textarea compact"
          spellCheck={false}
          value={endpoints}
          onChange={(event) => setEndpoints(event.target.value)}
          aria-label={`${version} 公网查询源`}
        />
      </div>
    </details>
  )
}

function formatLatencyTargets(values: LatencyTarget[]) {
  return values.map((item) => `${item.name} | ${item.url} | ${item.region}`).join('\n')
}

function parseLatencyTargets(value: string): LatencyTarget[] {
  return splitLines(value).map((line, index) => {
    const parts = line.split('|').map((item) => item.trim())
    if (
      parts.length !== 3 ||
      !parts[0] ||
      !parts[1] ||
      (parts[2] !== '国内' && parts[2] !== '国际')
    ) {
      throw new Error(`网站响应测试第 ${index + 1} 行格式无效，应为：名称 | URL | 国内或国际`)
    }
    return { id: '', name: parts[0], url: parts[1], region: parts[2] }
  })
}

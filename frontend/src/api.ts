import packageInfo from '../package.json'

export const appVersion = packageInfo.version

export type AuthState =
  | 'idle'
  | 'starting'
  | 'waiting_identity'
  | 'waiting_challenge'
  | 'authenticated'
  | 'failed'
  | 'stopping'
  | 'error'

export interface Profile {
  deviceName: string
  adapterLabel: string
  localMac: string
  username: string
  identity: string
  identitySuffix: string
  startDelayMs: number
  retryDelayMs: number
  debug: boolean
  rememberPassword: boolean
  passwordSet: boolean
}

export interface Adapter {
  deviceName: string
  name: string
  description: string
  mac: string
  ipv4: string[]
  ipv6: string[]
  up: boolean
  recommended: boolean
  interfaceIndex: number
  kind: 'ethernet' | 'wifi' | ''
}

export interface BootstrapData {
  profile: Profile
  diagnostics: DiagnosticSettings
  system: SystemPreferences
  adapters: Adapter[]
  networkInterfaces: NetworkInterface[]
  npcapAvailable: boolean
  adapterError?: string
  configurationError?: string
  backgroundWarning?: string
  cachedOverview?: OverviewResult
  authState: AuthState
  version: string
}

export interface SystemPreferences {
  priorityMode: 'automatic' | 'ethernet' | 'wifi'
  startAtLogin: boolean
  silentStart: boolean
  autoAuthenticate: boolean
  closeToTray: boolean
}

export interface AutomaticAuthenticationStatus {
  running: boolean
  paused: boolean
  message: string
}

export interface NetworkInterface {
  index: number
  name: string
  description: string
  kind: 'ethernet' | 'wifi' | 'virtual' | 'loopback' | 'tunnel' | 'ppp' | 'other'
  physical: boolean
  up: boolean
  mac: string
  ipv4: string[]
  ipv6: string[]
  linkSpeedMbps: number
  ipv4Metric: number
  ipv6Metric: number
}

export interface SettingsRequest {
  profile: Profile
  diagnostics: DiagnosticSettings
  system: SystemPreferences
}

export interface SettingsResult extends SettingsRequest {
  networkInterfaces: NetworkInterface[]
  warning?: string
}

export interface DiagnosticSettings {
  latencyTargets: LatencyTarget[]
  natServers: string[]
  ipv4Endpoints: string[]
  ipv6Endpoints: string[]
  ipv6Sites: string[]
  aaaaDomain: string
  ipv6LargeUrl: string
}

export interface LatencyTarget {
  id: string
  name: string
  url: string
  region: '国内' | '国际'
}

export interface AuthRequest extends Omit<Profile, 'passwordSet'> {
  password: string
}

export interface AuthEvent {
  state: AuthState
  level: 'debug' | 'info' | 'warning' | 'success' | 'error'
  message: string
  timestamp: string
}

export interface NATProbe {
  server: string
  serverIp?: string
  endpoint?: string
  latencyMs?: number
  error?: string
}

export interface NATResult {
  status: 'ok' | 'error'
  type: string
  summary: string
  localIp: string
  publicIp: string
  publicPort: number
  mappingBehavior: string
  filteringBehavior: string
  latencyMs: number
  serverCount: number
  rfc5780: boolean
  probes: NATProbe[]
  error?: string
}

export interface IPProbeResult {
  available: boolean
  address?: string
  latencyMs?: number
  error?: string
}

export interface IPv6Result {
  connectionType: string
  summary: string
  ipv4: IPProbeResult
  ipv6: IPProbeResult
  localGlobal: string[]
  localLink: string[]
  dnsAAAA: boolean
  sites: WebsiteProbeResult[]
  largePacket?: {
    available: boolean
    latencyMs?: number
    bytesRead?: number
    error?: string
  }
}

export type TraceProtocol = 'auto' | 'ipv4' | 'ipv6'

export interface TraceRequest {
  sessionId: string
  target: string
  protocol: TraceProtocol
  maxHops: number
  timeoutMs: number
  resolveHostnames: boolean
}

export interface TraceProbe {
  status: 'reply' | 'timeout' | 'unreachable'
  address?: string
  hostname?: string
  latencyMs?: number
  reached?: boolean
}

export interface TraceHop {
  number: number
  status: 'reply' | 'timeout' | 'unreachable' | 'destination'
  probes: TraceProbe[]
  averageMs?: number
  reached?: boolean
}

export interface TraceEvent {
  sessionId: string
  type: 'started' | 'hop' | 'completed' | 'cancelled' | 'error'
  target?: string
  address?: string
  protocol?: 'ipv4' | 'ipv6'
  hop?: TraceHop
  status?: 'reached' | 'unreachable' | 'max_hops' | 'cancelled' | 'error'
  reached?: boolean
  hopCount?: number
  durationMs?: number
  error?: string
}

export interface PingRequest {
  sessionId: string
  target: string
  protocol: TraceProtocol
  count: number
  timeoutMs: number
  intervalMs: number
}

export interface PingReply {
  sequence: number
  status: 'reply' | 'timeout' | 'unreachable'
  address?: string
  latencyMs?: number
}

export interface PingSummary {
  target: string
  address: string
  protocol: 'ipv4' | 'ipv6'
  status: string
  sent: number
  received: number
  lost: number
  lossPercent: number
  minMs?: number
  averageMs?: number
  maxMs?: number
  durationMs?: number
}

export interface PingEvent {
  sessionId: string
  type: 'started' | 'reply' | 'completed' | 'cancelled' | 'error'
  target?: string
  address?: string
  protocol?: 'ipv4' | 'ipv6'
  reply?: PingReply
  summary?: PingSummary
  error?: string
}

export interface PublicNetworkInfo {
  available: boolean
  address?: string
  source?: string
  isp?: string
  asn?: number
  asnOrganization?: string
  country?: string
  region?: string
  city?: string
  error?: string
}

export interface LatencyProbe {
  id: string
  name: string
  host: string
  url: string
  region: '国内' | '国际'
  status: 'pending' | 'ok' | 'timeout' | 'failed'
  address?: string
  statusCode?: number
  latencyMs?: number
  error?: string
}

export interface OverviewResult {
  ipv4: PublicNetworkInfo
  ipv6: PublicNetworkInfo
  probes: LatencyProbe[]
  checkedAt: string
}

export interface WebsiteProbeResult {
  url: string
  host: string
  available: boolean
  address?: string
  statusCode?: number
  latencyMs?: number
  error?: string
}

export interface BackendAPI {
  GetAutomaticAuthenticationStatus(): Promise<AutomaticAuthenticationStatus>
  ResumeAutomaticAuthentication(): Promise<void>
  Bootstrap(): Promise<BootstrapData>
  RefreshAdapters(): Promise<{ adapters: Adapter[]; npcapAvailable: boolean; error?: string }>
  Connect(request: AuthRequest): Promise<void>
  Logout(): Promise<void>
  CancelAuthentication(): Promise<void>
  SaveSettings(value: SettingsRequest): Promise<SettingsResult>
  CheckOverview(): Promise<OverviewResult>
  CheckPublicIPv4(): Promise<PublicNetworkInfo>
  CheckPublicIPv6(): Promise<PublicNetworkInfo>
  CheckLatency(id: string): Promise<LatencyProbe>
  CancelLatencyChecks(): Promise<void>
  CheckNAT(): Promise<NATResult>
  CheckIPv6(): Promise<IPv6Result>
  StartPing(request: PingRequest): Promise<void>
  CancelPing(sessionId: string): Promise<void>
  StartTraceroute(request: TraceRequest): Promise<void>
  CancelTraceroute(sessionId: string): Promise<void>
  OpenLink(url: string): Promise<void>
}

declare global {
  interface Window {
    go?: { desktop?: { App?: BackendAPI } }
  }
}

export const defaultProfile: Profile = {
  deviceName: '',
  adapterLabel: '',
  localMac: '',
  username: '',
  identity: '',
  identitySuffix: '',
  startDelayMs: 0,
  retryDelayMs: 2000,
  debug: false,
  rememberPassword: true,
  passwordSet: false
}

export const defaultDiagnostics: DiagnosticSettings = {
  latencyTargets: [
    { id: 'douyin', name: '字节抖音', url: 'https://www.douyin.com/', region: '国内' },
    { id: 'bilibili', name: 'Bilibili', url: 'https://www.bilibili.com/', region: '国内' },
    { id: 'wechat', name: '腾讯微信', url: 'https://weixin.qq.com/', region: '国内' },
    { id: 'taobao', name: '阿里淘宝', url: 'https://www.taobao.com/', region: '国内' },
    { id: 'github', name: 'GitHub', url: 'https://github.com/', region: '国际' },
    { id: 'telegram', name: 'Telegram', url: 'https://telegram.org/', region: '国际' },
    { id: 'x', name: 'X.com', url: 'https://x.com/', region: '国际' },
    { id: 'youtube', name: 'YouTube', url: 'https://www.youtube.com/', region: '国际' }
  ],
  natServers: [
    'stun.miwifi.com:3478',
    'stun.hitv.com:3478',
    'stun.chat.bilibili.com:3478',
    'stun.cloudflare.com:3478'
  ],
  ipv4Endpoints: [
    'https://4.ipw.cn',
    'https://api-ipv4.ip.sb/ip',
    'https://myip.ipip.net',
    'https://ipv4.icanhazip.com'
  ],
  ipv6Endpoints: ['https://6.ipw.cn', 'https://api-ipv6.ip.sb/ip', 'https://ipv6.icanhazip.com'],
  ipv6Sites: [
    'https://www.qq.com',
    'https://www.baidu.com',
    'https://www.taobao.com',
    'https://www.jd.com'
  ],
  aaaaDomain: 'www.qq.com',
  ipv6LargeUrl: ''
}

export const defaultSystem: SystemPreferences = {
  priorityMode: 'automatic',
  startAtLogin: false,
  silentStart: false,
  autoAuthenticate: false,
  closeToTray: true
}

const previewAPI: BackendAPI = {
  async GetAutomaticAuthenticationStatus() {
    return { running: false, paused: false, message: '浏览器预览无法运行自动认证后台' }
  },
  async ResumeAutomaticAuthentication() {
    throw new Error('请在桌面应用中恢复自动认证')
  },
  async Bootstrap() {
    return {
      profile: defaultProfile,
      diagnostics: defaultDiagnostics,
      system: defaultSystem,
      adapters: [],
      networkInterfaces: [],
      npcapAvailable: false,
      authState: 'idle',
      version: `${appVersion}-preview`
    }
  },
  async RefreshAdapters() {
    return { adapters: [], npcapAvailable: false, error: '浏览器预览无法读取本机网卡' }
  },
  async Connect() {
    throw new Error('请在 Wails 桌面应用中使用认证功能')
  },
  async Logout() {},
  async CancelAuthentication() {},
  async SaveSettings(value) {
    return { ...value, networkInterfaces: [] }
  },
  async CheckOverview() {
    throw new Error('请在 Wails 桌面应用中查看网络概览')
  },
  async CheckPublicIPv4() {
    throw new Error('请在 Wails 桌面应用中查看网络概览')
  },
  async CheckPublicIPv6() {
    throw new Error('请在 Wails 桌面应用中查看网络概览')
  },
  async CheckLatency() {
    throw new Error('请在 Wails 桌面应用中运行连接测试')
  },
  async CancelLatencyChecks() {},
  async CheckNAT() {
    throw new Error('请在 Wails 桌面应用中运行网络检测')
  },
  async CheckIPv6() {
    throw new Error('请在 Wails 桌面应用中运行网络检测')
  },
  async StartPing() {
    throw new Error('请在 Wails 桌面应用中运行 Ping 测试')
  },
  async CancelPing() {},
  async StartTraceroute() {
    throw new Error('请在 Wails 桌面应用中运行路由追踪')
  },
  async CancelTraceroute() {},
  async OpenLink(url: string) {
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}

export const api = (): BackendAPI => window.go?.desktop?.App ?? previewAPI

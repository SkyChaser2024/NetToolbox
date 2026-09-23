import type { BootstrapData, SettingsRequest } from './api'

export type Page = 'home' | 'auth' | 'nat' | 'ipv6' | 'trace' | 'settings'

export type Theme = 'system' | 'light' | 'dark'

export type SettingsSaver = (update: (current: BootstrapData) => SettingsRequest) => Promise<void>

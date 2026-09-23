import type { PingReply, PingSummary, TraceEvent, TraceHop, TraceProtocol } from '../../api'

export type TracePhase =
  'idle' | 'starting' | 'resolving' | 'running' | 'stopping' | 'completed' | 'cancelled' | 'error'

export type PingPhase = TracePhase

export interface PingViewState {
  target: string
  submittedTarget: string
  protocol: TraceProtocol
  count: number
  timeoutMs: number
  intervalMs: number
  advancedOpen: boolean
  sessionId: string
  phase: PingPhase
  resolvedTarget: string
  actualProtocol: 'ipv4' | 'ipv6' | ''
  replies: PingReply[]
  summary: PingSummary | null
  error: string
}

export interface TraceViewState {
  target: string
  submittedTarget: string
  protocol: TraceProtocol
  maxHops: number
  timeoutMs: number
  resolveHostnames: boolean
  advancedOpen: boolean
  sessionId: string
  phase: TracePhase
  resolvedTarget: string
  actualProtocol: 'ipv4' | 'ipv6' | ''
  hops: TraceHop[]
  durationMs: number
  resultStatus: TraceEvent['status'] | ''
  error: string
}

export const initialTraceState: TraceViewState = {
  target: '',
  protocol: 'auto',
  maxHops: 30,
  timeoutMs: 1000,
  resolveHostnames: true,
  advancedOpen: false,
  submittedTarget: '',
  sessionId: '',
  phase: 'idle',
  resolvedTarget: '',
  actualProtocol: '',
  hops: [],
  durationMs: 0,
  resultStatus: '',
  error: ''
}

export const initialPingState: PingViewState = {
  target: '',
  protocol: 'auto',
  count: 4,
  timeoutMs: 1000,
  intervalMs: 1000,
  advancedOpen: false,
  submittedTarget: '',
  sessionId: '',
  phase: 'idle',
  resolvedTarget: '',
  actualProtocol: '',
  replies: [],
  summary: null,
  error: ''
}

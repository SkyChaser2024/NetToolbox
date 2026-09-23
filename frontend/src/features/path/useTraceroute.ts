import { useEffect, useRef, useState } from 'react'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { api } from '../../api'
import type { TraceEvent } from '../../api'
import type { TraceViewState } from './types'
import { initialTraceState } from './types'
import { errorMessage, createSessionID } from '../../utils/format'

export function useTraceroute(active: boolean) {
  const [trace, setTrace] = useState<TraceViewState>(initialTraceState)
  useEffect(() => {
    if (!window.go?.desktop?.App) return
    return EventsOn('traceroute:event', (event: TraceEvent) => {
      setTrace((current) => {
        if (!current.sessionId || event.sessionId !== current.sessionId) return current
        if (current.phase === 'cancelled' && event.type !== 'cancelled') return current
        if (current.phase === 'stopping' && (event.type === 'started' || event.type === 'hop'))
          return current
        if (event.type === 'started') {
          return {
            ...current,
            phase: 'running',
            submittedTarget: event.target || current.submittedTarget,
            resolvedTarget: event.address || '',
            actualProtocol: event.protocol || '',
            error: ''
          }
        }
        if (event.type === 'hop' && event.hop) {
          const hops = current.hops.filter((hop) => hop.number !== event.hop?.number)
          hops.push(event.hop)
          hops.sort((left, right) => left.number - right.number)
          return { ...current, phase: 'running', hops }
        }
        if (event.type === 'completed') {
          return {
            ...current,
            phase: 'completed',
            resolvedTarget: event.address || current.resolvedTarget,
            actualProtocol: event.protocol || current.actualProtocol,
            durationMs: event.durationMs || 0,
            resultStatus: event.status || '',
            error: ''
          }
        }
        if (event.type === 'cancelled') {
          return {
            ...current,
            phase: 'cancelled',
            durationMs: event.durationMs || current.durationMs,
            resultStatus: 'cancelled'
          }
        }
        if (event.type === 'error') {
          return {
            ...current,
            phase: 'error',
            durationMs: event.durationMs || current.durationMs,
            resultStatus: 'error',
            error: event.error || '路由追踪失败'
          }
        }
        return current
      })
    })
  }, [])
  useEffect(() => {
    if (active || (trace.phase !== 'resolving' && trace.phase !== 'running') || !trace.sessionId)
      return
    const sessionId = trace.sessionId
    setTrace((current) =>
      current.sessionId === sessionId
        ? { ...current, phase: 'cancelled', resultStatus: 'cancelled' }
        : current
    )
    void api().CancelTraceroute(sessionId)
  }, [active, trace.phase, trace.sessionId])
  const sessionRef = useRef('')
  sessionRef.current = trace.sessionId
  useEffect(
    () => () => {
      if (sessionRef.current) void api().CancelTraceroute(sessionRef.current)
    },
    []
  )
  const updateTrace = (values: Partial<TraceViewState>) =>
    setTrace((current) => ({ ...current, ...values }))

  const startTraceroute = async () => {
    const target = trace.target.trim() || 'baidu.com'
    const sessionId = createSessionID('trace')
    const request = {
      sessionId,
      target,
      protocol: trace.protocol,
      maxHops: trace.maxHops,
      timeoutMs: trace.timeoutMs,
      resolveHostnames: trace.resolveHostnames
    }
    setTrace((current) => ({
      ...current,
      target,
      submittedTarget: target,
      sessionId,
      phase: 'starting',
      resolvedTarget: '',
      actualProtocol: '',
      hops: [],
      durationMs: 0,
      resultStatus: '',
      error: ''
    }))
    try {
      await api().StartTraceroute(request)
      setTrace((current) =>
        current.sessionId === sessionId && current.phase === 'starting'
          ? { ...current, phase: 'resolving' }
          : current
      )
    } catch (error) {
      setTrace((current) =>
        current.sessionId === sessionId
          ? {
              ...current,
              sessionId: '',
              phase: 'idle',
              resultStatus: '',
              error: errorMessage(error)
            }
          : current
      )
    }
  }

  const stopTraceroute = async () => {
    const sessionId = trace.sessionId
    if (!sessionId) return
    setTrace((current) =>
      current.sessionId === sessionId ? { ...current, phase: 'stopping' } : current
    )
    try {
      await api().CancelTraceroute(sessionId)
    } catch (error) {
      setTrace((current) =>
        current.sessionId === sessionId
          ? { ...current, phase: 'error', resultStatus: 'error', error: errorMessage(error) }
          : current
      )
    }
  }
  return { trace, updateTrace, startTraceroute, stopTraceroute }
}

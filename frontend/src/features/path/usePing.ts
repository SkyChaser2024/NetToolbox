import { useEffect, useRef, useState } from 'react'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { api } from '../../api'
import type { PingEvent } from '../../api'
import type { PingViewState } from './types'
import { initialPingState } from './types'
import { errorMessage, createSessionID } from '../../utils/format'

export function usePing(active: boolean) {
  const [ping, setPing] = useState<PingViewState>(initialPingState)
  useEffect(() => {
    if (!window.go?.desktop?.App) return
    return EventsOn('ping:event', (event: PingEvent) => {
      setPing((current) => {
        if (!current.sessionId || event.sessionId !== current.sessionId) return current
        if (current.phase === 'cancelled' && event.type !== 'cancelled') return current
        if (current.phase === 'stopping' && (event.type === 'started' || event.type === 'reply'))
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
        if (event.type === 'reply' && event.reply) {
          const replies = current.replies.filter(
            (reply) => reply.sequence !== event.reply?.sequence
          )
          replies.push(event.reply)
          replies.sort((left, right) => left.sequence - right.sequence)
          return { ...current, phase: 'running', replies }
        }
        if (event.type === 'completed') {
          return {
            ...current,
            phase: 'completed',
            resolvedTarget: event.summary?.address || current.resolvedTarget,
            actualProtocol: event.summary?.protocol || current.actualProtocol,
            summary: event.summary || current.summary,
            error: ''
          }
        }
        if (event.type === 'cancelled') {
          return {
            ...current,
            phase: 'cancelled',
            resolvedTarget: event.summary?.address || current.resolvedTarget,
            actualProtocol: event.summary?.protocol || current.actualProtocol,
            summary: event.summary || current.summary
          }
        }
        if (event.type === 'error') {
          return {
            ...current,
            phase: 'error',
            resolvedTarget: event.summary?.address || current.resolvedTarget,
            actualProtocol: event.summary?.protocol || current.actualProtocol,
            summary: event.summary || current.summary,
            error: event.error || 'Ping 测试失败'
          }
        }
        return current
      })
    })
  }, [])
  useEffect(() => {
    if (active || (ping.phase !== 'resolving' && ping.phase !== 'running') || !ping.sessionId)
      return
    const sessionId = ping.sessionId
    setPing((current) =>
      current.sessionId === sessionId ? { ...current, phase: 'cancelled' } : current
    )
    void api().CancelPing(sessionId)
  }, [active, ping.phase, ping.sessionId])
  const sessionRef = useRef('')
  sessionRef.current = ping.sessionId
  useEffect(
    () => () => {
      if (sessionRef.current) void api().CancelPing(sessionRef.current)
    },
    []
  )
  const updatePing = (values: Partial<PingViewState>) =>
    setPing((current) => ({ ...current, ...values }))

  const startPing = async () => {
    const target = ping.target.trim() || 'baidu.com'
    const sessionId = createSessionID('ping')
    const request = {
      sessionId,
      target,
      protocol: ping.protocol,
      count: ping.count,
      timeoutMs: ping.timeoutMs,
      intervalMs: ping.intervalMs
    }
    setPing((current) => ({
      ...current,
      target,
      submittedTarget: target,
      sessionId,
      phase: 'starting',
      resolvedTarget: '',
      actualProtocol: '',
      replies: [],
      summary: null,
      error: ''
    }))
    try {
      await api().StartPing(request)
      setPing((current) =>
        current.sessionId === sessionId && current.phase === 'starting'
          ? { ...current, phase: 'resolving' }
          : current
      )
    } catch (error) {
      setPing((current) =>
        current.sessionId === sessionId
          ? { ...current, sessionId: '', phase: 'idle', error: errorMessage(error) }
          : current
      )
    }
  }

  const stopPing = async () => {
    const sessionId = ping.sessionId
    if (!sessionId) return
    setPing((current) =>
      current.sessionId === sessionId ? { ...current, phase: 'stopping' } : current
    )
    try {
      await api().CancelPing(sessionId)
    } catch (error) {
      setPing((current) =>
        current.sessionId === sessionId
          ? { ...current, phase: 'error', error: errorMessage(error) }
          : current
      )
    }
  }
  return { ping, updatePing, startPing, stopPing }
}

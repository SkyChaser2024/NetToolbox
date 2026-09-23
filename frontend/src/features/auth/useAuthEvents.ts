import { useEffect, useState } from 'react'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import type { AuthEvent, AuthState } from '../../api'

export function useAuthEvents(initialState?: AuthState) {
  const [authState, setAuthState] = useState<AuthState>('idle')
  const [logs, setLogs] = useState<AuthEvent[]>([])
  useEffect(() => {
    if (initialState) setAuthState(initialState)
  }, [initialState])
  useEffect(() => {
    if (!window.go?.desktop?.App) return
    return EventsOn('auth:event', (event: AuthEvent) => {
      setAuthState(event.state)
      setLogs((current) => [...current.slice(-79), event])
    })
  }, [])
  return { authState, logs }
}

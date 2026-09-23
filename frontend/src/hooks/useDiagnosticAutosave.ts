import { useCallback, useEffect, useRef } from 'react'
import type { DiagnosticSettings } from '../api'
import type { SettingsSaver } from '../appTypes'
import { errorMessage } from '../utils/format'
import { useDiagnosticAutosaveState } from './DiagnosticDraftProvider'

export function useDiagnosticAutosave(
  draftKey: string,
  build: (current: DiagnosticSettings) => DiagnosticSettings,
  onSaveSettings: SettingsSaver,
  onError: (message: string) => void,
  editorId?: string
) {
  // Compare against the last queued value, not the last completed save: reverting
  // an in-flight edit must enqueue another update to restore the original value.
  const state = useDiagnosticAutosaveState(editorId, draftKey)
  const buildRef = useRef(build)
  useEffect(() => {
    buildRef.current = build
  }, [build])

  const flush = useCallback(() => {
    const snapshot = state.pending
    if (!snapshot) return
    state.pending = null
    const submission = { key: snapshot.key }
    state.submitted = submission
    void onSaveSettings((current) => ({
      profile: current.profile,
      system: current.system,
      // Capture this draft but apply it to the queue's latest settings snapshot.
      diagnostics: snapshot.build(current.diagnostics)
    })).catch((error) => {
      // Only invalidate this failed submission. A newer queued version must
      // retain its identity, even when it happens to contain the same value.
      if (state.submitted === submission) state.submitted = null
      onError(errorMessage(error))
    })
  }, [onSaveSettings, onError, state])

  useEffect(() => {
    state.pending =
      draftKey === state.submitted?.key ? null : { key: draftKey, build: buildRef.current }
    if (!state.pending) return
    const timer = setTimeout(flush, 700)
    return () => clearTimeout(timer)
  }, [draftKey, flush, state])

  // Feature pages unmount on navigation. Flush their latest pending edit rather
  // than discarding it when the debounce timer is cancelled during cleanup.
  useEffect(() => flush, [flush])
}

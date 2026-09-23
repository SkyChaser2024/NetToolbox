import { createContext, useCallback, useContext, useState } from 'react'
import type { ReactNode } from 'react'
import type { DiagnosticSettings } from '../api'

type DiagnosticBuilder = (current: DiagnosticSettings) => DiagnosticSettings

interface AutosaveState {
  submitted: { key: string } | null
  pending: { key: string; build: DiagnosticBuilder } | null
}

interface DraftStore {
  drafts: Map<string, string>
  autosaves: Map<string, AutosaveState>
}

const DraftContext = createContext<DraftStore | null>(null)

export function DiagnosticDraftProvider({ children }: { children: ReactNode }) {
  const [store] = useState<DraftStore>(() => ({ drafts: new Map(), autosaves: new Map() }))
  return <DraftContext.Provider value={store}>{children}</DraftContext.Provider>
}

// The app owns drafts across page navigation. Save responses update persisted
// settings separately, so a late response cannot replace more recent input.
export function useDiagnosticDraft(id: string, initial: string) {
  const store = useContext(DraftContext)
  const [value, setValue] = useState(() => store?.drafts.get(id) ?? initial)
  const update = useCallback(
    (next: string) => {
      store?.drafts.set(id, next)
      setValue(next)
    },
    [id, store]
  )
  return [value, update] as const
}

export function useDiagnosticAutosaveState(id: string | undefined, initialKey: string) {
  const store = useContext(DraftContext)
  const [state] = useState<AutosaveState>(() => {
    const existing = id ? store?.autosaves.get(id) : undefined
    if (existing) return existing
    const created = { submitted: { key: initialKey }, pending: null }
    if (id) store?.autosaves.set(id, created)
    return created
  })
  return state
}

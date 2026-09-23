import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api } from '../api'
import type { BootstrapData } from '../api'
import { errorMessage } from '../utils/format'
import type { SetStateAction } from 'react'
import { createSettingsSaver } from '../utils/settingsSaver'

export function useBootstrapSettings(onError: (message: string) => void) {
  const [bootstrap, setBootstrap] = useState<BootstrapData | null>(null)
  const [loaded, setLoaded] = useState(false)
  const bootstrapRef = useRef<BootstrapData | null>(null)
  const updateBootstrap = useCallback((update: SetStateAction<BootstrapData | null>) => {
    const value = typeof update === 'function' ? update(bootstrapRef.current) : update
    bootstrapRef.current = value
    setBootstrap(value)
  }, [])
  const saveSettings = useMemo(
    () =>
      createSettingsSaver(
        () => bootstrapRef.current,
        updateBootstrap,
        async (request) => {
          const result = await api().SaveSettings(request)
          if (result.warning) onError(result.warning)
          return result
        }
      ),
    [onError, updateBootstrap]
  )
  useEffect(() => {
    let disposed = false
    void api()
      .Bootstrap()
      .then((data) => {
        if (disposed) return
        updateBootstrap(data)
        if (data.configurationError) onError(data.configurationError)
        if (data.backgroundWarning) onError(data.backgroundWarning)
      })
      .catch((error) => {
        if (!disposed) onError(errorMessage(error))
      })
      .finally(() => {
        if (!disposed) setLoaded(true)
      })
    return () => {
      disposed = true
    }
  }, [onError, updateBootstrap])
  return { bootstrap, loaded, updateBootstrap, saveSettings }
}

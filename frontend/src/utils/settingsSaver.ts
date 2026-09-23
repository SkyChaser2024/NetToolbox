import type { BootstrapData, SettingsRequest, SettingsResult } from '../api'
import type { SettingsSaver } from '../appTypes'

// All feature editors share this queue. Commit the returned snapshot before
// starting the next operation, without waiting for a React render to occur.
export function createSettingsSaver(
  read: () => BootstrapData | null,
  write: (value: BootstrapData) => void,
  persist: (request: SettingsRequest) => Promise<SettingsResult>
): SettingsSaver {
  let queue = Promise.resolve()
  return (update) => {
    const operation = queue.then(async () => {
      const current = read()
      if (!current) return
      const stored = await persist(update(current))
      const latest = read()
      if (latest) write({ ...latest, ...stored })
    })
    queue = operation.catch(() => undefined)
    return operation
  }
}

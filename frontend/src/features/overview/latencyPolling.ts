const pollIntervalMs = 618 * 3
const maxConcurrentProbes = 4

// One scheduler for all targets. Samples accumulate at the normal cadence,
// rather than filling every target's history with an immediate request burst.
export function startLatencyPolling<T, R>({
  targets,
  probe,
  onResult,
  onError
}: {
  targets: readonly T[]
  probe: (target: T) => Promise<R>
  onResult: (target: T, result: R) => void
  onError?: (target: T, error: unknown) => void
}) {
  const jobs = targets.map((target) => ({ target, due: 0, running: false }))
  let stopped = false
  let active = 0
  let timer: ReturnType<typeof setTimeout> | undefined

  const pump = () => {
    if (stopped) return
    clearTimeout(timer)
    timer = undefined
    const now = Date.now()
    // Oldest due target wins, including targets which have never been sampled.
    // Fixed list order would starve the tail when earlier targets are slow.
    for (const job of [...jobs].sort((left, right) => left.due - right.due)) {
      if (active >= maxConcurrentProbes) break
      if (job.running || job.due > now) continue
      job.running = true
      active++
      void (async () => {
        try {
          const result = await probe(job.target)
          if (!stopped) onResult(job.target, result)
        } catch (error) {
          if (!stopped) onError?.(job.target, error)
        } finally {
          active--
          job.running = false
          job.due = Date.now() + pollIntervalMs
          pump()
        }
      })()
    }
    if (active >= maxConcurrentProbes) return
    const next = jobs.reduce((due, job) => (job.running ? due : Math.min(due, job.due)), Infinity)
    if (Number.isFinite(next)) timer = setTimeout(pump, Math.max(0, next - Date.now()))
  }

  pump()
  return () => {
    stopped = true
    clearTimeout(timer)
  }
}

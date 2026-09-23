import { PingTool } from './PingTool'
import { TracerouteTool } from './TracerouteTool'
import { PingViewState, TraceViewState } from './types'

export function NetworkPathPage({
  ping,
  trace,
  onPingChange,
  onPingStart,
  onPingStop,
  onTraceChange,
  onTraceStart,
  onTraceStop
}: {
  ping: PingViewState
  trace: TraceViewState
  onPingChange: (values: Partial<PingViewState>) => void
  onPingStart: () => void
  onPingStop: () => void
  onTraceChange: (values: Partial<TraceViewState>) => void
  onTraceStart: () => void
  onTraceStop: () => void
}) {
  return (
    <div className="tool-page trace-page">
      <PingTool value={ping} onChange={onPingChange} onStart={onPingStart} onStop={onPingStop} />
      <TracerouteTool
        value={trace}
        onChange={onTraceChange}
        onStart={onTraceStart}
        onStop={onTraceStop}
      />
    </div>
  )
}

import { useCallback, useId, useLayoutEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { createPortal } from 'react-dom'

export function LatencyTooltip({
  children,
  label,
  content
}: {
  children: ReactNode
  label: string
  content: ReactNode
}) {
  const triggerRef = useRef<HTMLSpanElement>(null)
  const tooltipRef = useRef<HTMLDivElement>(null)
  const tooltipId = useId()
  const [open, setOpen] = useState(false)
  const [position, setPosition] = useState({
    left: 12,
    top: 12,
    placement: 'bottom' as 'top' | 'bottom'
  })

  const updatePosition = useCallback(() => {
    const trigger = triggerRef.current
    if (!trigger) return
    const triggerRect = trigger.getBoundingClientRect()
    const tooltipRect = tooltipRef.current?.getBoundingClientRect()
    const width = tooltipRect?.width || 250
    const height = tooltipRect?.height || 104
    const viewportPadding = 12
    const gap = 8
    const below = triggerRect.bottom + gap
    const placement =
      below + height <= window.innerHeight - viewportPadding ||
      triggerRect.top < height + gap + viewportPadding
        ? 'bottom'
        : 'top'
    const unclampedLeft = triggerRect.left + triggerRect.width / 2 - width / 2
    const maxLeft = Math.max(viewportPadding, window.innerWidth - width - viewportPadding)
    setPosition({
      left: Math.min(maxLeft, Math.max(viewportPadding, unclampedLeft)),
      top:
        placement === 'bottom' ? below : Math.max(viewportPadding, triggerRect.top - height - gap),
      placement
    })
  }, [])

  useLayoutEffect(() => {
    if (!open) return
    updatePosition()
    const refresh = () => updatePosition()
    window.addEventListener('resize', refresh)
    window.addEventListener('scroll', refresh, true)
    return () => {
      window.removeEventListener('resize', refresh)
      window.removeEventListener('scroll', refresh, true)
    }
  }, [open, updatePosition])

  const show = () => {
    updatePosition()
    setOpen(true)
  }
  return (
    <>
      <span
        ref={triggerRef}
        className="service-tooltip-trigger"
        tabIndex={0}
        aria-label={label}
        aria-describedby={open ? tooltipId : undefined}
        onMouseEnter={show}
        onMouseLeave={() => setOpen(false)}
        onFocus={show}
        onBlur={() => setOpen(false)}
        onKeyDown={(event) => {
          if (event.key === 'Escape') setOpen(false)
        }}
      >
        {children}
      </span>
      {open &&
        createPortal(
          <div
            ref={tooltipRef}
            id={tooltipId}
            className="latency-tooltip"
            role="tooltip"
            data-placement={position.placement}
            style={{ left: position.left, top: position.top }}
          >
            {content}
          </div>,
          document.body
        )}
    </>
  )
}

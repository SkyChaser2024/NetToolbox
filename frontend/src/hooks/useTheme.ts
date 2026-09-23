import { useEffect, useState } from 'react'
import {
  WindowSetDarkTheme,
  WindowSetLightTheme,
  WindowSetSystemDefaultTheme
} from '../../wailsjs/runtime/runtime'
import type { Theme } from '../appTypes'

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(loadTheme)
  useEffect(() => {
    document.documentElement.dataset.theme = theme
    localStorage.setItem('theme', theme)
    if (!window.go?.desktop?.App) return
    const frame = window.requestAnimationFrame(() => {
      if (theme === 'dark') WindowSetDarkTheme()
      else if (theme === 'light') WindowSetLightTheme()
      else WindowSetSystemDefaultTheme()
    })
    return () => window.cancelAnimationFrame(frame)
  }, [theme])
  const cycleTheme = () =>
    setTheme((current) =>
      current === 'dark'
        ? 'light'
        : current === 'light'
          ? 'dark'
          : window.matchMedia('(prefers-color-scheme: dark)').matches
            ? 'light'
            : 'dark'
    )
  return { theme, cycleTheme }
}
function loadTheme(): Theme {
  const stored = localStorage.getItem('theme')
  return stored === 'light' || stored === 'dark' ? stored : 'system'
}

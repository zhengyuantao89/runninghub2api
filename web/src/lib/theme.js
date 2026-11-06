import { getStringPreference, preferenceKeys, setStringPreference } from './preferences'

export function normalizeThemeMode(value) {
  if (value === 'dark' || value === 'mono-dark') return 'dark'
  if (value === 'light') return 'light'
  return 'system'
}

export function getStoredThemeMode() {
  return normalizeThemeMode(getStringPreference(preferenceKeys.themeMode, 'system'))
}

export function resolveThemeMode(mode) {
  if (mode !== 'system') return mode
  if (typeof window === 'undefined') return 'light'
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export function applyThemeMode(mode) {
  if (typeof document === 'undefined') return
  const root = document.documentElement
  const resolved = resolveThemeMode(mode)
  root.dataset.themePreference = mode
  if (resolved === 'dark') {
    root.dataset.theme = 'dark'
    root.style.colorScheme = 'dark'
    return
  }
  delete root.dataset.theme
  root.style.colorScheme = 'light'
}

export function setStoredThemeMode(mode) {
  const normalized = normalizeThemeMode(mode)
  setStringPreference(preferenceKeys.themeMode, normalized)
  applyThemeMode(normalized)
}

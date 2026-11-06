export const preferenceKeys = {
  sidebarCollapsed: 'rh2api.sidebar-collapsed',
  themeMode: 'rh2api.theme-mode',
}

function storage() {
  if (typeof window === 'undefined') return null
  try {
    return window.localStorage
  } catch {
    return null
  }
}

export function getStringPreference(key, fallback = '') {
  const value = storage()?.getItem(key)
  return value == null ? fallback : value
}

export function setStringPreference(key, value) {
  storage()?.setItem(key, value)
}

export function getBooleanPreference(key, fallback = false) {
  const value = getStringPreference(key)
  if (!value) return fallback
  return value === 'true'
}

export function setBooleanPreference(key, value) {
  setStringPreference(key, value ? 'true' : 'false')
}

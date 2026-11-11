const ROLE_KEY = 'rh2api.role'

export function getRole() {
  return localStorage.getItem(ROLE_KEY) || ''
}

export function setRole(role) {
  const v = String(role || '').trim()
  if (v) localStorage.setItem(ROLE_KEY, v)
  else localStorage.removeItem(ROLE_KEY)
}

export function clearRole() {
  localStorage.removeItem(ROLE_KEY)
}

export function isAdminRole(role = getRole()) {
  return role === 'admin'
}

export function homePathForRole(role = getRole()) {
  return isAdminRole(role) ? '/admin' : '/app'
}

export function consoleBaseFromPath(path = '') {
  return String(path).startsWith('/admin') ? '/admin' : '/app'
}

export function withConsole(base, suffix = '') {
  const root = base === '/admin' ? '/admin' : '/app'
  const rest = String(suffix || '').replace(/^\//, '')
  return rest ? `${root}/${rest}` : root
}

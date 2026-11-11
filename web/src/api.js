import { clearRole, getRole, setRole } from './lib/auth'

const KEYS = {
  session: 'rh2api.session',
  // legacy cleanup
  auth: 'rh2api.authKey',
  admin: 'rh2api.adminKey',
}

export function getSession() {
  return localStorage.getItem(KEYS.session) || ''
}

export function setSession(token, role = '') {
  if (token) localStorage.setItem(KEYS.session, token)
  else localStorage.removeItem(KEYS.session)
  if (role) setRole(role)
}

export function clearSession() {
  localStorage.removeItem(KEYS.session)
  localStorage.removeItem(KEYS.auth)
  localStorage.removeItem(KEYS.admin)
  clearRole()
}

export { getRole, setRole }

async function request(path, { method = 'GET', body, formData, auth = true } = {}) {
  const headers = {}
  if (auth) {
    const token = getSession()
    if (token) headers.Authorization = `Bearer ${token}`
  }
  let payload = body
  if (formData) {
    payload = formData
  } else if (body != null) {
    headers['Content-Type'] = 'application/json'
    payload = JSON.stringify(body)
  }
  const res = await fetch(path, { method, headers, body: payload })
  const text = await res.text()
  let data = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    data = { raw: text }
  }
  if (!res.ok) {
    let message = data?.error || data?.message || `HTTP ${res.status}`
    const unauthorized =
      res.status === 401 || /unauthorized|未登录|登录已失效/i.test(String(message))
    if (unauthorized) {
      message = '未登录或登录已失效，请重新登录'
      if (auth) {
        clearSession()
        const pathNow = window.location.pathname || ''
        if (!pathNow.startsWith('/login')) {
          const redirect = `${pathNow}${window.location.search || ''}`
          window.location.assign(`/login?redirect=${encodeURIComponent(redirect)}`)
        }
      }
    }
    const err = new Error(message)
    err.status = res.status
    err.data = data
    throw err
  }
  return data
}

export const api = {
  health: () => request('/health', { auth: false }),
  login: (username, password) =>
    request('/api/login', { method: 'POST', body: { username, password }, auth: false }),
  register: (username, password) =>
    request('/api/register', { method: 'POST', body: { username, password }, auth: false }),
  registrationStatus: () => request('/api/registration', { auth: false }),
  logout: () => request('/api/logout', { method: 'POST' }),
  me: () => request('/api/me'),
  console: () => request('/api/console'),
  getSettings: () => request('/admin/settings'),
  updateSettings: (body) => request('/admin/settings', { method: 'PUT', body }),
  updateAdminCredentials: (body) =>
    request('/admin/credentials', { method: 'POST', body }),

  listKeys: () => request('/api/keys'),
  createKey: (name = '') => request('/api/keys', { method: 'POST', body: { name } }),
  deleteKey: (id) => request(`/api/keys/${id}`, { method: 'DELETE' }),

  listUsers: () => request('/admin/users'),
  createUser: (body) => request('/admin/users', { method: 'POST', body }),
  disableUser: (id) => request(`/admin/users/${id}/disable`, { method: 'POST' }),
  enableUser: (id) => request(`/admin/users/${id}/enable`, { method: 'POST' }),
  setUserPassword: (id, password) =>
    request(`/admin/users/${id}/password`, { method: 'POST', body: { password } }),

  listAccounts: (opts = {}) => {
    const q = new URLSearchParams()
    if (opts.page != null) q.set('page', String(opts.page))
    if (opts.pageSize != null) q.set('page_size', String(opts.pageSize))
    if (opts.q) q.set('q', String(opts.q))
    if (opts.owner) q.set('owner', String(opts.owner))
    const qs = q.toString()
    return request(`/admin/accounts${qs ? `?${qs}` : ''}`)
  },
  getAccount: (id) => request(`/admin/accounts/${id}`),
  createAccount: (account) => request('/admin/accounts', { method: 'POST', body: account }),
  importAccounts: (accounts) =>
    request('/admin/accounts/import', { method: 'POST', body: { accounts, source: 'manual' } }),
  pushAccounts: (body) => request('/api/accounts/push', { method: 'POST', body }),
  deleteAccount: (id) => request(`/admin/accounts/${id}`, { method: 'DELETE' }),
  refreshAccount: (id) => request(`/admin/accounts/${id}/refresh`, { method: 'POST' }),
  ensureAccountApiKey: (id, force = false) =>
    request(`/admin/accounts/${id}/ensure-api-key`, { method: 'POST', body: { force: !!force } }),
  ensureAccountsApiKey: (ids, force = false) =>
    request('/admin/accounts/ensure-api-key', { method: 'POST', body: { ids, force: !!force } }),
  claimDailyLogin: (ids = [], force = false) =>
    request('/admin/accounts/claim-daily-login', {
      method: 'POST',
      body: { ids: Array.isArray(ids) ? ids : [], force: !!force },
    }),
  disableAccount: (id) => request(`/admin/accounts/${id}/disable`, { method: 'POST' }),
  enableAccount: (id) => request(`/admin/accounts/${id}/enable`, { method: 'POST' }),
  setAccountMaxConcurrency: (id, max_concurrency) =>
    request(`/admin/accounts/${id}/max-concurrency`, {
      method: 'POST',
      body: { max_concurrency: Number(max_concurrency) || 1 },
    }),
  listAdminTasks: () => request('/admin/tasks'),
  listVideos: () => request('/v1/videos'),
  getVideo: (id) => request(`/v1/videos/${id}`),
  cancelVideo: (id) => request(`/v1/videos/${id}/cancel`, { method: 'POST' }),
  syncVideo: (id) => request(`/v1/videos/${id}/sync`, { method: 'POST' }),
  createVideoForm: (formData) => request('/v1/videos', { method: 'POST', formData }),
  getQCCatalog: (category = 'VIDEO', { mode = '', refresh = false } = {}) => {
    const q = new URLSearchParams()
    if (category) q.set('category', category)
    if (mode) q.set('mode', mode)
    if (refresh) q.set('refresh', '1')
    const qs = q.toString()
    return request(`/v1/qc/catalog${qs ? `?${qs}` : ''}`)
  },
  getWebappDemo: (webappId, { refresh = false } = {}) =>
    request(
      `/v1/webapps/${encodeURIComponent(webappId)}/demo${refresh ? '?refresh=1' : ''}`,
    ),
  listTemplates: () => request('/admin/templates'),
  getTemplate: (id) => request(`/admin/templates/${encodeURIComponent(id)}`),
  saveTemplate: (body) => request('/admin/templates', { method: 'POST', body }),
  deleteTemplate: (id) => request(`/admin/templates/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  listProxies: (group = '', page = 1, pageSize = 50) => {
    const q = new URLSearchParams()
    if (group) q.set('group', group)
    q.set('page', String(page || 1))
    q.set('page_size', String(pageSize || 50))
    return request(`/admin/proxies?${q.toString()}`)
  },
  listProxyGroups: () => request('/admin/proxies/groups'),
  importProxies: (items, source = 'manual') =>
    request('/admin/proxies/import', { method: 'POST', body: { items, source } }),
  importProxiesText: (text, group_name = 'nodes', exact_sync = true) =>
    request('/admin/proxies/import', {
      method: 'POST',
      body: { text, group_name, exact_sync, source: 'text' },
    }),
  getProxySubscription: () => request('/admin/proxies/subscriptions'),
  saveProxySubscription: (body) =>
    request('/admin/proxies/subscriptions', { method: 'PUT', body }),
  refreshProxySubscription: () =>
    request('/admin/proxies/subscriptions/refresh', { method: 'POST' }),
  deleteProxySubscription: (clearGroup = false) =>
    request('/admin/proxies/subscriptions', {
      method: 'DELETE',
      body: { clear_group: !!clearGroup },
    }),
  deleteProxy: (id) => request(`/admin/proxies/${id}`, { method: 'DELETE' }),
  enableProxy: (id) => request(`/admin/proxies/${id}/enable`, { method: 'POST' }),
  disableProxy: (id) => request(`/admin/proxies/${id}/disable`, { method: 'POST' }),
  testProxy: (id) => request(`/admin/proxies/${id}/test`, { method: 'POST' }),
}

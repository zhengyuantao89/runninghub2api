const KEYS = {
  session: 'rh2api.session',
  // legacy cleanup
  auth: 'rh2api.authKey',
  admin: 'rh2api.adminKey',
};

export function getSession() {
  return localStorage.getItem(KEYS.session) || '';
}

export function setSession(token) {
  if (token) localStorage.setItem(KEYS.session, token);
  else localStorage.removeItem(KEYS.session);
}

export function clearSession() {
  localStorage.removeItem(KEYS.session);
  localStorage.removeItem(KEYS.auth);
  localStorage.removeItem(KEYS.admin);
}

async function request(path, { method = 'GET', body, formData, auth = true } = {}) {
  const headers = {};
  if (auth) {
    const token = getSession();
    if (token) headers.Authorization = `Bearer ${token}`;
  }
  let payload = body;
  if (formData) {
    payload = formData;
  } else if (body != null) {
    headers['Content-Type'] = 'application/json';
    payload = JSON.stringify(body);
  }
  const res = await fetch(path, { method, headers, body: payload });
  const text = await res.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = { raw: text };
  }
  if (!res.ok) {
    let message = data?.error || data?.message || `HTTP ${res.status}`;
    const unauthorized =
      res.status === 401 || /unauthorized|未登录|登录已失效/i.test(String(message));
    if (unauthorized) {
      message = '未登录或登录已失效，请重新登录';
      // 会话失效：清本地态并回登录页（避免停在业务页只显示红字）
      if (auth) {
        clearSession();
        const path = window.location.pathname || '';
        if (!path.startsWith('/login')) {
          const redirect = `${path}${window.location.search || ''}`;
          window.location.assign(`/login?redirect=${encodeURIComponent(redirect)}`);
        }
      }
    }
    const err = new Error(message);
    err.status = res.status;
    err.data = data;
    throw err;
  }
  return data;
}

export const api = {
  health: () => request('/health', { auth: false }),
  login: (username, password) =>
    request('/api/login', { method: 'POST', body: { username, password }, auth: false }),
  logout: () => request('/api/logout', { method: 'POST' }),
  me: () => request('/api/me'),
  console: () => request('/api/console'),

  listAccounts: () => request('/admin/accounts'),
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
  getWebappDemo: (webappId, { refresh = false } = {}) =>
    request(
      `/v1/webapps/${encodeURIComponent(webappId)}/demo${refresh ? '?refresh=1' : ''}`,
    ),
  listTemplates: () => request('/admin/templates'),
  getTemplate: (id) => request(`/admin/templates/${encodeURIComponent(id)}`),
  saveTemplate: (body) => request('/admin/templates', { method: 'POST', body }),
  deleteTemplate: (id) => request(`/admin/templates/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  listProxies: (group = '') =>
    request(`/admin/proxies${group ? `?group=${encodeURIComponent(group)}` : ''}`),
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
};

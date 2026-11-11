import { createRouter, createWebHistory } from 'vue-router'
import { getSession, getRole, setRole, api } from './api'
import { homePathForRole, isAdminRole } from './lib/auth'
import AppShell from './layouts/AppShell.vue'
import Login from './views/Login.vue'
import Dashboard from './views/Dashboard.vue'
import Accounts from './views/Accounts.vue'
import Tasks from './views/Tasks.vue'
import Studio from './views/Studio.vue'
import Settings from './views/Settings.vue'
import ApiDocs from './views/ApiDocs.vue'
import Webapps from './views/Webapps.vue'
import Egress from './views/Egress.vue'
import ApiKeys from './views/ApiKeys.vue'
import Users from './views/Users.vue'

const userChildren = [
  { path: '', component: Dashboard, meta: { title: '概览中心' } },
  { path: 'accounts', component: Accounts, meta: { title: '号池管理' } },
  { path: 'webapps', component: Webapps, meta: { title: '应用模板' } },
  { path: 'tasks', component: Tasks, meta: { title: '任务队列' } },
  { path: 'studio', component: Studio, meta: { title: '试跑工作室' } },
  { path: 'keys', component: ApiKeys, meta: { title: 'API Key' } },
  { path: 'docs', component: ApiDocs, meta: { title: 'API 说明' } },
  { path: 'settings', component: Settings, meta: { title: '设置' } },
]

const adminChildren = [
  { path: '', component: Dashboard, meta: { title: '概览中心' } },
  { path: 'accounts', component: Accounts, meta: { title: '号池管理' } },
  { path: 'egress', component: Egress, meta: { title: '出口池' } },
  { path: 'webapps', component: Webapps, meta: { title: '应用模板' } },
  { path: 'tasks', component: Tasks, meta: { title: '任务队列' } },
  { path: 'studio', component: Studio, meta: { title: '试跑工作室' } },
  { path: 'keys', component: ApiKeys, meta: { title: 'API Key' } },
  { path: 'users', component: Users, meta: { title: '用户管理' } },
  { path: 'docs', component: ApiDocs, meta: { title: 'API 说明' } },
  { path: 'settings', component: Settings, meta: { title: '系统设置' } },
]

/** Old flat console paths → suffix under /app or /admin */
const legacyMap = {
  '/accounts': 'accounts',
  '/egress': 'egress',
  '/webapps': 'webapps',
  '/tasks': 'tasks',
  '/studio': 'studio',
  '/docs': 'docs',
  '/settings': 'settings',
  '/keys': 'keys',
  '/users': 'users',
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login, meta: { public: true, title: '登录' } },
    {
      path: '/app',
      component: AppShell,
      meta: { console: 'user' },
      children: userChildren,
    },
    {
      path: '/admin',
      component: AppShell,
      meta: { console: 'admin' },
      children: adminChildren,
    },
  ],
})

async function ensureRole() {
  let role = getRole()
  if (role) return role
  if (!getSession()) return ''
  try {
    const me = await api.me()
    role = me?.role || ''
    if (role) setRole(role)
  } catch {
    return ''
  }
  return role
}

function consoleHome(role) {
  return homePathForRole(role)
}

function mapLegacy(path, role) {
  const home = consoleHome(role)
  const admin = isAdminRole(role)
  if (path === '/' || path === '') return home

  const suffix = legacyMap[path]
  if (suffix == null) return home

  if (!admin && (suffix === 'egress' || suffix === 'users')) return home
  return `${home}/${suffix}`
}

router.beforeEach(async (to) => {
  if (to.meta.public) return true

  if (!getSession()) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }

  const role = await ensureRole()
  if (!role) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }

  const home = consoleHome(role)
  const admin = isAdminRole(role)
  const inConsole = to.matched.some((r) => r.meta?.console)

  // /, legacy flat paths, or unknown → once to console home
  if (!inConsole) {
    const next = mapLegacy(to.path, role)
    if (next === to.path) return true
    return next
  }

  if (to.path.startsWith('/admin') && !admin) {
    return home === to.path ? true : home
  }
  if (to.path.startsWith('/app') && admin) {
    return home === to.path ? true : home
  }

  return true
})

export default router

import { createRouter, createWebHistory } from 'vue-router'
import { getSession } from './api'
import Login from './views/Login.vue'
import Dashboard from './views/Dashboard.vue'
import Accounts from './views/Accounts.vue'
import Tasks from './views/Tasks.vue'
import Studio from './views/Studio.vue'
import Settings from './views/Settings.vue'
import ApiDocs from './views/ApiDocs.vue'
import Webapps from './views/Webapps.vue'
import Egress from './views/Egress.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login, meta: { public: true, title: '登录' } },
    { path: '/', component: Dashboard, meta: { title: '概览中心' } },
    { path: '/accounts', component: Accounts, meta: { title: '号池管理' } },
    { path: '/egress', component: Egress, meta: { title: '出口池' } },
    { path: '/webapps', component: Webapps, meta: { title: '应用模板' } },
    { path: '/tasks', component: Tasks, meta: { title: '任务队列' } },
    { path: '/studio', component: Studio, meta: { title: '试跑工作室' } },
    { path: '/docs', component: ApiDocs, meta: { title: 'API 说明' } },
    { path: '/settings', component: Settings, meta: { title: '系统设置' } },
  ],
})

router.beforeEach((to) => {
  if (to.meta.public) return true
  if (!getSession()) return '/login'
  return true
})

export default router

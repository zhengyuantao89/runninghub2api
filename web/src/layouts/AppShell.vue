<template>
  <div class="min-h-screen">
    <div class="flex min-h-screen flex-col lg:flex-row">
      <div
        v-if="isSidebarOpen"
        class="fixed inset-0 z-30 bg-black/20 lg:hidden"
        @click="isSidebarOpen = false"
      />
      <aside
        class="fixed inset-y-0 left-0 z-40 w-64 -translate-x-full overflow-x-hidden bg-card border-r border-border
               transition-[transform,width] duration-200 ease-out will-change-[transform,width] transform-gpu flex flex-col lg:static lg:translate-x-0 lg:w-[var(--sidebar-width)] lg:bg-card
               lg:border-b-0 lg:border-r lg:sticky lg:top-0 lg:h-screen"
        :class="[isSidebarOpen ? 'translate-x-0' : '']"
        :style="sidebarStyle"
      >
        <div
          class="flex h-16 items-center pt-4 lg:h-20 lg:pt-5"
          :class="isSidebarRail ? 'justify-center px-2' : 'justify-between px-6'"
        >
          <div class="flex w-full items-center" :class="isSidebarRail ? 'justify-center' : ''">
            <p v-if="!isSidebarRail" class="ui-section-title">runninghub2api</p>
            <span v-else class="text-sm font-semibold text-foreground" aria-label="runninghub2api">RH</span>
          </div>
        </div>

        <nav class="flex-1 pb-3 pt-3 lg:pt-4" :class="isSidebarRail ? 'px-2' : 'px-3'">
          <p
            v-if="!isSidebarRail"
            class="px-3 pb-2 text-xs uppercase tracking-[0.28em] text-muted-foreground"
          >
            导航
          </p>
          <div class="space-y-1">
            <RouterLink
              v-for="item in menuItems"
              :key="item.path"
              :to="item.path"
              class="group flex items-center overflow-hidden rounded-lg border border-transparent py-1.5 text-sm font-medium transition-colors"
              :class="navItemClassMap[item.path]"
              :title="isSidebarRail ? item.label : undefined"
              @click="isSidebarOpen = false"
            >
              <span
                class="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full border transition-colors"
                :class="navIconClassMap[item.path]"
              >
                <svg aria-hidden="true" viewBox="0 0 24 24" class="h-4 w-4" fill="currentColor">
                  <path :d="item.icon" />
                </svg>
              </span>
              <span v-if="!isSidebarRail" class="min-w-0 flex-1 truncate">{{ item.label }}</span>
            </RouterLink>
          </div>
        </nav>

        <div class="mt-auto border-t border-border py-3" :class="isSidebarRail ? 'px-2' : 'px-6'">
          <div class="flex items-center gap-3" :class="isSidebarRail ? 'justify-center' : ''">
            <Button
              v-if="!isSidebarRail"
              size="sm"
              variant="outline"
              block
              root-class="justify-center rounded-2xl text-muted-foreground"
              @click="handleLogout"
            >
              退出登录
            </Button>
            <Button
              size="xs"
              variant="outline"
              icon-only
              root-class="shrink-0 rounded-2xl text-muted-foreground"
              :title="isSidebarCollapsed ? '展开侧边栏' : '收起侧边栏'"
              @click="toggleSidebar"
            >
              <svg aria-hidden="true" viewBox="0 0 24 24" class="h-4 w-4 shrink-0" fill="currentColor">
                <path d="M6 4h2v16H6V4zm4 4h8v2h-8V8zm0 6h8v2h-8v-2z" />
              </svg>
            </Button>
          </div>
        </div>
      </aside>

      <main class="relative min-w-0 flex-1 overflow-hidden lg:ml-0">
        <header
          class="flex min-w-0 flex-col gap-4 border-b border-border bg-card px-6 py-5 lg:flex-row lg:items-center lg:justify-between lg:px-10"
        >
          <div class="flex items-center gap-3">
            <Button
              size="xs"
              variant="outline"
              icon-only
              root-class="lg:hidden"
              aria-label="打开导航"
              @click="isSidebarOpen = true"
            >
              <svg aria-hidden="true" viewBox="0 0 24 24" class="h-5 w-5" fill="currentColor">
                <path d="M4 6h16v2H4V6zm0 5h16v2H4v-2zm0 5h16v2H4v-2z" />
              </svg>
            </Button>
            <div class="flex min-w-0 flex-wrap items-center gap-2">
              <h2 class="text-xl font-semibold text-foreground lg:text-2xl">
                {{ currentPageTitle }}
              </h2>
            </div>
          </div>
          <div class="flex flex-wrap items-center gap-3">
            <Button size="sm" variant="outline" :title="themeButtonTitle" @click="cycleThemeMode">
              {{ themeButtonText }}
            </Button>
            <Button size="sm" variant="outline" title="刷新" @click="refreshPage">刷新</Button>
            <Button size="sm" variant="outline" title="接口信息" @click="router.push('/docs')">接口信息</Button>
          </div>
        </header>

        <div class="relative h-full overflow-x-hidden overflow-y-auto bg-card px-4 pb-10 pt-6 lg:px-10 lg:pt-10">
          <RouterView />
        </div>
      </main>
    </div>

  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { Button } from 'nanocat-ui'
import { api, clearSession } from '@/api'
import { getBooleanPreference, preferenceKeys, setBooleanPreference } from '@/lib/preferences'
import { applyThemeMode, getStoredThemeMode, setStoredThemeMode } from '@/lib/theme'

const route = useRoute()
const router = useRouter()
const isSidebarOpen = ref(false)
const isSidebarCollapsed = ref(getBooleanPreference(preferenceKeys.sidebarCollapsed, false))
const themeMode = ref(getStoredThemeMode())

const menuItems = [
  {
    path: '/',
    label: '概览中心',
    icon: 'M4 4h7v7H4V4zm9 0h7v4h-7V4zm0 6h7v10h-7V10zM4 13h7v7H4v-7z',
  },
  {
    path: '/accounts',
    label: '号池管理',
    icon: 'M12 12a3.5 3.5 0 1 0-3.5-3.5A3.5 3.5 0 0 0 12 12zm0 2c-4.1 0-7.5 2.2-7.5 5v1h15v-1c0-2.8-3.4-5-7.5-5z',
  },
  {
    path: '/egress',
    label: '出口池',
    icon: 'M12 2a10 10 0 1 0 10 10A10 10 0 0 0 12 2zm1 14.93V19h-2v-2.07A8 8 0 0 1 4.07 13H2v-2h2.07A8 8 0 0 1 11 4.07V2h2v2.07A8 8 0 0 1 19.93 11H22v2h-2.07A8 8 0 0 1 13 16.93z',
  },
  {
    path: '/webapps',
    label: '应用模板',
    icon: 'M4 4h7v7H4V4zm9 0h7v7h-7V4zM4 13h7v7H4v-7zm9 0h7v7h-7v-7z',
  },
  {
    path: '/tasks',
    label: '任务队列',
    icon: 'M4 6h16v2H4V6zm0 5h16v2H4v-2zm0 5h10v2H4v-2z',
  },
  {
    path: '/studio',
    label: '试跑工作室',
    icon: 'M5 4h14a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2h-5l-4 4v-4H5a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2zm1 3v6h12V7H6zm2 2 2.1 2.8 2.4-3.1L17 14H7l1-5z',
  },
  {
    path: '/docs',
    label: 'API 说明',
    icon: 'M6 3h9l5 5v13a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1zm8 1.5V9h4.5L14 4.5zM8 12h8v2H8v-2zm0 4h8v2H8v-2z',
  },
  {
    path: '/settings',
    label: '系统设置',
    icon: 'M4 6h10v2H4V6zm12 0h4v2h-4V6zM4 11h6v2H4v-2zm8 0h8v2h-8v-2zM4 16h10v2H4v-2zm12 0h4v2h-4v-2z',
  },
]

const isSidebarRail = computed(() => isSidebarCollapsed.value)
const sidebarStyle = computed(() => ({
  '--sidebar-width': isSidebarRail.value ? '4rem' : '16rem',
}))

const currentPageTitle = computed(() => {
  const item = menuItems.find((m) => isNavActive(m.path))
  return item?.label || route.meta?.title || '概览中心'
})

const themeButtonText = computed(() => {
  if (themeMode.value === 'dark') return '深色'
  if (themeMode.value === 'light') return '浅色'
  return '系统'
})
const themeButtonTitle = computed(() => `主题：${themeButtonText.value}`)

function isNavActive(path) {
  if (path === '/') return route.path === '/'
  return route.path === path || route.path.startsWith(`${path}/`)
}

function buildNavItemClass(path) {
  const base = isSidebarRail.value ? 'px-2 justify-center gap-0' : 'px-2.5 gap-3'
  if (isNavActive(path)) {
    return `${base} rounded-[0.9rem] border-[hsl(var(--primary)_/_0.28)] bg-[hsl(var(--primary)_/_0.08)] font-semibold text-foreground shadow-[inset_0_0_0_1px_hsl(var(--primary)_/_0.08)]`
  }
  return `${base} rounded-[0.9rem] border-transparent text-muted-foreground hover:border-border hover:bg-[hsl(var(--secondary)_/_0.55)] hover:text-foreground`
}

function buildNavIconClass(path) {
  if (isNavActive(path)) {
    return 'border-[hsl(var(--primary)_/_0.28)] bg-[hsl(var(--card))] text-foreground shadow-sm'
  }
  return 'border-border bg-[hsl(var(--card))] text-muted-foreground group-hover:border-[hsl(var(--foreground)_/_0.28)] group-hover:text-foreground'
}

const navItemClassMap = computed(() =>
  Object.fromEntries(menuItems.map((item) => [item.path, buildNavItemClass(item.path)])),
)
const navIconClassMap = computed(() =>
  Object.fromEntries(menuItems.map((item) => [item.path, buildNavIconClass(item.path)])),
)

function toggleSidebar() {
  isSidebarCollapsed.value = !isSidebarCollapsed.value
  setBooleanPreference(preferenceKeys.sidebarCollapsed, isSidebarCollapsed.value)
}

function cycleThemeMode() {
  const order = ['light', 'dark', 'system']
  const next = order[(order.indexOf(themeMode.value) + 1) % order.length]
  themeMode.value = next
  setStoredThemeMode(next)
}

function refreshPage() {
  window.location.reload()
}

async function handleLogout() {
  try {
    await api.logout()
  } catch {
    /* ignore */
  }
  clearSession()
  router.push('/login')
}

function onSystemThemeChange() {
  if (themeMode.value === 'system') applyThemeMode('system')
}

let media
onMounted(() => {
  applyThemeMode(themeMode.value)
  media = window.matchMedia('(prefers-color-scheme: dark)')
  media.addEventListener?.('change', onSystemThemeChange)
})
onBeforeUnmount(() => {
  media?.removeEventListener?.('change', onSystemThemeChange)
})
</script>

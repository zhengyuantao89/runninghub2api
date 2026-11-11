<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { Button, Input } from 'nanocat-ui'
import PagePanel from '@/components/ai/PagePanel.vue'
import TableShell from '@/components/ai/TableShell.vue'
import FilterToolbar from '@/components/ai/FilterToolbar.vue'
import TonePill from '@/components/TonePill.vue'
import { api } from '@/api'

const loading = ref(false)
const error = ref('')
const okmsg = ref('')
const items = ref([])
const groups = ref([])
const groupFilter = ref('')
const page = ref(1)
const pageSize = ref(50)
const total = ref(0)
const pages = ref(0)
const pageSizeOptions = [20, 50, 100, 200]
const showImport = ref(false)
const importMode = ref('text') // text | json
const importText = ref('')
const importGroup = ref('nodes')
const busyId = ref(0)
const subBusy = ref(false)
const intervals = ref([10, 30, 60])
const fixedGroup = ref('nodes')

const subForm = reactive({
  url: '',
  interval_minutes: 30,
  enabled: true,
})
const subscription = ref(null)

const pageRangeLabel = computed(() => {
  if (!total.value) return '共 0 条'
  const start = (page.value - 1) * pageSize.value + 1
  const end = Math.min(page.value * pageSize.value, total.value)
  return `第 ${start}-${end} 条 / 共 ${total.value} 条`
})

const importPlaceholder = computed(() =>
  importMode.value === 'text'
    ? 'socks5://user:pass@1.2.3.4:1080\nhttp://1.2.3.4:8080\n1.2.3.4:8080:user:pass'
    : '[{"group_name":"默认分组","proxy_type":"http","host":"1.2.3.4","port":8080}]',
)

function statusTone(st) {
  if (st === '可用') return 'success'
  if (st === '失败') return 'error'
  if (st === '禁用') return 'neutral'
  return 'neutral'
}

function applySubscription(sub) {
  subscription.value = sub && sub.id ? sub : null
  if (!subscription.value) return
  subForm.url = String(subscription.value.url || '')
  subForm.interval_minutes = Number(subscription.value.interval_minutes) || 30
  subForm.enabled = subscription.value.enabled !== false
}

async function load() {
  error.value = ''
  loading.value = true
  try {
    const [listRes, groupRes, subRes] = await Promise.all([
      api.listProxies(groupFilter.value, page.value, pageSize.value),
      api.listProxyGroups(),
      api.getProxySubscription(),
    ])
    items.value = Array.isArray(listRes?.items) ? listRes.items : Array.isArray(listRes) ? listRes : []
    total.value = Number(listRes?.total) || 0
    pages.value = Number(listRes?.pages) || 0
    if (listRes?.page_size) pageSize.value = Number(listRes.page_size) || pageSize.value
    const serverPage = Number(listRes?.page) || 1
    page.value = pages.value === 0 ? 1 : Math.min(Math.max(1, serverPage), pages.value)
    groups.value = Array.isArray(groupRes) ? groupRes : []
    intervals.value = Array.isArray(subRes?.intervals) ? subRes.intervals : [10, 30, 60]
    fixedGroup.value = subRes?.fixed_group || 'nodes'
    const sub = subRes?.subscription || (Array.isArray(subRes?.items) ? subRes.items[0] : null)
    applySubscription(sub)
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    loading.value = false
  }
}

function onGroupChange() {
  page.value = 1
  load()
}

function onPageSizeChange() {
  page.value = 1
  load()
}

function goPage(next) {
  const target = Math.max(1, Math.min(pages.value || 1, Number(next) || 1))
  if (target === page.value) return
  page.value = target
  load()
}

async function onSaveSubscription() {
  error.value = ''
  okmsg.value = ''
  subBusy.value = true
  try {
    const res = await api.saveProxySubscription({
      url: subForm.url.trim(),
      interval_minutes: Number(subForm.interval_minutes) || 30,
      enabled: !!subForm.enabled,
      refresh_now: true,
    })
    applySubscription(res.subscription || res)
    if (res.sync_error) {
      error.value = `已保存，同步失败：${res.sync_error}`
    } else {
      const added = res.added ?? res.subscription?.last_added
      const total = res.total ?? res.subscription?.last_total
      okmsg.value =
        added != null
          ? `订阅同步完成：共 ${total ?? '—'}，新增 ${res.added ?? 0}，更新 ${res.updated ?? 0}，删除 ${res.removed ?? 0}`
          : '订阅已保存'
    }
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    subBusy.value = false
  }
}

async function onRefreshSubscription() {
  error.value = ''
  okmsg.value = ''
  subBusy.value = true
  try {
    const res = await api.refreshProxySubscription()
    okmsg.value = `同步完成：共 ${res.total ?? 0}，新增 ${res.added ?? 0}，更新 ${res.updated ?? 0}，删除 ${res.removed ?? 0}`
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    subBusy.value = false
  }
}

async function onDeleteSubscription() {
  if (!confirm('删除订阅配置？可选同时清空 nodes 分组节点。')) return
  const clearGroup = confirm('同时清空 nodes 分组里的出口节点？')
  subBusy.value = true
  try {
    await api.deleteProxySubscription(clearGroup)
    subscription.value = null
    subForm.url = ''
    okmsg.value = '订阅已删除'
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    subBusy.value = false
  }
}

async function onImport() {
  error.value = ''
  okmsg.value = ''
  const raw = importText.value.trim()
  if (!raw) {
    error.value = '请粘贴内容'
    return
  }
  loading.value = true
  try {
    let res
    if (importMode.value === 'json') {
      let parsed
      try {
        parsed = JSON.parse(raw)
      } catch {
        error.value = 'JSON 解析失败'
        loading.value = false
        return
      }
      const list = Array.isArray(parsed)
        ? parsed
        : Array.isArray(parsed?.items)
          ? parsed.items
          : Array.isArray(parsed?.proxies)
            ? parsed.proxies
            : null
      if (!list?.length) {
        error.value = '需要 items/proxies 数组'
        loading.value = false
        return
      }
      res = await api.importProxies(list, 'manual')
      okmsg.value = `导入成功 ${res.imported || 0}，失败 ${res.failed || 0}`
    } else {
      res = await api.importProxiesText(raw, importGroup.value || 'nodes', true)
      okmsg.value = `文本同步完成：共 ${res.imported ?? res.total ?? 0}，新增 ${res.added ?? 0}，更新 ${res.updated ?? 0}，删除 ${res.removed ?? 0}`
    }
    showImport.value = false
    importText.value = ''
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    loading.value = false
  }
}

async function onTest(id) {
  busyId.value = id
  error.value = ''
  okmsg.value = ''
  try {
    const res = await api.testProxy(id)
    okmsg.value = res.ok
      ? `检测通过，延迟 ${res.latency}ms`
      : `检测失败：${res.error || 'unknown'}（${res.latency || 0}ms）`
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    busyId.value = 0
  }
}

async function onEnable(id) {
  busyId.value = id
  try {
    await api.enableProxy(id)
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    busyId.value = 0
  }
}

async function onDisable(id) {
  busyId.value = id
  try {
    await api.disableProxy(id)
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    busyId.value = 0
  }
}

async function onDelete(id) {
  if (!confirm(`删除出口 #${id}？已绑定账号会解除绑定。`)) return
  busyId.value = id
  try {
    await api.deleteProxy(id)
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    busyId.value = 0
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-5">
    <PagePanel>
      <header class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0">
          <h2 class="text-base font-semibold text-foreground">出口订阅</h2>
          <p class="mt-1.5 max-w-2xl text-xs leading-relaxed text-muted-foreground">
            填写导出 URL，定时拉取纯文本代理列表，精确同步到
            <code class="text-[11px]">{{ fixedGroup }}</code> 分组。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <Button size="sm" variant="outline" :disabled="subBusy || !subForm.url" @click="onRefreshSubscription">
            立即同步
          </Button>
          <Button size="sm" variant="primary" :disabled="subBusy || !subForm.url.trim()" @click="onSaveSubscription">
            保存并同步
          </Button>
        </div>
      </header>

      <div class="mt-4 grid gap-3 md:grid-cols-[1fr_auto_auto]">
        <label class="block space-y-1.5">
          <span class="text-xs text-muted-foreground">订阅 URL</span>
          <Input
            v-model="subForm.url"
            placeholder="https://example.com/api/export/all-proxies"
            class="w-full"
          />
        </label>
        <label class="block space-y-1.5">
          <span class="text-xs text-muted-foreground">间隔（分钟）</span>
          <select
            v-model.number="subForm.interval_minutes"
            class="h-10 w-full rounded-lg border border-border bg-background px-2 text-sm"
          >
            <option v-for="n in intervals" :key="n" :value="n">{{ n }}</option>
          </select>
        </label>
        <label class="flex items-end gap-2 pb-2 text-sm">
          <input v-model="subForm.enabled" type="checkbox" class="h-4 w-4 accent-primary" />
          启用定时
        </label>
      </div>

      <div v-if="subscription" class="mt-3 flex flex-wrap gap-3 text-xs text-muted-foreground">
        <span>上次同步：{{ subscription.last_sync_at || '—' }}</span>
        <TonePill
          :tone="subscription.last_sync_status === 'ok' ? 'success' : subscription.last_sync_status === 'error' ? 'error' : 'neutral'"
          :label="subscription.last_sync_status || '未同步'"
        />
        <span v-if="subscription.last_total != null">共 {{ subscription.last_total }}</span>
        <span v-if="subscription.last_error" class="text-[hsl(var(--tone-error-strong))]">
          {{ subscription.last_error }}
        </span>
        <button type="button" class="underline underline-offset-2" :disabled="subBusy" @click="onDeleteSubscription">
          删除订阅
        </button>
      </div>
    </PagePanel>

    <PagePanel>
      <header class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0">
          <h2 class="text-base font-semibold text-foreground">出口池</h2>
        </div>
        <div class="flex flex-wrap gap-2">
          <Button size="sm" variant="outline" :disabled="loading" @click="load">刷新</Button>
          <Button size="sm" variant="primary" @click="showImport = true">批量导入</Button>
        </div>
      </header>

      <FilterToolbar class="mt-4">
        <label class="flex items-center gap-2 text-xs text-muted-foreground">
          分组
          <select
            v-model="groupFilter"
            class="rounded-lg border border-border bg-background px-2 py-1.5 text-sm text-foreground"
            @change="onGroupChange"
          >
            <option value="">全部</option>
            <option v-for="g in groups" :key="g.group_name" :value="g.group_name">
              {{ g.group_name }} ({{ g.count }})
            </option>
          </select>
        </label>
        <label class="flex items-center gap-2 text-xs text-muted-foreground">
          每页
          <select
            v-model.number="pageSize"
            class="rounded-lg border border-border bg-background px-2 py-1.5 text-sm text-foreground"
            @change="onPageSizeChange"
          >
            <option v-for="n in pageSizeOptions" :key="n" :value="n">{{ n }}</option>
          </select>
        </label>
        <span class="text-xs text-muted-foreground">{{ pageRangeLabel }}</span>
      </FilterToolbar>

      <p v-if="okmsg" class="mt-3 text-sm text-[hsl(var(--tone-success-strong))]">{{ okmsg }}</p>
      <p v-if="error && !showImport" class="mt-3 text-sm text-[hsl(var(--tone-error-strong))]">{{ error }}</p>

      <TableShell class="mt-4">
        <table class="w-full min-w-[1100px] text-left text-sm">
          <thead class="text-xs uppercase tracking-[0.16em] text-muted-foreground">
            <tr>
              <th class="py-3 pr-4">ID</th>
              <th class="py-3 pr-4">分组</th>
              <th class="py-3 pr-4">节点</th>
              <th class="py-3 pr-4">类型</th>
              <th class="py-3 pr-4">状态</th>
              <th class="py-3 pr-4">延迟</th>
              <th class="py-3 pr-4">成功/失败</th>
              <th class="py-3 pr-4">地区</th>
              <th class="py-3 text-right">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="n in items" :key="n.id" class="border-t border-border/70">
              <td class="py-3 pr-4 font-mono text-xs">{{ n.id }}</td>
              <td class="py-3 pr-4">{{ n.group_name || '—' }}</td>
              <td class="py-3 pr-4">
                <div class="font-medium">{{ n.name || '—' }}</div>
                <div class="font-mono text-xs text-muted-foreground">{{ n.host }}:{{ n.port }}</div>
              </td>
              <td class="py-3 pr-4 font-mono text-xs">{{ n.proxy_type }}</td>
              <td class="py-3 pr-4">
                <TonePill :tone="statusTone(n.status)" :label="n.status || '—'" />
              </td>
              <td class="py-3 pr-4 font-mono text-xs">{{ n.latency != null ? `${n.latency}ms` : '—' }}</td>
              <td class="py-3 pr-4 font-mono text-xs">{{ n.success_count || 0 }}/{{ n.failure_count || 0 }}</td>
              <td class="py-3 pr-4 text-muted-foreground">{{ n.region || '—' }}</td>
              <td class="py-3 text-right">
                <div class="flex flex-wrap justify-end gap-2">
                  <Button size="xs" variant="outline" :disabled="busyId === n.id" @click="onTest(n.id)">检测</Button>
                  <Button
                    v-if="n.status !== '可用'"
                    size="xs"
                    variant="outline"
                    :disabled="busyId === n.id"
                    @click="onEnable(n.id)"
                  >
                    启用
                  </Button>
                  <Button
                    v-else
                    size="xs"
                    variant="outline"
                    :disabled="busyId === n.id"
                    @click="onDisable(n.id)"
                  >
                    禁用
                  </Button>
                  <Button size="xs" variant="outline" :disabled="busyId === n.id" @click="onDelete(n.id)">删除</Button>
                </div>
              </td>
            </tr>
            <tr v-if="!loading && !items.length">
              <td colspan="9" class="py-10 text-center text-sm text-muted-foreground">暂无出口节点</td>
            </tr>
          </tbody>
        </table>
      </TableShell>

      <div class="mt-4 flex flex-wrap items-center justify-between gap-3">
        <span class="text-xs text-muted-foreground">
          {{ pages > 0 ? `第 ${page} / ${pages} 页` : '—' }}
        </span>
        <div class="flex flex-wrap items-center gap-2">
          <Button size="sm" variant="outline" :disabled="loading || page <= 1" @click="goPage(1)">首页</Button>
          <Button size="sm" variant="outline" :disabled="loading || page <= 1" @click="goPage(page - 1)">上一页</Button>
          <Button size="sm" variant="outline" :disabled="loading || page >= pages" @click="goPage(page + 1)">下一页</Button>
          <Button size="sm" variant="outline" :disabled="loading || page >= pages" @click="goPage(pages)">末页</Button>
        </div>
      </div>
    </PagePanel>

    <div
      v-if="showImport"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
      @click.self="showImport = false"
    >
      <PagePanel class="w-full max-w-2xl space-y-4">
        <header>
          <h3 class="text-base font-semibold">批量导入出口</h3>
          <p class="mt-1 text-xs text-muted-foreground">
            文本模式：一行一个代理；JSON 模式：items / proxies 数组。
          </p>
        </header>
        <div class="flex flex-wrap gap-2">
          <Button
            size="sm"
            :variant="importMode === 'text' ? 'primary' : 'outline'"
            @click="importMode = 'text'"
          >
            纯文本行
          </Button>
          <Button
            size="sm"
            :variant="importMode === 'json' ? 'primary' : 'outline'"
            @click="importMode = 'json'"
          >
            JSON 数组
          </Button>
        </div>
        <label v-if="importMode === 'text'" class="block space-y-1.5">
          <span class="text-xs text-muted-foreground">写入分组（精确同步）</span>
          <Input v-model="importGroup" placeholder="nodes" />
        </label>
        <textarea
          v-model="importText"
          rows="14"
          class="w-full rounded-xl border border-border bg-background px-3 py-2 font-mono text-xs text-foreground"
          :placeholder="importPlaceholder"
        />
        <p v-if="error" class="text-sm text-[hsl(var(--tone-error-strong))]">{{ error }}</p>
        <div class="flex justify-end gap-2">
          <Button size="sm" variant="outline" @click="showImport = false">取消</Button>
          <Button size="sm" variant="primary" :disabled="loading" @click="onImport">导入</Button>
        </div>
      </PagePanel>
    </div>
  </div>
</template>

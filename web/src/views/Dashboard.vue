<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { Button, StatusPill } from 'nanocat-ui'
import MetricStrip from '@/components/ai/MetricStrip.vue'
import PagePanel from '@/components/ai/PagePanel.vue'
import { api } from '@/api'

const accounts = ref([])
const tasks = ref([])
const health = ref(null)
const error = ref('')
let timer
const toolbarButtonClass = 'h-8 justify-center px-3 text-xs'

function parseCoins(v) {
  const n = Number(String(v ?? '').replace(/[^\d.-]/g, ''))
  return Number.isFinite(n) ? Math.max(0, Math.floor(n)) : 0
}

const pool = computed(() => {
  const list = accounts.value || []
  const active = list.filter((x) => x.status === 'active')
  const dispatchable = list.filter((x) => x.dispatchable)
  const busy = list.filter((x) => x.busy)
  const disabled = list.filter((x) => x.status !== 'active')
  const cooling = list.filter((x) => x.cooldown_until)
  const noCoins = list.filter((x) => x.last_balance_at && parseCoins(x.remain_coins) <= 0)
  const withKey = list.filter((x) => x.has_api_key)
  const totalCoins = list.reduce((sum, x) => sum + parseCoins(x.remain_coins), 0)
  return {
    total: list.length,
    active: active.length,
    dispatchable: dispatchable.length,
    busy: busy.length,
    disabled: disabled.length,
    cooling: cooling.length,
    noCoins: noCoins.length,
    withKey: withKey.length,
    totalCoins,
  }
})

const job = computed(() => {
  const list = tasks.value || []
  const by = (s) => list.filter((x) => x.status === s).length
  const succeeded = by('succeeded')
  const failed = by('failed')
  const cancelled = by('cancelled')
  const queued = by('queued')
  const running = by('running')
  const done = succeeded + failed + cancelled
  const rate = done > 0 ? Math.round((succeeded / done) * 100) : 0
  const errorMap = {}
  for (const t of list) {
    if (t.status !== 'failed' && t.status !== 'cancelled') continue
    const key = t.error_code || 'UNKNOWN'
    errorMap[key] = (errorMap[key] || 0) + 1
  }
  const errors = Object.entries(errorMap)
    .map(([code, count]) => ({ code, count }))
    .sort((a, b) => b.count - a.count)
    .slice(0, 8)
  return {
    total: list.length,
    queued,
    running,
    succeeded,
    failed,
    cancelled,
    done,
    rate,
    inFlight: queued + running,
    errors,
  }
})

const metricItems = computed(() => [
  { key: 'pool', label: '号池总数', value: pool.value.total, meta: `可调度 ${pool.value.dispatchable}` },
  { key: 'coins', label: 'RH 币合计', value: pool.value.totalCoins, meta: `有 Key ${pool.value.withKey}` },
  { key: 'inflight', label: '在途任务', value: job.value.inFlight, meta: `排队 ${job.value.queued} · 运行 ${job.value.running}` },
  { key: 'rate', label: '成功率', value: `${job.value.rate}%`, meta: `成功 ${job.value.succeeded} / 完结 ${job.value.done}` },
])

const poolBars = computed(() => {
  const p = pool.value
  const max = Math.max(p.total, 1)
  return [
    { key: 'dispatchable', label: '可调度', value: p.dispatchable, tone: 'ok' },
    { key: 'busy', label: '占用中', value: p.busy, tone: 'info' },
    { key: 'cooling', label: '冷却中', value: p.cooling, tone: 'warn' },
    { key: 'noCoins', label: '无积分', value: p.noCoins, tone: 'err' },
    { key: 'disabled', label: '已禁用', value: p.disabled, tone: 'mute' },
  ].map((x) => ({ ...x, pct: Math.round((x.value / max) * 100) }))
})

const jobBars = computed(() => {
  const j = job.value
  const max = Math.max(j.total, 1)
  return [
    { key: 'queued', label: '排队', value: j.queued, tone: 'mute' },
    { key: 'running', label: '运行', value: j.running, tone: 'info' },
    { key: 'succeeded', label: '成功', value: j.succeeded, tone: 'ok' },
    { key: 'failed', label: '失败', value: j.failed, tone: 'err' },
    { key: 'cancelled', label: '取消', value: j.cancelled, tone: 'warn' },
  ].map((x) => ({ ...x, pct: Math.round((x.value / max) * 100) }))
})

const accountRows = computed(() => {
  return [...(accounts.value || [])]
    .sort((a, b) => {
      if (!!b.dispatchable !== !!a.dispatchable) return b.dispatchable ? 1 : -1
      return parseCoins(b.remain_coins) - parseCoins(a.remain_coins)
    })
    .slice(0, 12)
})

function poolTone(row) {
  if (row.dispatchable) return 'success'
  if (row.busy) return 'info'
  if (row.status !== 'active') return 'error'
  return 'neutral'
}

function poolLabel(row) {
  if (row.dispatchable) return '可调度'
  if (row.busy) return '占用中'
  if (row.status !== 'active') return '已禁用'
  if (row.last_balance_at && parseCoins(row.remain_coins) <= 0) return '无积分'
  if (row.cooldown_until) return '冷却中'
  return '不可用'
}

async function refresh() {
  error.value = ''
  try {
    health.value = await api.health()
    const [acc, task] = await Promise.all([api.listAccounts(), api.listAdminTasks()])
    accounts.value = acc.data || []
    tasks.value = task.data || []
  } catch (e) {
    error.value = e.message || String(e)
  }
}

onMounted(() => {
  refresh()
  timer = setInterval(refresh, 8000)
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="relative space-y-5">
    <PagePanel class="space-y-5">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <p class="text-sm text-muted-foreground">
          号池容量 · 积分 · 任务吞吐统计
          <span class="mx-1.5 text-border">·</span>
          {{ health?.ok ? '服务正常' : '检测中' }}
        </p>
        <div class="flex flex-wrap gap-2">
          <RouterLink
            to="/tasks"
            class="inline-flex h-8 items-center justify-center rounded-md border border-border px-3 text-xs text-foreground hover:bg-muted/50"
          >
            查看任务队列
          </RouterLink>
          <Button size="sm" variant="outline" :root-class="toolbarButtonClass" @click="refresh">
            刷新统计
          </Button>
        </div>
      </div>

      <MetricStrip :items="metricItems" columns-class="grid-cols-2 md:grid-cols-4" />

      <p v-if="error" class="text-sm text-[hsl(var(--tone-error-strong))]">{{ error }}</p>

      <div class="grid gap-4 lg:grid-cols-2">
        <section class="rounded-2xl border border-border/80 bg-card p-4">
          <div class="mb-4 flex items-end justify-between gap-3">
            <div>
              <h2 class="text-sm font-semibold text-foreground">号池结构</h2>
              <p class="mt-1 text-xs text-muted-foreground">按可调度 / 占用 / 冷却 / 积分拆分</p>
            </div>
            <RouterLink class="text-xs text-muted-foreground underline-offset-2 hover:underline" to="/accounts">
              号池管理
            </RouterLink>
          </div>
          <ul class="space-y-3">
            <li v-for="b in poolBars" :key="b.key">
              <div class="mb-1 flex items-center justify-between text-xs">
                <span class="text-muted-foreground">{{ b.label }}</span>
                <span class="font-medium text-foreground">{{ b.value }}</span>
              </div>
              <div class="h-2 overflow-hidden rounded-full bg-muted/60">
                <div
                  class="h-full rounded-full transition-all"
                  :class="{
                    'bg-[hsl(var(--tone-success-strong))]': b.tone === 'ok',
                    'bg-[hsl(var(--tone-info-strong))]': b.tone === 'info',
                    'bg-[hsl(var(--tone-warning-strong))]': b.tone === 'warn',
                    'bg-[hsl(var(--tone-error-strong))]': b.tone === 'err',
                    'bg-muted-foreground/40': b.tone === 'mute',
                  }"
                  :style="{ width: `${b.pct}%` }"
                />
              </div>
            </li>
          </ul>
        </section>

        <section class="rounded-2xl border border-border/80 bg-card p-4">
          <div class="mb-4 flex items-end justify-between gap-3">
            <div>
              <h2 class="text-sm font-semibold text-foreground">任务结构</h2>
              <p class="mt-1 text-xs text-muted-foreground">全量任务状态分布 · 成功率 {{ job.rate }}%</p>
            </div>
            <RouterLink class="text-xs text-muted-foreground underline-offset-2 hover:underline" to="/tasks">
              任务队列
            </RouterLink>
          </div>
          <ul class="space-y-3">
            <li v-for="b in jobBars" :key="b.key">
              <div class="mb-1 flex items-center justify-between text-xs">
                <span class="text-muted-foreground">{{ b.label }}</span>
                <span class="font-medium text-foreground">{{ b.value }}</span>
              </div>
              <div class="h-2 overflow-hidden rounded-full bg-muted/60">
                <div
                  class="h-full rounded-full transition-all"
                  :class="{
                    'bg-[hsl(var(--tone-success-strong))]': b.tone === 'ok',
                    'bg-[hsl(var(--tone-info-strong))]': b.tone === 'info',
                    'bg-[hsl(var(--tone-warning-strong))]': b.tone === 'warn',
                    'bg-[hsl(var(--tone-error-strong))]': b.tone === 'err',
                    'bg-muted-foreground/40': b.tone === 'mute',
                  }"
                  :style="{ width: `${b.pct}%` }"
                />
              </div>
            </li>
          </ul>
        </section>
      </div>

      <div class="grid gap-4 lg:grid-cols-2">
        <section class="rounded-2xl border border-border/80 bg-card p-4">
          <div class="mb-3">
            <h2 class="text-sm font-semibold text-foreground">账号积分快照</h2>
            <p class="mt-1 text-xs text-muted-foreground">按可调度优先，展示剩余 RH 币</p>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[420px] text-left text-sm">
              <thead class="text-xs uppercase tracking-[0.14em] text-muted-foreground">
                <tr>
                  <th class="py-2 pr-3">账号</th>
                  <th class="py-2 pr-3">状态</th>
                  <th class="py-2 pr-3">RH币</th>
                  <th class="py-2">在途</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="a in accountRows" :key="a.id" class="border-t border-border/60">
                  <td class="max-w-[180px] truncate py-2.5 pr-3">{{ a.email }}</td>
                  <td class="py-2.5 pr-3">
                    <StatusPill :tone="poolTone(a)" :label="poolLabel(a)" />
                  </td>
                  <td class="py-2.5 pr-3 font-medium">{{ a.remain_coins || '—' }}</td>
                  <td class="py-2.5 text-muted-foreground">{{ a.current_task_counts || '0' }}</td>
                </tr>
                <tr v-if="!accountRows.length">
                  <td colspan="4" class="py-8 text-center text-sm text-muted-foreground">暂无账号统计</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="rounded-2xl border border-border/80 bg-card p-4">
          <div class="mb-3">
            <h2 class="text-sm font-semibold text-foreground">失败原因分布</h2>
            <p class="mt-1 text-xs text-muted-foreground">按 error_code 汇总近期失败 / 取消</p>
          </div>
          <ul v-if="job.errors.length" class="space-y-2.5">
            <li
              v-for="e in job.errors"
              :key="e.code"
              class="flex items-center justify-between rounded-xl border border-border/60 px-3 py-2 text-sm"
            >
              <span class="font-mono text-xs text-foreground">{{ e.code }}</span>
              <span class="text-muted-foreground">{{ e.count }}</span>
            </li>
          </ul>
          <p v-else class="py-8 text-center text-sm text-muted-foreground">暂无失败记录</p>
        </section>
      </div>
    </PagePanel>
  </div>
</template>

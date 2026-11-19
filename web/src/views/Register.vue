<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { Button, Input } from 'nanocat-ui'
import PagePanel from '@/components/ai/PagePanel.vue'
import { api } from '@/api'

const quantity = ref(1)
const inviteCode = ref('')
const unused = ref(0)
const jobs = ref([])
const activeJobId = ref(0)
const logs = ref([])
const afterLogId = ref(0)
const loading = ref(false)
const starting = ref(false)
const clearing = ref(false)
const error = ref('')
const okmsg = ref('')
let pollTimer = 0

async function refreshPool() {
  try {
    const res = await api.listMailInboxes({ status: 'unused', limit: 1 })
    unused.value = res.unused ?? 0
  } catch {
    /* ignore */
  }
}

async function loadJobs() {
  const res = await api.listRegisterJobs()
  jobs.value = res.items || []
  if (!activeJobId.value && jobs.value[0]) {
    activeJobId.value = jobs.value[0].id
  }
}

async function loadLogs() {
  if (!activeJobId.value) return
  const res = await api.getRegisterJobLogs(activeJobId.value, afterLogId.value)
  if (res.job) {
    const idx = jobs.value.findIndex((j) => j.id === res.job.id)
    if (idx >= 0) jobs.value[idx] = res.job
    else jobs.value.unshift(res.job)
  }
  const batch = res.logs || []
  if (batch.length) {
    logs.value = logs.value.concat(batch)
    afterLogId.value = batch[batch.length - 1].id
  }
}

async function start() {
  error.value = ''
  okmsg.value = ''
  starting.value = true
  try {
    const res = await api.startRegisterJob({
      quantity: Number(quantity.value) || 1,
      invite_code: inviteCode.value,
    })
    activeJobId.value = res.job?.id || 0
    logs.value = []
    afterLogId.value = 0
    okmsg.value = `已启动任务 #${activeJobId.value}`
    await loadJobs()
    await loadLogs()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    starting.value = false
  }
}

async function cancel() {
  if (!activeJobId.value) return
  try {
    await api.cancelRegisterJob(activeJobId.value)
    okmsg.value = '已发送停止'
  } catch (e) {
    error.value = e.message || String(e)
  }
}

function selectJob(id) {
  activeJobId.value = id
  logs.value = []
  afterLogId.value = 0
  loadLogs()
}

async function clearFinishedJobs() {
  if (!jobs.value.some((j) => ['done', 'failed', 'cancelled'].includes(j.status))) {
    okmsg.value = '没有可清理的已结束任务'
    return
  }
  if (!confirm('清理已结束的注册任务（进行中的不会删）？')) return
  error.value = ''
  okmsg.value = ''
  clearing.value = true
  try {
    const res = await api.clearRegisterJobs()
    const n = Number(res?.deleted) || 0
    okmsg.value = n ? `已清理 ${n} 条最近任务` : '没有可清理的任务'
    await loadJobs()
    if (!jobs.value.some((j) => j.id === activeJobId.value)) {
      activeJobId.value = jobs.value[0]?.id || 0
      logs.value = []
      afterLogId.value = 0
      if (activeJobId.value) await loadLogs()
    }
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    clearing.value = false
  }
}

onMounted(async () => {
  loading.value = true
  try {
    await refreshPool()
    await loadJobs()
    await loadLogs()
    try {
      const s = await api.getSettings()
      if (s.register?.invite_code) inviteCode.value = s.register.invite_code
    } catch {
      /* ignore */
    }
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    loading.value = false
  }
  pollTimer = window.setInterval(() => {
    loadLogs()
    refreshPool()
  }, 2500)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<template>
  <div class="space-y-4">
    <PagePanel>
      <header class="mb-4">
        <h2 class="text-lg font-semibold text-foreground">账号注册</h2>
        <p class="mt-1 text-sm text-muted-foreground">可用邮箱 {{ unused }}</p>
      </header>

      <div class="mb-4 flex flex-wrap items-end gap-3">
        <div class="space-y-1">
          <label class="text-xs text-muted-foreground">数量</label>
          <Input v-model="quantity" type="number" size="sm" class="w-24" min="1" max="50" />
        </div>
        <div class="space-y-1">
          <label class="text-xs text-muted-foreground">邀请码（可空）</label>
          <Input v-model="inviteCode" size="sm" class="w-40" placeholder="可选" />
        </div>
        <Button size="sm" variant="primary" :disabled="starting || unused < 1" @click="start">
          {{ starting ? '启动中…' : '开始注册' }}
        </Button>
        <Button size="sm" variant="outline" :disabled="!activeJobId" @click="cancel">停止</Button>
      </div>

      <p v-if="okmsg" class="mb-2 text-sm text-emerald-600">{{ okmsg }}</p>
      <p v-if="error" class="mb-2 text-sm text-destructive">{{ error }}</p>

      <div class="grid gap-4 lg:grid-cols-[220px_1fr]">
        <div class="rounded-xl border border-border p-2">
          <div class="mb-2 flex items-center justify-between gap-2 px-1">
            <p class="text-xs text-muted-foreground">最近任务</p>
            <button
              type="button"
              class="text-[11px] text-muted-foreground underline-offset-2 hover:text-foreground hover:underline disabled:opacity-50"
              :disabled="clearing || loading"
              @click="clearFinishedJobs"
            >
              {{ clearing ? '清理中…' : '清理' }}
            </button>
          </div>
          <button
            v-for="j in jobs"
            :key="j.id"
            type="button"
            class="mb-1 flex w-full flex-col rounded-lg px-2 py-1.5 text-left text-xs hover:bg-muted/40"
            :class="activeJobId === j.id ? 'bg-muted/60' : ''"
            @click="selectJob(j.id)"
          >
            <span class="font-medium">#{{ j.id }} · {{ j.status }}</span>
            <span class="text-muted-foreground">成{{ j.success }}/败{{ j.failed }} · 量{{ j.quantity }}</span>
          </button>
          <p v-if="!jobs.length" class="px-1 py-4 text-xs text-muted-foreground">还没有任务</p>
        </div>
        <div class="max-h-[480px] overflow-auto rounded-xl border border-border bg-background p-3 font-mono text-xs leading-relaxed">
          <div v-for="line in logs" :key="line.id" class="whitespace-pre-wrap">
            <span class="text-muted-foreground">{{ line.created_at?.slice(11, 19) || '' }}</span>
            <span
              class="ml-2"
              :class="{
                'text-emerald-600': line.level === 'ok' || line.level === 'success',
                'text-amber-600': line.level === 'warning',
                'text-destructive': line.level === 'error',
              }"
            >[{{ line.level }}]</span>
            <span class="ml-1">{{ line.message }}</span>
          </div>
          <p v-if="!logs.length" class="text-muted-foreground">运行日志会出现在这里</p>
        </div>
      </div>
    </PagePanel>
  </div>
</template>

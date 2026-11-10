<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Button, ModalShell } from 'nanocat-ui'
import PagePanel from '@/components/ai/PagePanel.vue'
import TableShell from '@/components/ai/TableShell.vue'
import TonePill from '@/components/TonePill.vue'
import { api } from '@/api'
import { progressLabel, progressTone, statusLabel, statusTone } from '@/lib/taskLabels'

const rows = ref([])
const templates = ref([])
const error = ref('')
const okmsg = ref('')
const detail = ref(null)
const busyId = ref('')
let timer
const toolbarButtonClass = 'h-8 justify-center px-3 text-xs'

const templateById = computed(() => {
  const map = {}
  for (const t of templates.value || []) {
    if (t?.id) map[t.id] = t
  }
  return map
})

const templateByWebapp = computed(() => {
  const map = {}
  for (const t of templates.value || []) {
    const wid = String(t?.webapp_id || '').trim()
    if (wid) map[wid] = t
  }
  return map
})

function templateMeta(t) {
  if (!t) return { label: '', copy: '' }
  const byId = t.template_id ? templateById.value[t.template_id] : null
  const byWeb = t.webapp_id ? templateByWebapp.value[String(t.webapp_id).trim()] : null
  const hit = byId || byWeb
  if (hit?.name) {
    return { label: hit.name, copy: hit.id || t.template_id || t.webapp_id || '' }
  }
  if (t.template_id) return { label: t.template_id, copy: t.template_id }
  if (t.webapp_id) return { label: t.webapp_id, copy: t.webapp_id }
  return { label: '', copy: '' }
}

const detailLive = computed(() => {
  if (!detail.value?.id) return null
  return rows.value.find((x) => x.id === detail.value.id) || detail.value
})

const detailFields = computed(() => {
  const t = detailLive.value
  if (!t) return []
  return [
    { label: '本地 ID', value: t.id, mono: true, copy: true },
    { label: '状态', value: statusLabel(t.status) },
    { label: '账号', value: t.account_email || '—' },
    { label: '账号 ID', value: t.account_id || '—' },
    { label: 'RH Task', value: t.rh_task_id || '—', mono: true, copy: !!t.rh_task_id },
    { label: 'RH 状态', value: t.rh_status || '—' },
    { label: '进度', value: progressLabel(t) },
    { label: '排队', value: t.queue_count ? `${t.queue_index}/${t.queue_count}` : '—' },
    { label: '模板', value: templateMeta(t).label || '—', mono: !templateById.value[t.template_id]?.name && !templateByWebapp.value[String(t.webapp_id || '').trim()]?.name, copy: !!templateMeta(t).copy },
    { label: 'Webapp', value: t.webapp_id || '—', mono: true, copy: !!t.webapp_id },
    { label: '模板 ID', value: t.template_id || '—', mono: true, copy: !!t.template_id },
    { label: '时长', value: t.duration || '—' },
    { label: '画幅', value: t.aspect_ratio || '—' },
    { label: '尝试次数', value: t.attempts ?? 0 },
    { label: '错误码', value: t.error_code || '—' },
    { label: '错误信息', value: t.error || '—', wide: true },
    { label: '成品 URL', value: t.file_url || '—', mono: true, copy: !!t.file_url, link: t.file_url },
    { label: '预览 URL', value: t.preview_url || '—', mono: true, copy: !!t.preview_url, link: t.preview_url },
    { label: '图片路径', value: t.image_path || '—', mono: true, wide: true },
    { label: '创建时间', value: t.created_at || '—' },
    { label: '更新时间', value: t.updated_at || '—' },
    { label: '完成时间', value: t.finished_at || '—' },
    { label: 'Prompt', value: t.prompt || '—', wide: true, pre: true },
  ]
})

async function refreshTemplates() {
  try {
    const res = await api.listTemplates()
    templates.value = Array.isArray(res?.data) ? res.data : []
  } catch {
    templates.value = []
  }
}

async function refresh() {
  error.value = ''
  try {
    const res = await api.listAdminTasks()
    rows.value = res.data || []
  } catch (e) {
    error.value = e.message || String(e)
  }
}

async function cancel(id) {
  error.value = ''
  okmsg.value = ''
  try {
    await api.cancelVideo(id)
    await refresh()
  } catch (e) {
    error.value = e.message || String(e)
  }
}

async function syncStatus(t) {
  if (!t?.id) return
  error.value = ''
  okmsg.value = ''
  busyId.value = t.id
  try {
    const res = await api.syncVideo(t.id)
    await refresh()
    const task = res?.task || rows.value.find((x) => x.id === t.id)
    if (task?.status === 'succeeded') {
      okmsg.value = `${t.id} 已同步：完成`
    } else if (task?.status === 'failed') {
      okmsg.value = `${t.id} 已同步：失败（${task.error || task.error_code || '见详情'}）`
    } else {
      okmsg.value = `${t.id} 已查询：${progressLabel(task)} / ${task?.rh_status || task?.status || '—'}`
    }
  } catch (e) {
    // 接口失败时也可能带回最新 task
    await refresh()
    error.value = e.message || String(e)
  } finally {
    busyId.value = ''
  }
}

function canSync(t) {
  return !!t?.rh_task_id && (t.status === 'running' || t.status === 'queued')
}

function openDetail(t) {
  detail.value = { ...t }
}

function closeDetail() {
  detail.value = null
}

function copy(text) {
  navigator.clipboard?.writeText(text || '')
}

watch(rows, () => {
  if (!detail.value?.id) return
  const live = rows.value.find((x) => x.id === detail.value.id)
  if (live) detail.value = { ...live }
})

onMounted(() => {
  refreshTemplates()
  refresh()
  timer = setInterval(refresh, 3000)
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="relative space-y-5">
    <PagePanel class="space-y-5">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p class="text-sm text-muted-foreground">异步生视频队列 · create 后即可看到 RH Task 与进度</p>
          <p class="mt-1 text-xs text-muted-foreground">
            若中途重启导致一直「运行中」，点「查询状态」向 RH 拉一次结果；启动时也会自动恢复轮询。
          </p>
        </div>
        <Button size="sm" variant="outline" :root-class="toolbarButtonClass" @click="refresh">刷新列表</Button>
      </div>

      <p v-if="okmsg" class="text-sm text-[hsl(var(--tone-success-strong))]">{{ okmsg }}</p>
      <p v-if="error" class="text-sm text-[hsl(var(--tone-error-strong))]">{{ error }}</p>

      <TableShell>
        <table class="w-full min-w-[1200px] text-left text-sm">
          <thead class="text-xs uppercase tracking-[0.16em] text-muted-foreground">
            <tr>
              <th class="py-3 pr-5">本地 ID</th>
              <th class="py-3 pr-5">账号</th>
              <th class="py-3 pr-5">模板/应用</th>
              <th class="py-3 pr-5">RH Task</th>
              <th class="py-3 pr-5">状态</th>
              <th class="py-3 pr-5">进度</th>
              <th class="py-3 pr-5">成品</th>
              <th class="py-3 pr-5">错误</th>
              <th class="py-3 text-right">操作</th>
            </tr>
          </thead>
          <tbody class="text-sm text-foreground">
            <tr v-for="t in rows" :key="t.id" class="border-t border-border/70">
              <td class="py-3 pr-5">
                <button type="button" class="font-mono text-xs hover:underline" @click="copy(t.id)">{{ t.id }}</button>
              </td>
              <td class="py-3 pr-5">{{ t.account_email || '—' }}</td>
              <td class="py-3 pr-5">
                <button
                  v-if="templateMeta(t).label"
                  type="button"
                  class="max-w-[12rem] truncate text-left text-xs text-muted-foreground hover:underline"
                  :class="templateMeta(t).label === t.webapp_id || templateMeta(t).label === t.template_id ? 'font-mono' : ''"
                  :title="[t.template_id, t.webapp_id].filter(Boolean).join(' · ')"
                  @click="copy(templateMeta(t).copy)"
                >{{ templateMeta(t).label }}</button>
                <span v-else class="text-muted-foreground">—</span>
              </td>
              <td class="py-3 pr-5">
                <button
                  v-if="t.rh_task_id"
                  type="button"
                  class="font-mono text-xs text-muted-foreground hover:underline"
                  @click="copy(t.rh_task_id)"
                >{{ t.rh_task_id }}</button>
                <span v-else class="text-muted-foreground">—</span>
              </td>
              <td class="py-3 pr-5">
                <TonePill :tone="statusTone(t.status)" :label="statusLabel(t.status)" />
              </td>
              <td class="py-3 pr-5">
                <TonePill :tone="progressTone(t)" :label="progressLabel(t)" />
                <div
                  v-if="t.queue_count || t.rh_status"
                  class="mt-1 text-xs text-muted-foreground"
                >
                  <span v-if="t.queue_count">位次 {{ t.queue_index ?? 0 }}/{{ t.queue_count }}</span>
                  <span v-if="t.queue_count && t.rh_status"> · </span>
                  <span v-if="t.rh_status">{{ t.rh_status }}</span>
                </div>
              </td>
              <td class="py-3 pr-5">
                <template v-if="t.file_url">
                  <a class="font-medium text-foreground underline-offset-2 hover:underline" :href="t.file_url" target="_blank" rel="noreferrer">打开</a>
                  <span class="mx-1 text-muted-foreground">·</span>
                  <button type="button" class="text-muted-foreground hover:text-foreground" @click="copy(t.file_url)">复制</button>
                </template>
                <span v-else class="text-muted-foreground">—</span>
              </td>
              <td class="max-w-[220px] truncate py-3 pr-5 text-muted-foreground">{{ t.error || t.error_code || '—' }}</td>
              <td class="py-3 text-right">
                <div class="inline-flex flex-wrap items-center justify-end gap-2">
                  <Button size="xs" variant="outline" @click="openDetail(t)">详情</Button>
                  <Button
                    v-if="canSync(t)"
                    size="xs"
                    variant="outline"
                    :disabled="busyId === t.id"
                    @click="syncStatus(t)"
                  >
                    {{ busyId === t.id ? '查询中…' : '查询状态' }}
                  </Button>
                  <Button
                    v-if="t.status === 'queued' || t.status === 'running'"
                    size="xs"
                    variant="outline"
                    :disabled="busyId === t.id"
                    @click="cancel(t.id)"
                  >
                    取消
                  </Button>
                </div>
              </td>
            </tr>
            <tr v-if="!rows.length">
              <td colspan="9" class="py-8 text-center text-sm text-muted-foreground">暂无任务</td>
            </tr>
          </tbody>
        </table>
      </TableShell>
    </PagePanel>

    <ModalShell
      :open="!!detail"
      title="任务详情"
      :description="detailLive?.id || ''"
      size-class="max-w-2xl"
      close-on-overlay
      @close="closeDetail"
    >
      <div v-if="detailLive" class="max-h-[70vh] space-y-3 overflow-y-auto pr-1">
        <div class="flex flex-wrap items-center gap-2">
          <TonePill :tone="statusTone(detailLive.status)" :label="statusLabel(detailLive.status)" />
          <TonePill :tone="progressTone(detailLive)" :label="progressLabel(detailLive)" />
        </div>
        <dl class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div
            v-for="f in detailFields"
            :key="f.label"
            :class="f.wide ? 'sm:col-span-2' : ''"
            class="rounded-xl border border-border/70 bg-muted/20 px-3 py-2"
          >
            <dt class="text-[11px] uppercase tracking-[0.14em] text-muted-foreground">{{ f.label }}</dt>
            <dd class="mt-1 break-all text-sm text-foreground">
              <a
                v-if="f.link"
                class="font-mono text-xs underline-offset-2 hover:underline"
                :href="f.link"
                target="_blank"
                rel="noreferrer"
              >{{ f.value }}</a>
              <pre
                v-else-if="f.pre"
                class="whitespace-pre-wrap font-sans text-sm leading-relaxed"
              >{{ f.value }}</pre>
              <span :class="f.mono ? 'font-mono text-xs' : ''">{{ f.value }}</span>
              <button
                v-if="f.copy"
                type="button"
                class="ml-2 text-xs text-muted-foreground hover:text-foreground"
                @click="copy(f.value)"
              >复制</button>
            </dd>
          </div>
        </dl>
      </div>
      <template #footer>
        <Button
          v-if="canSync(detailLive)"
          size="sm"
          variant="outline"
          :root-class="toolbarButtonClass"
          :disabled="busyId === detailLive?.id"
          @click="syncStatus(detailLive)"
        >
          {{ busyId === detailLive?.id ? '查询中…' : '查询状态' }}
        </Button>
        <Button
          v-if="detailLive?.file_url"
          size="sm"
          variant="outline"
          :root-class="toolbarButtonClass"
          @click="copy(detailLive.file_url)"
        >
          复制成品链接
        </Button>
        <Button size="sm" variant="primary" :root-class="toolbarButtonClass" @click="closeDetail">关闭</Button>
      </template>
    </ModalShell>
  </div>
</template>

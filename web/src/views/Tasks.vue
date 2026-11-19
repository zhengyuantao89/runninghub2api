<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Button, Input, ModalShell } from 'nanocat-ui'
import PagePanel from '@/components/ai/PagePanel.vue'
import TableShell from '@/components/ai/TableShell.vue'
import TonePill from '@/components/TonePill.vue'
import { api, getRole } from '@/api'
import { consoleBaseFromPath, isAdminRole } from '@/lib/auth'
import { copyText } from '@/lib/copyText'
import { errorCodeLabel, errorCodeTitle, progressLabel, progressTone, statusLabel, statusTone } from '@/lib/taskLabels'

const route = useRoute()
const isAdmin = computed(() => isAdminRole(getRole()) || consoleBaseFromPath(route.path) === '/admin')
const ownerFilter = ref('')
const ownerNames = ref([])
const statusFilter = ref('')
const keyword = ref('')
let keywordTimer = 0

const statusOptions = [
  { value: '', label: '全部状态' },
  { value: 'queued', label: '排队中' },
  { value: 'running', label: '运行中' },
  { value: 'succeeded', label: '已完成' },
  { value: 'failed', label: '失败' },
  { value: 'cancelled', label: '已取消' },
]

const rows = ref([])
const templates = ref([])
const error = ref('')
const okmsg = ref('')
const detail = ref(null)
const busyId = ref('')
const loading = ref(false)
const page = ref(1)
const pageSize = ref(50)
const total = ref(0)
const pages = ref(0)
const pageSizeOptions = [20, 50, 100, 200]
let timer
const toolbarButtonClass = 'h-8 justify-center px-3 text-xs'

const pageRangeLabel = computed(() => {
  if (!total.value) return '共 0 条'
  const start = (page.value - 1) * pageSize.value + 1
  const end = Math.min(page.value * pageSize.value, total.value)
  return `第 ${start}-${end} 条 / 共 ${total.value} 条`
})

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

function isImageTask(t) {
  const wid = String(t?.webapp_id || '').trim().toLowerCase()
  if (wid.startsWith('qcimg:')) return true
  const res = String(t?.resolution || '').trim().toLowerCase()
  return res === '1k' || res === '2k' || res === '4k'
}

/** Strip "3:4 (Portrait Standard)" → "3:4". */
function shortRatio(v) {
  const s = String(v || '').trim()
  if (!s) return ''
  const m = s.match(/^(\d+\s*:\s*\d+)/)
  return m ? m[1].replace(/\s+/g, '') : s
}

/** Queue column: video → 比例/分辨率/时长；image → size/quality. */
function paramsLabel(t) {
  if (!t) return '—'
  if (isImageTask(t)) {
    const size = shortRatio(t.aspect_ratio) || String(t.aspect_ratio || '').trim() || '—'
    const quality = String(t.resolution || '').trim() || '—'
    return `${size} · ${quality}`
  }
  const parts = []
  const aspect = shortRatio(t.aspect_ratio)
  const res = String(t.resolution || '').trim()
  const dur = String(t.duration || '').trim()
  if (aspect) parts.push(aspect)
  if (res) parts.push(res)
  if (dur) parts.push(`${dur}s`)
  return parts.length ? parts.join(' · ') : '—'
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
    { label: '调用用户', value: t.owner || (t.user_id ? `#${t.user_id}` : '—') },
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
    { label: '类型', value: isImageTask(t) ? '图片' : '视频' },
    ...(isImageTask(t)
      ? [
          { label: 'size', value: shortRatio(t.aspect_ratio) || t.aspect_ratio || '—' },
          { label: 'quality', value: t.resolution || '—' },
        ]
      : [
          { label: '时长', value: t.duration || '—' },
          { label: '画幅', value: shortRatio(t.aspect_ratio) || t.aspect_ratio || '—' },
          { label: '分辨率', value: t.resolution || '—' },
        ]),
    { label: '尝试次数', value: t.attempts ?? 0 },
    { label: '错误码', value: errorCodeLabel(t) !== '—' ? errorCodeLabel(t) : (t.error_code || '—'), mono: true },
    { label: '网关码', value: t.error_code || '—' },
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

async function loadOwners() {
  if (!isAdmin.value) {
    ownerNames.value = []
    return
  }
  try {
    const res = await api.listUsers()
    const list = Array.isArray(res?.data) ? res.data : Array.isArray(res) ? res : []
    ownerNames.value = list.map((u) => String(u.username || '').trim()).filter(Boolean)
  } catch {
    ownerNames.value = []
  }
}

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
  loading.value = true
  try {
    const res = await api.listAdminTasks({
      page: page.value,
      pageSize: pageSize.value,
      owner: ownerFilter.value.trim(),
      q: keyword.value.trim(),
      status: statusFilter.value.trim(),
    })
    rows.value = res.data || []
    total.value = Number(res.total) || 0
    pages.value = Number(res.pages) || 0
    if (res.page_size) pageSize.value = Number(res.page_size) || pageSize.value
    const serverPage = Number(res.page) || 1
    page.value = pages.value === 0 ? 1 : Math.min(Math.max(1, serverPage), pages.value)
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    loading.value = false
  }
}

function onKeywordInput(v) {
  keyword.value = String(v ?? '')
  if (keywordTimer) clearTimeout(keywordTimer)
  keywordTimer = setTimeout(() => {
    page.value = 1
    refresh()
  }, 300)
}

function onOwnerChange() {
  page.value = 1
  refresh()
}

function onStatusChange() {
  page.value = 1
  refresh()
}

function onPageSizeChange() {
  page.value = 1
  refresh()
}

function goPage(next) {
  const target = Math.max(1, Math.min(pages.value || 1, Number(next) || 1))
  if (target === page.value) return
  page.value = target
  refresh()
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
    if (res?.requeued || task?.status === 'queued') {
      okmsg.value = `${t.id} 无 RH Task，已重新入队等待提交`
    } else if (task?.status === 'succeeded') {
      okmsg.value = `${t.id} 已同步：完成`
    } else if (task?.status === 'failed') {
      okmsg.value = `${t.id} 已同步：失败（${task.error || task.error_code || '见详情'}）`
    } else {
      okmsg.value = `${t.id} 已查询：${progressLabel(task)} / ${task?.rh_status || task?.status || '—'}`
    }
  } catch (e) {
    await refresh()
    error.value = e.message || String(e)
  } finally {
    busyId.value = ''
  }
}

function canSync(t) {
  // 运行中/排队都显示「查询」：有 RH id 则拉进度；无 id 的卡住任务会重新入队
  return t?.status === 'running' || t?.status === 'queued'
}

function openDetail(t) {
  detail.value = { ...t }
}

function closeDetail() {
  detail.value = null
}

async function copy(text) {
  const value = String(text ?? '').trim()
  if (!value || value === '—') {
    error.value = '没有可复制的内容'
    return
  }
  try {
    await copyText(value)
    error.value = ''
    okmsg.value = '已复制'
    window.setTimeout(() => {
      if (okmsg.value === '已复制') okmsg.value = ''
    }, 1200)
  } catch {
    error.value = '复制失败，请手动选中后复制'
  }
}

watch(rows, () => {
  if (!detail.value?.id) return
  const live = rows.value.find((x) => x.id === detail.value.id)
  if (live) detail.value = { ...live }
})

onMounted(() => {
  loadOwners()
  refreshTemplates()
  refresh()
  timer = setInterval(refresh, 3000)
})
onUnmounted(() => {
  if (keywordTimer) clearTimeout(keywordTimer)
  clearInterval(timer)
})
</script>

<template>
  <div class="relative space-y-5">
    <PagePanel class="space-y-5">
      <div class="flex flex-nowrap items-center gap-2 overflow-x-auto whitespace-nowrap">
        <Input
          :model-value="keyword"
          type="text"
          placeholder="搜索本地 ID / RH Task"
          root-class="h-8 w-56 shrink-0"
          @update:model-value="onKeywordInput"
        />
        <select
          v-model="statusFilter"
          class="h-8 shrink-0 rounded-xl border border-border bg-background px-3 text-xs text-foreground"
          @change="onStatusChange"
        >
          <option v-for="opt in statusOptions" :key="opt.value || 'all'" :value="opt.value">
            {{ opt.label }}
          </option>
        </select>
        <select
          v-if="isAdmin"
          v-model="ownerFilter"
          class="h-8 shrink-0 rounded-xl border border-border bg-background px-3 text-xs text-foreground"
          @change="onOwnerChange"
        >
          <option value="">全部用户</option>
          <option v-for="name in ownerNames" :key="name" :value="name">{{ name }}</option>
        </select>
        <label class="flex shrink-0 items-center gap-2 text-xs text-muted-foreground">
          每页
          <select
            v-model.number="pageSize"
            class="h-8 rounded-xl border border-border bg-background px-2 text-xs text-foreground"
            @change="onPageSizeChange"
          >
            <option v-for="n in pageSizeOptions" :key="n" :value="n">{{ n }}</option>
          </select>
        </label>
        <span class="shrink-0 text-xs text-muted-foreground">{{ pageRangeLabel }}</span>
        <Button size="sm" variant="outline" :root-class="toolbarButtonClass" :disabled="loading" @click="refresh">
          刷新列表
        </Button>
      </div>

      <TableShell>
        <table class="w-full min-w-[1320px] text-left text-sm">
          <thead class="text-xs uppercase tracking-[0.16em] text-muted-foreground">
            <tr>
              <th class="py-3 pr-5">本地 ID</th>
              <th v-if="isAdmin" class="py-3 pr-5">用户</th>
              <th class="py-3 pr-5">账号</th>
              <th class="py-3 pr-5">模板/应用</th>
              <th class="py-3 pr-5">参数</th>
              <th class="py-3 pr-5">RH Task</th>
              <th class="py-3 pr-5">类型</th>
              <th class="py-3 pr-5">状态</th>
              <th class="py-3 pr-5">进度</th>
              <th class="py-3 pr-5">成品</th>
              <th class="w-[10rem] py-3 pr-5">错误码</th>
              <th class="py-3 text-right">操作</th>
            </tr>
          </thead>
          <tbody class="text-sm text-foreground">
            <tr v-for="t in rows" :key="t.id" class="border-t border-border/70">
              <td class="py-3 pr-5">
                <button type="button" class="font-mono text-xs hover:underline" @click="copy(t.id)">{{ t.id }}</button>
              </td>
              <td v-if="isAdmin" class="py-3 pr-5">
                <span v-if="t.owner">{{ t.owner }}</span>
                <span v-else class="text-muted-foreground">—</span>
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
                <div
                  class="max-w-[14rem] truncate text-xs text-muted-foreground"
                  :title="paramsLabel(t)"
                >{{ paramsLabel(t) }}</div>
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
                <span
                  class="task-type-badge inline-flex h-8 w-8 items-center justify-center rounded-full"
                  :class="isImageTask(t) ? 'task-type-image' : 'task-type-video'"
                  :title="isImageTask(t) ? '图片任务' : '视频任务'"
                  :aria-label="isImageTask(t) ? '图片任务' : '视频任务'"
                >
                  <svg
                    v-if="isImageTask(t)"
                    aria-hidden="true"
                    viewBox="0 0 24 24"
                    class="h-4 w-4"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.9"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <rect x="3" y="5" width="18" height="14" rx="2" />
                    <circle cx="9" cy="10" r="1.5" fill="currentColor" stroke="none" />
                    <path d="M7 17l4-4 3 3 3-4 3 5" />
                  </svg>
                  <svg
                    v-else
                    aria-hidden="true"
                    viewBox="0 0 24 24"
                    class="h-4 w-4"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.9"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <rect x="3" y="6" width="13" height="12" rx="2" />
                    <path d="M16 10.5l5-2.5v8l-5-2.5v-3z" fill="currentColor" stroke="none" />
                  </svg>
                </span>
              </td>
              <td class="py-3 pr-5">
                <TonePill :tone="statusTone(t.status)" :label="statusLabel(t.status)" />
              </td>
              <td class="py-3 pr-5">
                <TonePill :tone="progressTone(t)" :label="progressLabel(t)" />
                <div
                  v-if="t.queue_count"
                  class="mt-1 text-xs text-muted-foreground"
                >
                  位次 {{ t.queue_index ?? 0 }}/{{ t.queue_count }}
                </div>
              </td>
              <td class="py-3 pr-5">
                <div v-if="t.file_url" class="inline-flex items-center gap-1.5">
                  <Button
                    tag="a"
                    size="xs"
                    variant="outline"
                    :href="t.file_url"
                    target="_blank"
                    rel="noreferrer"
                  >打开</Button>
                  <Button size="xs" variant="outline" @click="copy(t.file_url)">复制</Button>
                </div>
                <span v-else class="text-muted-foreground">—</span>
              </td>
              <td class="w-[10rem] max-w-[10rem] overflow-hidden py-3 pr-5">
                <div
                  class="block max-w-[10rem] truncate font-mono text-xs text-muted-foreground"
                  :title="errorCodeTitle(t)"
                >{{ errorCodeLabel(t) }}</div>
              </td>
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
                    {{ busyId === t.id ? '查询中…' : '查询' }}
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
            <tr v-if="!loading && !rows.length">
              <td :colspan="isAdmin ? 12 : 11" class="py-8 text-center text-sm text-muted-foreground">暂无任务</td>
            </tr>
            <tr v-if="loading && !rows.length">
              <td :colspan="isAdmin ? 12 : 11" class="py-8 text-center text-sm text-muted-foreground">加载中…</td>
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
          {{ busyId === detailLive?.id ? '查询中…' : '查询' }}
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

    <Teleport to="body">
      <div class="task-toast-stack" aria-live="polite">
        <div v-if="okmsg" class="task-toast task-toast--ok">{{ okmsg }}</div>
        <div v-if="error" class="task-toast task-toast--err">{{ error }}</div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.task-type-image {
  color: #0f766e;
  background: #ccfbf1;
  border: 1px solid #5eead4;
}

.task-type-video {
  color: #9a3412;
  background: #ffedd5;
  border: 1px solid #fdba74;
}

.task-toast-stack {
  position: fixed;
  right: 1.25rem;
  bottom: 1.25rem;
  z-index: 80;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 0.5rem;
  pointer-events: none;
}

.task-toast {
  max-width: min(22rem, calc(100vw - 2.5rem));
  padding: 0.65rem 0.9rem;
  border-radius: 0.75rem;
  border: 1px solid hsl(var(--border));
  background: hsl(var(--card) / 0.96);
  color: hsl(var(--card-foreground));
  font-size: 0.875rem;
  line-height: 1.35;
  box-shadow: var(--shadow-toast, 0 12px 30px rgba(0, 0, 0, 0.18));
  backdrop-filter: blur(8px);
}

.task-toast--ok {
  border-color: hsl(var(--tone-success-strong) / 0.35);
  color: hsl(var(--tone-success-strong));
}

.task-toast--err {
  border-color: hsl(var(--tone-error-strong) / 0.35);
  color: hsl(var(--tone-error-strong));
}
</style>

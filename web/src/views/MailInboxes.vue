<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Button, Input, ModalShell } from 'nanocat-ui'
import FilterToolbar from '@/components/ai/FilterToolbar.vue'
import MetricStrip from '@/components/ai/MetricStrip.vue'
import PagePanel from '@/components/ai/PagePanel.vue'
import TableShell from '@/components/ai/TableShell.vue'
import TonePill from '@/components/TonePill.vue'
import { api } from '@/api'
import { copyText } from '@/lib/copyText'

const rows = ref([])
const stats = ref({ all: 0, unused: 0, leased: 0, used: 0, dead: 0 })
const status = ref('')
const keyword = ref('')
const importText = ref('')
const showImport = ref(false)
const loading = ref(false)
const importing = ref(false)
const batchBusy = ref(false)
const selectedIds = ref(new Set())
const probingId = ref(0)
const error = ref('')
const okmsg = ref('')
const copiedKey = ref('')
const actionMenuId = ref(0)
const actionMenuStyle = ref({})
const actionMenuRow = ref(null)
let copiedTimer = 0
let keywordTimer = 0

const toolbarButtonClass = 'h-8 justify-center px-3 text-xs'

const statusOptions = [
  { value: '', label: '全部状态' },
  { value: 'unused', label: '可用' },
  { value: 'leased', label: '占用中' },
  { value: 'used', label: '已注册' },
  { value: 'dead', label: '失效' },
]

const metricItems = computed(() => [
  { key: 'all', label: '全部', value: String(stats.value.all || 0) },
  { key: 'unused', label: '可用', value: String(stats.value.unused || 0), valueClass: 'text-[hsl(var(--tone-success-strong))]' },
  { key: 'leased', label: '占用中', value: String(stats.value.leased || 0), valueClass: 'text-[hsl(var(--tone-warning-strong))]' },
  { key: 'used', label: '已注册', value: String(stats.value.used || 0) },
  { key: 'dead', label: '失效', value: String(stats.value.dead || 0), valueClass: 'text-[hsl(var(--tone-error-strong))]' },
])

const filtered = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  if (!q) return rows.value
  return rows.value.filter((row) => {
    const blob = `${row.email || ''} ${row.pickup_url || ''} ${row.status || ''} ${row.last_error || ''}`.toLowerCase()
    return blob.includes(q)
  })
})

const filteredIds = computed(() => filtered.value.map((row) => row.id))

const selectedCount = computed(() => {
  let n = 0
  for (const id of filteredIds.value) {
    if (selectedIds.value.has(id)) n += 1
  }
  return n
})

const allFilteredSelected = computed(
  () => filteredIds.value.length > 0 && selectedCount.value === filteredIds.value.length,
)

function statusTone(st) {
  if (st === 'unused') return 'success'
  if (st === 'leased') return 'warning'
  if (st === 'used') return 'neutral'
  if (st === 'dead') return 'error'
  return 'neutral'
}

function statusLabel(st) {
  if (st === 'unused') return '可用'
  if (st === 'leased') return '占用中'
  if (st === 'used') return '已注册'
  if (st === 'dead') return '失效'
  return st || '未知'
}

function isSelected(id) {
  return selectedIds.value.has(id)
}

function toggleRow(id, checked) {
  const next = new Set(selectedIds.value)
  if (checked) next.add(id)
  else next.delete(id)
  selectedIds.value = next
}

function toggleSelectAll(checked) {
  const next = new Set(selectedIds.value)
  for (const id of filteredIds.value) {
    if (checked) next.add(id)
    else next.delete(id)
  }
  selectedIds.value = next
}

function onHeaderCheck(e) {
  toggleSelectAll(!!e?.target?.checked)
}

function shortUrl(url) {
  const raw = String(url || '')
  if (!raw) return '—'
  if (raw.length <= 48) return raw
  try {
    const u = new URL(raw)
    const path = `${u.pathname}${u.search}`
    const tail = path.length > 32 ? `…${path.slice(-32)}` : path
    return `${u.host}${tail}`
  } catch {
    return `${raw.slice(0, 20)}…${raw.slice(-20)}`
  }
}

function placeActionMenu(anchorEl) {
  if (!anchorEl?.getBoundingClientRect) {
    actionMenuStyle.value = {}
    return
  }
  const rect = anchorEl.getBoundingClientRect()
  const menuWidth = 120
  const menuHeight = 96
  const gap = 4
  const left = Math.min(Math.max(8, rect.right - menuWidth), window.innerWidth - menuWidth - 8)
  const openUp = rect.bottom + gap + menuHeight > window.innerHeight - 8
  const top = openUp
    ? Math.max(8, rect.top - gap - menuHeight)
    : Math.min(rect.bottom + gap, window.innerHeight - menuHeight - 8)
  actionMenuStyle.value = {
    position: 'fixed',
    top: `${Math.round(top)}px`,
    left: `${Math.round(left)}px`,
    width: `${menuWidth}px`,
    zIndex: 80,
  }
}

function closeActionMenu() {
  actionMenuId.value = 0
  actionMenuRow.value = null
  actionMenuStyle.value = {}
}

function toggleActionMenu(row, event) {
  if (actionMenuId.value === row.id) {
    closeActionMenu()
    return
  }
  actionMenuId.value = row.id
  actionMenuRow.value = row
  placeActionMenu(event?.currentTarget)
}

function onDocClick(e) {
  if (!e.target?.closest?.('[data-mail-action-menu]')) closeActionMenu()
}

function onActionMenuReposition() {
  if (!actionMenuId.value) return
  closeActionMenu()
}

async function load() {
  error.value = ''
  loading.value = true
  try {
    const res = await api.listMailInboxes({ status: status.value, limit: 500 })
    rows.value = res.items || []
    stats.value = {
      all: Number(res.stats?.all ?? res.total ?? rows.value.length) || 0,
      unused: Number(res.stats?.unused ?? res.unused ?? 0) || 0,
      leased: Number(res.stats?.leased ?? 0) || 0,
      used: Number(res.stats?.used ?? 0) || 0,
      dead: Number(res.stats?.dead ?? 0) || 0,
    }
    const keep = new Set(rows.value.map((r) => r.id))
    const next = new Set()
    for (const id of selectedIds.value) {
      if (keep.has(id)) next.add(id)
    }
    selectedIds.value = next
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    loading.value = false
  }
}

function onKeywordInput(v) {
  keyword.value = String(v ?? '')
  if (keywordTimer) clearTimeout(keywordTimer)
  keywordTimer = setTimeout(() => {}, 0)
}

function openImport() {
  error.value = ''
  okmsg.value = ''
  importText.value = ''
  showImport.value = true
}

async function doImport() {
  error.value = ''
  okmsg.value = ''
  importing.value = true
  try {
    const res = await api.importMailInboxes(importText.value)
    okmsg.value = `导入 ${res.imported || 0}，更新 ${res.updated || 0}，跳过 ${res.skipped || 0}`
    if (res.errors?.length) {
      error.value = res.errors.slice(0, 8).join('；')
    } else {
      showImport.value = false
      importText.value = ''
    }
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    importing.value = false
  }
}

async function removeRow(id) {
  closeActionMenu()
  if (!confirm('删除这条邮箱？')) return
  try {
    await api.deleteMailInbox(id)
    const next = new Set(selectedIds.value)
    next.delete(id)
    selectedIds.value = next
    okmsg.value = '已删除'
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  }
}

async function batchDelete() {
  const ids = filteredIds.value.filter((id) => selectedIds.value.has(id))
  if (!ids.length) {
    error.value = '请先勾选要删除的邮箱'
    return
  }
  if (!confirm(`确认删除选中的 ${ids.length} 个邮箱？`)) return
  batchBusy.value = true
  error.value = ''
  okmsg.value = ''
  try {
    const res = await api.deleteMailInboxes(ids)
    selectedIds.value = new Set()
    okmsg.value = `已删除 ${res.deleted ?? ids.length} 个邮箱`
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    batchBusy.value = false
  }
}

async function testRow(id) {
  closeActionMenu()
  probingId.value = id
  error.value = ''
  try {
    const res = await api.testMailInbox(id)
    if (!res.ok) {
      error.value = res.error || '取件失败'
      okmsg.value = ''
      return
    }
    okmsg.value = res.has_code ? `探测到验证码：${res.code_preview}` : '链接可访问，暂无验证码'
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    probingId.value = 0
  }
}

async function copy(text, key) {
  try {
    await copyText(text)
    copiedKey.value = key
    if (copiedTimer) clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => {
      copiedKey.value = ''
    }, 1200)
  } catch {
    error.value = '复制失败'
  }
}

onMounted(() => {
  document.addEventListener('click', onDocClick)
  window.addEventListener('resize', onActionMenuReposition)
  window.addEventListener('scroll', onActionMenuReposition, true)
  load()
})

onUnmounted(() => {
  if (keywordTimer) clearTimeout(keywordTimer)
  if (copiedTimer) clearTimeout(copiedTimer)
  document.removeEventListener('click', onDocClick)
  window.removeEventListener('resize', onActionMenuReposition)
  window.removeEventListener('scroll', onActionMenuReposition, true)
})
</script>

<template>
  <div class="relative space-y-5">
    <PagePanel class="space-y-5">
      <MetricStrip :items="metricItems" columns-class="grid-cols-2 md:grid-cols-5" density="compact" />

      <div class="flex flex-col gap-3">
        <FilterToolbar :bordered="false">
          <Input
            :model-value="keyword"
            type="text"
            placeholder="搜索邮箱 / 取件链接 / 错误"
            block
            root-class="min-w-[14rem] flex-1 md:max-w-sm"
            @update:model-value="onKeywordInput"
          />
          <select
            v-model="status"
            class="h-9 rounded-xl border border-border bg-background px-3 text-sm text-foreground"
            @change="load"
          >
            <option v-for="opt in statusOptions" :key="opt.value || 'all'" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
        </FilterToolbar>

        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="text-xs text-muted-foreground">
            共 {{ stats.all }} 个邮箱 · 已选 {{ selectedCount }} · 注册成功会标「已注册」且不再被取用 · 仅 @icloud.com
          </p>
          <div class="flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="outline"
              :root-class="toolbarButtonClass"
              :disabled="batchBusy || !selectedCount"
              @click="batchDelete"
            >
              批量删除
            </Button>
            <Button size="sm" variant="outline" :root-class="toolbarButtonClass" :disabled="loading || batchBusy" @click="load">
              刷新列表
            </Button>
            <Button size="sm" variant="primary" :root-class="toolbarButtonClass" @click="openImport">
              导入 Inbox
            </Button>
          </div>
        </div>
      </div>

      <p v-if="okmsg && !showImport" class="text-sm text-[hsl(var(--tone-success-strong))]">{{ okmsg }}</p>
      <p v-if="error && !showImport" class="text-sm text-[hsl(var(--tone-error-strong))]">{{ error }}</p>

      <TableShell>
        <table class="w-full min-w-[960px] text-left text-sm">
          <thead class="text-xs uppercase tracking-[0.16em] text-muted-foreground">
            <tr>
              <th class="w-10 py-3 pr-3">
                <input
                  type="checkbox"
                  class="h-4 w-4 accent-primary"
                  :checked="allFilteredSelected"
                  :disabled="!filtered.length || batchBusy"
                  @change="onHeaderCheck"
                />
              </th>
              <th class="py-3 pr-5">邮箱</th>
              <th class="py-3 pr-5">状态</th>
              <th class="py-3 pr-5">取件链接</th>
              <th class="py-3 pr-5">错误</th>
              <th class="py-3 pl-3 text-center">操作</th>
            </tr>
          </thead>
          <tbody class="text-sm text-foreground">
            <tr v-for="row in filtered" :key="row.id" class="border-t border-border/70">
              <td class="py-3 pr-3">
                <input
                  type="checkbox"
                  class="h-4 w-4 accent-primary"
                  :checked="isSelected(row.id)"
                  :disabled="batchBusy"
                  @change="toggleRow(row.id, $event.target.checked)"
                />
              </td>
              <td class="py-3 pr-5">
                <button
                  type="button"
                  class="text-left font-medium text-foreground hover:underline"
                  @click="copy(row.email, `email-${row.id}`)"
                >
                  {{ copiedKey === `email-${row.id}` ? '已复制' : row.email }}
                </button>
              </td>
              <td class="py-3 pr-5">
                <TonePill :tone="statusTone(row.status)" :label="statusLabel(row.status)" />
              </td>
              <td class="py-3 pr-5">
                <button
                  type="button"
                  class="max-w-[28rem] truncate text-left font-mono text-xs text-muted-foreground hover:underline"
                  :title="row.pickup_url"
                  @click="copy(row.pickup_url, `url-${row.id}`)"
                >
                  {{ copiedKey === `url-${row.id}` ? '已复制' : shortUrl(row.pickup_url) }}
                </button>
              </td>
              <td class="py-3 pr-5">
                <span
                  class="block max-w-[14rem] truncate text-muted-foreground"
                  :title="row.last_error || ''"
                >
                  {{ row.last_error || '—' }}
                </span>
              </td>
              <td class="px-3 py-3 text-center" data-mail-action-menu>
                <Button
                  size="xs"
                  variant="outline"
                  :disabled="probingId === row.id || batchBusy"
                  @click.stop="toggleActionMenu(row, $event)"
                >
                  操作
                </Button>
              </td>
            </tr>
            <tr v-if="!loading && !filtered.length">
              <td colspan="6" class="py-8 text-center text-sm text-muted-foreground">
                {{ rows.length ? '没有匹配的邮箱' : '暂无邮箱。可点击右上角导入 Inbox。' }}
              </td>
            </tr>
            <tr v-if="loading && !rows.length">
              <td colspan="6" class="py-8 text-center text-sm text-muted-foreground">加载中…</td>
            </tr>
          </tbody>
        </table>
      </TableShell>
    </PagePanel>

    <Teleport to="body">
      <div
        v-if="actionMenuId && actionMenuRow"
        data-mail-action-menu
        class="overflow-hidden rounded-xl border border-border bg-card py-1 text-left shadow-lg"
        :style="actionMenuStyle"
        @click.stop
      >
        <button
          type="button"
          class="block w-full px-3 py-1.5 text-left text-xs hover:bg-muted/60 disabled:opacity-40"
          :disabled="probingId === actionMenuRow.id"
          @click="testRow(actionMenuRow.id)"
        >
          {{ probingId === actionMenuRow.id ? '探测中…' : '探测取件' }}
        </button>
        <button
          type="button"
          class="block w-full px-3 py-1.5 text-left text-xs text-[hsl(var(--tone-error-strong))] hover:bg-muted/60"
          @click="removeRow(actionMenuRow.id)"
        >
          删除
        </button>
      </div>
    </Teleport>

    <ModalShell
      :open="showImport"
      title="导入 Inbox"
      description="每行一条：邮箱----取件链接。仅接受 @icloud.com。"
      size-class="max-w-xl"
      close-on-overlay
      @close="showImport = false"
    >
      <div class="space-y-3">
        <label class="block text-xs font-medium text-muted-foreground">批量文本</label>
        <textarea
          v-model="importText"
          rows="10"
          class="w-full rounded-2xl border border-border bg-background px-3 py-2 font-mono text-xs outline-none focus:border-primary"
          spellcheck="false"
          placeholder="name@icloud.com----https://inbox.example/pickup/xxx"
        />
        <p v-if="error" class="text-sm text-[hsl(var(--tone-error-strong))]">{{ error }}</p>
        <p v-else-if="okmsg" class="text-sm text-[hsl(var(--tone-success-strong))]">{{ okmsg }}</p>
      </div>
      <template #footer>
        <Button size="sm" variant="outline" :root-class="toolbarButtonClass" @click="showImport = false">
          取消
        </Button>
        <Button
          size="sm"
          variant="primary"
          :root-class="toolbarButtonClass"
          :disabled="importing || !importText.trim()"
          @click="doImport"
        >
          {{ importing ? '导入中…' : '导入到邮箱池' }}
        </Button>
      </template>
    </ModalShell>
  </div>
</template>

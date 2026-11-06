<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { Button, Input, ModalShell } from 'nanocat-ui'
import FilterToolbar from '@/components/ai/FilterToolbar.vue'
import MetricStrip from '@/components/ai/MetricStrip.vue'
import PagePanel from '@/components/ai/PagePanel.vue'
import TableShell from '@/components/ai/TableShell.vue'
import TonePill from '@/components/TonePill.vue'
import { api } from '@/api'

const rows = ref([])
const error = ref('')
const okmsg = ref('')
const importText = ref('{"accounts":[{"email":"a@b.com","password":"plain","status":"active","source":"manual"}]}')
const loading = ref(false)
const batchBusy = ref(false)
const showImport = ref(false)
const showAdd = ref(false)
const keyword = ref('')
const busyId = ref(0)
const selectedIds = ref(new Set())
const detail = ref(null)
const detailLoading = ref(false)
const headerCheckEl = ref(null)
const toolbarButtonClass = 'h-8 justify-center px-3 text-xs'

const addForm = reactive({
  email: '',
  password: '',
  access_token: '',
  api_key: '',
  remain_coins: '',
  source: 'manual',
  status: 'active',
  external_id: '',
})

const metricItems = computed(() => {
  const list = rows.value || []
  const active = list.filter((x) => x.dispatchable).length
  const busy = list.filter((x) => x.busy).length
  const cooldown = list.filter((x) => x.cooldown_until).length
  const failed = list.filter((x) => Number(x.fail_count || 0) > 0).length
  return [
    { key: 'total', label: '号池总数', value: list.length },
    { key: 'active', label: '可调度', value: active },
    { key: 'busy', label: '占用中', value: busy },
    { key: 'cool', label: '冷却中', value: cooldown },
    { key: 'fail', label: '有失败记录', value: failed },
  ]
})

const filtered = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  if (!q) return rows.value
  return rows.value.filter((a) =>
    [a.id, a.email, a.status, a.source, a.api_type].join(' ').toLowerCase().includes(q),
  )
})

const filteredIds = computed(() => filtered.value.map((a) => a.id))

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

const someFilteredSelected = computed(
  () => selectedCount.value > 0 && selectedCount.value < filteredIds.value.length,
)

const detailFields = computed(() => {
  const a = detail.value
  if (!a) return []
  const dash = (v) => (v == null || v === '' ? '—' : String(v))
  return [
    { label: 'ID', value: dash(a.id), copy: true, mono: true },
    { label: '邮箱', value: dash(a.email), copy: true },
    { label: '状态', value: dash(a.status) },
    { label: '可调度', value: a.dispatchable ? '是' : '否' },
    { label: '占用中', value: a.busy ? '是' : '否' },
    { label: 'RH 币', value: dash(a.remain_coins) },
    { label: '在途任务', value: dash(a.current_task_counts || '0') },
    { label: 'API Type', value: dash(a.api_type) },
    { label: '来源', value: dash(a.source) },
    { label: '外部 ID', value: dash(a.external_id), copy: !!a.external_id, mono: true },
    { label: 'RH User ID', value: dash(a.rh_user_id), copy: !!a.rh_user_id, mono: true },
    { label: 'Identify', value: dash(a.identify), copy: !!a.identify, mono: true },
    { label: '指纹', value: a.has_fingerprint || a.fingerprint_json ? '有' : '无' },
    { label: '指纹 JSON', value: dash(a.fingerprint_json), copy: !!a.fingerprint_json, mono: true, wide: true, pre: true },
    { label: '绑定出口', value: a.egress_label || (a.egress_id ? `#${a.egress_id}` : '—'), mono: true },
    { label: '冷却至', value: dash(a.cooldown_until), mono: true },
    { label: '失败次数', value: dash(a.fail_count ?? 0) },
    { label: '最近余额时间', value: dash(a.last_balance_at), mono: true },
    { label: '每日积分领取日', value: dash(a.last_login_coin_at), mono: true },
    { label: 'Lease Task', value: dash(a.lease_task_id), mono: true, copy: !!a.lease_task_id },
    { label: 'Lease Until', value: dash(a.lease_until), mono: true },
    { label: '创建时间', value: dash(a.created_at), mono: true },
    { label: '更新时间', value: dash(a.updated_at), mono: true },
    { label: '密码', value: dash(a.password), copy: !!a.password, mono: true, wide: true },
    { label: 'API Key', value: dash(a.api_key), copy: !!a.api_key, mono: true, wide: true },
    { label: 'Access Token (AT)', value: dash(a.access_token), copy: !!a.access_token, mono: true, wide: true },
    { label: 'Refresh Token (RT)', value: dash(a.refresh_token), copy: !!a.refresh_token, mono: true, wide: true },
    { label: '最近错误', value: dash(a.last_error), wide: true, pre: true },
  ]
})

function resetAddForm() {
  addForm.email = ''
  addForm.password = ''
  addForm.access_token = ''
  addForm.api_key = ''
  addForm.remain_coins = ''
  addForm.source = 'manual'
  addForm.status = 'active'
  addForm.external_id = ''
}

function openAdd() {
  error.value = ''
  okmsg.value = ''
  resetAddForm()
  showAdd.value = true
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

function selectedRows() {
  return filtered.value.filter((a) => selectedIds.value.has(a.id))
}

async function refresh() {
  error.value = ''
  try {
    const res = await api.listAccounts()
    rows.value = res.data || []
  } catch (e) {
    error.value = e.message || String(e)
  }
}

async function doAdd() {
  error.value = ''
  okmsg.value = ''
  loading.value = true
  try {
    const email = String(addForm.email || '').trim()
    if (!email) throw new Error('邮箱必填')
    if (!addForm.password && !addForm.access_token && !addForm.api_key) {
      throw new Error('密码 / Access Token / API Key 至少填一项')
    }
    await api.createAccount({
      email,
      password: String(addForm.password || '').trim(),
      access_token: String(addForm.access_token || '').trim(),
      api_key: String(addForm.api_key || '').trim(),
      remain_coins: String(addForm.remain_coins || '').trim(),
      source: String(addForm.source || 'manual').trim() || 'manual',
      status: addForm.status === 'disabled' ? 'disabled' : 'active',
      external_id: String(addForm.external_id || '').trim(),
    })
    okmsg.value = `已添加 ${email}`
    showAdd.value = false
    await refresh()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    loading.value = false
  }
}

async function doImport() {
  error.value = ''
  okmsg.value = ''
  loading.value = true
  try {
    const parsed = JSON.parse(importText.value)
    const accounts = Array.isArray(parsed) ? parsed : parsed.accounts
    if (!Array.isArray(accounts)) throw new Error('需要 accounts 数组')
    const res = await api.importAccounts(accounts)
    okmsg.value = `导入成功 ${res.imported}，失败 ${res.failed}`
    showImport.value = false
    await refresh()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    loading.value = false
  }
}

async function refreshOne(row) {
  error.value = ''
  busyId.value = row.id
  try {
    await api.refreshAccount(row.id)
    await refresh()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    busyId.value = 0
  }
}

async function ensureKeyOne(row) {
  error.value = ''
  okmsg.value = ''
  let force = false
  if (row.has_api_key) {
    if (!window.confirm(`账号 ${row.email} 已有 API Key，继续将重置并作废旧 Key，是否继续？`)) return
    force = true
  }
  busyId.value = row.id
  try {
    const res = await api.ensureAccountApiKey(row.id, force)
    okmsg.value = res?.result?.message || `已补 Key：${row.email}`
    await refresh()
    if (detail.value?.id === row.id) {
      detail.value = await api.getAccount(row.id)
    }
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    busyId.value = 0
  }
}

async function toggleStatus(row) {
  error.value = ''
  busyId.value = row.id
  try {
    if (row.status === 'active') await api.disableAccount(row.id)
    else await api.enableAccount(row.id)
    await refresh()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    busyId.value = 0
  }
}

async function runBatch(label, worker) {
  const list = selectedRows()
  if (!list.length) {
    error.value = '请先勾选账号'
    return
  }
  error.value = ''
  okmsg.value = ''
  batchBusy.value = true
  let ok = 0
  const fails = []
  const succeeded = new Set()
  try {
    for (const row of list) {
      try {
        await worker(row)
        ok += 1
        succeeded.add(row.id)
      } catch (e) {
        fails.push(`${row.email || row.id}: ${e.message || e}`)
      }
    }
    if (succeeded.size) {
      const next = new Set(selectedIds.value)
      for (const id of succeeded) next.delete(id)
      selectedIds.value = next
    }
    await refresh()
    okmsg.value = `${label}完成：成功 ${ok}，失败 ${fails.length}`
    if (fails.length) error.value = fails.slice(0, 5).join('；')
  } finally {
    batchBusy.value = false
  }
}

async function batchDelete() {
  const list = selectedRows()
  if (!list.length) {
    error.value = '请先勾选账号'
    return
  }
  if (!window.confirm(`确定删除选中的 ${list.length} 个账号？此操作会直接从数据库删除，不可恢复。`)) {
    return
  }
  await runBatch('批量删除', (row) => api.deleteAccount(row.id))
}

async function batchRefresh() {
  await runBatch('批量刷新', (row) => api.refreshAccount(row.id))
}

async function batchClaimDailyLogin() {
  const list = selectedRows()
  const ids = list.map((r) => r.id)
  const scope = ids.length ? `选中的 ${ids.length} 个账号` : '全部活跃账号'
  if (!window.confirm(`用已保存 AT 调用 getUserInfo 领取今日登录积分（${scope}）？`)) {
    return
  }
  error.value = ''
  okmsg.value = ''
  batchBusy.value = true
  try {
    const res = await api.claimDailyLogin(ids, false)
    await refresh()
    okmsg.value = `每日积分 ${res.day || ''}：成功 ${res.success || 0}，跳过 ${res.skipped || 0}，失败 ${res.failed || 0}，合计 +${res.gained || 0}`
    if (Number(res.failed || 0) > 0) {
      const fails = (res.results || [])
        .filter((x) => !x.ok && !x.skipped)
        .slice(0, 5)
        .map((x) => `${x.email || x.id}: ${x.error}`)
      error.value = fails.join('；')
    }
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    batchBusy.value = false
  }
}

async function batchEnsureKey() {
  const list = selectedRows()
  if (!list.length) {
    error.value = '请先勾选账号'
    return
  }
  const needForce = list.some((r) => r.has_api_key)
  let force = false
  if (needForce) {
    if (
      !window.confirm(
        `选中账号中有 ${list.filter((r) => r.has_api_key).length} 个已有 Key。确定全部补/重置？已有 Key 的会作废旧 Key。`,
      )
    ) {
      return
    }
    force = true
  }
  error.value = ''
  okmsg.value = ''
  batchBusy.value = true
  try {
    const res = await api.ensureAccountsApiKey(
      list.map((r) => r.id),
      force,
    )
    await refresh()
    okmsg.value = `补 Key：成功 ${res.success || 0}，跳过 ${res.skipped || 0}，失败 ${res.failed || 0}`
    if (Number(res.failed || 0) > 0) {
      const fails = (res.results || [])
        .filter((x) => !x.ok)
        .slice(0, 5)
        .map((x) => `${x.email || x.id}: ${x.error}`)
      error.value = fails.join('；')
    } else {
      const next = new Set(selectedIds.value)
      for (const row of list) next.delete(row.id)
      selectedIds.value = next
    }
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    batchBusy.value = false
  }
}

async function batchToggleStatus() {
  await runBatch('批量启用/禁用', async (row) => {
    if (row.status === 'active') await api.disableAccount(row.id)
    else await api.enableAccount(row.id)
  })
}

let detailRequestSeq = 0

async function openDetail(row) {
  error.value = ''
  const seq = ++detailRequestSeq
  detailLoading.value = true
  detail.value = { id: row.id, email: row.email, status: row.status }
  try {
    const data = await api.getAccount(row.id)
    if (seq !== detailRequestSeq) return
    detail.value = data
  } catch (e) {
    if (seq !== detailRequestSeq) return
    error.value = e.message || String(e)
    detail.value = null
  } finally {
    if (seq === detailRequestSeq) detailLoading.value = false
  }
}

function closeDetail() {
  detailRequestSeq += 1
  detailLoading.value = false
  detail.value = null
}

function copy(text) {
  navigator.clipboard?.writeText(text || '')
}

function statusTone(status) {
  const s = String(status || '').toLowerCase()
  if (s === 'active') return 'success'
  if (s === 'disabled') return 'error'
  return 'warning'
}

function statusLabel(status) {
  const s = String(status || '').toLowerCase()
  if (s === 'active') return '启用'
  if (s === 'disabled') return '禁用'
  return status || '未知'
}

function dispatchTone(row) {
  if (row?.dispatchable) return 'success'
  if (row?.busy) return 'info'
  if (row?.cooldown_until) return 'warning'
  return 'error'
}

function dispatchLabel(row) {
  if (row?.dispatchable) return '可调度'
  if (row?.busy) return '占用中'
  if (row?.cooldown_until) return '冷却中'
  return '不可用'
}

function tokenTone(ok) {
  return ok ? 'success' : 'error'
}

function tokenLabel(kind, ok) {
  return ok ? `${kind}有效` : `${kind}无效`
}

function onHeaderCheck(e) {
  toggleSelectAll(e.target.checked)
}

watch([someFilteredSelected, headerCheckEl], () => {
  if (headerCheckEl.value) {
    headerCheckEl.value.indeterminate = someFilteredSelected.value
  }
})

onMounted(refresh)
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
            placeholder="搜索邮箱 / ID / 状态 / 来源"
            block
            root-class="min-w-[14rem] flex-1 md:max-w-sm"
            @update:model-value="keyword = String($event ?? '').trim()"
          />
        </FilterToolbar>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="text-xs text-muted-foreground">
            共 {{ filtered.length }} 个账号 · 已选 {{ selectedCount }} · 每号并发上限 1
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
            <Button
              size="sm"
              variant="outline"
              :root-class="toolbarButtonClass"
              :disabled="batchBusy || !selectedCount"
              @click="batchRefresh"
            >
              刷新账号
            </Button>
            <Button
              size="sm"
              variant="outline"
              :root-class="toolbarButtonClass"
              :disabled="batchBusy || !selectedCount"
              @click="batchEnsureKey"
            >
              批量补 Key
            </Button>
            <Button
              size="sm"
              variant="outline"
              :root-class="toolbarButtonClass"
              :disabled="batchBusy"
              @click="batchClaimDailyLogin"
            >
              领每日积分
            </Button>
            <Button
              size="sm"
              variant="outline"
              :root-class="toolbarButtonClass"
              :disabled="batchBusy || !selectedCount"
              @click="batchToggleStatus"
            >
              启用/禁用
            </Button>
            <Button size="sm" :root-class="toolbarButtonClass" @click="openAdd">
              添加账号
            </Button>
            <Button
              size="sm"
              variant="outline"
              :root-class="toolbarButtonClass"
              @click="error = ''; showImport = true"
            >
              批量导入
            </Button>
            <Button
              size="sm"
              variant="outline"
              :root-class="toolbarButtonClass"
              :disabled="loading || batchBusy"
              @click="refresh"
            >
              刷新列表
            </Button>
          </div>
        </div>
      </div>

      <p v-if="okmsg" class="text-sm text-[hsl(var(--tone-success-strong))]">{{ okmsg }}</p>
      <p v-if="error && !showImport && !showAdd && !detail" class="text-sm text-[hsl(var(--tone-error-strong))]">{{ error }}</p>

      <TableShell>
        <table class="w-full min-w-[1520px] text-left text-sm">
          <thead class="text-xs uppercase tracking-[0.16em] text-muted-foreground">
            <tr>
              <th class="w-10 py-3 pr-3">
                <input
                  ref="headerCheckEl"
                  type="checkbox"
                  class="h-4 w-4 accent-primary"
                  :checked="allFilteredSelected"
                  :disabled="!filtered.length"
                  @change="onHeaderCheck"
                />
              </th>
              <th class="py-3 pr-5">账号</th>
              <th class="py-3 pr-5">状态</th>
              <th class="py-3 pr-5">可调度</th>
              <th class="py-3 pr-5">RH币</th>
              <th class="py-3 pr-5">每日积分</th>
              <th class="py-3 pr-5">指纹</th>
              <th class="py-3 pr-5">出口</th>
              <th class="py-3 pr-5">在途</th>
              <th class="py-3 pr-5">Key</th>
              <th class="py-3 pr-5">Token</th>
              <th class="py-3 pr-5">冷却</th>
              <th class="py-3 pr-5">失败</th>
              <th class="py-3 pr-5">来源</th>
              <th class="py-3 text-right">操作</th>
            </tr>
          </thead>
          <tbody class="text-sm text-foreground">
            <tr v-for="a in filtered" :key="a.id" class="border-t border-border/70">
              <td class="py-3 pr-3">
                <input
                  type="checkbox"
                  class="h-4 w-4 accent-primary"
                  :checked="isSelected(a.id)"
                  @change="toggleRow(a.id, $event.target.checked)"
                />
              </td>
              <td class="py-3 pr-5">
                <button
                  type="button"
                  class="text-left font-medium text-foreground hover:underline"
                  @click="copy(a.email)"
                >
                  {{ a.email }}
                </button>
              </td>
              <td class="py-3 pr-5">
                <TonePill :tone="statusTone(a.status)" :label="statusLabel(a.status)" />
              </td>
              <td class="py-3 pr-5">
                <TonePill :tone="dispatchTone(a)" :label="dispatchLabel(a)" />
              </td>
              <td class="py-3 pr-5">{{ a.remain_coins || '—' }}</td>
              <td class="py-3 pr-5 font-mono text-xs text-muted-foreground">{{ a.last_login_coin_at || '—' }}</td>
              <td class="py-3 pr-5">
                <TonePill
                  :tone="a.has_fingerprint ? 'success' : 'neutral'"
                  :label="a.has_fingerprint ? '有指纹' : '无'"
                />
              </td>
              <td class="py-3 pr-5 font-mono text-xs text-muted-foreground">{{ a.egress_label || '—' }}</td>
              <td class="py-3 pr-5">{{ a.current_task_counts || '0' }}</td>
              <td class="py-3 pr-5">
                <TonePill
                  :tone="a.has_api_key ? 'success' : 'error'"
                  :label="a.has_api_key ? (a.api_type || '有Key') : '无Key'"
                />
              </td>
              <td class="py-3 pr-5">
                <div class="flex flex-wrap gap-1">
                  <TonePill :tone="tokenTone(a.has_access_token)" :label="tokenLabel('AT', a.has_access_token)" />
                  <TonePill :tone="tokenTone(a.has_refresh_token)" :label="tokenLabel('RT', a.has_refresh_token)" />
                </div>
              </td>
              <td class="py-3 pr-5 font-mono text-xs text-muted-foreground">{{ a.cooldown_until || '—' }}</td>
              <td class="py-3 pr-5">{{ a.fail_count }}</td>
              <td class="py-3 pr-5 text-muted-foreground">{{ a.source || '—' }}</td>
              <td class="py-3 text-right">
                <div class="flex flex-wrap justify-end gap-2">
                  <Button
                    size="xs"
                    variant="outline"
                    :disabled="busyId === a.id || batchBusy"
                    @click="openDetail(a)"
                  >
                    详情
                  </Button>
                  <Button
                    size="xs"
                    variant="outline"
                    :disabled="busyId === a.id || batchBusy"
                    @click="ensureKeyOne(a)"
                  >
                    {{ a.has_api_key ? '重置Key' : '补Key' }}
                  </Button>
                  <Button
                    size="xs"
                    variant="outline"
                    :disabled="busyId === a.id || batchBusy"
                    @click="refreshOne(a)"
                  >
                    刷新
                  </Button>
                  <Button
                    size="xs"
                    variant="outline"
                    :disabled="busyId === a.id || batchBusy"
                    @click="toggleStatus(a)"
                  >
                    {{ a.status === 'active' ? '禁用' : '启用' }}
                  </Button>
                </div>
              </td>
            </tr>
            <tr v-if="!filtered.length">
              <td colspan="12" class="py-8 text-center text-sm text-muted-foreground">
                暂无账号。可单独添加、批量导入，或通过推送接口同步。
              </td>
            </tr>
          </tbody>
        </table>
      </TableShell>
    </PagePanel>

    <ModalShell
      :open="showAdd"
      title="添加账号"
      description="单独录入一个 RunningHub 账号到号池。"
      size-class="max-w-lg"
      close-on-overlay
      @close="showAdd = false"
    >
      <div class="space-y-3">
        <label class="block space-y-1.5">
          <span class="text-xs font-medium text-muted-foreground">邮箱 *</span>
          <Input v-model="addForm.email" type="email" block placeholder="name@example.com" />
        </label>
        <label class="block space-y-1.5">
          <span class="text-xs font-medium text-muted-foreground">密码</span>
          <Input v-model="addForm.password" type="text" block placeholder="明文密码（优先用于登录）" />
        </label>
        <label class="block space-y-1.5">
          <span class="text-xs font-medium text-muted-foreground">Access Token（可选）</span>
          <Input v-model="addForm.access_token" type="text" block placeholder="JWT" />
        </label>
        <label class="block space-y-1.5">
          <span class="text-xs font-medium text-muted-foreground">API Key（可选）</span>
          <Input v-model="addForm.api_key" type="text" block placeholder="消费级 32 位 key" />
        </label>
        <div class="grid grid-cols-2 gap-3">
          <label class="block space-y-1.5">
            <span class="text-xs font-medium text-muted-foreground">RH 币（可选）</span>
            <Input v-model="addForm.remain_coins" type="text" block placeholder="如 1100" />
          </label>
          <label class="block space-y-1.5">
            <span class="text-xs font-medium text-muted-foreground">状态</span>
            <select
              v-model="addForm.status"
              class="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm outline-none focus:border-primary"
            >
              <option value="active">active</option>
              <option value="disabled">disabled</option>
            </select>
          </label>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <label class="block space-y-1.5">
            <span class="text-xs font-medium text-muted-foreground">来源</span>
            <Input v-model="addForm.source" type="text" block placeholder="manual" />
          </label>
          <label class="block space-y-1.5">
            <span class="text-xs font-medium text-muted-foreground">外部 ID</span>
            <Input v-model="addForm.external_id" type="text" block placeholder="外部系统 id（可选）" />
          </label>
        </div>
        <p class="text-xs text-muted-foreground">密码 / Access Token / API Key 至少填一项。</p>
        <p v-if="error" class="text-sm text-[hsl(var(--tone-error-strong))]">{{ error }}</p>
      </div>
      <template #footer>
        <Button size="sm" variant="outline" :root-class="toolbarButtonClass" @click="showAdd = false">
          取消
        </Button>
        <Button size="sm" variant="primary" :root-class="toolbarButtonClass" :disabled="loading" @click="doAdd">
          {{ loading ? '提交中…' : '添加到号池' }}
        </Button>
      </template>
    </ModalShell>

    <ModalShell
      :open="showImport"
      title="批量导入"
      description="粘贴 accounts JSON 批量导入号池。"
      size-class="max-w-xl"
      close-on-overlay
      @close="showImport = false"
    >
      <div class="space-y-3">
        <label class="block text-xs font-medium text-muted-foreground">导入 JSON</label>
        <textarea
          v-model="importText"
          rows="8"
          class="w-full rounded-2xl border border-border bg-background px-3 py-2 font-mono text-xs outline-none focus:border-primary"
          spellcheck="false"
        />
        <p v-if="error" class="text-sm text-[hsl(var(--tone-error-strong))]">{{ error }}</p>
      </div>
      <template #footer>
        <Button size="sm" variant="outline" :root-class="toolbarButtonClass" @click="showImport = false">
          取消
        </Button>
        <Button size="sm" variant="primary" :root-class="toolbarButtonClass" :disabled="loading" @click="doImport">
          {{ loading ? '导入中…' : '导入到号池' }}
        </Button>
      </template>
    </ModalShell>

    <ModalShell
      :open="!!detail || detailLoading"
      title="账号详情"
      :description="detail?.email || (detailLoading ? '加载中…' : '')"
      size-class="max-w-2xl"
      close-on-overlay
      @close="closeDetail"
    >
      <div v-if="detail" class="max-h-[70vh] space-y-3 overflow-y-auto pr-1">
        <div class="flex flex-wrap items-center gap-2">
          <TonePill :tone="statusTone(detail.status)" :label="statusLabel(detail.status)" />
          <TonePill :tone="dispatchTone(detail)" :label="dispatchLabel(detail)" />
          <TonePill
            :tone="detail.has_api_key || detail.api_key ? 'success' : 'error'"
            :label="detail.has_api_key || detail.api_key ? 'Key有效' : 'Key无效'"
          />
          <TonePill
            :tone="tokenTone(detail.has_access_token || !!detail.access_token)"
            :label="tokenLabel('AT', detail.has_access_token || !!detail.access_token)"
          />
          <TonePill
            :tone="tokenTone(detail.has_refresh_token || !!detail.refresh_token)"
            :label="tokenLabel('RT', detail.has_refresh_token || !!detail.refresh_token)"
          />
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
              <pre
                v-if="f.pre"
                class="whitespace-pre-wrap font-sans text-sm leading-relaxed"
              >{{ f.value }}</pre>
              <span v-else :class="f.mono ? 'font-mono text-xs' : ''">{{ f.value }}</span>
              <button
                v-if="f.copy && f.value && f.value !== '—'"
                type="button"
                class="ml-2 text-xs text-muted-foreground hover:text-foreground"
                @click="copy(f.value)"
              >复制</button>
            </dd>
          </div>
        </dl>
      </div>
      <p v-else-if="detailLoading" class="text-sm text-muted-foreground">正在加载账号详情…</p>
      <template #footer>
        <Button
          size="sm"
          variant="outline"
          :root-class="toolbarButtonClass"
          :disabled="!detail?.id || busyId === detail?.id || batchBusy"
          @click="ensureKeyOne(detail)"
        >
          {{ detail?.has_api_key || detail?.api_key ? '重置 Key' : '登录补 Key' }}
        </Button>
        <Button size="sm" variant="primary" :root-class="toolbarButtonClass" @click="closeDetail">关闭</Button>
      </template>
    </ModalShell>
  </div>
</template>

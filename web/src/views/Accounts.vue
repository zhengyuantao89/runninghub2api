<script setup>
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Button, Input, ModalShell } from 'nanocat-ui'
import FilterToolbar from '@/components/ai/FilterToolbar.vue'
import MetricStrip from '@/components/ai/MetricStrip.vue'
import PagePanel from '@/components/ai/PagePanel.vue'
import TableShell from '@/components/ai/TableShell.vue'
import TonePill from '@/components/TonePill.vue'
import { api, getRole } from '@/api'
import { consoleBaseFromPath, isAdminRole } from '@/lib/auth'
import { copyText } from '@/lib/copyText'
import { logClaimResponse, runLog } from '@/lib/runLog'

const route = useRoute()
const isAdmin = computed(() => isAdminRole(getRole()) || consoleBaseFromPath(route.path) === '/admin')
const ownerFilter = ref('')

const rows = ref([])
const error = ref('')
const okmsg = ref('')
const importText = ref('{"accounts":[{"email":"a@b.com","password":"plain","status":"active","source":"manual","max_concurrency":1}]}')
const loading = ref(false)
const batchBusy = ref(false)
const showImport = ref(false)
const showAdd = ref(false)
const keyword = ref('')
const page = ref(1)
const pageSize = ref(50)
const total = ref(0)
const pages = ref(0)
const pageSizeOptions = [50, 100, 200]
const ownerNames = ref([])
const busyId = ref(0)
let keywordTimer = 0
const actionMenuId = ref(0)
const actionMenuStyle = ref({})
const actionMenuRow = ref(null)
const selectedIds = ref(new Set())
const detail = ref(null)
const detailLoading = ref(false)
const detailMaxConcurrency = ref(1)
const savingConcurrency = ref(false)
const headerCheckEl = ref(null)
const toolbarButtonClass = 'h-8 justify-center px-3 text-xs'
const copiedKey = ref('')
let copiedTimer = 0

function placeActionMenu(anchorEl) {
  if (!anchorEl?.getBoundingClientRect) {
    actionMenuStyle.value = {}
    return
  }
  const rect = anchorEl.getBoundingClientRect()
  const menuWidth = 132
  const menuHeight = 152
  const gap = 4
  const left = Math.min(
    Math.max(8, rect.right - menuWidth),
    window.innerWidth - menuWidth - 8,
  )
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

function toggleActionMenu(id, event) {
  if (actionMenuId.value === id) {
    closeActionMenu()
    return
  }
  actionMenuId.value = id
  actionMenuRow.value = rows.value.find((row) => row.id === id) || null
  placeActionMenu(event?.currentTarget)
}

function closeActionMenu() {
  actionMenuId.value = 0
  actionMenuRow.value = null
  actionMenuStyle.value = {}
}

function onActionMenuReposition() {
  if (!actionMenuId.value) return
  closeActionMenu()
}

async function runRowAction(row, action) {
  closeActionMenu()
  if (action === 'detail') return openDetail(row)
  if (action === 'ensureKey') return ensureKeyOne(row)
  if (action === 'refresh') return refreshOne(row)
  if (action === 'claim') return claimOne(row)
  if (action === 'toggleStatus') return toggleStatus(row)
  if (action === 'delete') return deleteOne(row)
}

const addForm = reactive({
  email: '',
  password: '',
  access_token: '',
  api_key: '',
  remain_coins: '',
  max_concurrency: 1,
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
    { key: 'total', label: '号池总数', value: total.value },
    { key: 'active', label: '本页可调度', value: active },
    { key: 'busy', label: '本页占用', value: busy },
    { key: 'cool', label: '本页冷却', value: cooldown },
    { key: 'fail', label: '本页有失败', value: failed },
  ]
})

const ownerOptions = computed(() =>
  [...ownerNames.value].sort((a, b) => a.localeCompare(b, 'zh-CN')),
)

const pageRangeLabel = computed(() => {
  if (!total.value) return '共 0 条'
  const start = (page.value - 1) * pageSize.value + 1
  const end = Math.min(page.value * pageSize.value, total.value)
  return `第 ${start}-${end} 条 / 共 ${total.value} 条`
})

const filtered = computed(() => rows.value || [])

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
    { label: '归属用户', value: dash(a.owner || (a.user_id ? `#${a.user_id}` : '')) },
    { label: '邮箱', value: dash(a.email), copy: true },
    { label: '状态', value: dash(a.status) },
    { label: '可调度', value: a.dispatchable ? '是' : '否' },
    { label: '占用中', value: a.busy ? '是' : '否' },
    { label: '计费体系', value: dash(a.billing_version || 'OLD') },
    { label: 'RH 币', value: dash(a.remain_coins) },
    { label: '积分', value: dash(a.remain_points) },
    { label: '图片', value: freeQuotaLabel(a.free_image_remaining, a.free_image_total) },
    { label: '视频', value: freeQuotaLabel(a.free_video_remaining, a.free_video_total) },
    { label: '免费额度时间', value: dash(a.last_free_quota_at), mono: true },
    { label: '并发上限', value: dash(a.max_concurrency || 1) },
    { label: '本网关在途', value: `${a.inflight || 0} / ${a.max_concurrency || 1}` },
    { label: 'RH 在途', value: dash(a.current_task_counts || '0') },
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
    { label: '每日领取日', value: dash(a.last_login_coin_at), mono: true },
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
  addForm.max_concurrency = 1
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

async function refresh() {
  error.value = ''
  loading.value = true
  try {
    const res = await api.listAccounts({
      page: page.value,
      pageSize: pageSize.value,
      q: keyword.value.trim(),
      owner: ownerFilter.value.trim(),
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

function onPageSizeChange() {
  page.value = 1
  refresh()
}

function onOwnerChange() {
  page.value = 1
  refresh()
}

function onKeywordInput(v) {
  keyword.value = String(v ?? '')
  if (keywordTimer) clearTimeout(keywordTimer)
  keywordTimer = setTimeout(() => {
    page.value = 1
    refresh()
  }, 300)
}

function goPage(next) {
  const target = Math.max(1, Math.min(pages.value || 1, Number(next) || 1))
  if (target === page.value) return
  page.value = target
  refresh()
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
    const maxConc = Math.min(64, Math.max(1, Number(addForm.max_concurrency) || 1))
    await api.createAccount({
      email,
      password: String(addForm.password || '').trim(),
      access_token: String(addForm.access_token || '').trim(),
      api_key: String(addForm.api_key || '').trim(),
      remain_coins: String(addForm.remain_coins || '').trim(),
      max_concurrency: maxConc,
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

async function deleteOne(row) {
  if (!row?.id) return
  if (!window.confirm(`确定删除账号 ${row.email || row.id}？此操作会直接从数据库删除，不可恢复。`)) {
    return
  }
  error.value = ''
  busyId.value = row.id
  runLog.info(`正在删除 ${row.email || row.id}…`)
  try {
    await api.deleteAccount(row.id)
    if (detail.value?.id === row.id) closeDetail()
    const next = new Set(selectedIds.value)
    next.delete(row.id)
    selectedIds.value = next
    await refresh()
    runLog.success(`${row.email || row.id} 已删除`)
  } catch (e) {
    const msg = e.message || String(e)
    runLog.error(`${row.email || row.id} 删除失败：${msg}`)
    error.value = msg
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

async function claimOne(row) {
  if (!row?.id) return
  error.value = ''
  busyId.value = row.id
  runLog.info(`开始给 ${row.email} 领今日积分…`)
  try {
    const res = await api.claimDailyLogin([row.id], false)
    logClaimResponse(res, { emailHint: row.email })
    await refresh()
    if (detail.value?.id === row.id) {
      detail.value = await api.getAccount(row.id)
    }
  } catch (e) {
    const msg = e.message || String(e)
    runLog.error(`${row.email} 领取出错了：${msg}`)
    error.value = msg
  } finally {
    busyId.value = 0
  }
}

async function batchClaimDailyLogin() {
  const list = selectedRows()
  const ids = list.map((r) => r.id)
  const scope = ids.length ? `选中的 ${ids.length} 个账号` : '全部活跃账号'
  if (!window.confirm(`领取今日积分（${scope}；含因积分为 0 被禁用的号）？`)) {
    return
  }
  error.value = ''
  okmsg.value = ''
  batchBusy.value = true
  runLog.info(`开始批量领积分啦（${scope}）…`)
  try {
    const res = await api.claimDailyLogin(ids, false)
    logClaimResponse(res)
    await refresh()
    const total = Number(res.success || 0) + Number(res.skipped || 0) + Number(res.failed || 0)
    if (total === 0) {
      runLog.warning('没有可领的号（要 active，或者因没币被禁但仍有 AT/密码）')
    }
  } catch (e) {
    const msg = e.message || String(e)
    runLog.error(`批量领取整组翻车了：${msg}`)
    error.value = msg
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
  detailMaxConcurrency.value = Number(row.max_concurrency) || 1
  try {
    const data = await api.getAccount(row.id)
    if (seq !== detailRequestSeq) return
    detail.value = data
    detailMaxConcurrency.value = Number(data?.max_concurrency) || 1
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

async function saveDetailConcurrency() {
  if (!detail.value?.id) return
  error.value = ''
  okmsg.value = ''
  savingConcurrency.value = true
  try {
    const n = Math.min(64, Math.max(1, Number(detailMaxConcurrency.value) || 1))
    const acc = await api.setAccountMaxConcurrency(detail.value.id, n)
    detail.value = { ...detail.value, ...acc }
    detailMaxConcurrency.value = Number(acc.max_concurrency) || n
    okmsg.value = `${acc.email || detail.value.email} 并发上限已设为 ${detailMaxConcurrency.value}`
    await refresh()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    savingConcurrency.value = false
  }
}

async function copy(text, key = '') {
  const value = String(text ?? '').trim()
  if (!value || value === '—') {
    error.value = '没有可复制的内容'
    return
  }
  try {
    await copyText(value)
    error.value = ''
    if (key) {
      copiedKey.value = key
      window.clearTimeout(copiedTimer)
      copiedTimer = window.setTimeout(() => {
        if (copiedKey.value === key) copiedKey.value = ''
      }, 1500)
    } else {
      okmsg.value = '已复制'
      window.setTimeout(() => {
        if (okmsg.value === '已复制') okmsg.value = ''
      }, 1200)
    }
  } catch {
    error.value = '复制失败，请手动选中后复制'
  }
}

function freeQuotaLabel(remaining, total) {
  const t = Number(total || 0)
  if (!t) return '—'
  return `${Number(remaining || 0)}/${t}`
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
  return ok ? kind : `${kind}无`
}

function onHeaderCheck(e) {
  toggleSelectAll(e.target.checked)
}

watch([someFilteredSelected, headerCheckEl], () => {
  if (headerCheckEl.value) {
    headerCheckEl.value.indeterminate = someFilteredSelected.value
  }
})

function onDocClick(e) {
  if (!e.target?.closest?.('[data-account-action-menu]')) closeActionMenu()
}

onMounted(() => {
  document.addEventListener('click', onDocClick)
  window.addEventListener('resize', onActionMenuReposition)
  window.addEventListener('scroll', onActionMenuReposition, true)
  loadOwners()
  refresh()
})

onUnmounted(() => {
  if (keywordTimer) clearTimeout(keywordTimer)
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
            :placeholder="isAdmin ? '搜索邮箱 / ID / 状态 / 来源' : '搜索邮箱 / ID / 状态 / 来源'"
            block
            root-class="min-w-[14rem] flex-1 md:max-w-sm"
            @update:model-value="onKeywordInput"
          />
          <select
            v-if="isAdmin"
            v-model="ownerFilter"
            class="h-9 rounded-xl border border-border bg-background px-3 text-sm text-foreground"
            @change="onOwnerChange"
          >
            <option value="">全部用户</option>
            <option v-for="name in ownerOptions" :key="name" :value="name">{{ name }}</option>
          </select>
          <label class="flex items-center gap-2 text-xs text-muted-foreground">
            每页
            <select
              v-model.number="pageSize"
              class="h-9 rounded-xl border border-border bg-background px-2 text-sm text-foreground"
              @change="onPageSizeChange"
            >
              <option v-for="n in pageSizeOptions" :key="n" :value="n">{{ n }}</option>
            </select>
          </label>
          <span class="text-xs text-muted-foreground">{{ pageRangeLabel }}</span>
        </FilterToolbar>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="text-xs text-muted-foreground">
            共 {{ total }} 个账号 · 已选 {{ selectedCount }} · 并发可在详情按号设置（默认 1）
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
        <table class="w-full min-w-[1280px] text-left text-sm">
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
              <th v-if="isAdmin" class="py-3 pr-5">归属</th>
              <th class="py-3 px-3 text-center">Token</th>
              <th class="py-3 pr-5">状态</th>
              <th class="py-3 pr-5">可调度</th>
              <th class="py-3 pr-5">体系</th>
              <th class="py-3 pr-5">RH币</th>
              <th class="py-3 pr-5">积分</th>
              <th class="py-3 pr-5">图片</th>
              <th class="py-3 pr-5">视频</th>
              <th class="py-3 pr-5">领取日</th>
              <th class="py-3 pr-5">指纹</th>
              <th class="py-3 px-3 text-center">出口</th>
              <th class="py-3 px-3 text-center">并发</th>
              <th class="py-3 px-3 text-center">RH在途</th>
              <th class="py-3 px-3 text-center">Key</th>
              <th class="py-3 px-3 text-center">冷却</th>
              <th class="py-3 px-3 text-center">失败</th>
              <th class="py-3 pr-5">来源</th>
              <th class="py-3 pl-3 text-center">操作</th>
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
                  @click="copy(a.email, `email-${a.id}`)"
                >
                  {{ copiedKey === `email-${a.id}` ? '已复制' : a.email }}
                </button>
              </td>
              <td v-if="isAdmin" class="py-3 pr-5">
                <span v-if="a.owner" class="text-sm text-foreground">{{ a.owner }}</span>
                <span v-else class="text-muted-foreground">—</span>
              </td>
              <td class="px-3 py-3 text-center">
                <div class="inline-flex flex-wrap justify-center gap-1">
                  <TonePill :tone="tokenTone(a.has_access_token)" :label="tokenLabel('AT', a.has_access_token)" />
                  <TonePill :tone="tokenTone(a.has_refresh_token)" :label="tokenLabel('RT', a.has_refresh_token)" />
                </div>
              </td>
              <td class="py-3 pr-5">
                <TonePill :tone="statusTone(a.status)" :label="statusLabel(a.status)" />
              </td>
              <td class="py-3 pr-5">
                <TonePill :tone="dispatchTone(a)" :label="dispatchLabel(a)" />
              </td>
              <td class="py-3 pr-5 font-mono text-xs">{{ a.billing_version || 'OLD' }}</td>
              <td class="py-3 pr-5">{{ a.remain_coins || '—' }}</td>
              <td class="py-3 pr-5">{{ a.remain_points || '—' }}</td>
              <td class="py-3 pr-5 font-mono text-xs">{{ freeQuotaLabel(a.free_image_remaining, a.free_image_total) }}</td>
              <td class="py-3 pr-5 font-mono text-xs">{{ freeQuotaLabel(a.free_video_remaining, a.free_video_total) }}</td>
              <td class="py-3 pr-5 font-mono text-xs text-muted-foreground">{{ a.last_login_coin_at || '—' }}</td>
              <td class="py-3 pr-5">
                <TonePill
                  :tone="a.has_fingerprint ? 'success' : 'neutral'"
                  :label="a.has_fingerprint ? '有指纹' : '无'"
                />
              </td>
              <td class="px-3 py-3 text-center font-mono text-xs text-muted-foreground">{{ a.egress_label || '—' }}</td>
              <td class="px-3 py-3 text-center font-mono text-xs">
                {{ a.inflight || 0 }}/{{ a.max_concurrency || 1 }}
              </td>
              <td class="px-3 py-3 text-center">{{ a.current_task_counts || '0' }}</td>
              <td class="px-3 py-3 text-center">
                <div class="inline-flex justify-center">
                  <TonePill
                    :tone="a.has_api_key ? 'success' : 'error'"
                    :label="a.has_api_key ? (a.api_type || '有Key') : '无Key'"
                  />
                </div>
              </td>
              <td class="px-3 py-3 text-center font-mono text-xs text-muted-foreground">{{ a.cooldown_until || '—' }}</td>
              <td class="px-3 py-3 text-center">{{ a.fail_count }}</td>
              <td class="py-3 pr-5 text-muted-foreground">{{ a.source || '—' }}</td>
              <td class="px-3 py-3 text-center" data-account-action-menu>
                <Button
                  size="xs"
                  variant="outline"
                  :disabled="busyId === a.id || batchBusy"
                  @click.stop="toggleActionMenu(a.id, $event)"
                >
                  操作
                </Button>
              </td>
            </tr>
            <tr v-if="!loading && !filtered.length">
              <td colspan="21" class="py-8 text-center text-sm text-muted-foreground">
                暂无账号。可单独添加、批量导入，或通过推送接口同步。
              </td>
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

      <Teleport to="body">
        <div
          v-if="actionMenuId && actionMenuRow"
          data-account-action-menu
          class="overflow-hidden rounded-xl border border-border bg-card py-1 text-left shadow-lg"
          :style="actionMenuStyle"
          @click.stop
        >
          <button
            type="button"
            class="block w-full px-3 py-1.5 text-left text-xs hover:bg-muted/60"
            @click="runRowAction(actionMenuRow, 'detail')"
          >
            详情
          </button>
          <button
            type="button"
            class="block w-full px-3 py-1.5 text-left text-xs hover:bg-muted/60"
            :disabled="busyId === actionMenuRow.id || batchBusy"
            @click="runRowAction(actionMenuRow, 'ensureKey')"
          >
            {{ actionMenuRow.has_api_key ? '重置Key' : '补Key' }}
          </button>
          <button
            type="button"
            class="block w-full px-3 py-1.5 text-left text-xs hover:bg-muted/60"
            :disabled="busyId === actionMenuRow.id || batchBusy"
            @click="runRowAction(actionMenuRow, 'refresh')"
          >
            刷新
          </button>
          <button
            type="button"
            class="block w-full px-3 py-1.5 text-left text-xs hover:bg-muted/60"
            :disabled="busyId === actionMenuRow.id || batchBusy"
            @click="runRowAction(actionMenuRow, 'claim')"
          >
            领取积分
          </button>
          <button
            type="button"
            class="block w-full px-3 py-1.5 text-left text-xs hover:bg-muted/60"
            :disabled="busyId === actionMenuRow.id || batchBusy"
            @click="runRowAction(actionMenuRow, 'toggleStatus')"
          >
            {{ actionMenuRow.status === 'active' ? '禁用' : '启用' }}
          </button>
          <button
            type="button"
            class="block w-full px-3 py-1.5 text-left text-xs text-[hsl(var(--tone-error-strong))] hover:bg-muted/60"
            :disabled="busyId === actionMenuRow.id || batchBusy"
            @click="runRowAction(actionMenuRow, 'delete')"
          >
            删除
          </button>
        </div>
      </Teleport>
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
        <div class="grid grid-cols-3 gap-3">
          <label class="block space-y-1.5">
            <span class="text-xs font-medium text-muted-foreground">RH 币（可选）</span>
            <Input v-model="addForm.remain_coins" type="text" block placeholder="如 1100" />
          </label>
          <label class="block space-y-1.5">
            <span class="text-xs font-medium text-muted-foreground">并发上限</span>
            <Input v-model="addForm.max_concurrency" type="number" block placeholder="1" />
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
        <p class="text-xs text-muted-foreground">付费号可把并发上限设为平台允许值（如 5），免费号保持 1。</p>
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
        <div class="flex flex-wrap items-end gap-2 rounded-xl border border-border/70 bg-muted/20 px-3 py-2">
          <label class="min-w-[8rem] flex-1 space-y-1.5">
            <span class="text-[11px] uppercase tracking-[0.14em] text-muted-foreground">并发上限</span>
            <Input v-model="detailMaxConcurrency" type="number" block min="1" max="64" />
          </label>
          <Button
            size="sm"
            variant="outline"
            :root-class="toolbarButtonClass"
            :disabled="savingConcurrency || busyId === detail.id || batchBusy"
            @click="saveDetailConcurrency"
          >
            {{ savingConcurrency ? '保存中…' : '保存并发' }}
          </Button>
        </div>
        <p v-if="error" class="text-sm text-[hsl(var(--tone-error-strong))]">{{ error }}</p>
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
                @click="copy(f.value, f.label)"
              >{{ copiedKey === f.label ? '已复制' : '复制' }}</button>
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

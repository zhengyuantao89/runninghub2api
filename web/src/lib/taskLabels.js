const STATUS_ZH = {
  queued: '排队中',
  running: '运行中',
  succeeded: '已完成',
  failed: '失败',
  cancelled: '已取消',
}

const RH_STATUS_ZH = {
  QUEUED: '上游排队',
  RUNNING: '上游生成中',
  SUCCESS: '上游完成',
  FAILED: '上游失败',
  ERROR: '上游错误',
  CANCELLED: '上游已取消',
}

export function statusLabel(status) {
  if (!status) return '—'
  return STATUS_ZH[status] || status
}

export function statusTone(status) {
  switch (status) {
    case 'succeeded':
      return 'success'
    case 'running':
      return 'info'
    case 'queued':
      return 'warning'
    case 'failed':
      return 'error'
    case 'cancelled':
      return 'neutral'
    default:
      return 'neutral'
  }
}

export function progressLabel(t) {
  if (!t) return '—'
  if (t.progress_text) return t.progress_text
  if (t.queue_count) {
    const idx = t.queue_index ?? 0
    return `排队 ${idx}/${t.queue_count}`
  }
  if (t.rh_status) return RH_STATUS_ZH[t.rh_status] || t.rh_status
  switch (t.status) {
    case 'queued':
      return t.rh_task_id ? '已提交，等待上游排队' : '等待号池调度'
    case 'running':
      return '生成中'
    case 'succeeded':
      return '完成'
    case 'failed':
      return '失败'
    case 'cancelled':
      return '已取消'
    default:
      return '—'
  }
}

export function progressTone(t) {
  if (!t) return 'neutral'
  const rh = String(t.rh_status || '').toUpperCase()
  const text = String(t.progress_text || '')
  if (t.status === 'succeeded' || rh === 'SUCCESS' || text.includes('完成')) return 'success'
  if (t.status === 'failed' || /FAIL|ERROR/.test(rh) || text.includes('失败')) return 'error'
  if (t.status === 'cancelled' || rh.includes('CANCEL')) return 'neutral'
  if (rh === 'QUEUED' || text.includes('排队') || t.status === 'queued') return 'warning'
  if (rh === 'RUNNING' || text.includes('生成') || t.status === 'running') return 'info'
  return 'neutral'
}

export function rhStatusLabel(rh) {
  if (!rh) return ''
  return RH_STATUS_ZH[rh] || rh
}

/** Our gateway-level codes (store.error_code). */
const GATEWAY_ERROR_ZH = {
  RH: '上游错误',
  AUTH: '鉴权失败',
  BUSY: '上游繁忙',
  NO_COINS: '余额不足',
  PROMPT: '提示词/校验',
  TIMEOUT: '超时',
  EGRESS: '出口失败',
}

/** Official RunningHub OpenAPI code → identifier (doc-8287467). */
const RH_CODE_ID = {
  301: 'PARAMS_INVALID',
  380: 'WORKFLOW_NOT_EXISTS',
  412: 'TOKEN_INVALID',
  415: 'TASK_INSTANCE_MAXED',
  416: 'NOT_ENOUGH_WALLET',
  421: 'TASK_QUEUE_MAXED',
  423: 'TASK_NOT_FOUND',
  433: 'VALIDATE_PROMPT_FAILED',
  435: 'EXCLAPI_INSTANCE_NOT_FOUND',
  436: 'EXCLAPI_REQUIRED',
  500: 'UNKNOWN_ERROR',
  801: 'APIKEY_UNSUPPORTED_FREE_USER',
  802: 'APIKEY_UNAUTHORIZED',
  803: 'APIKEY_INVALID_NODE_INFO',
  804: 'APIKEY_TASK_IS_RUNNING',
  805: 'APIKEY_TASK_STATUS_ERROR',
  806: 'APIKEY_USER_NOT_FOUND',
  807: 'APIKEY_TASK_NOT_FOUND',
  808: 'APIKEY_UPLOAD_FAILED',
  809: 'APIKEY_FILE_SIZE_EXCEEDED',
  810: 'WORKFLOW_NOT_SAVED',
  811: 'CORPAPIKEY_INVALID',
  812: 'CORPAPIKEY_INSUFFICIENT_FUNDS',
  813: 'APIKEY_TASK_IS_QUEUED',
  901: 'WEBAPP_NOT_EXISTS',
  1000: 'UNKNOWN',
  1005: 'INTERNAL_ERROR',
  1011: 'SYSTEM_BUSY',
  1501: 'CONTENT_VERIFICATION_FAILED',
  1504: 'MODEL_TIMEOUT',
  1505: 'REAL_PERSON_PROHIBITED',
}

/**
 * Compact label for task queue "错误码" column.
 * Prefer official RH msg/code from error text; else gateway error_code.
 */
export function errorCodeLabel(t) {
  if (!t) return '—'
  const raw = String(t.error || '')
  const gate = String(t.error_code || '').trim().toUpperCase()

  // msg=UNKNOWN_ERROR / msg=TOKEN_INVALID
  const msgId = raw.match(/\bmsg=([A-Z][A-Z0-9_]{2,})\b/)
  if (msgId) return msgId[1]

  // code=433 → VALIDATE_PROMPT_FAILED (or raw code)
  const codeNum = raw.match(/\bcode=(\d{3,4})\b/)
  if (codeNum) {
    const n = Number(codeNum[1])
    return RH_CODE_ID[n] || String(n)
  }

  if (gate && gate !== 'RH') {
    return GATEWAY_ERROR_ZH[gate] ? `${gate}` : gate
  }
  if (gate === 'RH') return 'RH'
  return '—'
}

export function errorCodeTitle(t) {
  if (!t) return ''
  const parts = []
  const code = errorCodeLabel(t)
  if (code && code !== '—') parts.push(code)
  if (t.error_code) parts.push(`gateway=${t.error_code}`)
  if (t.error) parts.push(t.error)
  return parts.join('\n')
}

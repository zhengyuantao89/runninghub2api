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

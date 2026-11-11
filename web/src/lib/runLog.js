import { reactive } from 'vue'

const MAX_LINES = 120

const state = reactive({
  open: false,
  minimized: false,
  lines: [],
})

let seq = 1

function nowLabel() {
  const d = new Date()
  const pad = (n) => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function push(tone, text) {
  const msg = String(text || '').trim()
  if (!msg) return 0
  const id = seq++
  state.lines.push({
    id,
    tone: tone || 'info',
    text: msg,
    at: nowLabel(),
  })
  if (state.lines.length > MAX_LINES) {
    state.lines.splice(0, state.lines.length - MAX_LINES)
  }
  state.open = true
  state.minimized = false
  return id
}

export const runLogState = state

export const runLog = {
  open() {
    state.open = true
    state.minimized = false
  },
  close() {
    state.open = false
  },
  minimize() {
    state.minimized = true
  },
  restore() {
    state.minimized = false
    state.open = true
  },
  clear() {
    state.lines.splice(0, state.lines.length)
  },
  info(text) {
    return push('info', text)
  },
  success(text) {
    return push('success', text)
  },
  warning(text) {
    return push('warning', text)
  },
  error(text) {
    return push('error', text)
  },
}

/** 把领取接口返回翻成口语化日志行 */
export function logClaimResponse(res, { emailHint = '' } = {}) {
  if (!res) {
    runLog.error('领取接口没回东西，有点奇怪')
    return
  }
  const results = Array.isArray(res.results) ? res.results : []
  if (!results.length) {
    runLog.warning(
      emailHint
        ? `${emailHint} 这边没找到可领的号，可能状态不对`
        : '这批一个都没动到，可能没有可领的号',
    )
    return
  }

  for (const item of results) {
    const email = item.email || emailHint || `#${item.id || '?'}`
    if (item.skipped || item.already_granted) {
      const after = item.after_coins || ''
      const before = item.before_coins || ''
      const bal =
        after !== ''
          ? before !== '' && before !== after
            ? `，余额 ${before} → ${after}`
            : `，当前余额 ${after}`
          : ''
      if (item.error) {
        runLog.warning(`${email} 今天已经领过了，跳过；钱包没同步上：${item.error}${bal}`)
      } else {
        runLog.warning(`${email} 今天已经领过了，跳过${bal}`)
      }
      continue
    }
    if (item.ok) {
      const gained = Number(item.gained || 0)
      const gift = item.gift_amount ? `，礼包 ${item.gift_amount}` : ''
      if (gained > 0) {
        runLog.success(`${email} 领到了，+${gained}${gift}，稳`)
      } else {
        runLog.success(`${email} 领取成功了${gift || '，钱包也同步好了'}`)
      }
      continue
    }
    runLog.error(`${email} 没领成：${item.error || '未知错误'}`)
  }

  const ok = Number(res.success || 0)
  const skip = Number(res.skipped || 0)
  const fail = Number(res.failed || 0)
  const gained = Number(res.gained || 0)
  if (results.length > 1) {
    runLog.info(
      `这批搞完了（${res.day || '今日'}）：成功 ${ok}，跳过 ${skip}，失败 ${fail}，一共 +${gained}`,
    )
  }
}

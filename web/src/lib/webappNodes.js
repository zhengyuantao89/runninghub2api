export function nodeKey(n, i = 0) {
  return `${n?.nodeId || ''}::${n?.fieldName || ''}::${i}`
}

export function isImageNode(n) {
  const ft = String(n?.fieldType || '').toUpperCase()
  if (ft === 'IMAGE') return true
  const fn = String(n?.fieldName || '').toLowerCase()
  if (fn === 'image' || fn.includes('image')) return true
  const nn = String(n?.nodeName || '').toLowerCase()
  return nn.includes('loadimage') || nn.includes('load_image')
}

export function fieldTypeOf(n) {
  return String(n?.fieldType || '').toUpperCase().trim()
}

export function isUploadPlaceholder(v) {
  const s = String(v ?? '').trim().toLowerCase()
  return !s || s === '__upload__' || s === '__image__' || s === '@image' || s.startsWith('upload:')
}

export function defaultTextValue(v) {
  if (v == null) return ''
  if (typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean') {
    const s = String(v)
    return isUploadPlaceholder(s) ? '' : s
  }
  try {
    return JSON.stringify(v)
  } catch {
    return String(v)
  }
}

/** Parse RH fieldData (JSON string or already-parsed). */
export function parseFieldData(n) {
  const raw = n?.fieldData
  if (raw == null || raw === '') return null
  if (typeof raw === 'object') return raw
  const s = String(raw).trim()
  if (!s) return null
  try {
    return JSON.parse(s)
  } catch {
    return null
  }
}

function asOption(v, description = '') {
  if (v == null) return null
  if (typeof v === 'object') return null
  const s = String(v)
  return { value: s, label: s, description: description ? String(description) : '' }
}

/**
 * LIST / COMBO options from fieldData. Supports:
 * - [{name,index,description}, ...]  (RH apiCallDemo LIST)
 * - ["COMBO", { options:[...], default }]
 * - [["a","b","c"]]  nested string array
 * - ["a","b","c"]    flat primitives
 */
export function listOptions(n) {
  const data = parseFieldData(n)
  if (!Array.isArray(data) || !data.length) return []

  // ["COMBO", { options, default, tooltip }]
  if (String(data[0] || '').toUpperCase() === 'COMBO') {
    const meta = data[1] && typeof data[1] === 'object' && !Array.isArray(data[1]) ? data[1] : null
    const opts = Array.isArray(meta?.options) ? meta.options : []
    return opts.map((o) => asOption(o)).filter(Boolean)
  }

  // [["a","b","c"]] — single nested array of primitives
  if (data.length === 1 && Array.isArray(data[0])) {
    return data[0].map((o) => asOption(o)).filter(Boolean)
  }

  // [{name,index,description}, ...] RH object options (skip trailing {default})
  const looksLikeRhOpts = data.some(
    (item) =>
      item &&
      typeof item === 'object' &&
      !Array.isArray(item) &&
      (item.name != null || item.index != null),
  )
  if (looksLikeRhOpts) {
    const out = []
    for (const item of data) {
      if (!item || typeof item !== 'object' || Array.isArray(item)) continue
      const index = item.index != null ? String(item.index) : ''
      const name = item.name != null ? String(item.name) : ''
      if (!index && !name) continue
      out.push({
        value: index || name,
        label: name || index,
        description: item.description ? String(item.description) : '',
      })
    }
    return out
  }

  // flat ["a","b",8]
  if (data.every((x) => x == null || ['string', 'number', 'boolean'].includes(typeof x))) {
    return data.map((o) => asOption(o)).filter(Boolean)
  }

  return []
}

export function listDefault(n) {
  const data = parseFieldData(n)
  if (Array.isArray(data) && String(data[0] || '').toUpperCase() === 'COMBO') {
    const meta = data[1]
    if (meta && typeof meta === 'object' && meta.default != null) return String(meta.default)
  }
  if (Array.isArray(data)) {
    for (const item of data) {
      if (item && typeof item === 'object' && !Array.isArray(item) && item.default != null) {
        return String(item.default)
      }
    }
  }
  return defaultTextValue(n?.fieldValue)
}

function isComfyUnbounded(min, max) {
  return (
    typeof min === 'number' &&
    typeof max === 'number' &&
    !Number.isNaN(min) &&
    !Number.isNaN(max) &&
    min <= -99999 &&
    max >= 99999
  )
}

/** Comfy-style ["INT"|"FLOAT", { default, min, max, step }] */
export function numberMeta(n) {
  const data = parseFieldData(n)
  // COMBO masquerading as INT → not a free number field
  if (Array.isArray(data) && String(data[0] || '').toUpperCase() === 'COMBO') return null

  let meta = null
  let typeHint = fieldTypeOf(n)
  if (Array.isArray(data) && data.length >= 2 && data[1] && typeof data[1] === 'object' && !Array.isArray(data[1])) {
    meta = data[1]
    typeHint = String(data[0] || typeHint).toUpperCase()
  } else if (data && typeof data === 'object' && !Array.isArray(data)) {
    meta = data
  }
  if (!meta) {
    const ft = fieldTypeOf(n)
    if (ft !== 'INT' && ft !== 'FLOAT' && ft !== 'NUMBER') return null
    return {
      kind: ft === 'FLOAT' ? 'float' : 'int',
      min: undefined,
      max: undefined,
      step: ft === 'FLOAT' ? 'any' : 1,
      defaultValue: defaultTextValue(n?.fieldValue),
    }
  }
  const kind = typeHint === 'FLOAT' || fieldTypeOf(n) === 'FLOAT' ? 'float' : 'int'
  let min = meta.min != null && meta.min !== '' ? Number(meta.min) : undefined
  let max = meta.max != null && meta.max !== '' ? Number(meta.max) : undefined
  if (isComfyUnbounded(min, max)) {
    min = undefined
    max = undefined
  }
  return {
    kind,
    min,
    max,
    step: meta.step != null && meta.step !== '' ? Number(meta.step) : kind === 'float' ? 'any' : 1,
    defaultValue:
      meta.default != null && meta.default !== ''
        ? String(meta.default)
        : defaultTextValue(n?.fieldValue),
  }
}

export function isListNode(n) {
  if (isImageNode(n)) return false
  // Prefer actual options (covers INT+COMBO like bit_depth)
  if (listOptions(n).length > 0) return true
  return fieldTypeOf(n) === 'LIST'
}

export function isNumberNode(n) {
  if (isImageNode(n) || isListNode(n)) return false
  const ft = fieldTypeOf(n)
  if (ft === 'INT' || ft === 'FLOAT' || ft === 'NUMBER') return true
  const data = parseFieldData(n)
  if (Array.isArray(data) && data.length >= 1) {
    const t = String(data[0] || '').toUpperCase()
    return t === 'INT' || t === 'FLOAT'
  }
  return false
}

export function isMultiline(n) {
  if (isListNode(n) || isNumberNode(n)) return false
  const data = parseFieldData(n)
  if (Array.isArray(data) && data[1] && typeof data[1] === 'object' && data[1].multiline) {
    return true
  }
  const nn = String(n?.nodeName || '').toLowerCase()
  const fn = String(n?.fieldName || '').toLowerCase()
  const desc = String(n?.description || '').toLowerCase()
  if (nn.includes('multiline') || nn.includes('stringmultiline') || nn.includes('prompt')) return true
  if (fn.includes('prompt') || fn.includes('text') || desc.includes('prompt')) return true
  const v = defaultTextValue(n?.fieldValue)
  return v.length > 80 || v.includes('\n')
}

export function fieldLabel(n) {
  return (
    n?.description ||
    n?.nodeName ||
    n?.fieldName ||
    `节点 ${n?.nodeId || ''}`
  )
}

/** Initial form value preferring LIST/INT defaults from fieldData. */
export function initialFieldValue(n) {
  if (isListNode(n)) {
    const opts = listOptions(n)
    const cur = defaultTextValue(n?.fieldValue)
    if (cur && opts.some((o) => o.value === cur)) return cur
    const d = listDefault(n)
    if (d && opts.some((o) => o.value === d)) return d
    return opts[0]?.value || cur || ''
  }
  if (isNumberNode(n)) {
    const meta = numberMeta(n)
    return meta?.defaultValue || defaultTextValue(n?.fieldValue)
  }
  return defaultTextValue(n?.fieldValue)
}

export function optionsSummary(n) {
  if (isListNode(n)) {
    const opts = listOptions(n)
    if (!opts.length) return ''
    return opts.map((o) => o.label).join(' · ')
  }
  if (isNumberNode(n)) {
    const m = numberMeta(n)
    if (!m) return ''
    const parts = []
    if (m.min != null && !Number.isNaN(m.min)) parts.push(`min ${m.min}`)
    if (m.max != null && !Number.isNaN(m.max)) parts.push(`max ${m.max}`)
    if (m.step != null && m.step !== 'any' && m.step !== 1) parts.push(`step ${m.step}`)
    return parts.join(' · ')
  }
  return ''
}

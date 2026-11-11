import { computed, reactive, ref } from 'vue'
import { api } from '@/api'
import { progressLabel as formatProgress } from '@/lib/taskLabels'
import {
  defaultTextValue,
  initialFieldValue,
  isImageNode,
  isListNode,
  isMultiline,
  isNumberNode,
  listOptions,
  nodeKey,
  numberMeta,
  optionsSummary,
} from '@/lib/webappNodes'

const STORAGE_KEY = 'rh2api.studioDraft.v10'

export const QC_MODE_LABELS = {
  MULTIMODAL_REFERENCE: '多模态参考',
  SUBJECT_REFERENCE: '主体参考',
  FREE_CREATION: '自由创作',
  FIRST_LAST_FRAME: '首尾帧',
}

export function qcModeLabel(key) {
  return QC_MODE_LABELS[key] || key || '—'
}

/**
 * Official free image model families (HAR).
 * Same family: with refs → i2i sku, without → t2i sku.
 */
export const FREE_IMAGE_MODELS = [
  {
    id: 'nano_banana2',
    // Official picker label from Runow UI / models payload.
    label: 'Nano Banana2-Low-Cost',
    hint: '有参考图走图生，无图走文生',
    i2i: { binding_id: '2043951705349238786', sku_id: '2027196343409463297' },
    t2i: { binding_id: '2043954826653806594', sku_id: '2027192837726294017' },
  },
  {
    id: 'gpt_image_2',
    label: 'GPT Image2.0-Low-Cost',
    hint: '有参考图走图生，无图走文生',
    i2i: { binding_id: '2046586338670891009', sku_id: '2046503667076751361' },
    t2i: { binding_id: '2046581921695485954', sku_id: '2046514150500524033' },
  },
]

/** Official free video model + modes (HAR groupName Minimax H3-RH Enhanced). */
export const FREE_VIDEO_MODELS = [
  {
    id: 'minimax_h3_rh_enhanced',
    label: 'Minimax H3-RH Enhanced',
  },
]

/** Official free video creation modes under Minimax H3-RH Enhanced. */
export const FREE_VIDEO_CREATION_MODES = [
  {
    id: 'multimodal',
    label: '多模态参考',
    hint: '混合图片作为创作参考（最多 9 张）',
    maxRefs: 9,
    needsRefs: true,
  },
  {
    id: 'freeCreation',
    label: '自由创作',
    hint: '文生视频或图生视频，上传参考图即走图生',
    maxRefs: 1,
    needsRefs: false,
  },
]

/** Official QC freeCreation duration range (HAR: min 4 max 15 step 1). */
export const FREE_DURATION_OPTIONS = Array.from({ length: 12 }, (_, i) => String(i + 4))

/** Official QC video aspectRatio LIST labels. */
export const FREE_VIDEO_ASPECT_OPTIONS = [
  '1:1 (Square)',
  '2:3 (Portrait Photo)',
  '3:2 (Photo)',
  '3:4 (Portrait Standard)',
  '4:3 (Standard)',
  '9:16 (Portrait Widescreen)',
  '16:9 (Widescreen)',
  '21:9 (Ultrawide)',
]

/** Official QC video resolution LIST (HAR: 480p / 768p / 1080p). */
export const FREE_VIDEO_RESOLUTION_OPTIONS = ['480p', '768p', '1080p']

/** Official QC image aspect ratios (HAR image fee-preview / prepare). */
export const FREE_IMAGE_ASPECT_OPTIONS = ['1:1', '3:2', '2:3', '3:4', '4:3', '9:16', '16:9']

/** Official QC image resolution LIST (HAR: 1k / 2k / 4k). */
export const FREE_IMAGE_RESOLUTION_OPTIONS = ['1k', '2k', '4k']

/** @deprecated use FREE_VIDEO_* */
export const FREE_ASPECT_OPTIONS = FREE_VIDEO_ASPECT_OPTIONS
/** @deprecated use FREE_VIDEO_* */
export const FREE_RESOLUTION_OPTIONS = FREE_VIDEO_RESOLUTION_OPTIONS

const templates = ref([])
const templatesLoading = ref(false)
const selectedTemplateId = ref('')
const template = ref(null)
/** @type {import('vue').Ref<'free'|'qc'|'app'>} */
const channel = ref('free')
/** @type {import('vue').Ref<'video'|'image'>} */
const freeKind = ref('video')
/** @type {import('vue').Ref<'nano_banana2'|'gpt_image_2'>} */
const freeImageModel = ref('nano_banana2')
/** @type {import('vue').Ref<'multimodal'|'freeCreation'>} */
const freeVideoMode = ref('freeCreation')
const freePrompt = ref('人物动起来')
const freeDuration = ref('4')
const freeAspect = ref('3:2 (Photo)')
const freeResolution = ref('480p')
const FREE_IMAGE_KEY = 'first_frame'
/** Official free i2i maxUploadCount=10 (HAR models/binding). */
export const FREE_IMAGE_MAX_REFS = 10
export const FREE_IMAGE_REF_KEYS = Array.from({ length: FREE_IMAGE_MAX_REFS }, (_, i) => `ref_${i + 1}`)
export const FREE_VIDEO_MULTI_MAX_REFS = 9
export const FREE_VIDEO_MULTI_REF_KEYS = FREE_IMAGE_REF_KEYS.slice(0, FREE_VIDEO_MULTI_MAX_REFS)

/** Runow multi-model (submit_mode=qc) */
const qcCategory = ref('VIDEO')
const qcMode = ref('FREE_CREATION')
const qcBindingId = ref('')
const qcAllowPaid = ref(true)
const qcCatalog = ref(null)
const qcCatalogLoading = ref(false)
const qcCatalogError = ref('')

const fieldValues = reactive({})
/** @type {import('vue').Ref<Record<string, File>>} */
const imageFiles = ref({})
/** @type {import('vue').Ref<Record<string, string>>} */
const imagePreviews = ref({})
const dragOverKey = ref('')

const task = ref(null)
const error = ref('')
const submitting = ref(false)
const cancelling = ref(false)

let pollTimer = null
let hydrated = false
let selectSeq = 0

const nodes = computed(() => template.value?.node_info_list || [])
const imageNodes = computed(() => nodes.value.filter(isImageNode))
const textNodes = computed(() =>
  nodes.value.map((n, i) => ({ n, i })).filter(({ n }) => !isImageNode(n)),
)

const qcModes = computed(() => {
  const list = Array.isArray(qcCatalog.value?.modes) ? qcCatalog.value.modes : []
  return [...list].sort((a, b) => (a.sort || 0) - (b.sort || 0))
})

const qcModels = computed(() => {
  const list = Array.isArray(qcCatalog.value?.models) ? qcCatalog.value.models : []
  const mode = String(qcMode.value || '').trim()
  if (!mode) return list
  return list.filter((m) => Array.isArray(m.modes) && m.modes.includes(mode))
})

const qcSelectedModel = computed(() => {
  const id = String(qcBindingId.value || '').trim()
  if (!id) return null
  return qcModels.value.find((m) => m.binding_id === id) || null
})

const qcBillingBadge = computed(() => {
  const m = qcSelectedModel.value
  if (!m) return ''
  if (m.billing_hint === 'free_seen') return '抓包见免费'
  if (m.billing_hint === 'paid_seen') return '抓包见积分'
  if (m.billing_hint === 'mixed_seen') return '免费/积分均可'
  return '计费以 fee-preview 为准'
})

const canSubmit = computed(() => {
  if (submitting.value) return false
  if (channel.value === 'free') {
    const promptOK = String(freePrompt.value || '').trim().length >= 2
    if (freeKind.value === 'video' && freeVideoMode.value === 'multimodal') {
      return promptOK && freeVideoRefCount.value > 0
    }
    return promptOK
  }
  if (channel.value === 'qc') {
    const promptOK = String(freePrompt.value || '').trim().length >= 2
    const modelOK = !!qcSelectedModel.value?.binding_id && !!qcSelectedModel.value?.sku_id
    return promptOK && modelOK
  }
  if (!template.value) return false
  if (!nodes.value.length) return false
  for (const n of imageNodes.value) {
    const id = String(n.nodeId || '').trim()
    if (!id || !imageFiles.value[id]) return false
  }
  return true
})

const freeAspectOptions = computed(() =>
  freeKind.value === 'image' ? FREE_IMAGE_ASPECT_OPTIONS : FREE_VIDEO_ASPECT_OPTIONS,
)
const freeResolutionOptions = computed(() =>
  freeKind.value === 'image' ? FREE_IMAGE_RESOLUTION_OPTIONS : FREE_VIDEO_RESOLUTION_OPTIONS,
)
const qcAspectOptions = computed(() =>
  qcCategory.value === 'IMAGE' ? FREE_IMAGE_ASPECT_OPTIONS : FREE_VIDEO_ASPECT_OPTIONS,
)
const qcResolutionOptions = computed(() =>
  qcCategory.value === 'IMAGE' ? FREE_IMAGE_RESOLUTION_OPTIONS : FREE_VIDEO_RESOLUTION_OPTIONS,
)

const isActive = computed(() => task.value && ['queued', 'running'].includes(task.value.status))

const progressText = computed(() => {
  const t = task.value
  if (!t) return ''
  const text = formatProgress(t)
  return text === '—' ? '' : text
})

const freeImageRefCount = computed(() =>
  FREE_IMAGE_REF_KEYS.reduce((n, k) => n + (imageFiles.value[k] ? 1 : 0), 0),
)

/** Slots to render: filled refs in order + one empty add slot (until max 10). */
const freeImageRefSlots = computed(() => {
  const filled = FREE_IMAGE_REF_KEYS.filter((k) => !!imageFiles.value[k])
  if (filled.length >= FREE_IMAGE_MAX_REFS) return filled
  const nextEmpty = FREE_IMAGE_REF_KEYS.find((k) => !imageFiles.value[k])
  return nextEmpty ? [...filled, nextEmpty] : filled
})

const freeVideoModeMeta = computed(
  () => FREE_VIDEO_CREATION_MODES.find((m) => m.id === freeVideoMode.value) || FREE_VIDEO_CREATION_MODES[1],
)

const freeVideoRefCount = computed(() => {
  if (freeVideoMode.value === 'multimodal') {
    return FREE_VIDEO_MULTI_REF_KEYS.reduce((n, k) => n + (imageFiles.value[k] ? 1 : 0), 0)
  }
  return imageFiles.value[FREE_IMAGE_KEY] ? 1 : 0
})

const freeVideoRefSlots = computed(() => {
  if (freeVideoMode.value !== 'multimodal') return []
  const filled = FREE_VIDEO_MULTI_REF_KEYS.filter((k) => !!imageFiles.value[k])
  if (filled.length >= FREE_VIDEO_MULTI_MAX_REFS) return filled
  const nextEmpty = FREE_VIDEO_MULTI_REF_KEYS.find((k) => !imageFiles.value[k])
  return nextEmpty ? [...filled, nextEmpty] : filled
})

const primaryPreview = computed(() => {
  if (channel.value === 'free' || channel.value === 'qc') {
    if (imagePreviews.value[FREE_IMAGE_KEY]) {
      return imagePreviews.value[FREE_IMAGE_KEY]
    }
    for (const k of FREE_IMAGE_REF_KEYS) {
      if (imagePreviews.value[k]) return imagePreviews.value[k]
    }
  }
  for (const n of imageNodes.value) {
    const id = String(n.nodeId || '').trim()
    if (id && imagePreviews.value[id]) return imagePreviews.value[id]
  }
  return Object.values(imagePreviews.value)[0] || ''
})

function applyFreeKindDefaults(kind) {
  if (kind === 'image') {
    if (!FREE_IMAGE_ASPECT_OPTIONS.includes(freeAspect.value)) freeAspect.value = '9:16'
    if (!FREE_IMAGE_RESOLUTION_OPTIONS.includes(freeResolution.value)) freeResolution.value = '1k'
    if (!String(freePrompt.value || '').trim() || freePrompt.value === '人物动起来') {
      freePrompt.value = '一只在窗边睡觉的猫，柔和光线'
    }
  } else {
    if (!FREE_VIDEO_ASPECT_OPTIONS.includes(freeAspect.value)) freeAspect.value = '3:2 (Photo)'
    if (!FREE_VIDEO_RESOLUTION_OPTIONS.includes(freeResolution.value)) freeResolution.value = '480p'
    if (!String(freePrompt.value || '').trim() || freePrompt.value.includes('睡觉的猫')) {
      freePrompt.value = '人物动起来'
    }
  }
}

function setFreeKind(next) {
  const v = next === 'image' ? 'image' : 'video'
  if (freeKind.value === v) return
  freeKind.value = v
  clearImages()
  applyFreeKindDefaults(v)
  if (v === 'image' && !FREE_IMAGE_MODELS.some((m) => m.id === freeImageModel.value)) {
    freeImageModel.value = 'nano_banana2'
  }
  error.value = ''
  persist()
}

function setFreeImageModel(next) {
  const v = FREE_IMAGE_MODELS.some((m) => m.id === next) ? next : 'nano_banana2'
  if (freeImageModel.value === v) return
  freeImageModel.value = v
  error.value = ''
  persist()
}

function setFreeVideoMode(next) {
  const v = FREE_VIDEO_CREATION_MODES.some((m) => m.id === next) ? next : 'freeCreation'
  if (freeVideoMode.value === v) return
  freeVideoMode.value = v
  clearImages()
  error.value = ''
  persist()
}

const freeImageModelMeta = computed(
  () => FREE_IMAGE_MODELS.find((m) => m.id === freeImageModel.value) || FREE_IMAGE_MODELS[0],
)

function resetFieldsFromTemplate(t) {
  for (const k of Object.keys(fieldValues)) delete fieldValues[k]
  const list = t?.node_info_list || []
  list.forEach((n, i) => {
    if (isImageNode(n)) return
    fieldValues[nodeKey(n, i)] = initialFieldValue(n)
  })
}

function clearImages() {
  for (const url of Object.values(imagePreviews.value)) {
    if (url) URL.revokeObjectURL(url)
  }
  imageFiles.value = {}
  imagePreviews.value = {}
  dragOverKey.value = ''
}

function persist() {
  try {
    sessionStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        channel: channel.value,
        freeKind: freeKind.value,
        freeImageModel: freeImageModel.value,
        freeVideoMode: freeVideoMode.value,
        freePrompt: freePrompt.value,
        freeDuration: freeDuration.value,
        freeAspect: freeAspect.value,
        freeResolution: freeResolution.value,
        qcCategory: qcCategory.value,
        qcMode: qcMode.value,
        qcBindingId: qcBindingId.value,
        qcAllowPaid: !!qcAllowPaid.value,
        selectedTemplateId: selectedTemplateId.value,
        fieldValues: { ...fieldValues },
        taskId: task.value?.id || '',
      }),
    )
  } catch {
    /* ignore */
  }
}

async function loadQCCatalog({ refresh = false } = {}) {
  qcCatalogLoading.value = true
  qcCatalogError.value = ''
  try {
    const cat = String(qcCategory.value || 'VIDEO').toUpperCase()
    const res = await api.getQCCatalog(cat, { refresh })
    qcCatalog.value = res
    const modes = Array.isArray(res?.modes) ? res.modes : []
    if (modes.length && !modes.some((m) => m.mode_key === qcMode.value)) {
      const sorted = [...modes].sort((a, b) => (a.sort || 0) - (b.sort || 0))
      qcMode.value = sorted[0]?.mode_key || 'FREE_CREATION'
    }
    const models = Array.isArray(res?.models) ? res.models : []
    const filtered = models.filter(
      (m) => !qcMode.value || (Array.isArray(m.modes) && m.modes.includes(qcMode.value)),
    )
    if (!filtered.some((m) => m.binding_id === qcBindingId.value)) {
      qcBindingId.value = filtered[0]?.binding_id || ''
    }
  } catch (e) {
    qcCatalogError.value = e.message || String(e)
  } finally {
    qcCatalogLoading.value = false
  }
}

function setQCCategory(next) {
  const v = String(next || 'VIDEO').toUpperCase() === 'IMAGE' ? 'IMAGE' : 'VIDEO'
  if (qcCategory.value === v) return
  qcCategory.value = v
  qcBindingId.value = ''
  if (v === 'IMAGE') {
    if (!FREE_IMAGE_ASPECT_OPTIONS.includes(freeAspect.value)) freeAspect.value = '9:16'
    if (!FREE_IMAGE_RESOLUTION_OPTIONS.includes(freeResolution.value)) freeResolution.value = '1k'
  } else {
    if (!FREE_VIDEO_ASPECT_OPTIONS.includes(freeAspect.value)) freeAspect.value = '3:2 (Photo)'
    if (!FREE_VIDEO_RESOLUTION_OPTIONS.includes(freeResolution.value)) freeResolution.value = '480p'
  }
  clearImages()
  persist()
  loadQCCatalog()
}

function setQCMode(next) {
  const v = String(next || '').trim()
  if (!v || qcMode.value === v) return
  qcMode.value = v
  const filtered = qcModels.value
  if (!filtered.some((m) => m.binding_id === qcBindingId.value)) {
    qcBindingId.value = filtered[0]?.binding_id || ''
  }
  persist()
}

function setQCBinding(next) {
  qcBindingId.value = String(next || '').trim()
  persist()
}

function loadDraft() {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    return JSON.parse(raw)
  } catch {
    return null
  }
}

function stopPoll() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function startPoll(id) {
  if (!id) return
  stopPoll()
  pollTimer = setInterval(async () => {
    try {
      const t = await api.getVideo(id)
      task.value = t
      persist()
      if (['succeeded', 'failed', 'cancelled'].includes(t.status)) stopPoll()
    } catch (e) {
      error.value = e.message || String(e)
      stopPoll()
    }
  }, 2500)
}

async function refreshTemplates() {
  templatesLoading.value = true
  try {
    const res = await api.listTemplates()
    templates.value = Array.isArray(res?.data) ? res.data : []
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    templatesLoading.value = false
  }
}

function clearTaskState() {
  stopPoll()
  task.value = null
}

async function selectTemplate(id, { keepFields = false, keepTask = false } = {}) {
  error.value = ''
  const tid = String(id || '').trim()
  const prevId = selectedTemplateId.value
  const switching = !!prevId && prevId !== tid
  selectedTemplateId.value = tid
  const seq = ++selectSeq

  if (!keepTask && (switching || !tid)) {
    clearTaskState()
  }

  if (!tid) {
    template.value = null
    clearImages()
    for (const k of Object.keys(fieldValues)) delete fieldValues[k]
    persist()
    return
  }
  try {
    const t = await api.getTemplate(tid)
    if (seq !== selectSeq) return
    template.value = t
    if (!keepFields) {
      clearImages()
      resetFieldsFromTemplate(t)
    }
    persist()
  } catch (e) {
    if (seq !== selectSeq) return
    template.value = null
    error.value = e.message || String(e)
  }
}

function setImage(nodeId, file) {
  const id = String(nodeId || '').trim()
  if (!id) return
  if (!file || !String(file.type || '').startsWith('image/')) {
    error.value = '请选择图片文件'
    return
  }
  error.value = ''
  const prev = imagePreviews.value[id]
  if (prev) URL.revokeObjectURL(prev)
  // Normalize clipboard blobs (often unnamed) into a usable File.
  let next = file
  if (!(file instanceof File) || !file.name) {
    const ext = String(file.type || 'image/png').split('/')[1] || 'png'
    next = new File([file], `paste_${Date.now()}.${ext}`, { type: file.type || 'image/png' })
  }
  imageFiles.value = { ...imageFiles.value, [id]: next }
  imagePreviews.value = { ...imagePreviews.value, [id]: URL.createObjectURL(next) }
}

/** Add pasted/dropped images into the current free/qc image slots. Returns count added. */
function addPastedImages(fileList) {
  const files = [...(fileList || [])].filter((f) => f && String(f.type || '').startsWith('image/'))
  if (!files.length) return 0

  // Free video multimodal: fill up to 9 refs; freeCreation: single first-frame.
  if (channel.value === 'free' && freeKind.value === 'video') {
    if (freeVideoMode.value === 'multimodal') {
      let added = 0
      for (const file of files) {
        const empty = FREE_VIDEO_MULTI_REF_KEYS.find((k) => !imageFiles.value[k])
        if (!empty) break
        setImage(empty, file)
        added += 1
      }
      if (added === 0) error.value = `参考图已满（最多 ${FREE_VIDEO_MULTI_MAX_REFS} 张）`
      return added
    }
    setImage(FREE_IMAGE_KEY, files[0])
    return 1
  }

  // Free image / Runow QC: fill empty ref slots in order (max 10).
  let added = 0
  for (const file of files) {
    const empty = FREE_IMAGE_REF_KEYS.find((k) => !imageFiles.value[k])
    if (!empty) break
    setImage(empty, file)
    added += 1
  }
  if (added === 0) {
    error.value = `参考图已满（最多 ${FREE_IMAGE_MAX_REFS} 张）`
  }
  return added
}

function clearImage(nodeId) {
  const id = String(nodeId || '').trim()
  if (!id) return
  const prev = imagePreviews.value[id]
  if (prev) URL.revokeObjectURL(prev)
  const nextFiles = { ...imageFiles.value }
  const nextPreviews = { ...imagePreviews.value }
  delete nextFiles[id]
  delete nextPreviews[id]
  imageFiles.value = nextFiles
  imagePreviews.value = nextPreviews
}

async function hydrate(preferredTemplateId = '') {
  if (hydrated) {
    if (preferredTemplateId && preferredTemplateId !== selectedTemplateId.value) {
      await selectTemplate(preferredTemplateId)
    }
    return
  }
  hydrated = true
  await refreshTemplates()
  const draft = loadDraft()
  if (draft?.channel === 'app' || draft?.channel === 'free' || draft?.channel === 'qc') {
    channel.value = draft.channel
  }
  if (draft?.freeKind === 'image' || draft?.freeKind === 'video') {
    freeKind.value = draft.freeKind
  }
  if (FREE_IMAGE_MODELS.some((m) => m.id === draft?.freeImageModel)) {
    freeImageModel.value = draft.freeImageModel
  } else if (draft?.freeImageModel === 'i2i' || draft?.freeImageModel === 't2i') {
    // migrate old mistaken split
    freeImageModel.value = 'nano_banana2'
  }
  if (FREE_VIDEO_CREATION_MODES.some((m) => m.id === draft?.freeVideoMode)) {
    freeVideoMode.value = draft.freeVideoMode
  }
  applyFreeKindDefaults(freeKind.value)
  if (typeof draft?.freePrompt === 'string') freePrompt.value = draft.freePrompt
  if (typeof draft?.freeDuration === 'string' && FREE_DURATION_OPTIONS.includes(draft.freeDuration)) {
    freeDuration.value = draft.freeDuration
  }
  if (draft?.qcCategory === 'IMAGE' || draft?.qcCategory === 'VIDEO') {
    qcCategory.value = draft.qcCategory
  }
  if (typeof draft?.qcMode === 'string' && draft.qcMode) qcMode.value = draft.qcMode
  if (typeof draft?.qcBindingId === 'string') qcBindingId.value = draft.qcBindingId
  if (typeof draft?.qcAllowPaid === 'boolean') qcAllowPaid.value = draft.qcAllowPaid
  const aspectOpts =
    channel.value === 'qc'
      ? qcCategory.value === 'IMAGE'
        ? FREE_IMAGE_ASPECT_OPTIONS
        : FREE_VIDEO_ASPECT_OPTIONS
      : freeKind.value === 'image'
        ? FREE_IMAGE_ASPECT_OPTIONS
        : FREE_VIDEO_ASPECT_OPTIONS
  const resOpts =
    channel.value === 'qc'
      ? qcCategory.value === 'IMAGE'
        ? FREE_IMAGE_RESOLUTION_OPTIONS
        : FREE_VIDEO_RESOLUTION_OPTIONS
      : freeKind.value === 'image'
        ? FREE_IMAGE_RESOLUTION_OPTIONS
        : FREE_VIDEO_RESOLUTION_OPTIONS
  if (typeof draft?.freeAspect === 'string' && aspectOpts.includes(draft.freeAspect)) {
    freeAspect.value = draft.freeAspect
  }
  if (typeof draft?.freeResolution === 'string' && resOpts.includes(draft.freeResolution)) {
    freeResolution.value = draft.freeResolution
  }
  const preferred = String(preferredTemplateId || '').trim()
  if (preferred) channel.value = 'app'
  if (channel.value === 'qc') {
    await loadQCCatalog()
  }
  const draftTpl = String(draft?.selectedTemplateId || '').trim()
  const want = preferred || draftTpl
  const draftMatches = !!want && !!draftTpl && want === draftTpl

  if (want) {
    const exists = templates.value.some((t) => t.id === want)
    if (exists) {
      await selectTemplate(want, { keepTask: true })
      if (draftMatches && draft?.fieldValues && typeof draft.fieldValues === 'object') {
        for (const [k, v] of Object.entries(draft.fieldValues)) {
          fieldValues[k] = v == null ? '' : String(v)
        }
        persist()
      }
    }
  }

  // 只恢复「同一模板」草稿里的任务，避免深链模板 B 却挂上模板 A 的结果
  if (!draftMatches || !draft?.taskId) return
  try {
    const t = await api.getVideo(draft.taskId)
    task.value = t
    if (['queued', 'running'].includes(t.status)) startPoll(t.id)
    persist()
  } catch {
    /* gone */
  }
}

async function submit() {
  error.value = ''
  if (!canSubmit.value) {
    if (channel.value === 'free') {
      if (freeKind.value === 'image') {
        error.value = '官方免费生图需要提示词（参考图可选）'
      } else if (freeVideoMode.value === 'multimodal') {
        error.value = '多模态参考需要提示词 + 至少一张参考图'
      } else {
        error.value = '自由创作需要提示词（参考图可选）'
      }
    } else if (channel.value === 'qc') {
      error.value = !qcSelectedModel.value
        ? '请选择创作模式与模型'
        : '请填写提示词'
    } else {
      error.value = !template.value
        ? '请先选择已保存的应用模板'
        : imageNodes.value.length
          ? '请为所有图片节点上传图片'
          : '当前模板无可提交节点'
    }
    return
  }
  submitting.value = true
  try {
    const fd = new FormData()
    if (channel.value === 'free') {
      if (freeKind.value === 'image') {
        const model = freeImageModelMeta.value
        const refs = FREE_IMAGE_REF_KEYS.map((k) => imageFiles.value[k]).filter(Boolean)
        const path = refs.length ? model?.i2i : model?.t2i
        fd.append('submit_mode', 'qc_free_image')
        fd.append('free_image_model', model?.id || 'nano_banana2')
        fd.append('creation_mode', model?.id || 'nano_banana2')
        if (path?.binding_id) fd.append('binding_id', path.binding_id)
        if (path?.sku_id) fd.append('sku_id', path.sku_id)
        fd.append('prompt', String(freePrompt.value || '').trim())
        fd.append('aspect_ratio', String(freeAspect.value || '9:16').trim() || '9:16')
        fd.append('resolution', String(freeResolution.value || '1k').trim() || '1k')
        refs.forEach((file, i) => {
          if (i === 0) fd.append('image', file)
          else fd.append(`image_${i + 1}`, file)
        })
      } else {
        const mode = freeVideoMode.value === 'multimodal' ? 'multimodal' : 'freeCreation'
        fd.append('submit_mode', 'qc_free')
        fd.append('creation_mode', mode)
        fd.append('free_video_mode', mode)
        fd.append('prompt', String(freePrompt.value || '').trim())
        fd.append('duration', String(freeDuration.value || '4').trim() || '4')
        fd.append('aspect_ratio', String(freeAspect.value || '3:2 (Photo)').trim() || '3:2 (Photo)')
        fd.append('resolution', String(freeResolution.value || '480p').trim() || '480p')
        if (mode === 'multimodal') {
          const refs = FREE_VIDEO_MULTI_REF_KEYS.map((k) => imageFiles.value[k]).filter(Boolean)
          refs.forEach((file, i) => {
            if (i === 0) fd.append('image', file)
            else fd.append(`image_${i + 1}`, file)
          })
        } else {
          const file = imageFiles.value[FREE_IMAGE_KEY]
          if (file) fd.append('image', file)
        }
      }
    } else if (channel.value === 'qc') {
      const model = qcSelectedModel.value
      const isImage = qcCategory.value === 'IMAGE'
      fd.append('submit_mode', 'qc')
      fd.append('binding_id', model.binding_id)
      fd.append('sku_id', model.sku_id)
      fd.append('category_id', qcCategory.value)
      fd.append('creation_mode', String(qcMode.value || '').trim())
      fd.append('allow_paid', qcAllowPaid.value ? '1' : '0')
      fd.append('prompt', String(freePrompt.value || '').trim())
      if (!isImage) {
        fd.append('duration', String(freeDuration.value || '4').trim() || '4')
      }
      fd.append(
        'aspect_ratio',
        String(freeAspect.value || (isImage ? '9:16' : '3:2 (Photo)')).trim(),
      )
      fd.append(
        'resolution',
        String(freeResolution.value || (isImage ? '1k' : '480p')).trim(),
      )
      const refs = FREE_IMAGE_REF_KEYS.map((k) => imageFiles.value[k]).filter(Boolean)
      refs.forEach((file, i) => {
        if (i === 0) fd.append('image', file)
        else fd.append(`image_${i + 1}`, file)
      })
    } else {
      const list = nodes.value.map((n, i) => {
        const item = {
          nodeId: String(n.nodeId || ''),
          fieldName: String(n.fieldName || ''),
          fieldType: n.fieldType || '',
          nodeName: n.nodeName || '',
          description: n.description || '',
        }
        if (isImageNode(n)) {
          item.fieldValue = '__upload__'
        } else {
          const key = nodeKey(n, i)
          const raw = fieldValues[key]
          item.fieldValue = raw == null || raw === '' ? defaultTextValue(n.fieldValue) : raw
        }
        return item
      })
      fd.append('submit_mode', 'webapp')
      fd.append('webapp_id', template.value.webapp_id)
      if (template.value.id) fd.append('template_id', template.value.id)
      fd.append('node_info_list', JSON.stringify(list))
      for (const [nodeId, file] of Object.entries(imageFiles.value)) {
        if (file) fd.append(`image_${nodeId}`, file)
      }
    }
    const created = await api.createVideoForm(fd)
    task.value = created
    persist()
    startPoll(created.id)
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    submitting.value = false
  }
}

function setChannel(next) {
  const v = next === 'app' ? 'app' : next === 'qc' ? 'qc' : 'free'
  if (channel.value === v) return
  channel.value = v
  clearImages()
  error.value = ''
  persist()
  if (v === 'qc') {
    if (qcCategory.value === 'IMAGE') {
      if (!FREE_IMAGE_ASPECT_OPTIONS.includes(freeAspect.value)) freeAspect.value = '9:16'
      if (!FREE_IMAGE_RESOLUTION_OPTIONS.includes(freeResolution.value)) freeResolution.value = '1k'
    } else {
      if (!FREE_VIDEO_ASPECT_OPTIONS.includes(freeAspect.value)) freeAspect.value = '3:2 (Photo)'
      if (!FREE_VIDEO_RESOLUTION_OPTIONS.includes(freeResolution.value)) freeResolution.value = '480p'
    }
    loadQCCatalog()
  }
}

async function cancelTask() {
  if (!task.value?.id || cancelling.value) return
  cancelling.value = true
  try {
    await api.cancelVideo(task.value.id)
    const t = await api.getVideo(task.value.id)
    task.value = t
    persist()
    if (['succeeded', 'failed', 'cancelled'].includes(t.status)) stopPoll()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    cancelling.value = false
  }
}

function clearResult() {
  stopPoll()
  task.value = null
  error.value = ''
  persist()
}

export function useStudioStore() {
  return {
    templates,
    templatesLoading,
    selectedTemplateId,
    template,
    channel,
    freeKind,
    freeImageModel,
    freeImageModelMeta,
    FREE_IMAGE_MODELS,
    setFreeImageModel,
    freeVideoMode,
    freeVideoModeMeta,
    FREE_VIDEO_CREATION_MODES,
    FREE_VIDEO_MULTI_MAX_REFS,
    FREE_VIDEO_MULTI_REF_KEYS,
    freeVideoRefCount,
    freeVideoRefSlots,
    setFreeVideoMode,
    freePrompt,
    freeDuration,
    freeAspect,
    freeResolution,
    freeAspectOptions,
    freeResolutionOptions,
    qcAspectOptions,
    qcResolutionOptions,
    FREE_DURATION_OPTIONS,
    FREE_VIDEO_MODELS,
    FREE_ASPECT_OPTIONS,
    FREE_RESOLUTION_OPTIONS,
    FREE_VIDEO_ASPECT_OPTIONS,
    FREE_VIDEO_RESOLUTION_OPTIONS,
    FREE_IMAGE_ASPECT_OPTIONS,
    FREE_IMAGE_RESOLUTION_OPTIONS,
    FREE_IMAGE_KEY,
    FREE_IMAGE_MAX_REFS,
    FREE_IMAGE_REF_KEYS,
    freeImageRefCount,
    freeImageRefSlots,
    setFreeKind,
    qcCategory,
    qcMode,
    qcBindingId,
    qcAllowPaid,
    qcCatalog,
    qcCatalogLoading,
    qcCatalogError,
    qcModes,
    qcModels,
    qcSelectedModel,
    qcBillingBadge,
    loadQCCatalog,
    setQCCategory,
    setQCMode,
    setQCBinding,
    qcModeLabel,
    nodes,
    imageNodes,
    textNodes,
    fieldValues,
    imageFiles,
    imagePreviews,
    dragOverKey,
    task,
    error,
    submitting,
    cancelling,
    canSubmit,
    isActive,
    progressText,
    primaryPreview,
    nodeKey,
    isImageNode,
    isListNode,
    isNumberNode,
    isMultiline,
    listOptions,
    numberMeta,
    optionsSummary,
    refreshTemplates,
    selectTemplate,
    setChannel,
    setImage,
    addPastedImages,
    clearImage,
    submit,
    cancelTask,
    clearResult,
    persist,
    hydrate,
  }
}

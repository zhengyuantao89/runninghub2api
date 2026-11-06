import { computed, reactive, ref } from 'vue'
import { api } from '@/api'
import { progressLabel as formatProgress } from '@/lib/taskLabels'
import {
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

const STORAGE_KEY = 'rh2api.studioDraft.v3'

const templates = ref([])
const templatesLoading = ref(false)
const selectedTemplateId = ref('')
const template = ref(null)

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

const canSubmit = computed(() => {
  if (submitting.value || !template.value) return false
  if (!nodes.value.length) return false
  for (const n of imageNodes.value) {
    const id = String(n.nodeId || '').trim()
    if (!id || !imageFiles.value[id]) return false
  }
  return true
})

const isActive = computed(() => task.value && ['queued', 'running'].includes(task.value.status))

const progressText = computed(() => {
  const t = task.value
  if (!t) return ''
  const text = formatProgress(t)
  return text === '—' ? '' : text
})

const primaryPreview = computed(() => {
  for (const n of imageNodes.value) {
    const id = String(n.nodeId || '').trim()
    if (id && imagePreviews.value[id]) return imagePreviews.value[id]
  }
  return Object.values(imagePreviews.value)[0] || ''
})

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
        selectedTemplateId: selectedTemplateId.value,
        fieldValues: { ...fieldValues },
        taskId: task.value?.id || '',
      }),
    )
  } catch {
    /* ignore */
  }
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
  imageFiles.value = { ...imageFiles.value, [id]: file }
  imagePreviews.value = { ...imagePreviews.value, [id]: URL.createObjectURL(file) }
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
  const preferred = String(preferredTemplateId || '').trim()
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
  if (!template.value) {
    error.value = '请先选择已保存的应用模板'
    return
  }
  if (!canSubmit.value) {
    error.value = imageNodes.value.length ? '请为所有图片节点上传图片' : '当前模板无可提交节点'
    return
  }
  submitting.value = true
  try {
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
    const fd = new FormData()
    fd.append('webapp_id', template.value.webapp_id)
    fd.append('node_info_list', JSON.stringify(list))
    for (const [nodeId, file] of Object.entries(imageFiles.value)) {
      if (file) fd.append(`image_${nodeId}`, file)
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
    setImage,
    clearImage,
    submit,
    cancelTask,
    clearResult,
    persist,
    hydrate,
  }
}

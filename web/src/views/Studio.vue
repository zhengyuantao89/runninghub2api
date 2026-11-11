<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Button } from 'nanocat-ui'
import PagePanel from '@/components/ai/PagePanel.vue'
import TonePill from '@/components/TonePill.vue'
import { defaultTextValue, fieldLabel } from '@/lib/webappNodes'
import { statusLabel, statusTone } from '@/lib/taskLabels'
import { useStudioStore } from '@/stores/studio'

const route = useRoute()
const router = useRouter()

const {
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
  freePrompt,
  freeDuration,
  freeAspect,
  freeResolution,
  freeAspectOptions,
  freeResolutionOptions,
  FREE_DURATION_OPTIONS,
  FREE_VIDEO_MODELS,
  FREE_VIDEO_CREATION_MODES,
  FREE_VIDEO_MULTI_MAX_REFS,
  freeVideoMode,
  freeVideoModeMeta,
  freeVideoRefCount,
  freeVideoRefSlots,
  setFreeVideoMode,
  FREE_IMAGE_KEY,
  FREE_IMAGE_MAX_REFS,
  freeImageRefCount,
  freeImageRefSlots,
  qcCategory,
  qcMode,
  qcBindingId,
  qcAllowPaid,
  qcCatalogLoading,
  qcCatalogError,
  qcModes,
  qcModels,
  qcSelectedModel,
  qcBillingBadge,
  qcAspectOptions,
  qcResolutionOptions,
  loadQCCatalog,
  setQCCategory,
  setQCMode,
  setQCBinding,
  qcModeLabel,
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
  isListNode,
  isNumberNode,
  isMultiline,
  listOptions,
  numberMeta,
  optionsSummary,
  refreshTemplates,
  selectTemplate,
  setChannel,
  setFreeKind,
  setImage,
  addPastedImages,
  clearImage,
  submit,
  cancelTask,
  clearResult,
  persist,
  hydrate,
} = useStudioStore()

const fileInputs = ref({})

function onFile(nodeId, e) {
  setImage(nodeId, e.target.files?.[0] || null)
}

function onDrop(nodeId, e) {
  dragOverKey.value = ''
  setImage(nodeId, e.dataTransfer?.files?.[0] || null)
}

function onClearFile(nodeId) {
  clearImage(nodeId)
  const el = fileInputs.value[nodeId]
  if (el) el.value = ''
}

function collectClipboardImages(clipboardData) {
  if (!clipboardData) return []
  const out = []
  const seen = new Set()
  const push = (f) => {
    if (!f || !String(f.type || '').startsWith('image/')) return
    const key = `${f.type}:${f.size}:${f.name || ''}`
    if (seen.has(key)) return
    seen.add(key)
    out.push(f)
  }
  for (const item of clipboardData.items || []) {
    if (item?.kind === 'file' && String(item.type || '').startsWith('image/')) {
      push(item.getAsFile())
    }
  }
  for (const f of clipboardData.files || []) push(f)
  return out
}

function onPromptPaste(e) {
  const images = collectClipboardImages(e.clipboardData)
  if (!images.length) return
  // Keep text paste when mixed; only block default if we consumed images for refs.
  const added = addPastedImages(images)
  if (added > 0) {
    e.preventDefault()
  }
}

function copy(text) {
  if (!text) return
  navigator.clipboard?.writeText(text)
}

function mediaKindFromURL(url) {
  const raw = String(url || '').trim()
  if (!raw) return ''
  let path = raw
  try {
    path = new URL(raw, window.location.origin).pathname
  } catch {
    path = raw.split('?')[0].split('#')[0]
  }
  const ext = (path.match(/\.([a-z0-9]+)$/i) || [])[1]?.toLowerCase() || ''
  if (['png', 'jpg', 'jpeg', 'webp', 'gif', 'bmp', 'avif'].includes(ext)) return 'image'
  if (['mp4', 'webm', 'mov', 'm4v', 'mkv'].includes(ext)) return 'video'
  return ''
}

const resultMediaKind = computed(() => {
  const t = task.value
  if (!t?.file_url) return ''
  const fromURL = mediaKindFromURL(t.file_url)
  if (fromURL) return fromURL
  const wid = String(t.webapp_id || '')
  if (wid.startsWith('qcimg:')) return 'image'
  if (wid.startsWith('qc:')) return 'video'
  if (channel.value === 'free' && freeKind.value === 'image') return 'image'
  if (channel.value === 'qc' && qcCategory.value === 'IMAGE') return 'image'
  return 'video'
})

const resultIsImage = computed(() => resultMediaKind.value === 'image')

function defaultPlaceholder(n) {
  const s = defaultTextValue(n?.fieldValue)
  return s ? `默认：${s}` : ''
}

async function onPickTemplate(e) {
  const id = e.target.value
  await selectTemplate(id)
  const q = { ...route.query }
  if (id) q.template = id
  else delete q.template
  router.replace({ query: q })
}

function onChannel(next) {
  setChannel(next)
  if (next === 'free' || next === 'qc') {
    const q = { ...route.query }
    delete q.template
    router.replace({ query: q })
  }
}

watch(fieldValues, () => persist(), { deep: true })
  watch(
  [freePrompt, freeDuration, freeAspect, freeResolution, freeKind, freeImageModel, freeVideoMode, channel, qcCategory, qcMode, qcBindingId, qcAllowPaid],
  () => persist(),
)

onMounted(async () => {
  const fromQuery = String(route.query.template || '').trim()
  await hydrate(fromQuery)
})
</script>

<template>
  <div class="studio-page">
    <div class="studio-grid">
      <PagePanel class="studio-form">
        <header class="studio-form__head">
          <div class="studio-form__head-copy">
            <h2 class="studio-form__title">试跑参数</h2>
            <p class="studio-form__desc">
              官方免费快捷通道、Runow 全模型（免费/积分），或已保存的应用模板。
            </p>
          </div>
          <Button
            v-if="channel === 'app'"
            size="sm"
            variant="outline"
            root-class="studio-refresh-btn"
            :disabled="templatesLoading"
            @click="refreshTemplates"
          >
            {{ templatesLoading ? '刷新中…' : '刷新列表' }}
          </Button>
          <Button
            v-else-if="channel === 'qc'"
            size="sm"
            variant="outline"
            root-class="studio-refresh-btn"
            :disabled="qcCatalogLoading"
            @click="loadQCCatalog({ refresh: true })"
          >
            {{ qcCatalogLoading ? '刷新中…' : '刷新模型' }}
          </Button>
        </header>

        <div class="studio-field">
          <label class="studio-label">通道</label>
          <div class="studio-channel">
            <button
              type="button"
              class="studio-channel__btn"
              :class="{ 'is-active': channel === 'free' }"
              @click="onChannel('free')"
            >
              官方免费试用
            </button>
            <button
              type="button"
              class="studio-channel__btn"
              :class="{ 'is-active': channel === 'qc' }"
              @click="onChannel('qc')"
            >
              Runow 模型
            </button>
            <button
              type="button"
              class="studio-channel__btn"
              :class="{ 'is-active': channel === 'app' }"
              @click="onChannel('app')"
            >
              应用模板
            </button>
          </div>
        </div>

        <template v-if="channel === 'qc'">
          <div class="studio-field">
            <label class="studio-label">品类</label>
            <div class="studio-channel">
              <button
                type="button"
                class="studio-channel__btn"
                :class="{ 'is-active': qcCategory === 'VIDEO' }"
                @click="setQCCategory('VIDEO')"
              >
                视频
              </button>
              <button
                type="button"
                class="studio-channel__btn"
                :class="{ 'is-active': qcCategory === 'IMAGE' }"
                @click="setQCCategory('IMAGE')"
              >
                图片
              </button>
            </div>
          </div>

          <div class="studio-field">
            <label class="studio-label" for="qc-mode">创作模式</label>
            <select
              id="qc-mode"
              class="studio-select"
              :value="qcMode"
              :disabled="qcCatalogLoading || !qcModes.length"
              @change="setQCMode($event.target.value)"
            >
              <option v-for="m in qcModes" :key="m.mode_key" :value="m.mode_key">
                {{ qcModeLabel(m.mode_key) }}（{{ m.binding_count || 0 }}）
              </option>
            </select>
            <p v-if="qcCatalogError" class="studio-error">{{ qcCatalogError }}</p>
            <p v-else-if="qcCatalogLoading" class="studio-sub">加载模型目录…</p>
          </div>

          <div class="studio-field">
            <label class="studio-label" for="qc-model">模型</label>
            <select
              id="qc-model"
              class="studio-select"
              :value="qcBindingId"
              :disabled="qcCatalogLoading || !qcModels.length"
              @change="setQCBinding($event.target.value)"
            >
              <option value="">选择模型…</option>
              <option v-for="m in qcModels" :key="m.binding_id" :value="m.binding_id">
                {{ m.name || m.binding_id }}{{ m.billing_hint === 'free_seen' ? ' · 免费' : m.billing_hint === 'paid_seen' ? ' · 积分' : '' }}
              </option>
            </select>
            <p v-if="qcSelectedModel" class="studio-meta">
              <span class="mono">{{ qcSelectedModel.sku_id }}</span>
              <TonePill tone="neutral" :label="qcBillingBadge" />
            </p>
          </div>

          <div class="studio-field">
            <div class="studio-field__label-row">
              <label class="studio-label">参考图（可选）</label>
              <span class="studio-hint">{{ freeImageRefCount }}/{{ FREE_IMAGE_MAX_REFS }}</span>
            </div>
            <p class="studio-sub">参考图会按模型参数注入（首帧 / 多图）</p>
            <div class="studio-thumbs">
              <div
                v-for="(key, idx) in freeImageRefSlots"
                :key="key"
                class="studio-thumb"
                :class="{ 'is-over': dragOverKey === key, 'has-file': !!imagePreviews[key] }"
                :title="imageFiles[key]?.name || '添加参考图'"
                @dragenter.prevent="dragOverKey = key"
                @dragover.prevent="dragOverKey = key"
                @dragleave.prevent="dragOverKey = ''"
                @drop.prevent="onDrop(key, $event)"
                @click="fileInputs[key]?.click()"
              >
                <input
                  :ref="(el) => { if (el) fileInputs[key] = el }"
                  type="file"
                  accept="image/*"
                  class="sr-only"
                  @change="onFile(key, $event)"
                >
                <template v-if="imagePreviews[key]">
                  <img :src="imagePreviews[key]" alt="" class="studio-thumb__img">
                  <span class="studio-thumb__idx">{{ idx + 1 }}</span>
                  <button
                    type="button"
                    class="studio-thumb__remove"
                    aria-label="移除"
                    @click.stop="onClearFile(key)"
                  >
                    ×
                  </button>
                </template>
                <template v-else>
                  <span class="studio-thumb__plus">+</span>
                </template>
              </div>
            </div>
          </div>

          <div class="studio-field">
            <div class="studio-field__label-row">
              <label class="studio-label" for="qc-prompt">提示词</label>
              <span class="studio-hint">可粘贴图片作参考图</span>
            </div>
            <textarea
              id="qc-prompt"
              v-model="freePrompt"
              rows="4"
              class="studio-textarea"
              placeholder="描述你想生成的内容"
              @paste="onPromptPaste"
            />
          </div>

          <div class="studio-field studio-field--row">
            <div v-if="qcCategory === 'VIDEO'" class="studio-field">
              <label class="studio-label" for="qc-duration">时长（秒）</label>
              <select id="qc-duration" v-model="freeDuration" class="studio-select studio-field-control">
                <option v-for="d in FREE_DURATION_OPTIONS" :key="d" :value="d">{{ d }} 秒</option>
              </select>
            </div>
            <div class="studio-field">
              <label class="studio-label" for="qc-aspect">画幅</label>
              <select id="qc-aspect" v-model="freeAspect" class="studio-select studio-field-control">
                <option v-for="a in qcAspectOptions" :key="a" :value="a">{{ a }}</option>
              </select>
            </div>
            <div class="studio-field">
              <label class="studio-label" for="qc-resolution">分辨率</label>
              <select id="qc-resolution" v-model="freeResolution" class="studio-select studio-field-control">
                <option v-for="r in qcResolutionOptions" :key="r" :value="r">{{ r }}</option>
              </select>
            </div>
          </div>

          <div class="studio-field">
            <label class="studio-check">
              <input v-model="qcAllowPaid" type="checkbox">
              允许消耗账号积分（fee-preview 非免费时）
            </label>
          </div>
        </template>

        <template v-else-if="channel === 'free'">
          <div class="studio-field">
            <label class="studio-label">免费类型</label>
            <div class="studio-channel">
              <button
                type="button"
                class="studio-channel__btn"
                :class="{ 'is-active': freeKind === 'video' }"
                @click="setFreeKind('video')"
              >
                免费视频
              </button>
              <button
                type="button"
                class="studio-channel__btn"
                :class="{ 'is-active': freeKind === 'image' }"
                @click="setFreeKind('image')"
              >
                免费生图
              </button>
            </div>
          </div>

          <template v-if="freeKind === 'image'">
            <div class="studio-field">
              <label class="studio-label">免费模型</label>
              <div class="studio-channel">
                <button
                  v-for="m in FREE_IMAGE_MODELS"
                  :key="m.id"
                  type="button"
                  class="studio-channel__btn"
                  :class="{ 'is-active': freeImageModel === m.id }"
                  @click="setFreeImageModel(m.id)"
                >
                  {{ m.label }}
                </button>
              </div>
              <p class="studio-sub">{{ freeImageModelMeta?.hint }}</p>
            </div>
          </template>

          <template v-if="freeKind === 'video'">
            <div class="studio-field">
              <label class="studio-label">免费模型</label>
              <div class="studio-channel">
                <button
                  v-for="m in FREE_VIDEO_MODELS"
                  :key="m.id"
                  type="button"
                  class="studio-channel__btn is-active"
                  disabled
                >
                  {{ m.label }}
                </button>
              </div>
            </div>
            <div class="studio-field">
              <label class="studio-label">创作模式</label>
              <div class="studio-channel">
                <button
                  v-for="m in FREE_VIDEO_CREATION_MODES"
                  :key="m.id"
                  type="button"
                  class="studio-channel__btn"
                  :class="{ 'is-active': freeVideoMode === m.id }"
                  @click="setFreeVideoMode(m.id)"
                >
                  {{ m.label }}
                </button>
              </div>
              <p class="studio-sub">{{ freeVideoModeMeta?.hint }}</p>
            </div>
            <div v-if="freeVideoMode === 'multimodal'" class="studio-field">
              <div class="studio-field__label-row">
                <label class="studio-label">参考图</label>
                <span class="studio-hint">{{ freeVideoRefCount }}/{{ FREE_VIDEO_MULTI_MAX_REFS }}</span>
              </div>
              <div class="studio-thumbs">
                <div
                  v-for="(key, idx) in freeVideoRefSlots"
                  :key="key"
                  class="studio-thumb"
                  :class="{ 'is-over': dragOverKey === key, 'has-file': !!imagePreviews[key] }"
                  :title="imageFiles[key]?.name || '添加参考图'"
                  @dragenter.prevent="dragOverKey = key"
                  @dragover.prevent="dragOverKey = key"
                  @dragleave.prevent="dragOverKey = ''"
                  @drop.prevent="onDrop(key, $event)"
                  @click="fileInputs[key]?.click()"
                >
                  <input
                    :ref="(el) => { if (el) fileInputs[key] = el }"
                    type="file"
                    accept="image/*"
                    class="sr-only"
                    @change="onFile(key, $event)"
                  >
                  <template v-if="imagePreviews[key]">
                    <img :src="imagePreviews[key]" alt="" class="studio-thumb__img">
                    <span class="studio-thumb__idx">{{ idx + 1 }}</span>
                    <button
                      type="button"
                      class="studio-thumb__remove"
                      aria-label="移除"
                      @click.stop="onClearFile(key)"
                    >
                      ×
                    </button>
                  </template>
                  <template v-else>
                    <span class="studio-thumb__plus">+</span>
                  </template>
                </div>
              </div>
            </div>
            <div v-else class="studio-field">
              <div class="studio-field__label-row">
                <label class="studio-label">参考图（可选）</label>
                <span class="studio-hint">{{ imagePreviews[FREE_IMAGE_KEY] ? '图生' : '文生' }}</span>
              </div>
              <div class="studio-thumbs">
                <div
                  class="studio-thumb"
                  :class="{ 'is-over': dragOverKey === FREE_IMAGE_KEY, 'has-file': !!imagePreviews[FREE_IMAGE_KEY] }"
                  :title="imageFiles[FREE_IMAGE_KEY]?.name || '上传参考图'"
                  @dragenter.prevent="dragOverKey = FREE_IMAGE_KEY"
                  @dragover.prevent="dragOverKey = FREE_IMAGE_KEY"
                  @dragleave.prevent="dragOverKey = ''"
                  @drop.prevent="onDrop(FREE_IMAGE_KEY, $event)"
                  @click="fileInputs[FREE_IMAGE_KEY]?.click()"
                >
                  <input
                    :ref="(el) => { if (el) fileInputs[FREE_IMAGE_KEY] = el }"
                    type="file"
                    accept="image/*"
                    class="sr-only"
                    @change="onFile(FREE_IMAGE_KEY, $event)"
                  >
                  <template v-if="imagePreviews[FREE_IMAGE_KEY]">
                    <img :src="imagePreviews[FREE_IMAGE_KEY]" alt="" class="studio-thumb__img">
                    <button
                      type="button"
                      class="studio-thumb__remove"
                      aria-label="移除"
                      @click.stop="onClearFile(FREE_IMAGE_KEY)"
                    >
                      ×
                    </button>
                  </template>
                  <template v-else>
                    <span class="studio-thumb__plus">+</span>
                  </template>
                </div>
              </div>
            </div>
          </template>

          <template v-else>
            <div class="studio-field">
              <div class="studio-field__label-row">
                <label class="studio-label">参考图（可选）</label>
                <span class="studio-hint">
                  {{ freeImageRefCount > 0 ? '图生' : '文生' }} · {{ freeImageRefCount }}/{{ FREE_IMAGE_MAX_REFS }}
                </span>
              </div>
              <div class="studio-thumbs">
                <div
                  v-for="(key, idx) in freeImageRefSlots"
                  :key="key"
                  class="studio-thumb"
                  :class="{ 'is-over': dragOverKey === key, 'has-file': !!imagePreviews[key] }"
                  :title="imageFiles[key]?.name || '添加参考图'"
                  @dragenter.prevent="dragOverKey = key"
                  @dragover.prevent="dragOverKey = key"
                  @dragleave.prevent="dragOverKey = ''"
                  @drop.prevent="onDrop(key, $event)"
                  @click="fileInputs[key]?.click()"
                >
                  <input
                    :ref="(el) => { if (el) fileInputs[key] = el }"
                    type="file"
                    accept="image/*"
                    class="sr-only"
                    @change="onFile(key, $event)"
                  >
                  <template v-if="imagePreviews[key]">
                    <img :src="imagePreviews[key]" alt="" class="studio-thumb__img">
                    <span class="studio-thumb__idx">{{ idx + 1 }}</span>
                    <button
                      type="button"
                      class="studio-thumb__remove"
                      aria-label="移除"
                      @click.stop="onClearFile(key)"
                    >
                      ×
                    </button>
                  </template>
                  <template v-else>
                    <span class="studio-thumb__plus">+</span>
                  </template>
                </div>
              </div>
            </div>
          </template>

          <div class="studio-field">
            <div class="studio-field__label-row">
              <label class="studio-label" for="free-prompt">提示词</label>
              <span class="studio-hint">可粘贴图片作参考图</span>
            </div>
            <textarea
              id="free-prompt"
              v-model="freePrompt"
              rows="4"
              class="studio-textarea"
              :placeholder="freeKind === 'image' ? '例如：一只在窗边睡觉的猫' : '例如：人物动起来'"
              @paste="onPromptPaste"
            />
          </div>

          <div class="studio-field studio-field--row">
            <div v-if="freeKind === 'video'" class="studio-field">
              <label class="studio-label" for="free-duration">时长（秒）</label>
              <select id="free-duration" v-model="freeDuration" class="studio-select studio-field-control">
                <option v-for="d in FREE_DURATION_OPTIONS" :key="d" :value="d">{{ d }} 秒</option>
              </select>
            </div>
            <div class="studio-field">
              <label class="studio-label" for="free-aspect">画幅</label>
              <select id="free-aspect" v-model="freeAspect" class="studio-select studio-field-control">
                <option v-for="a in freeAspectOptions" :key="a" :value="a">{{ a }}</option>
              </select>
            </div>
            <div class="studio-field">
              <label class="studio-label" for="free-resolution">分辨率</label>
              <select id="free-resolution" v-model="freeResolution" class="studio-select studio-field-control">
                <option v-for="r in freeResolutionOptions" :key="r" :value="r">{{ r }}</option>
              </select>
            </div>
          </div>
        </template>

        <template v-else>
        <div class="studio-field">
          <label class="studio-label" for="studio-template">已保存模板</label>
          <select
            id="studio-template"
            class="studio-select"
            :value="selectedTemplateId"
            :disabled="templatesLoading"
            @change="onPickTemplate"
          >
            <option value="">选择模板…</option>
            <option v-for="t in templates" :key="t.id" :value="t.id">
              {{ t.name || t.webapp_id }}（{{ t.node_count || 0 }} 节点）
            </option>
          </select>
          <p v-if="!templatesLoading && !templates.length" class="studio-empty-hint">
            还没有保存的模板，请先去「应用模板」拉取并保存。
          </p>
          <p v-else-if="template" class="studio-meta">
            <span class="mono">{{ template.webapp_id }}</span>
            <TonePill
              v-if="template.source"
              :tone="template.source === 'apiCallDemo' ? 'success' : 'neutral'"
              :label="template.source"
            />
          </p>
        </div>

        <template v-if="template">
          <div
            v-for="n in imageNodes"
            :key="`img-${n.nodeId}-${n.fieldName}`"
            class="studio-field"
          >
            <label class="studio-label">{{ fieldLabel(n) }}</label>
            <p class="studio-sub">nodeId {{ n.nodeId }} · {{ n.fieldName }}</p>
            <div
              class="studio-drop"
              :class="{ 'is-over': dragOverKey === n.nodeId, 'has-file': !!imagePreviews[n.nodeId] }"
              @dragenter.prevent="dragOverKey = n.nodeId"
              @dragover.prevent="dragOverKey = n.nodeId"
              @dragleave.prevent="dragOverKey = ''"
              @drop.prevent="onDrop(n.nodeId, $event)"
              @click="fileInputs[n.nodeId]?.click()"
            >
              <input
                :ref="(el) => { if (el) fileInputs[n.nodeId] = el }"
                type="file"
                accept="image/*"
                class="sr-only"
                @change="onFile(n.nodeId, $event)"
              >
              <template v-if="imagePreviews[n.nodeId]">
                <img :src="imagePreviews[n.nodeId]" alt="" class="studio-drop__thumb">
                <div class="studio-drop__meta">
                  <p class="studio-drop__name">{{ imageFiles[n.nodeId]?.name }}</p>
                  <p class="studio-drop__sub">点击或拖拽可替换</p>
                  <button type="button" class="studio-link" @click.stop="onClearFile(n.nodeId)">移除</button>
                </div>
              </template>
              <template v-else>
                <p class="studio-drop__title">上传图片</p>
                <p class="studio-drop__sub">JPG / PNG / WEBP</p>
              </template>
            </div>
          </div>

          <div
            v-for="{ n, i } in textNodes"
            :key="nodeKey(n, i)"
            class="studio-field"
          >
            <div class="studio-field__label-row">
              <label class="studio-label" :for="`f-${nodeKey(n, i)}`">{{ fieldLabel(n) }}</label>
              <span class="studio-hint">{{ n.fieldType || n.fieldName }}</span>
            </div>
            <p class="studio-sub">nodeId {{ n.nodeId }} · {{ n.fieldName }}</p>
            <p v-if="optionsSummary(n)" class="studio-options-hint">{{ optionsSummary(n) }}</p>
            <select
              v-if="isListNode(n) && listOptions(n).length"
              :id="`f-${nodeKey(n, i)}`"
              v-model="fieldValues[nodeKey(n, i)]"
              class="studio-select studio-field-control"
            >
              <option
                v-for="opt in listOptions(n)"
                :key="opt.value"
                :value="opt.value"
                :title="opt.description"
              >
                {{ opt.label }}{{ opt.description ? ` — ${opt.description}` : '' }}
              </option>
            </select>
            <input
              v-else-if="isNumberNode(n)"
              :id="`f-${nodeKey(n, i)}`"
              v-model="fieldValues[nodeKey(n, i)]"
              type="number"
              class="studio-textarea studio-input studio-field-control"
              :min="numberMeta(n)?.min"
              :max="numberMeta(n)?.max"
              :step="numberMeta(n)?.step ?? 1"
              :placeholder="defaultPlaceholder(n)"
            >
            <textarea
              v-else-if="isMultiline(n)"
              :id="`f-${nodeKey(n, i)}`"
              v-model="fieldValues[nodeKey(n, i)]"
              rows="5"
              class="studio-textarea"
              :placeholder="defaultPlaceholder(n)"
            />
            <input
              v-else
              :id="`f-${nodeKey(n, i)}`"
              v-model="fieldValues[nodeKey(n, i)]"
              type="text"
              class="studio-textarea studio-input"
              :placeholder="defaultPlaceholder(n)"
            >
          </div>
        </template>
        </template>

        <div class="studio-actions">
          <Button size="md" variant="primary" block :disabled="!canSubmit" @click="submit">
            {{ submitting ? '提交中…' : isActive ? '重新提交' : (channel === 'free' ? '开始免费试用' : channel === 'qc' ? '开始 Runow 生成' : '开始生成') }}
          </Button>
          <Button
            v-if="isActive"
            size="md"
            variant="outline"
            :disabled="cancelling"
            @click="cancelTask"
          >
            {{ cancelling ? '取消中…' : '取消任务' }}
          </Button>
          <Button
            v-if="task && !isActive"
            size="md"
            variant="outline"
            @click="clearResult"
          >
            清除结果
          </Button>
        </div>

        <p v-if="error" class="studio-error">{{ error }}</p>
      </PagePanel>

      <PagePanel class="studio-preview">
        <header class="studio-preview__head">
          <div>
            <h2 class="studio-form__title">预览 / 结果</h2>
            <p class="studio-form__desc">有参考图时先预览；结果就绪后在此展示。</p>
          </div>
          <TonePill v-if="task" :tone="statusTone(task.status)" :label="statusLabel(task.status)" />
        </header>

        <div class="studio-stage" :class="{ 'is-busy': isActive }">
          <img
            v-if="task?.file_url && resultIsImage"
            :src="task.file_url"
            alt="result"
            class="studio-stage__media"
          >
          <video
            v-else-if="task?.file_url"
            :src="task.file_url"
            class="studio-stage__media"
            controls
            autoplay
          />
          <img v-else-if="primaryPreview" :src="primaryPreview" alt="preview" class="studio-stage__media">
          <div v-else class="studio-stage__empty">
            <p v-if="channel === 'free'">{{ primaryPreview ? '确认左侧参数后提交官方免费试用' : '填写提示词后提交（参考图可选）' }}</p>
            <p v-else-if="channel === 'qc'">{{ qcSelectedModel ? '确认模式/模型与参数后提交' : '选择创作模式与模型' }}</p>
            <p v-else>{{ template ? '填写左侧参数后提交' : '请先选择模板' }}</p>
            <span>生成完成后会自动展示结果</span>
          </div>
          <div v-if="isActive" class="studio-stage__overlay">
            <div class="studio-spinner" />
            <p>{{ progressText || '处理中…' }}</p>
          </div>
        </div>

        <div v-if="task" class="studio-task">
          <div class="studio-task__row">
            <span class="studio-task__k">本地 ID</span>
            <button type="button" class="studio-task__v mono" @click="copy(task.id)">{{ task.id }}</button>
          </div>
          <div class="studio-task__row">
            <span class="studio-task__k">RH Task</span>
            <button
              v-if="task.rh_task_id"
              type="button"
              class="studio-task__v mono"
              @click="copy(task.rh_task_id)"
            >{{ task.rh_task_id }}</button>
            <span v-else class="studio-task__v muted">提交后自动回填</span>
          </div>
          <div class="studio-task__row">
            <span class="studio-task__k">进度</span>
            <span class="studio-task__v">{{ progressText || '—' }}</span>
          </div>
          <div v-if="task.account_email" class="studio-task__row">
            <span class="studio-task__k">账号</span>
            <span class="studio-task__v">{{ task.account_email }}</span>
          </div>
          <div v-if="task.file_url" class="studio-task__row">
            <span class="studio-task__k">{{ resultIsImage ? '结果' : '成片' }}</span>
            <a class="studio-task__v link" :href="task.file_url" target="_blank" rel="noreferrer">打开文件</a>
          </div>
          <p v-if="task.error" class="studio-error mt-2">{{ task.error }}</p>
        </div>
      </PagePanel>
    </div>
  </div>
</template>

<style scoped>
.studio-page {
  min-height: calc(100vh - 9rem);
}

.studio-grid {
  display: grid;
  gap: 1.25rem;
}

@media (min-width: 1100px) {
  .studio-grid {
    grid-template-columns: minmax(340px, 440px) minmax(0, 1fr);
    align-items: start;
  }
}

.studio-form,
.studio-preview {
  display: flex;
  flex-direction: column;
  gap: 1.15rem;
}

.studio-form__head,
.studio-preview__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.studio-form__head-copy {
  min-width: 0;
  flex: 1;
}

.studio-form__head :deep(.studio-refresh-btn) {
  flex-shrink: 0;
  white-space: nowrap;
  padding-inline: 0.9rem;
  min-height: 2.1rem;
  border-radius: 999px;
}

.studio-form__title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 600;
  color: hsl(var(--foreground));
}

.studio-form__desc {
  margin: 0.35rem 0 0;
  font-size: 0.8125rem;
  line-height: 1.5;
  color: hsl(var(--muted-foreground));
}

.studio-field {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.studio-field--row {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.75rem;
}

@media (max-width: 720px) {
  .studio-field--row {
    grid-template-columns: 1fr;
  }
}

.studio-channel {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.studio-channel__btn {
  border: 1px solid hsl(var(--border));
  background: hsl(var(--card));
  color: hsl(var(--muted-foreground));
  border-radius: 999px;
  padding: 0.45rem 0.9rem;
  font-size: 0.8125rem;
  font-weight: 600;
  cursor: pointer;
}

.studio-channel__btn.is-active {
  border-color: hsl(var(--primary));
  color: hsl(var(--foreground));
  background: hsl(var(--primary) / 0.12);
}

.studio-field__label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.studio-label {
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  color: hsl(var(--muted-foreground));
}

.studio-hint {
  font-size: 0.7rem;
  color: hsl(var(--muted-foreground));
}

.studio-sub {
  margin: 0;
  font-size: 0.7rem;
  color: hsl(var(--muted-foreground) / 0.9);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.studio-check {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.8125rem;
  color: hsl(var(--foreground));
  cursor: pointer;
}

.studio-check input {
  width: 1rem;
  height: 1rem;
}

.studio-options-hint {
  margin: 0;
  font-size: 0.68rem;
  line-height: 1.4;
  color: hsl(var(--muted-foreground) / 0.85);
  word-break: break-word;
}

.studio-field-control.studio-select {
  min-height: 2.75rem;
  cursor: pointer;
}

.studio-field-control.studio-input {
  min-height: 2.75rem;
}

.studio-empty-hint,
.studio-meta {
  margin: 0;
  font-size: 0.75rem;
  color: hsl(var(--muted-foreground));
}

.studio-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}

.studio-meta .mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  word-break: break-all;
}

.studio-textarea,
.studio-select,
.studio-input {
  width: 100%;
  border-radius: 1rem;
  border: 1px solid hsl(var(--border));
  background: hsl(var(--background));
  color: hsl(var(--foreground));
  padding: 0.75rem 0.9rem;
  font-size: 0.875rem;
  outline: none;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.studio-textarea {
  min-height: 6.5rem;
  resize: vertical;
  line-height: 1.55;
}

.studio-input {
  min-height: 0;
}

.studio-textarea:focus,
.studio-select:focus,
.studio-input:focus {
  border-color: hsl(var(--primary));
  box-shadow: 0 0 0 3px hsl(var(--foreground) / 0.06);
}

.studio-select {
  appearance: none;
  background-image: linear-gradient(45deg, transparent 50%, hsl(var(--muted-foreground)) 50%),
    linear-gradient(135deg, hsl(var(--muted-foreground)) 50%, transparent 50%);
  background-position: calc(100% - 18px) calc(50% - 3px), calc(100% - 12px) calc(50% - 3px);
  background-size: 6px 6px, 6px 6px;
  background-repeat: no-repeat;
  padding-right: 2rem;
}

.studio-ref-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(12rem, 1fr));
  gap: 0.75rem;
}

.studio-thumbs {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(4.5rem, 4.5rem));
  gap: 0.5rem;
}

.studio-thumb {
  position: relative;
  width: 4.5rem;
  height: 4.5rem;
  border-radius: 0.75rem;
  border: 1.5px dashed hsl(var(--border));
  background: hsl(var(--muted) / 0.35);
  cursor: pointer;
  overflow: hidden;
  display: grid;
  place-items: center;
  transition: border-color 0.15s ease, transform 0.15s ease;
}

.studio-thumb:hover,
.studio-thumb.is-over {
  border-color: hsl(var(--foreground) / 0.45);
  transform: translateY(-1px);
}

.studio-thumb.has-file {
  border-style: solid;
  border-color: hsl(var(--border));
  background: hsl(var(--card));
}

.studio-thumb__img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.studio-thumb__plus {
  font-size: 1.35rem;
  line-height: 1;
  color: hsl(var(--muted-foreground));
  font-weight: 400;
}

.studio-thumb__idx {
  position: absolute;
  left: 0.28rem;
  top: 0.28rem;
  min-width: 1.05rem;
  height: 1.05rem;
  padding: 0 0.2rem;
  border-radius: 999px;
  background: hsl(var(--foreground) / 0.72);
  color: hsl(var(--background));
  font-size: 0.65rem;
  font-weight: 600;
  display: grid;
  place-items: center;
}

.studio-thumb__remove {
  position: absolute;
  right: 0.2rem;
  top: 0.2rem;
  width: 1.15rem;
  height: 1.15rem;
  border: 0;
  border-radius: 999px;
  background: hsl(var(--foreground) / 0.78);
  color: hsl(var(--background));
  font-size: 0.85rem;
  line-height: 1;
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.12s ease;
}

.studio-thumb:hover .studio-thumb__remove,
.studio-thumb:focus-within .studio-thumb__remove {
  opacity: 1;
}

.studio-drop {
  display: flex;
  align-items: center;
  gap: 0.9rem;
  min-height: 6.5rem;
  padding: 1rem;
  border-radius: 1.15rem;
  border: 1.5px dashed hsl(var(--border));
  background: linear-gradient(180deg, hsl(var(--background)) 0%, hsl(var(--card)) 100%);
  cursor: pointer;
  transition: border-color 0.15s ease, transform 0.15s ease;
}

.studio-drop--ref {
  min-height: 5.5rem;
  padding: 0.85rem;
}

.studio-drop:hover,
.studio-drop.is-over {
  border-color: hsl(var(--foreground) / 0.45);
  transform: translateY(-1px);
}

.studio-drop.has-file {
  border-style: solid;
}

.studio-drop__thumb {
  width: 4.5rem;
  height: 4.5rem;
  object-fit: cover;
  border-radius: 0.85rem;
  border: 1px solid hsl(var(--border));
  flex-shrink: 0;
}

.studio-drop__title {
  margin: 0;
  font-size: 0.875rem;
  font-weight: 500;
  color: hsl(var(--foreground));
}

.studio-drop__name {
  margin: 0;
  font-size: 0.8125rem;
  font-weight: 500;
  color: hsl(var(--foreground));
  word-break: break-all;
}

.studio-drop__sub {
  margin: 0.2rem 0 0;
  font-size: 0.75rem;
  color: hsl(var(--muted-foreground));
}

.studio-link {
  margin-top: 0.35rem;
  border: 0;
  background: transparent;
  padding: 0;
  font-size: 0.75rem;
  color: hsl(var(--foreground));
  text-decoration: underline;
  text-underline-offset: 2px;
  cursor: pointer;
}

.studio-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem;
  padding-top: 0.25rem;
}

.studio-error {
  margin: 0;
  font-size: 0.8125rem;
  color: hsl(var(--tone-error-strong));
}

.studio-stage {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 320px;
  overflow: hidden;
  border-radius: 1.25rem;
  border: 1px solid hsl(var(--border));
  background:
    radial-gradient(circle at 30% 20%, hsl(var(--foreground) / 0.04), transparent 40%),
    hsl(var(--background));
}

.studio-stage__media {
  max-height: 420px;
  max-width: 100%;
  object-fit: contain;
}

.studio-stage__empty {
  text-align: center;
  color: hsl(var(--muted-foreground));
  padding: 2rem;
}

.studio-stage__empty p {
  margin: 0;
  font-size: 0.9rem;
  font-weight: 500;
  color: hsl(var(--foreground));
}

.studio-stage__empty span {
  display: block;
  margin-top: 0.4rem;
  font-size: 0.75rem;
}

.studio-stage__overlay {
  position: absolute;
  inset: 0;
  display: grid;
  place-content: center;
  gap: 0.75rem;
  background: hsl(var(--background) / 0.72);
  backdrop-filter: blur(2px);
  text-align: center;
  font-size: 0.8125rem;
  color: hsl(var(--foreground));
}

.studio-spinner {
  width: 1.6rem;
  height: 1.6rem;
  margin: 0 auto;
  border-radius: 999px;
  border: 2px solid hsl(var(--border));
  border-top-color: hsl(var(--foreground));
  animation: studio-spin 0.8s linear infinite;
}

@keyframes studio-spin {
  to { transform: rotate(360deg); }
}

.studio-task {
  display: grid;
  gap: 0.55rem;
  padding: 0.85rem 1rem;
  border-radius: 1rem;
  border: 1px solid hsl(var(--border));
  background: hsl(var(--background));
}

.studio-task__row {
  display: grid;
  grid-template-columns: 4.5rem minmax(0, 1fr);
  gap: 0.75rem;
  align-items: start;
  font-size: 0.8125rem;
}

.studio-task__k {
  color: hsl(var(--muted-foreground));
}

.studio-task__v {
  min-width: 0;
  text-align: left;
  word-break: break-all;
  color: hsl(var(--foreground));
}

.studio-task__v.mono,
button.studio-task__v {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.75rem;
  border: 0;
  background: transparent;
  padding: 0;
  cursor: pointer;
}

.studio-task__v.mono:hover,
button.studio-task__v:hover {
  text-decoration: underline;
  text-underline-offset: 2px;
}

.studio-task__v.muted {
  color: hsl(var(--muted-foreground));
}

.studio-task__v.link {
  font-weight: 500;
  text-decoration: underline;
  text-underline-offset: 2px;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.mt-2 {
  margin-top: 0.5rem;
}
</style>

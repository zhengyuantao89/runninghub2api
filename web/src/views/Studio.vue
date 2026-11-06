<script setup>
import { onMounted, ref, watch } from 'vue'
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
  setImage,
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

function copy(text) {
  if (!text) return
  navigator.clipboard?.writeText(text)
}

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

watch(fieldValues, () => persist(), { deep: true })

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
              先在「应用模板」拉取并保存，再在此选择模板；按节点填写参数后提交。
            </p>
          </div>
          <Button
            size="sm"
            variant="outline"
            root-class="studio-refresh-btn"
            :disabled="templatesLoading"
            @click="refreshTemplates"
          >
            {{ templatesLoading ? '刷新中…' : '刷新列表' }}
          </Button>
        </header>

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

        <div class="studio-actions">
          <Button size="md" variant="primary" block :disabled="!canSubmit" @click="submit">
            {{ submitting ? '提交中…' : isActive ? '重新提交' : '开始生成' }}
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
            <p class="studio-form__desc">有参考图时先预览；成片就绪后在此播放。</p>
          </div>
          <TonePill v-if="task" :tone="statusTone(task.status)" :label="statusLabel(task.status)" />
        </header>

        <div class="studio-stage" :class="{ 'is-busy': isActive }">
          <video v-if="task?.file_url" :src="task.file_url" class="studio-stage__media" controls autoplay />
          <img v-else-if="primaryPreview" :src="primaryPreview" alt="preview" class="studio-stage__media">
          <div v-else class="studio-stage__empty">
            <p>{{ template ? '填写左侧参数后提交' : '请先选择模板' }}</p>
            <span>生成完成后会自动切换成片播放器</span>
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
            <span class="studio-task__k">成片</span>
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

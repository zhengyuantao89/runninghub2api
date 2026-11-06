<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Button, Input } from 'nanocat-ui'
import PagePanel from '@/components/ai/PagePanel.vue'
import TableShell from '@/components/ai/TableShell.vue'
import TonePill from '@/components/TonePill.vue'
import { api } from '@/api'
import { isListNode, isNumberNode, optionsSummary } from '@/lib/webappNodes'

const router = useRouter()

const webappId = ref('')
const accessPassword = ref('')
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const demo = ref(null)
const copied = ref('')
const savedTemplates = ref([])
const savedLoading = ref(false)
const saveMsg = ref('')
let fetchSeq = 0

const base = computed(() => window.location.origin.replace(/\/$/, ''))

const nodes = computed(() => demo.value?.node_info_list || [])

const gatewayCurl = computed(() => {
  const id = (webappId.value || demo.value?.webapp_id || '').trim() || '<webappId>'
  return `curl -s ${base.value}/v1/webapps/${id}/demo \\\n  -H "Authorization: Bearer <RH2API_API_KEY>"`
})

const submitExample = computed(() => {
  if (!demo.value?.example?.body) return ''
  // 下游示例不带 access_password：模板保存后由网关自动注入
  return JSON.stringify(demo.value.example.body, null, 2)
})

const needsAccessPassword = computed(
  () =>
    !!(demo.value?.access_encrypted || demo.value?.has_access_password) ||
    !!String(accessPassword.value || '').trim(),
)

const alreadySaved = computed(() => {
  const id = demo.value?.webapp_id
  if (!id) return null
  return savedTemplates.value.find((t) => t.webapp_id === id) || null
})

function formatFieldValue(v) {
  if (v == null || v === '') return '—'
  if (typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean') return String(v)
  try {
    return JSON.stringify(v)
  } catch {
    return String(v)
  }
}

function demoTone(d) {
  if (d?.cached || d?.source === 'saved_template') return 'info'
  if (d?.source === 'apiCallDemo') return 'success'
  return 'neutral'
}

function demoLabel(d) {
  if (d?.cached || d?.source === 'saved_template') {
    return d?.saved_from ? `本地模板 · ${d.saved_from}` : '本地已保存'
  }
  if (d?.source === 'apiCallDemo') return '官方 apiCallDemo'
  return d?.source || 'detail'
}

async function loadSaved() {
  savedLoading.value = true
  try {
    const res = await api.listTemplates()
    savedTemplates.value = Array.isArray(res?.data) ? res.data : []
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    savedLoading.value = false
  }
}

async function fetchDemo() {
  error.value = ''
  saveMsg.value = ''
  demo.value = null
  const id = webappId.value.trim()
  if (!id) {
    error.value = '请填写 webappId'
    return
  }
  const seq = ++fetchSeq
  loading.value = true
  try {
    // 管理端「拉取」强制走 RH；下游默认会命中已保存模板
    const res = await api.getWebappDemo(id, { refresh: true })
    if (seq !== fetchSeq) return
    demo.value = res
  } catch (e) {
    if (seq !== fetchSeq) return
    error.value = e.message || String(e)
  } finally {
    if (seq === fetchSeq) loading.value = false
  }
}

async function saveTemplate() {
  if (!demo.value?.webapp_id) return
  error.value = ''
  saveMsg.value = ''
  saving.value = true
  try {
    const pwd = String(accessPassword.value || '').trim()
    if (demo.value.access_encrypted && !pwd && !alreadySaved.value?.has_access_password) {
      throw new Error('该应用开启了加密访问，请填写访问密码后再保存')
    }
    const saved = await api.saveTemplate({
      webapp_id: demo.value.webapp_id,
      name: demo.value.name || demo.value.webapp_id,
      description: demo.value.description || '',
      source: demo.value.source || '',
      access_password: pwd,
      node_info_list: demo.value.node_info_list || [],
      example: demo.value.example || null,
    })
    saveMsg.value = alreadySaved.value ? '已更新保存' : '已保存到模板库'
    await loadSaved()
    if (saved?.id) {
      saveMsg.value = pwd
        ? '已保存（含访问密码，下游提交无需再传）'
        : '已保存，可在试跑工作室选择'
    }
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    saving.value = false
  }
}

async function removeSaved(id) {
  if (!id) return
  error.value = ''
  try {
    await api.deleteTemplate(id)
    await loadSaved()
    if (demo.value && savedTemplates.value.every((t) => t.webapp_id !== demo.value.webapp_id)) {
      saveMsg.value = ''
    }
  } catch (e) {
    error.value = e.message || String(e)
  }
}

function openInStudio(id) {
  router.push({ path: '/studio', query: { template: id } })
}

async function loadSavedIntoDemo(t) {
  error.value = ''
  saveMsg.value = ''
  webappId.value = t.webapp_id
  try {
    const full = await api.getTemplate(t.id)
    accessPassword.value = full.access_password || ''
    demo.value = {
      webapp_id: full.webapp_id,
      name: full.name,
      description: full.description,
      source: full.source,
      node_info_list: full.node_info_list || [],
      example: full.example || null,
      has_access_password: !!full.has_access_password || !!full.access_password,
      cached: true,
    }
  } catch (e) {
    error.value = e.message || String(e)
  }
}

async function copy(text, key) {
  try {
    await navigator.clipboard.writeText(text)
    copied.value = key
    setTimeout(() => {
      if (copied.value === key) copied.value = ''
    }, 1500)
  } catch {
    error.value = '复制失败'
  }
}

onMounted(loadSaved)
</script>

<template>
  <div class="webapps-page space-y-5">
    <PagePanel class="space-y-5">
      <header class="webapps-head">
        <div class="min-w-0">
          <h2 class="text-base font-semibold text-foreground">应用模板</h2>
          <p class="mt-1.5 max-w-2xl text-xs leading-relaxed text-muted-foreground">
            拉取 RH 应用节点并保存到模板库；试跑工作室可直接选用。下游用
            <code class="rounded bg-muted/60 px-1 font-mono text-[11px]">/v1/webapps/{'{id}'}/demo</code>
            + 系统 API Key。
          </p>
        </div>
        <TonePill
          v-if="demo"
          :tone="demoTone(demo)"
          :label="demoLabel(demo)"
        />
      </header>

      <section class="webapps-query">
        <label class="webapps-query__label" for="webapp-id-input">webappId</label>
        <div class="webapps-query__row">
          <Input
            id="webapp-id-input"
            v-model="webappId"
            type="text"
            size="md"
            block
            root-class="webapps-query__input"
            placeholder="从 RH 应用页 URL 复制，例如 2054244556132102145"
            @keydown.enter.prevent="fetchDemo"
          />
          <Button
            size="md"
            variant="primary"
            root-class="webapps-query__btn"
            :disabled="loading"
            @click="fetchDemo"
          >
            {{ loading ? '拉取中…' : '拉取模板' }}
          </Button>
        </div>
        <label class="webapps-query__label" for="access-password-input">访问密码（加密应用）</label>
        <div class="webapps-query__row">
          <Input
            id="access-password-input"
            v-model="accessPassword"
            type="text"
            size="md"
            block
            root-class="webapps-query__input"
            placeholder="如 asdfghjkl；保存到模板后下游提交无需再传"
          />
        </div>
        <p class="mt-1.5 text-[11px] leading-relaxed text-muted-foreground">
          部分应用需访问密码才能运行。拉取后在此填写并「保存到模板」，网关提交时会自动带上。
        </p>
        <p v-if="error" class="webapps-query__error">{{ error }}</p>
        <p v-else-if="saveMsg" class="webapps-query__ok">{{ saveMsg }}</p>
      </section>

      <section class="webapps-code">
        <div class="webapps-code__bar">
          <p class="webapps-code__title">给下游的拉取示例</p>
          <Button size="xs" variant="outline" @click="copy(gatewayCurl, 'curl')">
            {{ copied === 'curl' ? '已复制' : '复制' }}
          </Button>
        </div>
        <pre class="webapps-code__pre"><code>{{ gatewayCurl }}</code></pre>
      </section>
    </PagePanel>

    <PagePanel class="space-y-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h3 class="text-sm font-semibold text-foreground">已保存模板</h3>
        <Button size="xs" variant="outline" :disabled="savedLoading" @click="loadSaved">
          {{ savedLoading ? '加载中…' : '刷新' }}
        </Button>
      </div>
      <TableShell>
        <table class="w-full text-left text-xs">
          <thead>
            <tr class="border-b border-border text-muted-foreground">
              <th class="px-3 py-2.5 font-medium">名称</th>
              <th class="px-3 py-2.5 font-medium">webappId</th>
              <th class="px-3 py-2.5 font-medium">节点</th>
              <th class="px-3 py-2.5 font-medium">密码</th>
              <th class="px-3 py-2.5 font-medium">来源</th>
              <th class="px-3 py-2.5 font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="t in savedTemplates"
              :key="t.id"
              class="border-b border-border/60"
            >
              <td class="px-3 py-2.5">{{ t.name || '—' }}</td>
              <td class="max-w-[14rem] truncate px-3 py-2.5 font-mono" :title="t.webapp_id">
                {{ t.webapp_id }}
              </td>
              <td class="px-3 py-2.5">{{ t.node_count ?? '—' }}</td>
              <td class="px-3 py-2.5">
                <TonePill
                  :tone="t.has_access_password ? 'success' : 'neutral'"
                  :label="t.has_access_password ? '已存' : '无'"
                />
              </td>
              <td class="px-3 py-2.5">
                <TonePill
                  v-if="t.source"
                  :tone="t.source === 'apiCallDemo' ? 'success' : 'neutral'"
                  :label="t.source"
                />
                <span v-else>—</span>
              </td>
              <td class="px-3 py-2.5">
                <div class="flex flex-wrap gap-1.5">
                  <Button size="xs" variant="outline" @click="loadSavedIntoDemo(t)">查看</Button>
                  <Button size="xs" variant="primary" @click="openInStudio(t.id)">试跑</Button>
                  <Button size="xs" variant="outline" @click="removeSaved(t.id)">删除</Button>
                </div>
              </td>
            </tr>
            <tr v-if="!savedTemplates.length">
              <td colspan="6" class="px-3 py-8 text-center text-muted-foreground">
                暂无保存的模板，拉取后点「保存到模板库」
              </td>
            </tr>
          </tbody>
        </table>
      </TableShell>
    </PagePanel>

    <PagePanel v-if="demo" class="space-y-4">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h3 class="text-sm font-semibold text-foreground">{{ demo.name || '未命名应用' }}</h3>
            <TonePill
              v-if="needsAccessPassword"
              :tone="accessPassword || demo.has_access_password ? 'success' : 'error'"
              :label="accessPassword || demo.has_access_password ? '加密·已设密码' : '加密·需访问密码'"
            />
          </div>
          <p class="mt-1 break-all font-mono text-xs text-muted-foreground">{{ demo.webapp_id }}</p>
          <p v-if="demo.rh_key_account" class="mt-1 text-xs text-muted-foreground">
            使用号池 Key：{{ demo.rh_key_account }}
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <Button size="sm" variant="primary" :disabled="saving" @click="saveTemplate">
            {{ saving ? '保存中…' : alreadySaved ? '更新保存' : '保存到模板库' }}
          </Button>
          <Button
            v-if="alreadySaved"
            size="sm"
            variant="outline"
            @click="openInStudio(alreadySaved.id)"
          >
            去试跑
          </Button>
          <Button size="sm" variant="outline" :disabled="!submitExample" @click="copy(submitExample, 'body')">
            {{ copied === 'body' ? '已复制 body' : '复制提交 body' }}
          </Button>
        </div>
      </div>

      <TableShell>
        <table class="w-full text-left text-xs">
          <thead>
            <tr class="border-b border-border text-muted-foreground">
              <th class="px-3 py-2.5 font-medium">nodeId</th>
              <th class="px-3 py-2.5 font-medium">nodeName</th>
              <th class="px-3 py-2.5 font-medium">fieldName</th>
              <th class="px-3 py-2.5 font-medium">fieldType</th>
              <th class="px-3 py-2.5 font-medium">默认值</th>
              <th class="px-3 py-2.5 font-medium">可选/范围</th>
              <th class="px-3 py-2.5 font-medium">说明</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(n, i) in nodes"
              :key="`${n.nodeId}-${n.fieldName}-${i}`"
              class="border-b border-border/60"
            >
              <td class="px-3 py-2.5 font-mono">{{ n.nodeId }}</td>
              <td class="px-3 py-2.5">{{ n.nodeName || '—' }}</td>
              <td class="px-3 py-2.5 font-mono">{{ n.fieldName }}</td>
              <td class="px-3 py-2.5">
                <TonePill
                  v-if="n.fieldType"
                  :tone="
                    String(n.fieldType).toUpperCase() === 'IMAGE'
                      ? 'info'
                      : isListNode(n)
                        ? 'success'
                        : isNumberNode(n)
                          ? 'info'
                          : 'neutral'
                  "
                  :label="n.fieldType"
                />
                <span v-else>—</span>
              </td>
              <td
                class="max-w-[12rem] truncate px-3 py-2.5 font-mono text-muted-foreground"
                :title="formatFieldValue(n.fieldValue)"
              >
                {{ formatFieldValue(n.fieldValue) }}
              </td>
              <td
                class="max-w-[16rem] truncate px-3 py-2.5 text-muted-foreground"
                :title="optionsSummary(n) || ''"
              >
                {{ optionsSummary(n) || '—' }}
              </td>
              <td
                class="max-w-[14rem] truncate px-3 py-2.5 text-muted-foreground"
                :title="n.description || ''"
              >
                {{ n.description || '—' }}
              </td>
            </tr>
            <tr v-if="!nodes.length">
              <td colspan="7" class="px-3 py-8 text-center text-muted-foreground">无节点</td>
            </tr>
          </tbody>
        </table>
      </TableShell>

      <section v-if="submitExample" class="webapps-code">
        <div class="webapps-code__bar">
          <p class="webapps-code__title">下游提交 body（改本应用 node_info_list；加密应用密码已由模板注入）</p>
          <Button size="xs" variant="outline" @click="copy(submitExample, 'body2')">
            {{ copied === 'body2' ? '已复制' : '复制' }}
          </Button>
        </div>
        <pre class="webapps-code__pre"><code>{{ submitExample }}</code></pre>
      </section>
    </PagePanel>
  </div>
</template>

<style scoped>
.webapps-head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.webapps-query {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 14px 16px;
  border: 1px solid hsl(var(--border) / 0.85);
  border-radius: 16px;
  background: hsl(var(--muted) / 0.22);
}

.webapps-query__label {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.02em;
  color: hsl(var(--muted-foreground));
}

.webapps-query__row {
  display: flex;
  flex-wrap: wrap;
  align-items: stretch;
  gap: 10px;
}

.webapps-query__row :deep(.webapps-query__input),
.webapps-query__row :deep(input) {
  width: 100%;
  min-width: 0;
}

.webapps-query__row :deep(.webapps-query__input) {
  flex: 1 1 18rem;
}

.webapps-query__btn {
  flex: 0 0 auto;
  min-width: 7.5rem;
  justify-content: center;
}

.webapps-query__error {
  margin: 0;
  font-size: 12px;
  color: hsl(var(--tone-error-strong));
}

.webapps-query__ok {
  margin: 0;
  font-size: 12px;
  color: hsl(var(--tone-success-strong, 142 45% 32%));
}

.webapps-code {
  border: 1px solid hsl(var(--border) / 0.8);
  border-radius: 14px;
  background: hsl(var(--background));
  overflow: hidden;
}

.webapps-code__bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid hsl(var(--border) / 0.7);
  background: hsl(var(--muted) / 0.18);
}

.webapps-code__title {
  margin: 0;
  font-size: 12px;
  font-weight: 500;
  color: hsl(var(--muted-foreground));
}

.webapps-code__pre {
  margin: 0;
  padding: 12px 14px;
  overflow-x: auto;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  line-height: 1.55;
  color: hsl(var(--foreground) / 0.88);
}
</style>

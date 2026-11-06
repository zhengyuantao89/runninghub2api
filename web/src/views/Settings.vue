<script setup>
import { onMounted, ref } from 'vue'
import { Button, StatusPill } from 'nanocat-ui'
import PagePanel from '@/components/ai/PagePanel.vue'
import { api } from '@/api'

const loading = ref(false)
const error = ref('')
const username = ref('')
const apiKey = ref('')
const showKey = ref(false)
const copied = ref(false)

async function load() {
  error.value = ''
  loading.value = true
  try {
    const res = await api.console()
    username.value = res.username || 'admin'
    apiKey.value = res.api_key || ''
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    loading.value = false
  }
}

async function copyKey() {
  if (!apiKey.value) return
  try {
    await navigator.clipboard.writeText(apiKey.value)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 1500)
  } catch {
    error.value = '复制失败，请手动选中复制'
  }
}

onMounted(load)
</script>

<template>
  <div class="settings-page">
    <div class="settings-grid">
      <PagePanel class="settings-card">
        <header class="settings-head">
          <div>
            <h2 class="settings-title">系统设置</h2>
            <p class="settings-desc">控制台用账号密码登录；API Key 给下游调用和账号推送用。</p>
          </div>
          <StatusPill :tone="apiKey ? 'success' : 'neutral'" :label="apiKey ? '已登录' : '未加载'" />
        </header>

        <div class="settings-field">
          <div class="settings-field__label-row">
            <span class="settings-label">登录账号</span>
          </div>
          <p class="settings-value">{{ username || '—' }}</p>
        </div>

        <div class="settings-field">
          <div class="settings-field__label-row">
            <span class="settings-label">API Key</span>
            <span class="settings-hint">下游调用 / 账号推送填这个</span>
          </div>
          <div class="settings-input-wrap">
            <code class="settings-key">{{ showKey ? (apiKey || '—') : (apiKey ? '••••••••••••••••••••' : '—') }}</code>
            <button type="button" class="settings-eye" :disabled="!apiKey" @click="showKey = !showKey">
              {{ showKey ? '隐藏' : '显示' }}
            </button>
          </div>
          <div class="settings-actions">
            <Button size="md" variant="primary" :disabled="!apiKey" @click="copyKey">
              {{ copied ? '已复制' : '复制 API Key' }}
            </Button>
            <Button size="md" variant="outline" :disabled="loading" @click="load">刷新</Button>
          </div>
        </div>

        <p v-if="error" class="settings-err">{{ error }}</p>
        <p v-else class="settings-ok">
          工作室提交任务不需要再填 Key：登录后直接用。外部同步账号时把这把 API Key 填到推送端即可。
        </p>
      </PagePanel>

      <PagePanel class="settings-side">
        <h3 class="settings-side__title">说明</h3>
        <ul class="settings-tips">
          <li>控制台账号密码在服务器 <code>.env</code>：<code>RH2API_ADMIN_USER</code> / <code>RH2API_ADMIN_PASSWORD</code>。</li>
          <li>API Key 在服务器 <code>.env</code>：<code>RH2API_API_KEY</code>（兼容旧的 <code>RH2API_AUTH_KEY</code>）。</li>
          <li>外部系统同步账号时，把 Sync Token / 推送密钥改成这里复制的 API Key。</li>
        </ul>
      </PagePanel>
    </div>
  </div>
</template>

<style scoped>
.settings-page {
  min-height: calc(100vh - 9rem);
}

.settings-grid {
  display: grid;
  gap: 1.25rem;
  max-width: 920px;
}

@media (min-width: 900px) {
  .settings-grid {
    grid-template-columns: minmax(0, 1.2fr) minmax(240px, 0.8fr);
    align-items: start;
  }
}

.settings-card,
.settings-side {
  display: flex;
  flex-direction: column;
  gap: 1.15rem;
}

.settings-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.settings-title {
  font-size: 1.125rem;
  font-weight: 600;
  color: hsl(var(--foreground));
}

.settings-desc {
  margin-top: 0.35rem;
  font-size: 0.875rem;
  color: hsl(var(--muted-foreground));
}

.settings-field {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.settings-field__label-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
}

.settings-label {
  font-size: 0.875rem;
  font-weight: 500;
  color: hsl(var(--foreground));
}

.settings-hint {
  font-size: 0.75rem;
  color: hsl(var(--muted-foreground));
}

.settings-value {
  font-size: 0.95rem;
  color: hsl(var(--foreground));
}

.settings-input-wrap {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-height: 2.75rem;
  border: 1px solid hsl(var(--border));
  border-radius: 0.9rem;
  background: hsl(var(--background));
  padding: 0.55rem 0.75rem;
}

.settings-key {
  flex: 1;
  overflow-wrap: anywhere;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.8rem;
  color: hsl(var(--foreground));
}

.settings-eye {
  flex-shrink: 0;
  border: 0;
  background: transparent;
  color: hsl(var(--muted-foreground));
  font-size: 0.75rem;
  cursor: pointer;
}

.settings-eye:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.settings-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
}

.settings-ok {
  font-size: 0.875rem;
  color: hsl(var(--tone-success-strong));
}

.settings-err {
  font-size: 0.875rem;
  color: hsl(var(--tone-error-strong));
}

.settings-side__title {
  font-size: 1rem;
  font-weight: 600;
}

.settings-tips {
  display: grid;
  gap: 0.75rem;
  margin: 0;
  padding-left: 1.1rem;
  font-size: 0.875rem;
  color: hsl(var(--muted-foreground));
}

.settings-tips code {
  font-size: 0.78rem;
}
</style>

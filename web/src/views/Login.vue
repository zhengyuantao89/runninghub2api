<template>
  <div class="min-h-screen px-4">
    <div class="flex min-h-screen items-center justify-center">
      <div class="w-full max-w-md rounded-[2.5rem] border border-border bg-card p-10 shadow-2xl shadow-black/10">
        <div class="text-center">
          <div class="mb-3 flex justify-center">
            <BrandMark size="xl" :decorative="false" />
          </div>
          <h1 class="text-3xl font-semibold text-foreground">runninghub2api</h1>
          <p class="mt-2 text-sm text-muted-foreground">
            {{ mode === 'register' ? '注册普通用户' : '控制台登录' }}
          </p>
        </div>

        <div v-if="registrationEnabled" class="mt-6 grid grid-cols-2 gap-2 rounded-2xl border border-border p-1">
          <button
            type="button"
            class="rounded-xl px-3 py-2 text-sm font-medium transition-colors"
            :class="mode === 'login' ? 'bg-[hsl(var(--primary)_/_0.12)] text-foreground' : 'text-muted-foreground'"
            @click="mode = 'login'"
          >
            登录
          </button>
          <button
            type="button"
            class="rounded-xl px-3 py-2 text-sm font-medium transition-colors"
            :class="mode === 'register' ? 'bg-[hsl(var(--primary)_/_0.12)] text-foreground' : 'text-muted-foreground'"
            @click="mode = 'register'"
          >
            注册
          </button>
        </div>

        <form class="mt-8 space-y-6" @submit.prevent="submit">
          <div class="space-y-2">
            <label for="username" class="ui-field-label text-sm font-medium text-foreground">账号</label>
            <Input
              id="username"
              v-model="username"
              type="text"
              size="md"
              block
              autocomplete="username"
              placeholder="用户名"
              :disabled="loading"
            />
          </div>

          <div class="space-y-2">
            <label for="password" class="ui-field-label text-sm font-medium text-foreground">密码</label>
            <Input
              id="password"
              v-model="password"
              type="password"
              size="md"
              block
              autocomplete="current-password"
              :placeholder="mode === 'register' ? '至少 6 位' : '输入密码'"
              :disabled="loading"
            />
          </div>

          <Button
            type="submit"
            size="md"
            variant="primary"
            block
            :disabled="loading || !username.trim() || !password"
          >
            {{ loading ? '处理中...' : mode === 'register' ? '注册并登录' : '登录' }}
          </Button>

          <p v-if="error" class="text-center text-sm text-[hsl(var(--tone-error-strong))]">{{ error }}</p>
          <p v-else-if="!registrationEnabled" class="text-center text-xs text-muted-foreground">
            公开注册已关闭，请使用已有账号或联系管理员
          </p>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Button, Input } from 'nanocat-ui'
import BrandMark from '@/components/BrandMark.vue'
import { api, clearSession, setSession } from '@/api'
import { homePathForRole, isAdminRole } from '@/lib/auth'

const router = useRouter()
const route = useRoute()
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)
const mode = ref('login')
const registrationEnabled = ref(false)

function postLoginPath(role) {
  const home = homePathForRole(role)
  const raw = String(route.query.redirect || '').trim()
  if (!raw || !raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/login')) {
    return home
  }
  if (isAdminRole(role)) {
    if (raw.startsWith('/admin')) return raw
    return home
  }
  if (raw.startsWith('/app')) return raw
  return home
}

async function submit() {
  error.value = ''
  loading.value = true
  try {
    clearSession()
    const res =
      mode.value === 'register'
        ? await api.register(username.value.trim(), password.value)
        : await api.login(username.value.trim(), password.value)
    if (!res?.token) throw new Error(mode.value === 'register' ? '注册失败：未返回会话' : '登录失败：未返回会话')
    setSession(res.token, res.role || 'user')
    await api.me()
    router.replace(postLoginPath(res.role || 'user'))
  } catch (e) {
    clearSession()
    error.value = e.message || String(e)
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  try {
    const res = await api.registrationStatus()
    registrationEnabled.value = !!res?.enabled
  } catch {
    registrationEnabled.value = false
  }
})
</script>

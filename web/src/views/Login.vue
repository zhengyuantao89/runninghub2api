<template>
  <div class="min-h-screen px-4">
    <div class="flex min-h-screen items-center justify-center">
      <div class="w-full max-w-md rounded-[2.5rem] border border-border bg-card p-10 shadow-2xl shadow-black/10">
        <div class="text-center">
          <h1 class="text-3xl font-semibold text-foreground">runninghub2api</h1>
          <p class="mt-2 text-sm text-muted-foreground">控制台登录</p>
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
              placeholder="admin"
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
              placeholder="输入密码"
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
            {{ loading ? '登录中...' : '登录' }}
          </Button>

          <p v-if="error" class="text-center text-sm text-[hsl(var(--tone-error-strong))]">{{ error }}</p>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Button, Input } from 'nanocat-ui'
import { api, clearSession, setSession } from '@/api'

const router = useRouter()
const route = useRoute()
const username = ref('admin')
const password = ref('')
const error = ref('')
const loading = ref(false)

function postLoginPath() {
  const raw = String(route.query.redirect || '').trim()
  if (!raw || !raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/login')) return '/'
  return raw
}

async function submit() {
  error.value = ''
  loading.value = true
  try {
    clearSession()
    const res = await api.login(username.value.trim(), password.value)
    if (!res?.token) throw new Error('登录失败：未返回会话')
    setSession(res.token)
    await api.me()
    router.replace(postLoginPath())
  } catch (e) {
    clearSession()
    error.value = e.message || String(e)
  } finally {
    loading.value = false
  }
}
</script>

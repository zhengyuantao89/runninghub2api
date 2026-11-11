<script setup>
import { onMounted, reactive, ref } from 'vue'
import { Button, Input, ModalShell } from 'nanocat-ui'
import PagePanel from '@/components/ai/PagePanel.vue'
import TableShell from '@/components/ai/TableShell.vue'
import TonePill from '@/components/TonePill.vue'
import { api } from '@/api'

const rows = ref([])
const loading = ref(false)
const error = ref('')
const okmsg = ref('')
const busyId = ref(0)
const showCreate = ref(false)
const showPassword = ref(false)
const passwordTarget = ref(null)
const form = reactive({
  username: '',
  password: '',
  role: 'user',
})
const newPassword = ref('')

async function load() {
  error.value = ''
  loading.value = true
  try {
    const res = await api.listUsers()
    rows.value = res?.data || []
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    loading.value = false
  }
}

async function createUser() {
  error.value = ''
  okmsg.value = ''
  try {
    await api.createUser({
      username: form.username.trim(),
      password: form.password,
      role: form.role,
    })
    showCreate.value = false
    form.username = ''
    form.password = ''
    form.role = 'user'
    okmsg.value = '用户已创建'
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  }
}

async function toggleStatus(u) {
  if (!u?.id) return
  busyId.value = u.id
  error.value = ''
  try {
    if (u.status === 'disabled') await api.enableUser(u.id)
    else await api.disableUser(u.id)
    okmsg.value = u.status === 'disabled' ? '已启用' : '已禁用'
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    busyId.value = 0
  }
}

function openPassword(u) {
  passwordTarget.value = u
  newPassword.value = ''
  showPassword.value = true
}

async function savePassword() {
  if (!passwordTarget.value?.id) return
  error.value = ''
  busyId.value = passwordTarget.value.id
  try {
    await api.setUserPassword(passwordTarget.value.id, newPassword.value)
    showPassword.value = false
    okmsg.value = '密码已更新'
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    busyId.value = 0
  }
}

function roleLabel(role) {
  return role === 'admin' ? '管理员' : '用户'
}

function statusTone(status) {
  return status === 'active' ? 'success' : 'neutral'
}

function statusLabel(status) {
  return status === 'active' ? '正常' : '禁用'
}

onMounted(load)
</script>

<template>
  <div class="space-y-5">
    <PagePanel>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="text-lg font-semibold text-foreground">用户管理</h2>
          <p class="mt-1 text-sm text-muted-foreground">管理员可创建用户、启停账号、重置密码；不能代用户提交生视频。</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <Button size="sm" variant="primary" @click="showCreate = true">新建用户</Button>
          <Button size="sm" variant="outline" :disabled="loading" @click="load">刷新</Button>
        </div>
      </div>
      <p v-if="error" class="mt-3 text-sm text-[hsl(var(--tone-error-strong))]">{{ error }}</p>
      <p v-else-if="okmsg" class="mt-3 text-sm text-[hsl(var(--tone-success-strong))]">{{ okmsg }}</p>
    </PagePanel>

    <PagePanel>
      <TableShell>
        <table class="w-full min-w-[720px] text-left text-sm">
          <thead>
            <tr class="border-b border-border text-muted-foreground">
              <th class="px-3 py-2.5 font-medium">ID</th>
              <th class="px-3 py-2.5 font-medium">用户名</th>
              <th class="px-3 py-2.5 font-medium">角色</th>
              <th class="px-3 py-2.5 font-medium">状态</th>
              <th class="px-3 py-2.5 font-medium">创建时间</th>
              <th class="px-3 py-2.5 font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in rows" :key="u.id" class="border-b border-border/70">
              <td class="px-3 py-2.5 font-mono text-xs">{{ u.id }}</td>
              <td class="px-3 py-2.5">{{ u.username }}</td>
              <td class="px-3 py-2.5">{{ roleLabel(u.role) }}</td>
              <td class="px-3 py-2.5">
                <TonePill :tone="statusTone(u.status)" :label="statusLabel(u.status)" />
              </td>
              <td class="px-3 py-2.5 text-muted-foreground">{{ u.created_at || '—' }}</td>
              <td class="px-3 py-2.5">
                <div class="flex flex-wrap gap-1.5">
                  <Button size="xs" variant="outline" :disabled="busyId === u.id" @click="toggleStatus(u)">
                    {{ u.status === 'disabled' ? '启用' : '禁用' }}
                  </Button>
                  <Button size="xs" variant="outline" @click="openPassword(u)">改密</Button>
                </div>
              </td>
            </tr>
            <tr v-if="!rows.length">
              <td colspan="6" class="px-3 py-8 text-center text-muted-foreground">暂无用户</td>
            </tr>
          </tbody>
        </table>
      </TableShell>
    </PagePanel>

    <ModalShell
      :open="showCreate"
      title="新建用户"
      description="创建控制台用户；管理员拥有全部权限（含试跑与 API）。"
      size-class="max-w-lg"
      close-on-overlay
      @close="showCreate = false"
    >
      <div class="space-y-4 p-1">
        <div class="space-y-1.5">
          <label class="text-sm font-medium">用户名</label>
          <Input v-model="form.username" size="md" block placeholder="至少 3 位" />
        </div>
        <div class="space-y-1.5">
          <label class="text-sm font-medium">密码</label>
          <Input v-model="form.password" type="password" size="md" block placeholder="至少 6 位" />
        </div>
        <div class="space-y-1.5">
          <label class="text-sm font-medium">角色</label>
          <select
            v-model="form.role"
            class="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
          >
            <option value="user">普通用户</option>
            <option value="admin">管理员</option>
          </select>
        </div>
        <div class="flex justify-end gap-2">
          <Button size="sm" variant="outline" @click="showCreate = false">取消</Button>
          <Button size="sm" variant="primary" @click="createUser">创建</Button>
        </div>
      </div>
    </ModalShell>

    <ModalShell
      :open="showPassword"
      title="重置密码"
      :description="passwordTarget?.username || ''"
      size-class="max-w-lg"
      close-on-overlay
      @close="showPassword = false"
    >
      <div class="space-y-4 p-1">
        <div class="space-y-1.5">
          <label class="text-sm font-medium">新密码</label>
          <Input v-model="newPassword" type="password" size="md" block placeholder="至少 6 位" />
        </div>
        <div class="flex justify-end gap-2">
          <Button size="sm" variant="outline" @click="showPassword = false">取消</Button>
          <Button size="sm" variant="primary" :disabled="!newPassword" @click="savePassword">保存</Button>
        </div>
      </div>
    </ModalShell>
  </div>
</template>

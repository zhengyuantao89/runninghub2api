<script setup>
import { onMounted, reactive, ref } from 'vue'
import { Button, Input, StatusPill } from 'nanocat-ui'
import PagePanel from '@/components/ai/PagePanel.vue'
import TableShell from '@/components/ai/TableShell.vue'
import { api } from '@/api'
import { copyText } from '@/lib/copyText'

const rows = ref([])
const loading = ref(false)
const creating = ref(false)
const error = ref('')
const okmsg = ref('')
const name = ref('')
const busyId = ref(0)
const revealed = reactive({})
const copiedId = ref(0)

function maskKey(k) {
  const plain = String(k?.key || '').trim()
  if (plain) {
    if (plain.length <= 12) return '••••••••••••'
    return `${plain.slice(0, 8)}${'•'.repeat(Math.min(20, plain.length - 12))}${plain.slice(-4)}`
  }
  const prefix = String(k?.key_prefix || '').trim()
  return prefix ? `${prefix}${'•'.repeat(16)}` : '••••••••••••••••'
}

function displayKey(k) {
  if (revealed[k.id] && k.key) return k.key
  return maskKey(k)
}

function toggleReveal(k) {
  if (!k?.key) {
    error.value = '旧 Key 无法回显，请删除后重新创建'
    return
  }
  revealed[k.id] = !revealed[k.id]
}

async function load() {
  error.value = ''
  loading.value = true
  try {
    const res = await api.listKeys()
    rows.value = res?.data || []
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    loading.value = false
  }
}

async function create() {
  error.value = ''
  okmsg.value = ''
  creating.value = true
  try {
    const res = await api.createKey(name.value.trim())
    name.value = ''
    okmsg.value = '已创建'
    await load()
    const id = res?.meta?.id
    if (id) revealed[id] = false
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    creating.value = false
  }
}

async function remove(id) {
  if (!id) return
  error.value = ''
  busyId.value = id
  try {
    await api.deleteKey(id)
    delete revealed[id]
    okmsg.value = '已删除'
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    busyId.value = 0
  }
}

async function copyKey(k) {
  const plain = String(k?.key || '').trim()
  if (!plain) {
    error.value = '旧 Key 无法复制，请删除后重新创建'
    return
  }
  error.value = ''
  try {
    await copyText(plain)
    copiedId.value = k.id
    okmsg.value = '已复制到剪贴板'
    setTimeout(() => {
      if (copiedId.value === k.id) copiedId.value = 0
    }, 1500)
  } catch {
    error.value = '复制失败，请手动选中密钥后复制'
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-5">
    <PagePanel>
      <header class="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 class="text-lg font-semibold text-foreground">个人 API Key</h2>
          <p class="mt-1 text-sm text-muted-foreground">
            用于调用 <code>/v1/videos</code>。默认掩码显示，点击可显示或复制。
          </p>
        </div>
        <StatusPill :tone="rows.length ? 'success' : 'neutral'" :label="`${rows.length} 把`" />
      </header>

      <div class="flex flex-wrap items-end gap-3">
        <div class="min-w-[12rem] flex-1 space-y-1.5">
          <label class="text-sm font-medium text-foreground">备注名（可选）</label>
          <Input v-model="name" size="md" block placeholder="例如：本机 / CI" :disabled="creating" />
        </div>
        <Button size="md" variant="primary" :disabled="creating" @click="create">
          {{ creating ? '创建中...' : '新建 Key' }}
        </Button>
        <Button size="md" variant="outline" :disabled="loading" @click="load">刷新</Button>
      </div>

      <p v-if="error" class="mt-3 text-sm text-[hsl(var(--tone-error-strong))]">{{ error }}</p>
      <p v-else-if="okmsg" class="mt-3 text-sm text-[hsl(var(--tone-success-strong))]">{{ okmsg }}</p>
    </PagePanel>

    <PagePanel>
      <TableShell>
        <table class="w-full min-w-[720px] text-left text-sm">
          <thead>
            <tr class="border-b border-border text-muted-foreground">
              <th class="px-3 py-2.5 font-medium">名称</th>
              <th class="px-3 py-2.5 font-medium">密钥</th>
              <th class="px-3 py-2.5 font-medium">创建时间</th>
              <th class="px-3 py-2.5 font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="k in rows" :key="k.id" class="border-b border-border/70">
              <td class="px-3 py-2.5">{{ k.name || '—' }}</td>
              <td class="px-3 py-2.5">
                <button
                  type="button"
                  class="max-w-[28rem] break-all text-left font-mono text-xs text-foreground hover:underline"
                  :title="k.key ? (revealed[k.id] ? '点击隐藏' : '点击显示') : '旧 Key 无法回显'"
                  @click="toggleReveal(k)"
                >
                  {{ displayKey(k) }}
                </button>
              </td>
              <td class="px-3 py-2.5 text-muted-foreground">{{ k.created_at || '—' }}</td>
              <td class="px-3 py-2.5">
                <div class="flex flex-wrap gap-1.5">
                  <Button size="xs" variant="outline" :disabled="!k.key" @click="toggleReveal(k)">
                    {{ revealed[k.id] ? '隐藏' : '显示' }}
                  </Button>
                  <Button size="xs" variant="primary" :disabled="!k.key" @click="copyKey(k)">
                    {{ copiedId === k.id ? '已复制' : '复制' }}
                  </Button>
                  <Button
                    size="xs"
                    variant="outline"
                    :disabled="busyId === k.id"
                    @click="remove(k.id)"
                  >
                    删除
                  </Button>
                </div>
              </td>
            </tr>
            <tr v-if="!rows.length">
              <td colspan="4" class="px-3 py-8 text-center text-muted-foreground">暂无 API Key</td>
            </tr>
          </tbody>
        </table>
      </TableShell>
    </PagePanel>
  </div>
</template>

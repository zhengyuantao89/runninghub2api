<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Button, Input, StatusPill } from 'nanocat-ui'
import PagePanel from '@/components/ai/PagePanel.vue'
import { api, getRole } from '@/api'
import { consoleBaseFromPath, isAdminRole } from '@/lib/auth'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const saving = ref(false)
const savingCreds = ref(false)
const savingCaptcha = ref(false)
const savingRegister = ref(false)
const error = ref('')
const okmsg = ref('')
const username = ref('')
const role = ref(getRole())
const pollAuth = ref('apikey')
const registrationEnabled = ref(false)
const uploadKeepHours = ref(24)
const uploadKeepDraft = ref(24)
const credCurrentPassword = ref('')
const credUsername = ref('')
const credPassword = ref('')
const credPassword2 = ref('')
const captchaProvider = ref('off')
const captchaApiKey = ref('')
const captchaApiBase = ref('')
const captchaTimeout = ref(180)
const captchaProviders = ref({})
const captchaDefaultBases = {
  yescaptcha: 'https://api.yescaptcha.com',
  '2captcha': 'https://api.2captcha.com',
  capsolver: 'https://api.capsolver.com',
}
const registerInvite = ref('')
const registerConc = ref('1')
const registerIntervalMin = ref('8')
const registerIntervalMax = ref('15')
const registerOtpSec = ref('180')

const isAdmin = computed(() => isAdminRole(role.value) || consoleBaseFromPath(route.path) === '/admin')
const keysPath = computed(() => `${consoleBaseFromPath(route.path)}/keys`)
const uploadPresets = [0, 6, 24, 72, 168]
const captchaBasePlaceholder = computed(
  () => captchaDefaultBases[captchaProvider.value] || '留空=当前平台默认端点',
)

function applyCaptchaProviderFields(provider, bag) {
  const p = provider || 'off'
  const fromBag = bag?.providers?.[p]
  if (fromBag) {
    captchaApiKey.value = fromBag.api_key || ''
    captchaApiBase.value = fromBag.api_base || ''
    return
  }
  if (p === (bag?.provider || '')) {
    captchaApiKey.value = bag?.api_key || ''
    captchaApiBase.value = bag?.api_base || ''
    return
  }
  captchaApiKey.value = ''
  captchaApiBase.value = ''
}

function onCaptchaProviderChange() {
  applyCaptchaProviderFields(captchaProvider.value, {
    provider: captchaProvider.value,
    providers: captchaProviders.value,
  })
}

function flashOk(msg) {
  error.value = ''
  okmsg.value = msg
}

function flashErr(e) {
  okmsg.value = ''
  error.value = e?.message || String(e)
}

async function load() {
  error.value = ''
  loading.value = true
  try {
    const [consoleRes, settingsRes] = await Promise.all([api.console(), api.getSettings()])
    username.value = consoleRes.username || ''
    role.value = consoleRes.role || getRole()
    credUsername.value = consoleRes.username || ''
    registrationEnabled.value = !!(consoleRes.registration_enabled ?? settingsRes.registration_enabled)
    pollAuth.value = settingsRes.poll_auth === 'at' ? 'at' : 'apikey'
    const hours = Number(settingsRes.upload_keep_hours)
    uploadKeepHours.value = Number.isFinite(hours) ? Math.max(0, Math.min(720, Math.floor(hours))) : 24
    uploadKeepDraft.value = uploadKeepHours.value
    captchaProviders.value = settingsRes.captcha?.providers || {}
    captchaProvider.value = settingsRes.captcha?.provider || 'off'
    captchaTimeout.value = Number(settingsRes.captcha?.timeout_sec) || 180
    applyCaptchaProviderFields(captchaProvider.value, settingsRes.captcha)
    registerInvite.value = settingsRes.register?.invite_code || ''
    registerConc.value = String(settingsRes.register?.concurrency || '1')
    registerIntervalMin.value = String(settingsRes.register?.interval_sec_min || '8')
    registerIntervalMax.value = String(settingsRes.register?.interval_sec_max || '15')
    registerOtpSec.value = String(settingsRes.register?.otp_timeout_sec || '180')
  } catch (e) {
    flashErr(e)
  } finally {
    loading.value = false
  }
}

async function savePollAuth(next) {
  const mode = next === 'at' ? 'at' : 'apikey'
  saving.value = true
  try {
    const res = await api.updateSettings({ poll_auth: mode })
    pollAuth.value = res.poll_auth === 'at' ? 'at' : 'apikey'
    flashOk(pollAuth.value === 'apikey' ? '已切换为 API Key 轮询' : '已切换为 Access Token 轮询')
  } catch (e) {
    flashErr(e)
  } finally {
    saving.value = false
  }
}

async function saveRegistration(next) {
  saving.value = true
  try {
    const res = await api.updateSettings({ registration_enabled: !!next })
    registrationEnabled.value = !!res.registration_enabled
    flashOk(registrationEnabled.value ? '已开启公开注册' : '已关闭公开注册')
  } catch (e) {
    flashErr(e)
  } finally {
    saving.value = false
  }
}

async function saveUploadKeep(hours) {
  const n = Math.max(0, Math.min(720, Math.floor(Number(hours))))
  if (!Number.isFinite(n)) {
    flashErr(new Error('保留时长无效'))
    return
  }
  saving.value = true
  try {
    const res = await api.updateSettings({ upload_keep_hours: n })
    uploadKeepHours.value = Number(res.upload_keep_hours ?? n)
    uploadKeepDraft.value = uploadKeepHours.value
    flashOk(
      uploadKeepHours.value === 0
        ? '已设为结束后尽快清理：上传文件 / 任务记录 / 注册记录'
        : `保留 ${uploadKeepHours.value} 小时后清理：上传文件 / 任务记录 / 注册记录`,
    )
  } catch (e) {
    flashErr(e)
  } finally {
    saving.value = false
  }
}

async function saveCaptcha() {
  if (!isAdmin.value) return
  savingCaptcha.value = true
  try {
    const res = await api.updateSettings({
      captcha: {
        provider: captchaProvider.value,
        api_key: captchaApiKey.value,
        api_base: captchaApiBase.value,
        timeout_sec: Number(captchaTimeout.value) || 180,
      },
    })
    captchaProviders.value = res?.captcha?.providers || captchaProviders.value
    applyCaptchaProviderFields(captchaProvider.value, res?.captcha)
    const name =
      captchaProvider.value === 'off'
        ? '未启用'
        : captchaProvider.value === 'yescaptcha'
          ? 'YesCaptcha'
          : captchaProvider.value === '2captcha'
            ? '2Captcha'
            : captchaProvider.value === 'capsolver'
              ? 'CapSolver'
              : captchaProvider.value
    flashOk(`打码设置已保存（${name} 独立 Key/端点）`)
    await load()
  } catch (e) {
    flashErr(e)
  } finally {
    savingCaptcha.value = false
  }
}

async function saveRegisterDefaults() {
  if (!isAdmin.value) return
  savingRegister.value = true
  try {
    let minSec = Math.max(0, Number(registerIntervalMin.value) || 0)
    let maxSec = Math.max(0, Number(registerIntervalMax.value) || 0)
    if (maxSec < minSec) maxSec = minSec
    await api.updateSettings({
      register: {
        invite_code: registerInvite.value,
        concurrency: Number(registerConc.value) || 1,
        interval_sec_min: minSec,
        interval_sec_max: maxSec,
        otp_timeout_sec: Number(registerOtpSec.value) || 180,
      },
    })
    flashOk('注册默认参数已保存')
    await load()
  } catch (e) {
    flashErr(e)
  } finally {
    savingRegister.value = false
  }
}

async function saveAdminCredentials() {
  const current = String(credCurrentPassword.value || '')
  const nextUser = String(credUsername.value || '').trim().toLowerCase()
  const nextPass = String(credPassword.value || '')
  const nextPass2 = String(credPassword2.value || '')
  if (!current) {
    flashErr(new Error('请填写当前密码以确认身份'))
    return
  }
  if (!nextUser && !nextPass) {
    flashErr(new Error('请填写新用户名或新密码'))
    return
  }
  if (nextUser && nextUser.length < 3) {
    flashErr(new Error('用户名至少 3 位'))
    return
  }
  if (nextPass) {
    if (nextPass.length < 6) {
      flashErr(new Error('新密码至少 6 位'))
      return
    }
    if (nextPass !== nextPass2) {
      flashErr(new Error('两次输入的新密码不一致'))
      return
    }
  }
  savingCreds.value = true
  try {
    const body = { current_password: current }
    if (nextUser && nextUser !== String(username.value || '').toLowerCase()) body.username = nextUser
    if (nextPass) body.password = nextPass
    if (!body.username && !body.password) {
      flashErr(new Error('没有需要更新的内容'))
      return
    }
    const res = await api.updateAdminCredentials(body)
    username.value = res.username || nextUser || username.value
    credUsername.value = username.value
    credCurrentPassword.value = ''
    credPassword.value = ''
    credPassword2.value = ''
    flashOk('管理员账号已更新，写入数据库；后续部署不会回写')
  } catch (e) {
    flashErr(e)
  } finally {
    savingCreds.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="settings-page">
    <div class="settings-toolbar">
      <div>
        <h2 class="settings-page-title">{{ isAdmin ? '系统设置' : '账户设置' }}</h2>
        <p class="settings-page-desc">
          {{
            isAdmin
              ? '按模块分开管理：账号、轮询、打码、注册与清理。'
              : '账号信息、轮询通道，以及你自己的打码/RH 注册参数（仅作用于你的注册任务）。'
          }}
        </p>
      </div>
      <div class="settings-toolbar__right">
        <StatusPill :tone="username ? 'success' : 'neutral'" :label="username ? '已登录' : '未加载'" />
        <Button size="sm" variant="outline" :disabled="loading" @click="load">刷新</Button>
      </div>
    </div>

    <p v-if="error" class="settings-banner settings-banner--err">{{ error }}</p>
    <p v-else-if="okmsg" class="settings-banner settings-banner--ok">{{ okmsg }}</p>

    <div class="settings-grid">
      <PagePanel class="settings-card">
        <header class="settings-card-head">
          <h3 class="settings-card-title">账号信息</h3>
          <p class="settings-card-desc">当前登录身份</p>
        </header>
        <div class="settings-kv">
          <div>
            <span class="settings-label">登录账号</span>
            <p class="settings-value">{{ username || '—' }}</p>
          </div>
          <div>
            <span class="settings-label">角色</span>
            <p class="settings-value">{{ role === 'admin' ? '管理员' : '普通用户' }}</p>
          </div>
        </div>
        <div v-if="!isAdmin" class="settings-actions">
          <Button size="md" variant="primary" @click="router.push(keysPath)">管理 API Key</Button>
        </div>
      </PagePanel>

      <PagePanel class="settings-card">
        <header class="settings-card-head">
          <h3 class="settings-card-title">任务进度轮询</h3>
          <p class="settings-card-desc">提交仍走账号 AT；仅查询进度可选通道</p>
        </header>
        <div class="settings-poll">
          <button
            type="button"
            class="settings-poll__btn"
            :class="{ active: pollAuth === 'apikey' }"
            :disabled="saving || loading"
            @click="savePollAuth('apikey')"
          >
            API Key（推荐）
          </button>
          <button
            type="button"
            class="settings-poll__btn"
            :class="{ active: pollAuth === 'at' }"
            :disabled="saving || loading"
            @click="savePollAuth('at')"
          >
            Access Token
          </button>
        </div>
        <p class="settings-poll__desc">
          {{
            pollAuth === 'apikey'
              ? '使用账号 consumer API Key 调 /task/openapi/status 与 outputs；缺 Key 时会自动补，失败再回退 AT。'
              : '沿用原逻辑：Bearer Access Token 查 /task/list 与 output history。'
          }}
        </p>
      </PagePanel>

      <PagePanel v-if="isAdmin" class="settings-card">
        <header class="settings-card-head">
          <h3 class="settings-card-title">管理员账号</h3>
          <p class="settings-card-desc">改完写入数据库，部署不会覆盖</p>
        </header>
        <div class="settings-cred-grid">
          <div class="space-y-1.5">
            <label class="text-sm font-medium text-foreground">当前密码（必填）</label>
            <Input
              v-model="credCurrentPassword"
              type="password"
              size="md"
              block
              autocomplete="current-password"
              placeholder="确认身份"
              :disabled="savingCreds || loading"
            />
          </div>
          <div class="space-y-1.5">
            <label class="text-sm font-medium text-foreground">新用户名（可选）</label>
            <Input
              v-model="credUsername"
              type="text"
              size="md"
              block
              autocomplete="username"
              placeholder="至少 3 位"
              :disabled="savingCreds || loading"
            />
          </div>
          <div class="space-y-1.5">
            <label class="text-sm font-medium text-foreground">新密码（可选）</label>
            <Input
              v-model="credPassword"
              type="password"
              size="md"
              block
              autocomplete="new-password"
              placeholder="至少 6 位"
              :disabled="savingCreds || loading"
            />
          </div>
          <div class="space-y-1.5">
            <label class="text-sm font-medium text-foreground">确认新密码</label>
            <Input
              v-model="credPassword2"
              type="password"
              size="md"
              block
              autocomplete="new-password"
              placeholder="再输入一次"
              :disabled="savingCreds || loading"
            />
          </div>
        </div>
        <div class="settings-actions">
          <Button size="md" variant="primary" :disabled="savingCreds || loading" @click="saveAdminCredentials">
            {{ savingCreds ? '保存中...' : '保存管理员账号' }}
          </Button>
        </div>
      </PagePanel>

      <PagePanel class="settings-card">
        <header class="settings-card-head">
          <h3 class="settings-card-title">打码平台</h3>
          <p class="settings-card-desc">
            {{
              isAdmin
                ? 'Turnstile；各平台 Key 与端点分开保存，切换不会串用（全局默认）'
                : 'Turnstile；填写你自己的 Key，仅用于你的注册任务（未填时回退管理员全局配置）'
            }}
          </p>
        </header>
        <div class="settings-cred-grid">
          <div class="space-y-1.5">
            <label class="text-sm font-medium text-foreground">平台</label>
            <select
              v-model="captchaProvider"
              class="h-10 w-full rounded-lg border border-border bg-background px-3 text-sm"
              :disabled="savingCaptcha || loading"
              @change="onCaptchaProviderChange"
            >
              <option value="off">未启用</option>
              <option value="yescaptcha">YesCaptcha</option>
              <option value="2captcha">2Captcha</option>
              <option value="capsolver">CapSolver</option>
            </select>
          </div>
          <div class="space-y-1.5">
            <label class="text-sm font-medium text-foreground">超时（秒）</label>
            <Input v-model="captchaTimeout" type="number" size="md" block :disabled="savingCaptcha || loading" />
          </div>
          <div class="space-y-1.5 settings-span-2">
            <label class="text-sm font-medium text-foreground">
              API Key
              <span class="ml-1 font-normal text-muted-foreground">（按平台分开保存）</span>
            </label>
            <Input
              v-model="captchaApiKey"
              type="password"
              size="md"
              block
              placeholder="clientKey"
              :disabled="savingCaptcha || loading || captchaProvider === 'off'"
            />
          </div>
          <div class="space-y-1.5 settings-span-2">
            <label class="text-sm font-medium text-foreground">API Base（可空=该平台默认）</label>
            <Input
              v-model="captchaApiBase"
              type="text"
              size="md"
              block
              :placeholder="captchaBasePlaceholder"
              :disabled="savingCaptcha || loading || captchaProvider === 'off'"
            />
          </div>
        </div>
        <div class="settings-actions">
          <Button size="md" variant="primary" :disabled="savingCaptcha || loading" @click="saveCaptcha">
            {{ savingCaptcha ? '保存中...' : '保存打码设置' }}
          </Button>
        </div>
      </PagePanel>

      <PagePanel class="settings-card">
        <header class="settings-card-head">
          <h3 class="settings-card-title">RH 协议注册</h3>
          <p class="settings-card-desc">
            {{
              isAdmin
                ? '仅 @icloud.com；默认邀请码与并发节奏（全局默认）'
                : '仅 @icloud.com；你的默认邀请码与并发节奏（仅作用于你的注册任务）'
            }}
          </p>
        </header>
        <div class="settings-cred-grid">
          <div class="space-y-1.5 settings-span-2">
            <label class="text-sm font-medium text-foreground">默认邀请码</label>
            <Input
              v-model="registerInvite"
              type="text"
              size="md"
              block
              placeholder="可选"
              :disabled="savingRegister || loading"
            />
          </div>
          <div class="space-y-1.5">
            <label class="text-sm font-medium text-foreground">并发</label>
            <Input v-model="registerConc" type="number" size="md" block :disabled="savingRegister || loading" />
          </div>
          <div class="space-y-1.5">
            <label class="text-sm font-medium text-foreground">间隔最小秒</label>
            <Input v-model="registerIntervalMin" type="number" size="md" block :disabled="savingRegister || loading" />
          </div>
          <div class="space-y-1.5">
            <label class="text-sm font-medium text-foreground">间隔最大秒</label>
            <Input v-model="registerIntervalMax" type="number" size="md" block :disabled="savingRegister || loading" />
          </div>
          <div class="space-y-1.5 settings-span-2">
            <p class="text-xs text-muted-foreground">每个账号成功/失败后，随机等待该秒数范围再开始下一个（避免同邮箱重发码）。</p>
          </div>
          <div class="space-y-1.5">
            <label class="text-sm font-medium text-foreground">OTP 超时秒</label>
            <Input v-model="registerOtpSec" type="number" size="md" block :disabled="savingRegister || loading" />
          </div>
        </div>
        <div class="settings-actions">
          <Button size="md" variant="primary" :disabled="savingRegister || loading" @click="saveRegisterDefaults">
            {{ savingRegister ? '保存中...' : '保存注册参数' }}
          </Button>
        </div>
      </PagePanel>

      <PagePanel v-if="isAdmin" class="settings-card">
        <header class="settings-card-head">
          <h3 class="settings-card-title">公开注册</h3>
          <p class="settings-card-desc">控制登录页是否开放本站用户注册</p>
        </header>
        <div class="settings-poll">
          <button
            type="button"
            class="settings-poll__btn"
            :class="{ active: registrationEnabled }"
            :disabled="saving || loading"
            @click="saveRegistration(true)"
          >
            开启
          </button>
          <button
            type="button"
            class="settings-poll__btn"
            :class="{ active: !registrationEnabled }"
            :disabled="saving || loading"
            @click="saveRegistration(false)"
          >
            关闭
          </button>
        </div>
      </PagePanel>

      <PagePanel v-if="isAdmin" class="settings-card">
        <header class="settings-card-head">
          <h3 class="settings-card-title">自动清理</h3>
          <p class="settings-card-desc">
            上传文件与已结束任务/注册记录的保留时长（0–720 小时）。「尽快」只清上传文件，不删任务列表。
          </p>
        </header>
        <div class="settings-poll">
          <button
            v-for="h in uploadPresets"
            :key="h"
            type="button"
            class="settings-poll__btn"
            :class="{ active: uploadKeepHours === h }"
            :disabled="saving || loading"
            @click="saveUploadKeep(h)"
          >
            {{ h === 0 ? '仅清文件' : h === 168 ? '7天' : `${h}h` }}
          </button>
        </div>
        <div class="settings-actions">
          <input
            v-model.number="uploadKeepDraft"
            type="number"
            min="0"
            max="720"
            step="1"
            class="settings-hours-input"
            :disabled="saving || loading"
          />
          <Button
            size="md"
            variant="primary"
            :disabled="saving || loading || Number(uploadKeepDraft) === uploadKeepHours"
            @click="saveUploadKeep(uploadKeepDraft)"
          >
            保存时长
          </Button>
        </div>
        <p class="settings-poll__desc">
          当前：{{
            uploadKeepHours === 0
              ? '尽快清理上传文件；任务队列记录保留'
              : `保留 ${uploadKeepHours} 小时后清理上传文件与已结束任务/注册记录`
          }}。排队/运行中的视频任务与进行中的注册不会删。服务器已临时调回 72h，避免再次清空。
        </p>
      </PagePanel>

      <PagePanel class="settings-card settings-card--tips">
        <header class="settings-card-head">
          <h3 class="settings-card-title">说明</h3>
        </header>
        <ul class="settings-tips">
          <li>普通用户：自管号池与任务；用个人 API Key 调 <code>/v1/videos</code>；可自配打码/注册参数。</li>
          <li>管理员：可查看/管理全部用户与号池，禁止代提交生视频。</li>
          <li>RH 协议注册强制 <code>@icloud.com</code>；打码与注册参数分卡保存，用户配置优先于全局。</li>
          <li>首个管理员仅首次用环境变量初始化；之后以后台修改为准。</li>
        </ul>
      </PagePanel>
    </div>
  </div>
</template>

<style scoped>
.settings-page {
  min-height: calc(100vh - 9rem);
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.settings-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem 1rem;
  width: 100%;
}

.settings-toolbar__right {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}

.settings-page-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: hsl(var(--foreground));
}

.settings-page-desc {
  margin-top: 0.3rem;
  font-size: 0.875rem;
  color: hsl(var(--muted-foreground));
}

.settings-banner {
  width: 100%;
  margin: 0;
  border-radius: 0.75rem;
  padding: 0.65rem 0.9rem;
  font-size: 0.875rem;
}

.settings-banner--ok {
  background: hsl(142 40% 96%);
  color: hsl(142 45% 28%);
}

.settings-banner--err {
  background: hsl(0 70% 97%);
  color: hsl(var(--tone-error-strong));
}

.settings-grid {
  display: grid;
  gap: 1rem;
  width: 100%;
  grid-template-columns: 1fr;
  align-items: start;
}

@media (min-width: 900px) {
  .settings-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .settings-card--tips {
    grid-column: 1 / -1;
  }
}

.settings-card {
  display: flex;
  flex-direction: column;
  gap: 0.95rem;
}

.settings-card-head {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.settings-card-title {
  font-size: 1rem;
  font-weight: 600;
  color: hsl(var(--foreground));
}

.settings-card-desc {
  margin: 0;
  font-size: 0.8rem;
  color: hsl(var(--muted-foreground));
}

.settings-kv {
  display: grid;
  gap: 0.85rem;
}

@media (min-width: 520px) {
  .settings-kv {
    grid-template-columns: 1fr 1fr;
  }
}

.settings-label {
  font-size: 0.75rem;
  font-weight: 500;
  color: hsl(var(--muted-foreground));
}

.settings-value {
  margin-top: 0.2rem;
  font-size: 0.95rem;
  color: hsl(var(--foreground));
}

.settings-poll {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.settings-poll__btn {
  height: 2.25rem;
  border-radius: 0.75rem;
  border: 1px solid hsl(var(--border));
  background: hsl(var(--background));
  padding: 0 0.9rem;
  font-size: 0.8125rem;
  font-weight: 600;
  color: hsl(var(--muted-foreground));
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease, color 0.15s ease;
}

.settings-poll__btn:hover:not(:disabled) {
  border-color: hsl(var(--foreground) / 0.25);
  color: hsl(var(--foreground));
}

.settings-poll__btn.active {
  border-color: hsl(142 45% 40%);
  background: hsl(142 40% 96%);
  color: hsl(142 45% 28%);
}

.settings-poll__btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.settings-poll__desc {
  margin: 0;
  font-size: 0.75rem;
  line-height: 1.5;
  color: hsl(var(--muted-foreground));
}

.settings-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
  align-items: center;
}

.settings-hours-input {
  width: 6.5rem;
  height: 2.25rem;
  border-radius: 0.75rem;
  border: 1px solid hsl(var(--border));
  background: hsl(var(--background));
  padding: 0 0.75rem;
  font-size: 0.875rem;
  color: hsl(var(--foreground));
}

.settings-cred-grid {
  display: grid;
  gap: 0.75rem;
}

@media (min-width: 640px) {
  .settings-cred-grid {
    grid-template-columns: 1fr 1fr;
  }

  .settings-span-2 {
    grid-column: 1 / -1;
  }
}

.settings-tips {
  display: grid;
  gap: 0.65rem;
  margin: 0;
  padding-left: 1.1rem;
  font-size: 0.875rem;
  color: hsl(var(--muted-foreground));
}

.settings-tips code {
  font-size: 0.78rem;
}
</style>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { Button } from 'nanocat-ui'
import PagePanel from '@/components/ai/PagePanel.vue'
import { api } from '@/api'

const base = computed(() => window.location.origin.replace(/\/$/, ''))
const apiKey = ref('<API_KEY>')
const copied = ref('')

onMounted(async () => {
  try {
    const res = await api.console()
    if (res?.api_key) apiKey.value = res.api_key
  } catch {
    /* keep placeholder */
  }
})

const sections = computed(() => [
  {
    id: 'downstream-flow',
    title: '下游接入（三步）',
    desc: '把下面示例直接发给对接方。他们只用系统设置里的 API Key，不需要 RunningHub 账号或 RH Key。webappId 从 RH 应用页 URL 复制。',
    examples: [
      {
        label: '① 拉取应用节点模板',
        lang: 'bash',
        code: `# 把 <webappId> 换成实际应用 ID
curl -s ${base.value}/v1/webapps/<webappId>/demo \\
  -H "Authorization: Bearer ${apiKey.value}"

# 响应里重点看：
# - node_info_list：各输入节点
# - example.body：可直接改完再提交的 JSON`,
      },
      {
        label: '② 按模板提交任务',
        lang: 'bash',
        code: `curl -s -X POST ${base.value}/v1/videos \\
  -H "Authorization: Bearer ${apiKey.value}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "webapp_id": "<webappId>",
    "node_info_list": [
      { "nodeId": "266", "fieldName": "image", "fieldType": "IMAGE", "fieldValue": "__upload__" },
      { "nodeId": "249", "fieldName": "value", "fieldValue": "你的提示词" }
    ],
    "image_base64": "<图片 base64，IMAGE 节点需要时>"
  }'

# 也可用 multipart：node_info_list=JSON字符串 + image=@文件
# IMAGE 节点 fieldValue 填 __upload__`,
      },
      {
        label: '③ 轮询结果',
        lang: 'bash',
        code: `curl -s ${base.value}/v1/videos/<返回的 id> \\
  -H "Authorization: Bearer ${apiKey.value}"
# 直到 status = succeeded / failed；成功时看 file_url`,
      },
    ],
  },
  {
    id: 'auth',
    title: '鉴权',
    desc: '控制台登录后浏览器自动带会话。下游调用与账号推送请用系统设置里的 API Key。',
    examples: [
      {
        label: '请求头（下游）',
        lang: 'http',
        code: `Authorization: Bearer ${apiKey.value}
# 或
X-API-Key: ${apiKey.value}`,
      },
    ],
  },
  {
    id: 'health',
    title: '健康检查',
    desc: '无需鉴权。',
    examples: [
      {
        label: 'curl',
        lang: 'bash',
        code: `curl -s ${base.value}/health`,
      },
      {
        label: '响应',
        lang: 'json',
        code: `{
  "ok": true,
  "service": "runninghub2api"
}`,
      },
    ],
  },
  {
    id: 'webapp-demo',
    title: '拉取 AI 应用调用示例',
    desc: '若后台已保存该 webappId 模板，直接返回本地示例（不耗号池 RH Key）；未保存时再用 RH apiCallDemo / 公开 detail。加 ?refresh=1 可强制刷新。下游只需我们的 API Key。',
    examples: [
      {
        label: 'curl',
        lang: 'bash',
        code: `curl -s ${base.value}/v1/webapps/2099427470494662657/demo \\
  -H "Authorization: Bearer ${apiKey.value}"`,
      },
      {
        label: '响应要点',
        lang: 'json',
        code: `{
  "webapp_id": "2099427470494662657",
  "name": "应用名",
  "source": "apiCallDemo",
  "node_info_list": [ { "nodeId": "266", "fieldName": "image", "fieldType": "IMAGE", "...": "..." } ],
  "example": { "path": "/v1/videos", "body": { "webapp_id": "...", "node_info_list": [] } }
}`,
      },
    ],
  },
  {
    id: 'create-video',
    title: '创建视频任务',
    desc: '兼容旧版 prompt+image；也支持通用 webapp_id + node_info_list（先调 /demo 拿节点）。有空闲号时同步提交并返回 rh_task_id。',
    examples: [
      {
        label: 'curl · 旧版 multipart（默认应用）',
        lang: 'bash',
        code: `curl -s -X POST ${base.value}/v1/videos \\
  -H "Authorization: Bearer ${apiKey.value}" \\
  -F "prompt=SC1: soft morning light, slow push-in, cinematic" \\
  -F "image=@./demo.png" \\
  -F "duration=8" \\
  -F "aspect_ratio=3:4 (Portrait Standard)"`,
      },
      {
        label: 'curl · 通用 JSON（任意 AI 应用）',
        lang: 'bash',
        code: `curl -s -X POST ${base.value}/v1/videos \\
  -H "Authorization: Bearer ${apiKey.value}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "webapp_id": "2065271095023980545",
    "node_info_list": [
      { "nodeId": "75", "fieldName": "image", "fieldType": "IMAGE", "fieldValue": "__upload__" },
      { "nodeId": "6", "fieldName": "text", "fieldValue": "正向提示词" },
      { "nodeId": "7", "fieldName": "text", "fieldValue": "反向提示词" },
      { "nodeId": "90", "fieldName": "value", "fieldValue": "5" }
    ],
    "image_base64": "<首帧图 base64>"
  }'`,
      },
      {
        label: '成功响应（已拿到上游 ID）',
        lang: 'json',
        code: `{
  "id": "vid_c1cabb93e76f",
  "status": "running",
  "rh_task_id": "2118246629706653697",
  "webapp_id": "2099427470494662657",
  "account_id": 2,
  "account_email": "user@example.com"
}`,
      },
      {
        label: '排队响应（暂无空闲账号）',
        lang: 'json',
        code: `{
  "id": "vid_3e5485b098f5",
  "status": "queued",
  "rh_task_id": "",
  "message": "queued: no available account yet; poll GET /v1/videos/{id} for rh_task_id"
}`,
      },
    ],
  },
  {
    id: 'get-video',
    title: '查询任务',
    desc: '轮询直到 succeeded / failed；进度字段 progress_text、rh_status、queue_index/queue_count 会持续更新。',
    examples: [
      {
        label: 'curl',
        lang: 'bash',
        code: `curl -s ${base.value}/v1/videos/vid_c1cabb93e76f \\
  -H "Authorization: Bearer ${apiKey.value}"`,
      },
      {
        label: '响应示例',
        lang: 'json',
        code: `{
  "id": "vid_c1cabb93e76f",
  "status": "running",
  "rh_task_id": "2118246629706653697",
  "rh_status": "QUEUED",
  "progress_text": "排队中 12/276",
  "queue_index": 12,
  "queue_count": 276,
  "account_email": "user@example.com",
  "file_url": "",
  "created_at": "2026-11-05T07:40:00Z",
  "updated_at": "2026-11-05T07:41:00Z"
}`,
      },
      {
        label: '完成时',
        lang: 'json',
        code: `{
  "id": "vid_c1cabb93e76f",
  "status": "succeeded",
  "rh_task_id": "2118246629706653697",
  "progress_text": "完成",
  "file_url": "https://.../output/video/xxx.mp4",
  "preview_url": "https://.../preview.jpg"
}`,
      },
    ],
  },
  {
    id: 'list-cancel',
    title: '列表与取消',
    examples: [
      {
        label: '列出任务',
        lang: 'bash',
        code: `curl -s ${base.value}/v1/videos \\
  -H "Authorization: Bearer ${apiKey.value}"`,
      },
      {
        label: '取消任务',
        lang: 'bash',
        code: `curl -s -X POST ${base.value}/v1/videos/vid_c1cabb93e76f/cancel \\
  -H "Authorization: Bearer ${apiKey.value}"`,
      },
    ],
  },
  {
    id: 'js-poll',
    title: 'JS 完整示例（创建 + 轮询）',
    examples: [
      {
        label: 'fetch',
        lang: 'javascript',
        code: `const BASE = '${base.value}'
const API_KEY = '${apiKey.value}'

async function createAndWait(file, prompt) {
  const fd = new FormData()
  fd.append('prompt', prompt)
  fd.append('image', file)
  fd.append('duration', '8')

  const created = await fetch(BASE + '/v1/videos', {
    method: 'POST',
    headers: { Authorization: 'Bearer ' + API_KEY },
    body: fd,
  }).then((r) => r.json())

  console.log('local:', created.id, 'rh:', created.rh_task_id)

  for (;;) {
    await new Promise((r) => setTimeout(r, 5000))
    const task = await fetch(BASE + '/v1/videos/' + created.id, {
      headers: { Authorization: 'Bearer ' + API_KEY },
    }).then((r) => r.json())

    console.log(task.status, task.progress_text, task.rh_task_id)
    if (['succeeded', 'failed', 'cancelled'].includes(task.status)) {
      return task
    }
  }
}`,
      },
    ],
  },
  {
    id: 'push-accounts',
    title: '推送账号（同步）',
    desc: '使用系统设置中的 API Key。支持单条 / 批量 upsert。',
    examples: [
      {
        label: '单条推送',
        lang: 'bash',
        code: `curl -s -X POST ${base.value}/api/accounts/push \\
  -H "Authorization: Bearer ${apiKey.value}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "account": {
      "email": "user@example.com",
      "password": "plain-password",
      "api_key": "optional-32hex-consumer-key",
      "remain_coins": "1100",
      "source": "sync",
      "external_id": "1",
      "status": "active"
    }
  }'`,
      },
      {
        label: '批量推送',
        lang: 'bash',
        code: `curl -s -X POST ${base.value}/api/accounts/push \\
  -H "Authorization: Bearer ${apiKey.value}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "source": "sync",
    "accounts": [
      { "email": "a@x.com", "password": "p1", "external_id": "11" },
      { "email": "b@x.com", "password": "p2", "external_id": "12" }
    ]
  }'`,
      },
      {
        label: '响应',
        lang: 'json',
        code: `{
  "imported": 1,
  "failed": 0,
  "data": [{ "id": 4, "email": "user@example.com", "dispatchable": true, "...": "..." }],
  "errors": []
}`,
      },
      {
        label: '等价路径',
        lang: 'text',
        code: `POST /api/accounts/push
POST /admin/sync/accounts
POST /admin/accounts/import`,
      },
    ],
  },
  {
    id: 'admin',
    title: '管理接口摘要',
    examples: [
      {
        label: '常用 Admin API',
        lang: 'bash',
        code: `# 号池列表（下游可用 API Key；控制台用登录会话）
curl -s ${base.value}/admin/accounts -H "Authorization: Bearer ${apiKey.value}"

# 单独添加账号
curl -s -X POST ${base.value}/admin/accounts \\
  -H "Authorization: Bearer ${apiKey.value}" \\
  -H "Content-Type: application/json" \\
  -d '{"email":"a@x.com","password":"pass","source":"manual"}'

# 刷新积分/在途（会自动补 api_key）
curl -s -X POST ${base.value}/admin/accounts/2/refresh \\
  -H "Authorization: Bearer ${apiKey.value}"

# 启用 / 禁用
curl -s -X POST ${base.value}/admin/accounts/2/enable -H "Authorization: Bearer ${apiKey.value}"
curl -s -X POST ${base.value}/admin/accounts/2/disable -H "Authorization: Bearer ${apiKey.value}"

# 账号详情（含 key / AT / RT 等）
curl -s ${base.value}/admin/accounts/2 -H "Authorization: Bearer ${apiKey.value}"

# 删除账号（硬删除；占用中返回 409）
curl -s -X DELETE ${base.value}/admin/accounts/2 -H "Authorization: Bearer ${apiKey.value}"

# 任务列表
curl -s ${base.value}/admin/tasks -H "Authorization: Bearer ${apiKey.value}"`,
      },
    ],
  },
])

async function copy(text, key) {
  try {
    await navigator.clipboard.writeText(text)
    copied.value = key
    setTimeout(() => {
      if (copied.value === key) copied.value = ''
    }, 1500)
  } catch {
    /* ignore */
  }
}
</script>

<template>
  <div class="relative space-y-5">
    <PagePanel class="space-y-5">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p class="text-sm text-muted-foreground">
            Base：<code class="font-mono text-xs">{{ base }}</code>
          </p>
          <p class="mt-1 text-xs text-muted-foreground">
            示例中的 Key 取自当前浏览器已保存的密钥（未保存则用默认占位）。
          </p>
        </div>
        <div class="flex flex-wrap gap-2 text-xs">
          <a
            v-for="s in sections"
            :key="s.id"
            :href="`#${s.id}`"
            class="rounded-full border border-border px-2.5 py-1 text-muted-foreground hover:text-foreground"
          >{{ s.title }}</a>
        </div>
      </div>

      <section
        v-for="sec in sections"
        :id="sec.id"
        :key="sec.id"
        class="scroll-mt-6 rounded-2xl border border-border/80 bg-card p-4 md:p-5"
      >
        <h2 class="text-sm font-semibold text-foreground">{{ sec.title }}</h2>
        <p v-if="sec.desc" class="mt-1 text-xs text-muted-foreground">{{ sec.desc }}</p>

        <div v-for="(ex, idx) in sec.examples" :key="`${sec.id}-${idx}`" class="mt-4">
          <div class="mb-1.5 flex items-center justify-between gap-2">
            <p class="text-xs font-medium text-muted-foreground">{{ ex.label }}</p>
            <Button
              size="xs"
              variant="outline"
              @click="copy(ex.code, `${sec.id}-${idx}`)"
            >
              {{ copied === `${sec.id}-${idx}` ? '已复制' : '复制' }}
            </Button>
          </div>
          <pre
            class="overflow-x-auto rounded-xl border border-border/70 bg-muted/30 p-3 font-mono text-[11px] leading-relaxed text-foreground"
          ><code>{{ ex.code }}</code></pre>
        </div>
      </section>
    </PagePanel>
  </div>
</template>

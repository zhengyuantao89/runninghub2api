<script setup>
import { computed, onMounted, ref } from 'vue'
import { Button } from 'nanocat-ui'
import PagePanel from '@/components/ai/PagePanel.vue'
import { api } from '@/api'

const base = computed(() => window.location.origin.replace(/\/$/, ''))
const apiKey = ref('<API_KEY>')
const copied = ref('')
const activeId = ref('openai-models')

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
    id: 'openai-models',
    tab: '模型 ID',
    group: 'OpenAI',
    title: 'OpenAI 兼容 · 模型 ID',
    desc: '官方免费生图/生视频对外只传 model，不要传 submit_mode。先 GET /v1/models。',
    examples: [
      {
        label: '列出模型',
        lang: 'bash',
        code: `curl -s ${base.value}/v1/models \\
  -H "Authorization: Bearer ${apiKey.value}"`,
      },
      {
        label: '当前可用 model',
        lang: 'text',
        code: `生图（默认同步）
  nano-banana2-low-cost          Nano Banana2-Low-Cost
  gpt-image-2-low-cost           GPT Image2.0-Low-Cost

生视频（默认异步）
  minimax-h3-rh-enhanced              自由创作（文生/图生）
  minimax-h3-rh-enhanced-multimodal   多模态参考（最多 9 张）`,
      },
    ],
  },
  {
    id: 'openai-images',
    tab: '生图',
    group: 'OpenAI',
    title: 'OpenAI 兼容 · 生图',
    desc: 'POST /v1/images/generations。默认同步等到出图；async=true 则返回任务 id，再 GET /v1/images/{id}。',
    examples: [
      {
        label: '同步生图',
        lang: 'bash',
        code: `curl -s -X POST ${base.value}/v1/images/generations \\
  -H "Authorization: Bearer ${apiKey.value}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "nano-banana2-low-cost",
    "prompt": "一只在窗边睡觉的猫，柔和光线",
    "size": "9:16",
    "quality": "1k"
  }'
# → data[0].url`,
      },
      {
        label: '异步生图',
        lang: 'bash',
        code: `curl -s -X POST "${base.value}/v1/images/generations?async=1" \\
  -H "Authorization: Bearer ${apiKey.value}" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"gpt-image-2-low-cost","prompt":"水彩小猫"}'

curl -s ${base.value}/v1/images/<id> \\
  -H "Authorization: Bearer ${apiKey.value}"`,
      },
    ],
  },
  {
    id: 'openai-videos',
    tab: '生视频',
    group: 'OpenAI',
    title: 'OpenAI 兼容 · 生视频',
    desc: 'POST /v1/videos + model。默认异步；sync=true 可阻塞到成片。应用模板仍用 webapp_id。',
    examples: [
      {
        label: '异步生视频',
        lang: 'bash',
        code: `curl -s -X POST ${base.value}/v1/videos \\
  -H "Authorization: Bearer ${apiKey.value}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "minimax-h3-rh-enhanced",
    "prompt": "人物动起来",
    "seconds": "4",
    "size": "3:2 (Photo)"
  }'

curl -s ${base.value}/v1/videos/<id> \\
  -H "Authorization: Bearer ${apiKey.value}"
# 直到 status=succeeded，取 file_url`,
      },
      {
        label: '多模态参考',
        lang: 'bash',
        code: `curl -s -X POST ${base.value}/v1/videos \\
  -H "Authorization: Bearer ${apiKey.value}" \\
  -F model=minimax-h3-rh-enhanced-multimodal \\
  -F prompt='人物动起来' \\
  -F seconds=4 \\
  -F image=@./a.png \\
  -F image_2=@./b.png`,
      },
    ],
  },
  {
    id: 'downstream-flow',
    tab: '应用模板',
    group: '模板',
    title: '应用模板接入（三步）',
    desc: '每个 RH 应用节点不同，不要写死请求体。先拉 demo，再改 example.body 提交。',
    examples: [
      {
        label: '① 拉取本应用节点模板',
        lang: 'bash',
        code: `# webappId 从 RH 应用页 URL 复制，例如 ai-detail/2054244556132102145
curl -s ${base.value}/v1/webapps/<webappId>/demo \\
  -H "Authorization: Bearer ${apiKey.value}"

# 响应里重点看：
# - node_info_list：本应用输入节点（以本次为准）
# - example.body：拷出来改 fieldValue 再提交
# - has_access_password：true 表示后台已存访问密码，提交时网关自动注入`,
      },
      {
        label: '② 按 demo 改 body 后提交',
        lang: 'bash',
        code: `# 不要照抄固定 nodeId；把上一步 example.body 存成 payload.json 再改
curl -s -X POST ${base.value}/v1/videos \\
  -H "Authorization: Bearer ${apiKey.value}" \\
  -H "Content-Type: application/json" \\
  -d @payload.json

# payload.json 形态（节点必须来自该应用的 demo）：
# {
#   "webapp_id": "<webappId>",
#   "node_info_list": [ /* 来自 demo，只改 fieldValue */ ],
#   "image_base64": "<IMAGE 节点需要时>"
# }
# multipart 也行：node_info_list=JSON字符串 + image=@文件
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
    tab: '鉴权',
    group: '基础',
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
    tab: '健康检查',
    group: '基础',
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
    tab: '拉 demo',
    group: '模板',
    title: '拉取 AI 应用调用示例',
    desc: '先拉再改：响应里的 node_info_list / example.body 只对当前 webappId 有效。',
    examples: [
      {
        label: 'curl',
        lang: 'bash',
        code: `curl -s ${base.value}/v1/webapps/<webappId>/demo \\
  -H "Authorization: Bearer ${apiKey.value}"`,
      },
      {
        label: '响应要点',
        lang: 'json',
        code: `{
  "webapp_id": "<webappId>",
  "name": "应用名",
  "source": "apiCallDemo | saved_template",
  "has_access_password": false,
  "node_info_list": [ { "nodeId": "...", "fieldName": "...", "fieldType": "IMAGE|LIST|..." } ],
  "example": { "path": "/v1/videos", "body": { "webapp_id": "...", "node_info_list": [] } }
}`,
      },
    ],
  },
  {
    id: 'create-video',
    tab: '创建任务',
    group: '模板',
    title: '创建任务',
    desc: '推荐：用 /demo 返回的 example.body 改完再 POST。旧版 prompt+image 仅兼容默认应用。',
    examples: [
      {
        label: 'curl · 按模板 JSON（推荐）',
        lang: 'bash',
        code: `# 节点必须来自该 webappId 的 /demo，勿复用其它应用的 nodeId
curl -s -X POST ${base.value}/v1/videos \\
  -H "Authorization: Bearer ${apiKey.value}" \\
  -H "Content-Type: application/json" \\
  -d @payload-from-demo.json`,
      },
      {
        label: 'curl · 旧版 multipart（仅默认应用）',
        lang: 'bash',
        code: `curl -s -X POST ${base.value}/v1/videos \\
  -H "Authorization: Bearer ${apiKey.value}" \\
  -F "prompt=SC1: soft morning light, slow push-in, cinematic" \\
  -F "image=@./demo.png" \\
  -F "duration=8" \\
  -F "aspect_ratio=3:4 (Portrait Standard)"`,
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
    tab: '查询任务',
    group: '模板',
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
    tab: '列表/取消',
    group: '模板',
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
    tab: 'JS 示例',
    group: '模板',
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
    tab: '推送账号',
    group: '管理',
    title: '推送账号（同步）',
    desc: '用个人 API Key（或管理员 Key）即可。普通用户推入自己的号池；管理员可指定 user_id。',
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
    tab: '管理接口',
    group: '管理',
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

const tabGroups = computed(() => {
  const order = ['OpenAI', '模板', '基础', '管理']
  const map = new Map()
  for (const s of sections.value) {
    const g = s.group || '其他'
    if (!map.has(g)) map.set(g, [])
    map.get(g).push(s)
  }
  return order.filter((g) => map.has(g)).map((g) => ({ name: g, items: map.get(g) }))
})

const activeSection = computed(
  () => sections.value.find((s) => s.id === activeId.value) || sections.value[0],
)

function selectTab(id) {
  activeId.value = id
  copied.value = ''
}

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
  <div class="api-docs">
    <PagePanel class="api-docs__panel">
      <header class="api-docs__head">
        <div>
          <p class="api-docs__base">
            Base：<code>{{ base }}</code>
          </p>
          <p class="api-docs__hint">示例 Key 取自当前浏览器已保存的密钥。</p>
        </div>
      </header>

      <div class="api-docs__layout">
        <nav class="api-docs__nav" aria-label="API 分类">
          <div v-for="g in tabGroups" :key="g.name" class="api-docs__group">
            <p class="api-docs__group-label">{{ g.name }}</p>
            <div class="api-docs__tabs" role="tablist">
              <button
                v-for="s in g.items"
                :key="s.id"
                type="button"
                role="tab"
                class="api-docs__tab"
                :class="{ 'is-active': activeId === s.id }"
                :aria-selected="activeId === s.id"
                @click="selectTab(s.id)"
              >
                {{ s.tab || s.title }}
              </button>
            </div>
          </div>
        </nav>

        <section
          v-if="activeSection"
          class="api-docs__card"
          role="tabpanel"
          :aria-label="activeSection.title"
        >
          <h2 class="api-docs__title">{{ activeSection.title }}</h2>
          <p v-if="activeSection.desc" class="api-docs__desc">{{ activeSection.desc }}</p>

          <div
            v-for="(ex, idx) in activeSection.examples"
            :key="`${activeSection.id}-${idx}`"
            class="api-docs__ex"
          >
            <div class="api-docs__ex-head">
              <p class="api-docs__ex-label">{{ ex.label }}</p>
              <Button
                size="xs"
                variant="outline"
                @click="copy(ex.code, `${activeSection.id}-${idx}`)"
              >
                {{ copied === `${activeSection.id}-${idx}` ? '已复制' : '复制' }}
              </Button>
            </div>
            <pre class="api-docs__code"><code>{{ ex.code }}</code></pre>
          </div>
        </section>
      </div>
    </PagePanel>
  </div>
</template>

<style scoped>
.api-docs__panel {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.api-docs__head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}

.api-docs__base {
  font-size: 0.875rem;
  color: hsl(var(--muted-foreground));
}

.api-docs__base code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.75rem;
}

.api-docs__hint {
  margin-top: 0.25rem;
  font-size: 0.75rem;
  color: hsl(var(--muted-foreground));
}

.api-docs__layout {
  display: grid;
  gap: 1rem;
}

@media (min-width: 900px) {
  .api-docs__layout {
    grid-template-columns: 11.5rem minmax(0, 1fr);
    align-items: start;
  }
}

.api-docs__nav {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
  padding: 0.75rem;
  border-radius: 1rem;
  border: 1px solid hsl(var(--border) / 0.8);
  background: hsl(var(--muted) / 0.25);
}

.api-docs__group-label {
  margin: 0 0 0.35rem;
  font-size: 0.65rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: hsl(var(--muted-foreground));
}

.api-docs__tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

@media (min-width: 900px) {
  .api-docs__tabs {
    flex-direction: column;
    flex-wrap: nowrap;
  }
}

.api-docs__tab {
  appearance: none;
  border: 1px solid transparent;
  background: transparent;
  color: hsl(var(--muted-foreground));
  border-radius: 0.65rem;
  padding: 0.4rem 0.7rem;
  font-size: 0.8rem;
  text-align: left;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.api-docs__tab:hover {
  color: hsl(var(--foreground));
  background: hsl(var(--background) / 0.7);
}

.api-docs__tab.is-active {
  color: hsl(var(--foreground));
  background: hsl(var(--background));
  border-color: hsl(var(--border));
  font-weight: 600;
  box-shadow: 0 1px 2px hsl(var(--foreground) / 0.04);
}

.api-docs__card {
  border-radius: 1rem;
  border: 1px solid hsl(var(--border) / 0.8);
  background: hsl(var(--card));
  padding: 1rem 1.1rem 1.15rem;
  min-height: 16rem;
}

@media (min-width: 768px) {
  .api-docs__card {
    padding: 1.15rem 1.25rem 1.35rem;
  }
}

.api-docs__title {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 600;
  color: hsl(var(--foreground));
}

.api-docs__desc {
  margin: 0.4rem 0 0;
  font-size: 0.75rem;
  line-height: 1.5;
  color: hsl(var(--muted-foreground));
}

.api-docs__ex {
  margin-top: 1rem;
}

.api-docs__ex-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-bottom: 0.4rem;
}

.api-docs__ex-label {
  margin: 0;
  font-size: 0.75rem;
  font-weight: 500;
  color: hsl(var(--muted-foreground));
}

.api-docs__code {
  margin: 0;
  overflow-x: auto;
  border-radius: 0.75rem;
  border: 1px solid hsl(var(--border) / 0.7);
  background: hsl(var(--muted) / 0.3);
  padding: 0.75rem;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  line-height: 1.55;
  color: hsl(var(--foreground));
  white-space: pre;
}
</style>

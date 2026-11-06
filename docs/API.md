# runninghub2api 调用说明

控制台侧栏「API 说明」可查看可复制示例（会带上当前 API Key）。

## 鉴权

| 场景 | Header |
|------|--------|
| 控制台 | 登录后 Session（`Authorization: Bearer <session>`） |
| 下游 / 推送账号 | `Authorization: Bearer <RH2API_API_KEY>` |

也可用 `X-API-Key`。

## 核心流程（任意 AI 应用）

每个 RH 应用的节点不同，**不要写死请求体**。标准三步：

1. 用 `webapp_id` 拉 demo，拿到该应用的 `node_info_list` / `example.body`
2. **按该模板改** `fieldValue`（图、提示词、下拉选项等）后 `POST /v1/videos`
3. 轮询 `GET /v1/videos/{id}` 直到成功，取 `file_url`

下游只需本网关 API Key，不需要 RunningHub 账号。  
加密访问应用：在控制台「应用模板」拉取后填访问密码并保存；**下游提交不必再传密码**，网关会按已存模板自动注入。

### 1. 拉取本应用调用模板

`GET /v1/webapps/{webapp_id}/demo`

```bash
# webapp_id 从 RH 应用页 URL 复制，例如：
# https://www.runninghub.ai/ai-detail/2054244556132102145
curl -s http://127.0.0.1:8088/v1/webapps/2054244556132102145/demo \
  -H "Authorization: Bearer $RH2API_API_KEY"
```

- 后台已保存该 `webapp_id` 时直接返回本地模板（`cached=true`，不耗号池 RH Key）
- 未保存时用号池 RH Key 调官方 `apiCallDemo`，否则回退公开 detail
- `?refresh=1` 强制从 RH 刷新
- 响应重点：`node_info_list`、`example.body`；若 `has_access_password=true` 表示模板已存访问密码

IMAGE 节点示例值一般为 `__upload__`，提交时配合 `image` / `image_base64` / `images_base64`。

### 2. 按模板提交（改 example.body）

`POST /v1/videos`（JSON 或 multipart）

先把上一步 `example.body` 拷出来，**只改当前应用需要的字段**，再提交：

```bash
curl -s -X POST http://127.0.0.1:8088/v1/videos \
  -H "Authorization: Bearer $RH2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d @payload.json
```

`payload.json` 形态（节点以 demo 为准，下面仅为示意）：

```json
{
  "webapp_id": "2054244556132102145",
  "node_info_list": [
    { "nodeId": "<来自 demo>", "fieldName": "image", "fieldType": "IMAGE", "fieldValue": "__upload__" },
    { "nodeId": "<来自 demo>", "fieldName": "text", "fieldValue": "你的提示词" }
  ],
  "image_base64": "<需要上传图时>"
}
```

多图可用 `images_base64: { "<nodeId>": "<base64>" }`，或 multipart 字段 `image_<nodeId>`。

可选：请求里带 `access_password` 可覆盖模板密码；一般不需要——模板里存好即可。

有空闲账号时同步返回上游 ID：

```json
{
  "id": "vid_xxx",
  "status": "running",
  "rh_task_id": "2118246629706653697",
  "webapp_id": "2054244556132102145",
  "account_email": "user@example.com"
}
```

## 旧版默认应用（兼容）

仍支持 `prompt` + `image`（走内置默认 webapp 节点映射）。新对接请用上面的「按模板提交」。

```bash
curl -s -X POST http://127.0.0.1:8088/v1/videos \
  -H "Authorization: Bearer $RH2API_API_KEY" \
  -F "prompt=SC1: soft light, slow push-in" \
  -F "image=@./demo.png" \
  -F "duration=8"
```

## 查询 / 列表 / 取消

```bash
curl -s http://127.0.0.1:8088/v1/videos/vid_xxx -H "Authorization: Bearer $RH2API_API_KEY"
curl -s http://127.0.0.1:8088/v1/videos -H "Authorization: Bearer $RH2API_API_KEY"
curl -s -X POST http://127.0.0.1:8088/v1/videos/vid_xxx/cancel -H "Authorization: Bearer $RH2API_API_KEY"
```

轮询字段：`status`、`rh_task_id`、`progress_text`、`file_url`。

## 推送账号（同步）

`POST /api/accounts/push`（等价：`/admin/sync/accounts`）

```bash
curl -s -X POST http://127.0.0.1:8088/api/accounts/push \
  -H "Authorization: Bearer $RH2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "account": {
      "email": "user@example.com",
      "password": "plain",
      "source": "sync",
      "external_id": "1",
      "status": "active",
      "max_concurrency": 5
    }
  }'
```

批量：`{ "accounts": [ ... ] }`。`max_concurrency` 可选（1..64，默认 1）；未传则更新时保留原值。

设置并发：

```bash
curl -s -X POST http://127.0.0.1:8088/admin/accounts/2/max-concurrency \
  -H "Authorization: Bearer $RH2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"max_concurrency":5}'
```

## 管理（需登录 Session 或 API Key）

```bash
curl -s http://127.0.0.1:8088/admin/accounts -H "Authorization: Bearer $RH2API_API_KEY"
curl -s -X POST http://127.0.0.1:8088/admin/accounts \
  -H "Authorization: Bearer $RH2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"email":"a@x.com","password":"pass","max_concurrency":1}'
curl -s -X POST http://127.0.0.1:8088/admin/accounts/2/refresh \
  -H "Authorization: Bearer $RH2API_API_KEY"
```

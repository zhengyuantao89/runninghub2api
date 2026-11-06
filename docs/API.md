# runninghub2api 调用说明

控制台侧栏「API 说明」可查看可复制示例（会带上当前 API Key）。

## 鉴权

| 场景 | Header |
|------|--------|
| 控制台 | 登录后 Session（`Authorization: Bearer <session>`） |
| 下游 / 推送账号 | `Authorization: Bearer <RH2API_API_KEY>` |

也可用 `X-API-Key`。

## 任意 AI 应用（推荐）

下游只需本网关的 `RH2API_API_KEY`，不需要 RunningHub 账号。  
控制台「应用模板」页可用号池账号的 RH Key 预览官方 demo；「API 说明」页可复制发给对接方。

### 1. 拉取调用示例

`GET /v1/webapps/{webapp_id}/demo`

```bash
curl -s http://127.0.0.1:8088/v1/webapps/2099427470494662657/demo \
  -H "Authorization: Bearer $RH2API_API_KEY"
```

若后台模板库已保存该 `webapp_id`，直接返回本地示例（`cached=true`，不耗号池 RH Key）；未保存时再用号池 RH `api_key` 调官方 `apiCallDemo`，否则回退公开 detail。加 `?refresh=1` 可强制从 RH 刷新。  
响应含 `node_info_list` 和可直接改的 `example.body`。IMAGE 节点示例值一般为 `__upload__`，提交时配合 `image` / `image_base64`。

### 2. 按节点提交

`POST /v1/videos`（JSON 或 multipart）

```bash
curl -s -X POST http://127.0.0.1:8088/v1/videos \
  -H "Authorization: Bearer $RH2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "webapp_id": "2065271095023980545",
    "node_info_list": [
      { "nodeId": "75", "fieldName": "image", "fieldType": "IMAGE", "fieldValue": "__upload__" },
      { "nodeId": "6", "fieldName": "text", "fieldValue": "正向提示词" },
      { "nodeId": "7", "fieldName": "text", "fieldValue": "反向提示词" },
      { "nodeId": "90", "fieldName": "value", "fieldValue": "5" }
    ],
    "image_base64": "<首帧图 base64>"
  }'
```

多图可用 `images_base64: { "<nodeId>": "<base64>" }`，或 multipart 字段 `image_<nodeId>`。

## 旧版默认应用（兼容）

仍支持 `prompt` + `image`（走内置默认 webapp 节点映射）：

```bash
curl -s -X POST http://127.0.0.1:8088/v1/videos \
  -H "Authorization: Bearer $RH2API_API_KEY" \
  -F "prompt=SC1: soft light, slow push-in" \
  -F "image=@./demo.png" \
  -F "duration=8"
```

有空闲账号时同步返回上游 ID：

```json
{
  "id": "vid_xxx",
  "status": "running",
  "rh_task_id": "2118246629706653697",
  "webapp_id": "2099427470494662657",
  "account_email": "user@example.com"
}
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
      "status": "active"
    }
  }'
```

批量：`{ "accounts": [ ... ] }`。

## 管理（需登录 Session 或 API Key）

```bash
curl -s http://127.0.0.1:8088/admin/accounts -H "Authorization: Bearer $RH2API_API_KEY"
curl -s -X POST http://127.0.0.1:8088/admin/accounts \
  -H "Authorization: Bearer $RH2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"email":"a@x.com","password":"pass"}'
curl -s -X POST http://127.0.0.1:8088/admin/accounts/2/refresh \
  -H "Authorization: Bearer $RH2API_API_KEY"
```

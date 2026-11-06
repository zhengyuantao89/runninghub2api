# runninghub2api

独立 Go 网关：号池 + 异步生视频任务，走 RunningHub **网页 WebApp** 链路，最终返回 `file_url`。

账号可通过控制台添加，或由外部系统调用 `/admin/sync/accounts`（`/api/accounts/push`）同步进来。

## 快速启动

```bash
cd runninghub2api
cp .env.example .env
# 编辑 RH2API_ADMIN_USER / RH2API_ADMIN_PASSWORD / RH2API_API_KEY

# 后端
go mod tidy
go run ./cmd/runninghub2api

# 前端（另开终端；开发热更新）
cd web
npm install
npm run dev
# → http://127.0.0.1:5178  （vite 代理到 :8088）

# 或构建后由 Go 同端口托管
cd web && npm run build
# 打开 http://127.0.0.1:8088/  用账号密码登录
```

健康检查：`GET http://127.0.0.1:8088/health`

管理台：账号密码登录后，工作室/号池直接可用。API Key 在「系统设置」可复制，给下游调用和账号推送。

## 导入账号

```bash
curl -X POST http://127.0.0.1:8088/admin/sync/accounts \
  -H "Authorization: Bearer $RH2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"accounts":[{"email":"a@b.com","password":"plain","source":"sync","status":"active"}]}'
```

## 下游生视频 / 任意 AI 应用

```bash
# 1) 按 webappId 拉节点示例（几乎所有 AI 应用通用）
curl http://127.0.0.1:8088/v1/webapps/<webappId>/demo \
  -H "Authorization: Bearer $RH2API_API_KEY"

# 2a) 通用提交：webapp_id + node_info_list（IMAGE 节点 fieldValue=__upload__）
curl -X POST http://127.0.0.1:8088/v1/videos \
  -H "Authorization: Bearer $RH2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"webapp_id":"...","node_info_list":[...],"image_base64":"..."}'

# 2b) 旧版默认应用：prompt + image
curl -X POST http://127.0.0.1:8088/v1/videos \
  -H "Authorization: Bearer $RH2API_API_KEY" \
  -F prompt='动作描述' \
  -F image=@./ref.png

# 轮询
curl http://127.0.0.1:8088/v1/videos/vid_xxx \
  -H "Authorization: Bearer $RH2API_API_KEY"
```

成功时响应含 `file_url`（CDN 直链）。免费号成片可能含水印；会员号同步进来后同接口无水印。详见 `docs/API.md`。

## 管理

- `GET /admin/accounts`
- `GET /admin/tasks`
- `POST /admin/accounts/import`（同 sync）

## 环境变量

见 `.env.example`。启动时会读取同目录 `.env`（不覆盖已有环境变量）。

## 生产部署

### 方式 A：systemd 二进制（推荐，无 Docker）

```bash
# 本机构建
cd web && npm ci && npm run build && cd ..
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o runninghub2api ./cmd/runninghub2api

# 上传到服务器 /opt/runninghub2api 后：
# - runninghub2api
# - web/dist/
# - .env
# - deploy/runninghub2api.service + deploy/install.sh
bash deploy/install.sh
```

默认监听 `:8088`。对外建议 Nginx/Caddy 反代并开 HTTPS。

### 方式 B：Docker Compose

```bash
cp .env.example .env   # 必改密钥
docker compose up -d --build
```

数据卷：`rh2api_data` → 容器 `/app/data`。

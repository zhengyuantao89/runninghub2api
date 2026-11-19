# RH Protocol 注册 + Inbox 邮箱池

日期：2026-11-13  
状态：已确认

## 目标

在 runninghub2api 内落地：

1. Inbox 邮箱池：导入 `邮箱----取件链接`，注册时从此池取号并取 OTP  
2. RH protocol 注册（Go 原生简化版）：egress + 指纹头 → Turnstile 打码 → 发码 → OTP → register → pwdLogin → 补 API Key → 写入号池  

强制：**仅接受 `@icloud.com`**（不含 me.com / mac.com；导入与注册均过滤）。

## 非目标（本阶段）

- Cloak / curl_cffi / browser 模式  
- EmailPlus 在线领号  
- 外部系统自动推送（注册成功直接入库本号池）

## 数据

### mail_inboxes

- email（唯一）  
- pickup_url  
- api_base / api_key（从链接解析，可空）  
- status: unused | leased | used | dead  
- last_error, user_id, created_at, updated_at, leased_at, used_at  

### register_jobs / register_job_logs

- 批量任务进度与口语化日志  

## 流程

1. 导入 Inbox（非 icloud 跳过）  
2. 开始注册 N 个：占 unused 邮箱 → 选 egress → Turnstile → sendCode → 轮询 OTP → register → pwdLogin → createConsumer → accounts 入库 → 邮箱标 used  
3. 失败：写日志；鉴权失效标 dead，其它可回 unused  

## 配置

系统设置：captcha.provider / api_key / api_base；register.invite_code / concurrency / interval_ms  

## UI

- 邮箱池页  
- 账号注册页（数量、邀请码、开始、运行日志）  

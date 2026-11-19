# Design: QC 多模态图/视频/音频 + @ 素材引用

Date: 2026-11-14  
Status: approved  
Approach: **A**（协议对齐 + Studio 仿官网；不做实时 fee-preview 全复刻）

## Goal

对齐官方 Runow「多模态参考」：

1. 素材池支持 **图片 / 视频 / 音频**（不仅图片）。
2. 提示词里用 **`@` 选择已上传素材** 插入 `@Image N` / `@Video N` / `@Audio N`。
3. 提交时：全部已上传素材写入 `refImage*` / `refVideo*` / `refAudio*`；prompt **原样**提交（含用户插入的 `@…`，不自动补全、不强制每个素材都被 `@`）。

参考 HAR：`参考/runninghub11注册生视频.devtools`（binding `2103076210080342018` / sku `2102602670923161601`）。

## Non-goals

- 复刻官网每次击键触发的 fee-preview。
- `GET .../submission` 幂等查询（可后续加）。
- 自由创作（`freeCreation`）支持视频/音频。
- 改动 webapp 模板路径。
- 通用 QC（`submit_mode=qc`）全量字段编辑器（本设计以官方免费多模态为主；通用 QC 可复用同一套 upload/`ref*` 组装，但 Studio `@` 优先落在免费多模态）。

## Official protocol (HAR)

| 项 | 值 |
| --- | --- |
| 模式 | `creationMode: multimodal` |
| 图 | `refImage1..9`，accept JPG/PNG，单槽 maxUploadCount=1，maxUploadSize=100MB |
| 视频 | `refVideo1..3`，accept MP4 |
| 音频 | `refAudio1..3`，accept MP3/WAV |
| Prompt 引用 | 字面量 `@Image N` / `@Video N` / `@Audio N`（N 从 1 起，对应同序号 `ref*`） |
| 上传 | `POST /api/storage/getCredentials` `{"scene":"qc"}` → COS PUT → CDN URL |
| 提交 | fee-preview → project/create → page/list → prepare → commit → batchStatus |

用户操作语义（产品确认）：

> 上传后进素材池；提示词要引用时打 `@`，再选对应素材。未 `@` 的素材仍可随 `ref*` 提交；服务端不改写 prompt。

## Current gaps

- `buildQCFreeCreateRequest` / Studio 多模态仅 `refImage1..N`。
- `UploadQCImage` 可复用 COS，但 API/表单只收 `image` / `image_N`。
- 队列持久化 `node_info_json` 仅 `FieldType: IMAGE`。
- Studio 无视频/音频槽位、无 `@` 选择器。

## Design

### 1. 素材模型（前后端共用语义）

```text
Asset = { kind: "image"|"video"|"audio", index: 1..max, file|url, preview? }
```

| kind | params key | max (该免费 multimodal SKU) | MIME / 扩展名 |
| --- | --- | --- | --- |
| image | `refImage{n}` | 9 | image/jpeg, image/png（`.jpg` `.jpeg` `.png`；实现可顺带收 webp 但文档以官方为准） |
| video | `refVideo{n}` | 3 | video/mp4（`.mp4`） |
| audio | `refAudio{n}` | 3 | audio/mpeg, audio/wav（`.mp3` `.wav`） |

索引按 **同 kind 上传顺序** 分配（第 1 个图 → Image 1 → `refImage1`）。删除中间项后重新编号，并尽量修正 prompt 里已有 `@` 序号（见 §3）；若修正复杂，首版可在删除时提示用户检查 `@` 引用。

### 2. 后端：`SubmitQCFreeVideo`（multimodal）

- 扩展 `RunInput`（或并行字段）：
  - 保留现有图片：`ImagePath` / `ExtraImages` / `FirstFrameURL` / IMAGE nodes
  - 新增：`ExtraVideos` / `ExtraAudios`（`map[string][]byte` 或有序切片 + 可选 URL 列表）
- COS：将 `UploadQCImage` 泛化为 `UploadQCFile(data, filename)`（按扩展名猜 Content-Type）；图/视/音共用。
- `buildQCFreeCreateRequest`（multimodal）：
  - 写入所有非空 `refImage*` / `refVideo*` / `refAudio*`
  - **不**改写 `prompt`
  - 至少存在 **任意一种** 参考素材；允许纯视频或纯音频（放宽当前「必须至少一张图」）
- 校验失败信息中文化，与 Studio/API 一致。

### 3. HTTP / 队列持久化

Multipart（`submit_mode=qc_free` + `creation_mode=multimodal`）：

| 字段 | 含义 |
| --- | --- |
| `image` / `image_2`…`image_9` | 图（兼容现有） |
| `video` / `video_2` / `video_3` | 视频 |
| `audio` / `audio_2` / `audio_3` | 音频 |
| `prompt` | 原样（可含 `@Image 1` 等） |

JSON 可选：`video_urls` / `audio_urls` / `image_urls`（已是 QC CDN 或 http(s) URL 时跳过本地上传）。

队列：扩展现有 `node_info_json` 落盘方式：

- `FieldType`: `IMAGE` | `VIDEO` | `AUDIO`
- `FieldValue`: 本地路径
- `NodeID`: `qc_ref_image_1` / `qc_ref_video_1` / `qc_ref_audio_1` …

Worker 读回后按类型上传并填入对应 `ref*`。

解析 multipart 上限：建议将 `ParseMultipartForm` 从 64MB 提到足够覆盖官方 100MB/文件的量级（例如 256–512MB），并在文档注明单文件上限。

### 4. OpenAPI / 文档

- `minimax-h3-rh-enhanced-multimodal`：文档与示例增加 `video` / `audio` 及 `@` 约定。
- 不强制客户端插入 `@`；说明「与官网一致：需要引用时在 prompt 写 `@Image N` 等」。

### 5. Studio UI

**素材区（多模态）**

- 三组槽位：参考图（最多 9）、参考视频（最多 3）、参考音频（最多 3）。
- 每组显示占用计数；accept 与上表一致。
- 提交 FormData 映射 §3 字段。

**提示词 `@` 选择器**

- 在 prompt 输入框检测到触发字符 `@`（光标前）：弹出当前已上传素材列表（按 kind+index，如 `Image 1`、`Video 1`）。
- 选择后在光标处插入 `@Image 1`（或 Video/Audio），关闭弹层。
- 无素材时提示先上传。
- 不自动在上传时往 prompt 追加 `@`。

**校验**

- prompt 非空。
- 至少 1 个任意类型素材。
- 类型/数量超限时前端拦截。

**文案**

- 模式 hint 从「最多 9 张图」改为「图最多 9 / 视频最多 3 / 音频最多 3；提示词中 `@` 引用素材」。

### 6. 测试

- 单元：`buildQCFreeCreateRequest` 同时带 image/video/audio URL → params 键正确；prompt 不变。
- 单元：仅 video / 仅 audio 可通过 multimodal 组装。
- 单元：normalize MIME/扩展名拒绝非法类型。
-（可选）Studio 组件测：`@` 插入字符串。

### 7. 实现顺序

1. Provider：`UploadQCFile` + multimodal `refVideo*`/`refAudio*` + 测试  
2. HTTP：multipart/JSON + `node_info` 持久化 VIDEO/AUDIO  
3. OpenAPI 文档与示例  
4. Studio：分区上传  
5. Studio：`@` 选择器  
6. 手动对照 HAR 字段再测一次提交（有账号时）

## Risks

- 大文件：multipart 内存与超时；需提高 limit，必要时后续改流式。
- 删除/重排素材导致 `@` 序号错位；首版可提示，二期做自动重写。
- 通用 QC 路径暂不强制同一套 UI，避免 scope 膨胀。

## Success criteria

- 可用与 HAR 相同形态提交：`refImage1` + `refVideo1` + `refAudio1` + 含 `@Image 1`/`@Video 1`/`@Audio 1` 的 prompt，且 `feePreview.free/welfareFree`。
- Studio 多模态可上传三类素材，并通过 `@` 插入引用。
- 现有仅图片多模态 / 自由创作路径回归通过。

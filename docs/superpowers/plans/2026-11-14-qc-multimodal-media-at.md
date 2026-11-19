# QC Multimodal Image/Video/Audio + @ Mentions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align free multimodal QC with official Runow: upload image/video/audio (auto-classified by MIME/ext), submit as `refImage*`/`refVideo*`/`refAudio*`, and Studio `@` picker inserts `@Image N` / `@Video N` / `@Audio N` into the prompt without rewriting it on submit.

**Architecture:** Extend `runninghub` QC free-video path to upload any media via COS and assemble multimodal params; persist VIDEO/AUDIO paths in `node_info_json` like images; Studio multimodal uses a unified drop + typed slots and an `@` autocomplete on the prompt textarea.

**Tech Stack:** Go (provider/httpapi/tasks), Vue 3 Studio (`web/src/stores/studio.js`, `Studio.vue`)

**Spec:** `docs/superpowers/specs/2026-11-14-qc-multimodal-media-at-design.md`

---

## File map

| File | Responsibility |
| --- | --- |
| `internal/provider/runninghub/qc.go` | Constants, `UploadQCFile`, multimodal param build, `SubmitQCFreeVideo` uploads |
| `internal/provider/runninghub/media_kind.go` | MIME/ext → `image`/`video`/`audio`/reject |
| `internal/provider/runninghub/client.go` | Expand `guessContentType` for mp4/mp3/wav |
| `internal/provider/runninghub/webapp.go` | `RunInput` optional URL lists if needed (prefer NodeInfoList) |
| `internal/provider/runninghub/qc_test.go` | Unit tests for build + classify |
| `internal/httpapi/server.go` | Multipart video/audio, persist nodes, size limit, validation |
| `internal/httpapi/openai_compat.go` | Forward video/audio multipart for public multimodal model |
| `web/src/stores/studio.js` | Assets, classify, FormData, `@` helpers, canSubmit |
| `web/src/views/Studio.vue` | Slots UI + `@` picker |
| `web/src/views/ApiDocs.vue` | Docs/examples |

---

### Task 1: Media kind classifier + Content-Type

**Files:**
- Create: `internal/provider/runninghub/media_kind.go`
- Modify: `internal/provider/runninghub/client.go` (`guessContentType`)
- Test: `internal/provider/runninghub/media_kind_test.go`

- [x] **Step 1: Write failing tests for ClassifyMediaKind**

```go
package runninghub

import "testing"

func TestClassifyMediaKind(t *testing.T) {
	cases := []struct {
		name, mime, want string
	}{
		{"a.png", "image/png", "image"},
		{"a.JPG", "image/jpeg", "image"},
		{"a.mp4", "video/mp4", "video"},
		{"a.wav", "audio/wav", "audio"},
		{"a.mp3", "audio/mpeg", "audio"},
		{"a.txt", "text/plain", ""},
		{"a.mp4", "", "video"}, // ext fallback
	}
	for _, c := range cases {
		got := ClassifyMediaKind(c.name, c.mime)
		if got != c.want {
			t.Fatalf("%s mime=%q got=%q want=%q", c.name, c.mime, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Implement ClassifyMediaKind + guessContentType mp4/mp3/wav**

```go
// media_kind.go
package runninghub

import (
	"path/filepath"
	"strings"
)

const (
	MediaKindImage = "image"
	MediaKindVideo = "video"
	MediaKindAudio = "audio"
)

// ClassifyMediaKind returns image|video|audio or "" if unsupported (official multimodal accepts).
func ClassifyMediaKind(filename, mime string) string {
	mime = strings.ToLower(strings.TrimSpace(mime))
	switch mime {
	case "image/jpeg", "image/jpg", "image/png":
		return MediaKindImage
	case "video/mp4":
		return MediaKindVideo
	case "audio/mpeg", "audio/mp3", "audio/wav", "audio/x-wav", "audio/wave":
		return MediaKindAudio
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png":
		return MediaKindImage
	case ".mp4":
		return MediaKindVideo
	case ".mp3":
		return MediaKindAudio
	case ".wav":
		return MediaKindAudio
	}
	return ""
}
```

Update `guessContentType` in `client.go`:

```go
case ".mp4":
	return "video/mp4"
case ".mp3":
	return "audio/mpeg"
case ".wav":
	return "audio/wav"
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/provider/runninghub/ -run TestClassifyMediaKind -count=1`

Expected: PASS

- [ ] **Step 4: Commit** (if user requested commits; otherwise skip until asked)

---

### Task 2: Multimodal refVideo/refAudio in create request + upload

**Files:**
- Modify: `internal/provider/runninghub/qc.go`
- Modify: `internal/provider/runninghub/qc_test.go`

- [ ] **Step 1: Add constants**

```go
DefaultQCFreeVideoMaxVideos = 3
DefaultQCFreeVideoMaxAudios = 3
```

- [ ] **Step 2: Change `buildQCFreeCreateRequest` to accept videoURLs/audioURLs**

Signature:

```go
func buildQCFreeCreateRequest(prompt string, imageURLs, videoURLs, audioURLs []string, aspect, duration, resolution, mode, projectID, pageID, clientReqID string) map[string]any
```

Multimodal branch: write `refImage1..`, `refVideo1..`, `refAudio1..` (cap at max). Prompt unchanged. `ResolveQCFreeVideoModel(mode, len(imageURLs))` stays (multimodal ignores count for sku).

Keep `UploadQCImage` as thin wrapper calling:

```go
func (c *Client) UploadQCFile(data []byte, filename string) (string, error)
```

- [ ] **Step 3: Update `SubmitQCFreeVideo` multimodal path**

- Collect IMAGE / VIDEO / AUDIO from `NodeInfoList` by `FieldType` (and ExtraImages for images).
- Upload each with `UploadQCFile` using original filename/ext.
- Validation: multimodal requires `len(images)+len(videos)+len(audios) > 0` (not image-only).
- Error: `多模态参考至少需要一个参考素材（图/视频/音频）`.

- [ ] **Step 4: Unit tests**

Extend `TestBuildQCFreeCreateRequest` with:

```go
multiAV := buildQCFreeCreateRequest(
  "@Image 1 x @Video 1 y @Audio 1 z",
  []string{"https://cdn.example/a.png"},
  []string{"https://cdn.example/v.mp4"},
  []string{"https://cdn.example/a.wav"},
  "3:4 (Portrait Standard)", "10", "1080p",
  QCFreeVideoModeMultimodal, "p1", "page1", "req-av",
)
pm, _ := multiAV["params"].(map[string]any)
// assert refImage1, refVideo1, refAudio1, prompt unchanged, no firstFrameUrl
```

Also video-only multimodal params (no refImage).

Update all existing `buildQCFreeCreateRequest` call sites to pass `nil, nil` for video/audio.

- [ ] **Step 5: Run**

`go test ./internal/provider/runninghub/ -count=1`

Expected: PASS

---

### Task 3: HTTP multipart + node_info persistence

**Files:**
- Modify: `internal/httpapi/server.go`

- [ ] **Step 1: Raise ParseMultipartForm** for video create path to `512 << 20` (512MB).

- [ ] **Step 2: Parse `video`/`video_2`/`video_3` and `audio`/`audio_2`/`audio_3` into side maps (or write files immediately).

Prefer: save all media files to `uploads/` and append `NodeInput` with correct `FieldType`/`NodeID`:

- `qc_ref_image_N` / IMAGE
- `qc_ref_video_N` / VIDEO  
- `qc_ref_audio_N` / AUDIO

- [ ] **Step 3: Multimodal validation** — any media count > 0 (imagePath + image nodes + video + audio).

- [ ] **Step 4: Optional JSON `video_urls` / `audio_urls` arrays → nodes with FieldValue=URL (Submit path already accepts http prefix for IMAGE; extend for VIDEO/AUDIO).

- [ ] **Step 5: Smoke compile**

`go test ./internal/httpapi/ -count=1`

---

### Task 4: OpenAPI compat forward video/audio

**Files:**
- Modify: `internal/httpapi/openai_compat.go` (`buildFreeVideoCreateRequest`)
- Modify: `web/src/views/ApiDocs.vue`

- [ ] Forward multipart keys `video`, `video_*`, `audio`, `audio_*` like images.
- [ ] Update multimodal curl example with `-F video=@./ref.mp4 -F audio=@./voice.wav` and prompt with `@Image 1`…  
- [ ] Models list blurb: 图≤9 / 视频≤3 / 音频≤3；可用 `@` 引用。

---

### Task 5: Studio media pool + auto-classify

**Files:**
- Modify: `web/src/stores/studio.js`
- Modify: `web/src/views/Studio.vue`

- [ ] Constants: `FREE_VIDEO_MULTI_MAX_VIDEOS=3`, `FREE_VIDEO_MULTI_MAX_AUDIOS=3`; keys `video_1..3`, `audio_1..3`.
- [ ] `classifyMediaFile(file)` → image|video|audio|null (match backend).
- [ ] Unified add for multimodal: classify → fill next empty slot of that kind; reject unsupported.
- [ ] `canSubmit`: multimodal needs prompt + (image|video|audio count > 0).
- [ ] `submit()` FormData: append videos/audios with field names `video`/`video_2`…, `audio`/`audio_2`….
- [ ] UI: three rows (图/视频/音频) with counts; accept attributes; video/audio preview (filename or `<video>`/`<audio>` mini).
- [ ] Hint text update.

---

### Task 6: Studio `@` mention picker

**Files:**
- Modify: `web/src/stores/studio.js`
- Modify: `web/src/views/Studio.vue`

- [ ] On multimodal prompt `input`/`keydown`: if caret preceded by `@` (start or non-word), open menu of uploaded assets labeled `Image N` / `Video N` / `Audio N`.
- [ ] On select: insert `@Image N` (etc.) at caret, close menu.
- [ ] Empty pool → show “请先上传素材”.
- [ ] Escape/click-away closes.

---

### Task 7: Verify

- [ ] `go test ./internal/provider/runninghub/ ./internal/httpapi/ -count=1`
- [ ] Frontend build if available: `cd web && npm run build` (or project’s usual command)
- [ ] Manual: multimodal with image+video+audio + `@` prompt fields match HAR shape (local log of createReq if needed)

---

## Spec coverage check

| Spec item | Task |
| --- | --- |
| MIME auto-classify | 1, 5 |
| refImage/Video/Audio params | 2 |
| UploadQCFile | 2 |
| Multipart + node_info | 3 |
| OpenAPI/docs | 4 |
| Studio slots | 5 |
| `@` picker | 6 |
| Tests | 1, 2, 7 |
| No auto prompt rewrite | 2, 6 |

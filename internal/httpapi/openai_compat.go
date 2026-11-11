package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/trae/runninghub2api/internal/provider/runninghub"
	"github.com/trae/runninghub2api/internal/store"
)

const (
	defaultSyncTimeout = 10 * time.Minute
	pollInterval       = 2 * time.Second
)

// OpenAI-style routes:
//   POST /v1/images/generations  — 官方免费生图（默认同步）
//   GET  /v1/images/{id}         — 异步生图任务查询
//   POST /v1/videos              — 官方免费生视频 / 应用模板（默认异步；已有 handleVideos）
//   GET  /v1/videos/{id}         — 异步生视频 / 模板任务查询

func (s *Server) handleImages(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && strings.TrimRight(r.URL.Path, "/") == "/v1/images/generations":
		s.handleImageGenerations(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/images/"):
		s.handleImageByID(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/models"), "/")
	if path == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"object": "list",
			"data":   openAIModelCatalog(),
		})
		return
	}
	m, ok := findOpenAIModel(path)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": map[string]any{
				"message": "model not found: " + path,
				"type":    "invalid_request_error",
				"code":    "model_not_found",
			},
		})
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleImageGenerations(w http.ResponseWriter, r *http.Request) {
	req, modelID, syncMode, err := s.buildFreeImageCreateRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": err.Error(), "type": "invalid_request_error"},
		})
		return
	}
	status, body, _ := s.invokeCreateVideo(req)
	if status >= 400 && asStringMap(body)["id"] == nil {
		writeJSON(w, status, body)
		return
	}
	id := strings.TrimSpace(fmt.Sprint(body["id"]))
	if id == "" {
		writeJSON(w, status, body)
		return
	}
	if !syncMode {
		writeJSON(w, status, map[string]any{
			"id":         id,
			"object":     "image.generation",
			"created":    time.Now().Unix(),
			"status":     body["status"],
			"mode":       "async",
			"model":      modelID,
			"rh_task_id": body["rh_task_id"],
			"message":    "async: poll GET /v1/images/" + id,
		})
		return
	}
	task, waitErr := s.waitTaskTerminal(r.Context(), id, syncTimeoutFrom(r))
	if waitErr != nil {
		writeJSON(w, http.StatusGatewayTimeout, map[string]any{
			"id": id, "object": "image.generation", "mode": "sync", "model": modelID,
			"status": task.Status, "error": waitErr.Error(), "url": task.FileURL,
		})
		return
	}
	if task.Status != "succeeded" {
		code := http.StatusBadGateway
		if task.Status == "cancelled" {
			code = http.StatusConflict
		}
		writeJSON(w, code, map[string]any{
			"id": id, "object": "image.generation", "mode": "sync", "model": modelID,
			"status": task.Status, "error": firstNonEmpty(task.Error, task.ProgressText), "error_code": task.ErrorCode,
		})
		return
	}
	// OpenAI Images API shape (+ mode/model extensions).
	writeJSON(w, http.StatusOK, map[string]any{
		"created": time.Now().Unix(),
		"data":    []map[string]any{{"url": task.FileURL}},
		"id":      id,
		"object":  "list",
		"mode":    "sync",
		"model":   modelID,
		"status":  "succeeded",
	})
}

func (s *Server) handleImageByID(w http.ResponseWriter, r *http.Request) {
	a, _ := actorFromRequest(r)
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/images/"), "/")
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing id"})
		return
	}
	task, err := s.store.GetTask(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	if !a.IsAdmin() && task.UserID != a.UserID {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "无权查看该任务"})
		return
	}
	out := map[string]any{
		"id":            task.ID,
		"object":        "image.generation",
		"created":       parseUnix(task.CreatedAt),
		"status":        task.Status,
		"mode":          "async",
		"progress_text": task.ProgressText,
		"rh_task_id":    task.RHTaskID,
		"webapp_id":     task.WebappID,
		"account_email": task.AccountEmail,
		"error":         task.Error,
		"error_code":    task.ErrorCode,
	}
	if task.FileURL != "" {
		out["data"] = []map[string]any{{"url": task.FileURL}}
		out["url"] = task.FileURL
	}
	writeJSON(w, http.StatusOK, out)
}

// wrapVideoCreateOpenAI runs after existing createVideo path when caller wants
// OpenAI-shaped sync/async semantics for free video (no webapp template).
func (s *Server) maybeOpenAIVideoResponse(w http.ResponseWriter, r *http.Request, createFn func(http.ResponseWriter, *http.Request)) bool {
	if r.Method != http.MethodPost {
		return false
	}
	syncMode, explicit, freeVideo := s.inspectVideoCreateMode(r)
	if !freeVideo && !explicit {
		return false
	}
	if !freeVideo {
		// Template create with explicit sync/async flag.
		rec := httptest.NewRecorder()
		createFn(rec, r)
		status, body, _ := decodeRecorder(rec)
		id := strings.TrimSpace(fmt.Sprint(body["id"]))
		if !syncMode || id == "" {
			writeJSON(w, status, withMode(body, "async"))
			return true
		}
		task, waitErr := s.waitTaskTerminal(r.Context(), id, syncTimeoutFrom(r))
		writeOpenAIVideoSync(w, id, "", task, waitErr)
		return true
	}

	req, modelID, err := s.buildFreeVideoCreateRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": err.Error(), "type": "invalid_request_error"},
		})
		return true
	}
	status, body, _ := s.invokeCreateVideo(req)
	id := strings.TrimSpace(fmt.Sprint(body["id"]))
	if id == "" {
		writeJSON(w, status, body)
		return true
	}
	if status >= 400 && fmt.Sprint(body["status"]) == "failed" {
		writeJSON(w, status, body)
		return true
	}
	if !syncMode {
		writeJSON(w, status, map[string]any{
			"id":         id,
			"object":     "video",
			"created_at": time.Now().Unix(),
			"status":     mapVideoStatus(fmt.Sprint(body["status"])),
			"mode":       "async",
			"model":      modelID,
			"rh_task_id": body["rh_task_id"],
			"progress":   0,
			"message":    "async: poll GET /v1/videos/" + id,
		})
		return true
	}
	task, waitErr := s.waitTaskTerminal(r.Context(), id, syncTimeoutFrom(r))
	writeOpenAIVideoSync(w, id, modelID, task, waitErr)
	return true
}

func writeOpenAIVideoSync(w http.ResponseWriter, id, modelID string, task store.VideoTask, waitErr error) {
	if modelID == "" {
		modelID = ModelMinimaxH3RHEnhanced
	}
	if waitErr != nil {
		writeJSON(w, http.StatusGatewayTimeout, map[string]any{
			"id": id, "object": "video", "mode": "sync", "model": modelID,
			"status": mapVideoStatus(task.Status), "error": waitErr.Error(), "url": task.FileURL,
		})
		return
	}
	if task.Status != "succeeded" {
		code := http.StatusBadGateway
		if task.Status == "cancelled" {
			code = http.StatusConflict
		}
		writeJSON(w, code, map[string]any{
			"id": id, "object": "video", "mode": "sync", "model": modelID,
			"status": mapVideoStatus(task.Status),
			"error":  firstNonEmpty(task.Error, task.ProgressText), "error_code": task.ErrorCode,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         id,
		"object":     "video",
		"created_at": parseUnix(task.CreatedAt),
		"status":     "completed",
		"mode":       "sync",
		"progress":   100,
		"model":      modelID,
		"url":        task.FileURL,
	})
}

func (s *Server) invokeCreateVideo(r *http.Request) (int, map[string]any, []byte) {
	rec := httptest.NewRecorder()
	s.createVideo(rec, r)
	return decodeRecorder(rec)
}

func decodeRecorder(rec *httptest.ResponseRecorder) (int, map[string]any, []byte) {
	raw := rec.Body.Bytes()
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	if body == nil {
		body = map[string]any{"error": strings.TrimSpace(string(raw))}
	}
	return rec.Code, body, raw
}

func withMode(body map[string]any, mode string) map[string]any {
	if body == nil {
		body = map[string]any{}
	}
	body["mode"] = mode
	body["object"] = "video"
	return body
}

func (s *Server) waitTaskTerminal(ctx context.Context, id string, timeout time.Duration) (store.VideoTask, error) {
	if timeout <= 0 {
		timeout = defaultSyncTimeout
	}
	deadline := time.Now().Add(timeout)
	var last store.VideoTask
	for {
		if ctx.Err() != nil {
			return last, ctx.Err()
		}
		t, err := s.store.GetTask(id)
		if err != nil {
			return last, err
		}
		last = t
		switch t.Status {
		case "succeeded", "failed", "cancelled":
			return t, nil
		}
		if time.Now().After(deadline) {
			return last, fmt.Errorf("sync timeout after %s (status=%s); poll GET later with id=%s", timeout, t.Status, id)
		}
		if s.worker != nil && strings.TrimSpace(t.RHTaskID) != "" {
			_ = s.worker.SyncAndMaybePoll(id, false)
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func syncTimeoutFrom(r *http.Request) time.Duration {
	raw := strings.TrimSpace(r.URL.Query().Get("timeout"))
	if raw == "" {
		raw = strings.TrimSpace(r.Header.Get("X-Sync-Timeout"))
	}
	if raw == "" {
		return defaultSyncTimeout
	}
	if sec, err := strconv.Atoi(raw); err == nil && sec > 0 {
		return time.Duration(sec) * time.Second
	}
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return d
	}
	return defaultSyncTimeout
}

// wantSync: images default true; videos default false.
func parseWantSync(r *http.Request, defaultSync bool) bool {
	q := r.URL.Query()
	if v := strings.TrimSpace(q.Get("async")); v != "" {
		return !parseTruthy(v)
	}
	if v := strings.TrimSpace(q.Get("sync")); v != "" {
		return parseTruthy(v)
	}
	if v := strings.TrimSpace(r.Header.Get("X-Sync")); v != "" {
		return parseTruthy(v)
	}
	return defaultSync
}

func (s *Server) inspectVideoCreateMode(r *http.Request) (syncMode, explicit, freeVideo bool) {
	q := r.URL.Query()
	if _, ok := q["sync"]; ok {
		explicit = true
	}
	if _, ok := q["async"]; ok {
		explicit = true
	}
	if strings.TrimSpace(r.Header.Get("X-Sync")) != "" {
		explicit = true
	}
	syncMode = parseWantSync(r, false)

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		_ = r.ParseMultipartForm(64 << 20)
		mode := strings.TrimSpace(r.FormValue("submit_mode"))
		if mode == runninghub.SubmitModeQCFree || mode == "qc_free" {
			freeVideo = true
		}
		if v := strings.TrimSpace(r.FormValue("sync")); v != "" {
			explicit = true
			syncMode = parseTruthy(v)
		}
		if v := strings.TrimSpace(r.FormValue("async")); v != "" {
			explicit = true
			syncMode = !parseTruthy(v)
		}
		model := strings.TrimSpace(r.FormValue("model"))
		if strings.TrimSpace(r.FormValue("webapp_id")) == "" && model != "" && isPublicVideoModel(model) {
			freeVideo = true
		}
		return
	}
	// Peek JSON without consuming permanently.
	raw, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(raw))
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	if body == nil {
		return
	}
	if _, ok := body["sync"]; ok {
		explicit = true
		syncMode = parseTruthy(fmt.Sprint(body["sync"]))
	}
	if _, ok := body["async"]; ok {
		explicit = true
		syncMode = !parseTruthy(fmt.Sprint(body["async"]))
	}
	mode := strings.TrimSpace(fmt.Sprint(body["submit_mode"]))
	if mode == runninghub.SubmitModeQCFree || mode == "qc_free" {
		freeVideo = true
	}
	webapp := strings.TrimSpace(fmt.Sprint(body["webapp_id"]))
	model := strings.TrimSpace(fmt.Sprint(body["model"]))
	if webapp == "" || webapp == "<nil>" {
		webapp = ""
	}
	if model == "<nil>" {
		model = ""
	}
	if webapp == "" && model != "" && isPublicVideoModel(model) {
		freeVideo = true
	}
	if webapp == "" && model == "" && (mode == "" || mode == runninghub.SubmitModeQCFree) &&
		strings.TrimSpace(fmt.Sprint(body["prompt"])) != "" && body["node_info_list"] == nil {
		// OpenAI-style video body without model → default free video model
		freeVideo = true
	}
	return
}

func (s *Server) buildFreeImageCreateRequest(r *http.Request) (*http.Request, string, bool, error) {
	syncMode := parseWantSync(r, true) // 生图默认同步
	ct := r.Header.Get("Content-Type")
	fields := map[string]string{}
	var files []namedFile
	modelID := ModelNanoBanana2LowCost

	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			return nil, "", false, err
		}
		if v := strings.TrimSpace(r.FormValue("sync")); v != "" {
			syncMode = parseTruthy(v)
		}
		if v := strings.TrimSpace(r.FormValue("async")); v != "" {
			syncMode = !parseTruthy(v)
		}
		prompt := strings.TrimSpace(r.FormValue("prompt"))
		if prompt == "" {
			return nil, "", false, fmt.Errorf("prompt required")
		}
		fields["prompt"] = prompt
		rawModel := firstNonEmpty(r.FormValue("model"), r.FormValue("free_image_model"))
		modelID = normalizePublicModelID(rawModel)
		if !isPublicImageModel(modelID) {
			return nil, "", false, fmt.Errorf("unknown image model %q; use GET /v1/models", rawModel)
		}
		family := publicImageFamily(modelID)
		fields["free_image_model"] = family
		fields["creation_mode"] = family
		if v := strings.TrimSpace(r.FormValue("aspect_ratio")); v != "" {
			fields["aspect_ratio"] = v
		} else if v := strings.TrimSpace(r.FormValue("size")); v != "" {
			fields["aspect_ratio"] = mapImageSizeToAspect(v)
		} else {
			fields["aspect_ratio"] = "9:16"
		}
		if v := strings.TrimSpace(r.FormValue("resolution")); v != "" {
			fields["resolution"] = v
		} else if v := strings.TrimSpace(r.FormValue("quality")); v != "" {
			fields["resolution"] = mapImageQuality(v)
		} else {
			fields["resolution"] = "1k"
		}
		if r.MultipartForm != nil {
			idx := 0
			for key, fhs := range r.MultipartForm.File {
				if len(fhs) == 0 {
					continue
				}
				if key != "image" && key != "image[]" && !strings.HasPrefix(key, "image_") && key != "input_reference" {
					continue
				}
				for _, fh := range fhs {
					f, err := fh.Open()
					if err != nil {
						continue
					}
					b, _ := io.ReadAll(f)
					f.Close()
					if len(b) == 0 {
						continue
					}
					idx++
					name := "image"
					if idx > 1 {
						name = fmt.Sprintf("image_%d", idx)
					}
					files = append(files, namedFile{field: name, filename: fh.Filename, data: b})
				}
			}
		}
	} else {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, "", false, err
		}
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, "", false, fmt.Errorf("invalid json")
		}
		if v, ok := body["sync"]; ok {
			syncMode = parseTruthy(fmt.Sprint(v))
		}
		if v, ok := body["async"]; ok {
			syncMode = !parseTruthy(fmt.Sprint(v))
		}
		prompt := strings.TrimSpace(fmt.Sprint(body["prompt"]))
		if prompt == "" {
			return nil, "", false, fmt.Errorf("prompt required")
		}
		fields["prompt"] = prompt
		rawModel := firstNonEmpty(fmt.Sprint(body["model"]), fmt.Sprint(body["free_image_model"]))
		modelID = normalizePublicModelID(rawModel)
		if !isPublicImageModel(modelID) {
			return nil, "", false, fmt.Errorf("unknown image model %q; use GET /v1/models", rawModel)
		}
		family := publicImageFamily(modelID)
		fields["free_image_model"] = family
		fields["creation_mode"] = family
		if v := strings.TrimSpace(fmt.Sprint(body["aspect_ratio"])); v != "" && v != "<nil>" {
			fields["aspect_ratio"] = v
		} else if v := strings.TrimSpace(fmt.Sprint(body["size"])); v != "" && v != "<nil>" {
			fields["aspect_ratio"] = mapImageSizeToAspect(v)
		} else {
			fields["aspect_ratio"] = "9:16"
		}
		if v := strings.TrimSpace(fmt.Sprint(body["resolution"])); v != "" && v != "<nil>" {
			fields["resolution"] = v
		} else if v := strings.TrimSpace(fmt.Sprint(body["quality"])); v != "" && v != "<nil>" {
			fields["resolution"] = mapImageQuality(v)
		} else {
			fields["resolution"] = "1k"
		}
		if b64 := strings.TrimSpace(fmt.Sprint(body["image_base64"])); b64 != "" && b64 != "<nil>" {
			rawImg, err := decodeMaybeBase64(b64)
			if err == nil && len(rawImg) > 0 {
				files = append(files, namedFile{field: "image", filename: "ref.png", data: rawImg})
			}
		}
		if arr, ok := body["images_base64"].([]any); ok {
			for i, item := range arr {
				rawImg, err := decodeMaybeBase64(fmt.Sprint(item))
				if err != nil || len(rawImg) == 0 {
					continue
				}
				name := "image"
				if i > 0 {
					name = fmt.Sprintf("image_%d", i+1)
				}
				files = append(files, namedFile{field: name, filename: fmt.Sprintf("ref_%d.png", i+1), data: rawImg})
			}
		}
	}
	fields["submit_mode"] = runninghub.SubmitModeQCFreeImage
	req, err := newMultipartRequest(r, fields, files)
	return req, modelID, syncMode, err
}

func (s *Server) buildFreeVideoCreateRequest(r *http.Request) (*http.Request, string, error) {
	ct := r.Header.Get("Content-Type")
	fields := map[string]string{}
	var files []namedFile
	modelID := ModelMinimaxH3RHEnhanced

	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			return nil, "", err
		}
		prompt := strings.TrimSpace(r.FormValue("prompt"))
		if prompt == "" {
			return nil, "", fmt.Errorf("prompt required")
		}
		fields["prompt"] = prompt
		rawModel := firstNonEmpty(r.FormValue("model"), ModelMinimaxH3RHEnhanced)
		modelID = normalizePublicModelID(rawModel)
		if !isPublicVideoModel(modelID) {
			return nil, "", fmt.Errorf("unknown video model %q; use GET /v1/models", rawModel)
		}
		mode := publicVideoCreationMode(modelID)
		if v := strings.TrimSpace(r.FormValue("creation_mode")); v != "" {
			mode = runninghub.NormalizeQCFreeVideoMode(v)
		}
		fields["creation_mode"] = mode
		fields["free_video_mode"] = mode
		fields["duration"] = firstNonEmpty(r.FormValue("duration"), r.FormValue("seconds"), "4")
		fields["aspect_ratio"] = firstNonEmpty(r.FormValue("aspect_ratio"), r.FormValue("size"), "3:2 (Photo)")
		fields["resolution"] = firstNonEmpty(r.FormValue("resolution"), "480p")
		if r.MultipartForm != nil {
			idx := 0
			for key, fhs := range r.MultipartForm.File {
				if len(fhs) == 0 {
					continue
				}
				if key != "image" && key != "input_reference" && !strings.HasPrefix(key, "image_") {
					continue
				}
				for _, fh := range fhs {
					f, err := fh.Open()
					if err != nil {
						continue
					}
					b, _ := io.ReadAll(f)
					f.Close()
					if len(b) == 0 {
						continue
					}
					idx++
					name := "image"
					if idx > 1 {
						name = fmt.Sprintf("image_%d", idx)
					}
					files = append(files, namedFile{field: name, filename: fh.Filename, data: b})
				}
			}
		}
	} else {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, "", err
		}
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, "", fmt.Errorf("invalid json")
		}
		prompt := strings.TrimSpace(fmt.Sprint(body["prompt"]))
		if prompt == "" {
			return nil, "", fmt.Errorf("prompt required")
		}
		fields["prompt"] = prompt
		rawModel := firstNonEmpty(fmt.Sprint(body["model"]), ModelMinimaxH3RHEnhanced)
		modelID = normalizePublicModelID(rawModel)
		if !isPublicVideoModel(modelID) {
			return nil, "", fmt.Errorf("unknown video model %q; use GET /v1/models", rawModel)
		}
		mode := publicVideoCreationMode(modelID)
		if v := strings.TrimSpace(fmt.Sprint(body["creation_mode"])); v != "" && v != "<nil>" {
			mode = runninghub.NormalizeQCFreeVideoMode(v)
		}
		fields["creation_mode"] = mode
		fields["free_video_mode"] = mode
		fields["duration"] = firstNonEmpty(fmt.Sprint(body["duration"]), fmt.Sprint(body["seconds"]), "4")
		fields["aspect_ratio"] = firstNonEmpty(fmt.Sprint(body["aspect_ratio"]), fmt.Sprint(body["size"]), "3:2 (Photo)")
		fields["resolution"] = firstNonEmpty(fmt.Sprint(body["resolution"]), "480p")
		if b64 := strings.TrimSpace(fmt.Sprint(body["image_base64"])); b64 != "" && b64 != "<nil>" {
			rawImg, err := decodeMaybeBase64(b64)
			if err == nil && len(rawImg) > 0 {
				files = append(files, namedFile{field: "image", filename: "ref.png", data: rawImg})
			}
		}
	}
	fields["submit_mode"] = runninghub.SubmitModeQCFree
	req, err := newMultipartRequest(r, fields, files)
	return req, modelID, err
}

type namedFile struct {
	field    string
	filename string
	data     []byte
}

func newMultipartRequest(orig *http.Request, fields map[string]string, files []namedFile) (*http.Request, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if strings.TrimSpace(v) == "" || v == "<nil>" {
			continue
		}
		_ = mw.WriteField(k, v)
	}
	for _, f := range files {
		w, err := mw.CreateFormFile(f.field, firstNonEmpty(f.filename, "upload.bin"))
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(f.data); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(orig.Context(), http.MethodPost, orig.URL.Path, &buf)
	if err != nil {
		return nil, err
	}
	req.Header = orig.Header.Clone()
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if a, ok := actorFromRequest(orig); ok {
		req = req.WithContext(withActor(orig.Context(), a))
	} else {
		req = req.WithContext(orig.Context())
	}
	return req, nil
}

func mapImageSizeToAspect(size string) string {
	s := strings.TrimSpace(size)
	switch strings.ToLower(s) {
	case "1024x1024", "1:1", "square":
		return "1:1"
	case "1024x1792", "9:16", "portrait":
		return "9:16"
	case "1792x1024", "16:9", "landscape":
		return "16:9"
	default:
		if s != "" {
			return s
		}
		return "9:16"
	}
}

func mapImageQuality(q string) string {
	switch strings.ToLower(strings.TrimSpace(q)) {
	case "hd", "high", "2k":
		return "2k"
	case "4k", "ultra":
		return "4k"
	case "low", "standard", "1k", "medium":
		return "1k"
	default:
		return firstNonEmpty(q, "1k")
	}
}

func mapVideoStatus(st string) string {
	switch strings.ToLower(strings.TrimSpace(st)) {
	case "succeeded", "success", "completed":
		return "completed"
	case "failed", "error":
		return "failed"
	case "cancelled", "canceled":
		return "cancelled"
	case "queued":
		return "queued"
	case "running":
		return "in_progress"
	default:
		return st
	}
}

func parseUnix(rfc3339 string) int64 {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(rfc3339))
	if err != nil {
		return time.Now().Unix()
	}
	return t.Unix()
}

func asStringMap(v map[string]any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	return v
}

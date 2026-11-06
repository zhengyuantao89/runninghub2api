package httpapi

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/trae/runninghub2api/internal/accounts"
	"github.com/trae/runninghub2api/internal/config"
	"github.com/trae/runninghub2api/internal/egress"
	"github.com/trae/runninghub2api/internal/provider/runninghub"
	"github.com/trae/runninghub2api/internal/store"
	"github.com/trae/runninghub2api/internal/tasks"
)

type Server struct {
	cfg          config.Config
	store        *store.Store
	pool         *accounts.Pool
	worker       *tasks.Worker
	dailyClaimer *accounts.DailyLoginClaimer
	egress       *egress.Manager
	mux          *http.ServeMux
	sessions     *sessionStore
}

func New(cfg config.Config, st *store.Store, pool *accounts.Pool, worker *tasks.Worker, dailyClaimer *accounts.DailyLoginClaimer) *Server {
	s := &Server{
		cfg:          cfg,
		store:        st,
		pool:         pool,
		worker:       worker,
		dailyClaimer: dailyClaimer,
		egress:       egress.NewManager(st),
		mux:          http.NewServeMux(),
		sessions:     newSessionStore(7 * 24 * time.Hour),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		s.mux.ServeHTTP(w, r)
	})
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/api/login", s.handleLogin)
	s.mux.HandleFunc("/api/logout", s.withSession(s.handleLogout))
	s.mux.HandleFunc("/api/me", s.withSession(s.handleMe))
	s.mux.HandleFunc("/api/console", s.withSession(s.handleConsole))
	s.mux.HandleFunc("/admin/settings", s.withSessionOrAPIKey(s.handleAdminSettings))

	// Browser session OR external API key.
	s.mux.HandleFunc("/v1/videos", s.withSessionOrAPIKey(s.handleVideos))
	s.mux.HandleFunc("/v1/videos/", s.withSessionOrAPIKey(s.handleVideoByID))
	s.mux.HandleFunc("/v1/webapps/", s.withSessionOrAPIKey(s.handleWebappByID))
	s.mux.HandleFunc("/admin/accounts/import", s.withSessionOrAPIKey(s.handleImportAccounts))
	s.mux.HandleFunc("/admin/accounts/ensure-api-key", s.withSessionOrAPIKey(s.handleEnsureAPIKeys))
	s.mux.HandleFunc("/admin/accounts/claim-daily-login", s.withSessionOrAPIKey(s.handleClaimDailyLogin))
	s.mux.HandleFunc("/admin/accounts/", s.withSessionOrAPIKey(s.handleAdminAccountByID))
	s.mux.HandleFunc("/admin/accounts", s.withSessionOrAPIKey(s.handleAdminAccounts))
	s.mux.HandleFunc("/admin/tasks", s.withSessionOrAPIKey(s.handleAdminTasks))
	s.mux.HandleFunc("/admin/templates", s.withSessionOrAPIKey(s.handleAdminTemplates))
	s.mux.HandleFunc("/admin/templates/", s.withSessionOrAPIKey(s.handleAdminTemplateByID))
	s.mux.HandleFunc("/admin/proxies/import", s.withSessionOrAPIKey(s.handleImportProxies))
	s.mux.HandleFunc("/admin/proxies/groups", s.withSessionOrAPIKey(s.handleProxyGroups))
	s.mux.HandleFunc("/admin/proxies/subscriptions/refresh", s.withSessionOrAPIKey(s.handleProxySubscriptionRefresh))
	s.mux.HandleFunc("/admin/proxies/subscriptions", s.withSessionOrAPIKey(s.handleProxySubscriptions))
	s.mux.HandleFunc("/admin/proxies/", s.withSessionOrAPIKey(s.handleAdminProxyByID))
	s.mux.HandleFunc("/admin/proxies", s.withSessionOrAPIKey(s.handleAdminProxies))

	// Downstream / gpt-plus push: API key (legacy aliases still accepted).
	s.mux.HandleFunc("/admin/sync/accounts", s.withAPIKey(s.handlePushAccounts))
	s.mux.HandleFunc("/api/accounts/push", s.withAPIKey(s.handlePushAccounts))
	s.mux.HandleFunc("/api/proxies/push", s.withAPIKey(s.handlePushProxies))
	s.mux.Handle("/", s.spaHandler())
}

func (s *Server) spaHandler() http.HandlerFunc {
	webDir := s.cfg.WebDir
	index := filepath.Join(webDir, "index.html")
	fileServer := http.FileServer(http.Dir(webDir))
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			http.ServeFile(w, r, index)
			return
		}
		full := filepath.Join(webDir, filepath.Clean("/"+path))
		if st, err := os.Stat(full); err == nil && !st.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		if _, err := os.Stat(index); err == nil {
			http.ServeFile(w, r, index)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"service": "runninghub2api",
			"hint":    "web UI not built; run: cd web && npm i && npm run build",
		})
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "runninghub2api"})
}

func (s *Server) bearerToken(r *http.Request) string {
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		token = strings.TrimSpace(r.Header.Get("X-API-Key"))
	}
	return token
}

func (s *Server) hasAPIKey(r *http.Request) bool {
	token := s.bearerToken(r)
	if token == "" {
		return false
	}
	for _, k := range s.cfg.ExternalKeys() {
		if token == k {
			return true
		}
	}
	return false
}

func (s *Server) hasSession(r *http.Request) bool {
	return s.sessions.Valid(s.bearerToken(r))
}

func (s *Server) withAPIKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.hasAPIKey(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (s *Server) withSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.hasSession(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (s *Server) withSessionOrAPIKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.hasSession(r) || s.hasAPIKey(r) {
			next(w, r)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	user := strings.TrimSpace(body.Username)
	pass := strings.TrimSpace(body.Password)
	if user == "" || pass == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "username/password required"})
		return
	}
	if user != s.cfg.AdminUser || pass != s.cfg.AdminPassword {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "账号或密码错误"})
		return
	}
	token := s.sessions.Create()
	writeJSON(w, http.StatusOK, map[string]any{
		"token":    token,
		"username": s.cfg.AdminUser,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.sessions.Revoke(s.bearerToken(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"username": s.cfg.AdminUser,
	})
}

func (s *Server) handleConsole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"username":  s.cfg.AdminUser,
		"api_key":   s.cfg.APIKey,
		"poll_auth": s.resolvePollAuth(),
		"hint":      "下游调用与账号推送请使用此 API Key；控制台登录后工作室无需再填 Key。",
	})
}

func (s *Server) resolvePollAuth() string {
	if v, err := s.store.GetSetting(store.SettingPollAuth); err == nil && strings.TrimSpace(v) != "" {
		return config.NormalizePollAuth(v)
	}
	return config.NormalizePollAuth(s.cfg.PollAuth)
}

func (s *Server) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"poll_auth":         s.resolvePollAuth(),
			"poll_auth_default": config.NormalizePollAuth(s.cfg.PollAuth),
		})
	case http.MethodPut, http.MethodPost:
		var body struct {
			PollAuth string `json:"poll_auth"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
		mode := config.NormalizePollAuth(body.PollAuth)
		if err := s.store.SetSetting(store.SettingPollAuth, mode); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "poll_auth": mode})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleVideos(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.store.ListTasks(100)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": list})
	case http.MethodPost:
		s.createVideo(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWebappByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/webapps/")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "webapp_id required"})
		return
	}
	webappID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	switch {
	case action == "demo" && r.Method == http.MethodGet:
		forceRefresh := r.URL.Query().Get("refresh") == "1" ||
			strings.EqualFold(r.URL.Query().Get("refresh"), "true")
		// 后台已保存该应用时，默认直接返回本地模板，避免再消耗号池 RH Key
		if !forceRefresh {
			if tpl, err := s.store.GetWebappTemplateByWebappID(webappID); err == nil {
				writeJSON(w, http.StatusOK, demoFromSavedTemplate(tpl))
				return
			} else if !errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
		}
		rhKey := ""
		rhEmail := ""
		if acc, err := s.store.FindAccountWithAPIKey(); err == nil {
			rhKey = strings.TrimSpace(acc.APIKey)
			rhEmail = acc.Email
		}
		demo, err := runninghub.FetchWebappDemo(s.cfg.BaseURL, webappID, rhKey)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
		if rhEmail != "" {
			demo["rh_key_account"] = rhEmail
		}
		demo["cached"] = false
		writeJSON(w, http.StatusOK, demo)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) createVideo(w http.ResponseWriter, r *http.Request) {
	prompt := ""
	webappID := s.cfg.WebappID
	templateID := ""
	duration := "8"
	aspect := "3:4 (Portrait Standard)"
	accessPassword := ""
	var imagePath string
	var nodeInfoList []runninghub.NodeInput
	extraImages := map[string][]byte{}

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		prompt = strings.TrimSpace(r.FormValue("prompt"))
		if v := r.FormValue("webapp_id"); v != "" {
			webappID = v
		}
		if v := strings.TrimSpace(r.FormValue("template_id")); v != "" {
			templateID = v
		}
		if v := r.FormValue("duration"); v != "" {
			duration = v
		}
		if v := r.FormValue("aspect_ratio"); v != "" {
			aspect = v
		}
		if v := strings.TrimSpace(r.FormValue("access_password")); v != "" {
			accessPassword = v
		}
		if raw := strings.TrimSpace(r.FormValue("node_info_list")); raw != "" {
			if err := json.Unmarshal([]byte(raw), &nodeInfoList); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid node_info_list json"})
				return
			}
		}
		uploads := filepath.Join(s.cfg.DataDir, "uploads")
		_ = os.MkdirAll(uploads, 0o755)
		if file, hdr, err := r.FormFile("image"); err == nil {
			defer file.Close()
			name := time.Now().Format("20060102_150405") + "_" + sanitizeName(hdr.Filename)
			dst := filepath.Join(uploads, name)
			out, err := os.Create(dst)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			if _, err := io.Copy(out, file); err != nil {
				out.Close()
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			out.Close()
			imagePath = dst
		}
		if r.MultipartForm != nil {
			for key, files := range r.MultipartForm.File {
				if !strings.HasPrefix(key, "image_") || len(files) == 0 {
					continue
				}
				nodeID := strings.TrimPrefix(key, "image_")
				f, err := files[0].Open()
				if err != nil {
					continue
				}
				b, _ := io.ReadAll(f)
				f.Close()
				if len(b) > 0 {
					extraImages[nodeID] = b
				}
			}
		}
	} else {
		var body struct {
			Prompt         string                 `json:"prompt"`
			ImagePath      string                 `json:"image_path"`
			ImageBase64    string                 `json:"image_base64"`
			ImagesBase64   map[string]string      `json:"images_base64"`
			WebappID       string                 `json:"webapp_id"`
			TemplateID     string                 `json:"template_id"`
			Duration       string                 `json:"duration"`
			AspectRatio    string                 `json:"aspect_ratio"`
			AccessPassword string                 `json:"access_password"`
			NodeInfoList   []runninghub.NodeInput `json:"node_info_list"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
		prompt = strings.TrimSpace(body.Prompt)
		nodeInfoList = body.NodeInfoList
		if body.WebappID != "" {
			webappID = body.WebappID
		}
		if strings.TrimSpace(body.TemplateID) != "" {
			templateID = strings.TrimSpace(body.TemplateID)
		}
		if body.Duration != "" {
			duration = body.Duration
		}
		if body.AspectRatio != "" {
			aspect = body.AspectRatio
		}
		if strings.TrimSpace(body.AccessPassword) != "" {
			accessPassword = strings.TrimSpace(body.AccessPassword)
		}
		if body.ImagePath != "" {
			imagePath = body.ImagePath
		} else if body.ImageBase64 != "" {
			raw, err := decodeMaybeBase64(body.ImageBase64)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid image_base64"})
				return
			}
			uploads := filepath.Join(s.cfg.DataDir, "uploads")
			_ = os.MkdirAll(uploads, 0o755)
			dst := filepath.Join(uploads, time.Now().Format("20060102_150405")+"_image.png")
			if err := os.WriteFile(dst, raw, 0o644); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			imagePath = dst
		}
		for nodeID, b64 := range body.ImagesBase64 {
			raw, err := decodeMaybeBase64(b64)
			if err != nil || len(raw) == 0 {
				continue
			}
			extraImages[strings.TrimSpace(nodeID)] = raw
		}
	}

	useNodes := len(nodeInfoList) > 0
	if !useNodes && (prompt == "" || imagePath == "") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "prompt+image 或 webapp_id+node_info_list 必填其一"})
		return
	}
	if useNodes && strings.TrimSpace(webappID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "webapp_id required when using node_info_list"})
		return
	}
	if useNodes && prompt == "" {
		for _, n := range nodeInfoList {
			ft := strings.ToUpper(strings.TrimSpace(n.FieldType))
			if ft == "IMAGE" {
				continue
			}
			if s := strings.TrimSpace(fmt.Sprint(n.FieldValue)); s != "" && s != "<nil>" && len(s) >= 4 {
				prompt = s
				break
			}
		}
	}

	nodeInfoJSON := ""
	if useNodes {
		b, err := json.Marshal(nodeInfoList)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid node_info_list"})
			return
		}
		nodeInfoJSON = string(b)
	}

	id := "vid_" + randomID(12)
	ts := time.Now().UTC().Format(time.RFC3339)

	// 未显式传 template_id 时，按 webapp_id 反查已保存模板
	if templateID == "" && strings.TrimSpace(webappID) != "" {
		if tpl, err := s.store.GetWebappTemplateByWebappID(webappID); err == nil {
			templateID = tpl.ID
		}
	}

	// 先占号再落库，避免 queued 窗口被 worker 重复领取
	acc, acquireErr := s.pool.Acquire(id)
	status := "queued"
	if acquireErr == nil {
		status = "running"
	} else if !errors.Is(acquireErr, accounts.ErrUnavailable) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": acquireErr.Error()})
		return
	}

	task := store.VideoTask{
		ID: id, Status: status, Prompt: prompt, ImagePath: imagePath,
		WebappID: webappID, TemplateID: templateID, Duration: duration, AspectRatio: aspect,
		NodeInfoJSON: nodeInfoJSON,
		CreatedAt:    ts, UpdatedAt: ts,
	}
	if acquireErr == nil {
		task.AccountID = acc.ID
		task.AccountEmail = acc.Email
		task.Attempts = 1
	}
	if err := s.store.CreateTask(task); err != nil {
		if acquireErr == nil {
			_ = s.pool.ReleaseOK(acc.ID)
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	if acquireErr != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"id": id, "status": "queued", "rh_task_id": "",
			"webapp_id": webappID, "template_id": templateID,
			"message": "queued: no available account yet; poll GET /v1/videos/{id} for rh_task_id",
		})
		return
	}

	_ = s.store.UpdateTaskRunning(id, acc.ID, acc.Email, "")
	if accessPassword == "" {
		accessPassword = s.store.GetWebappAccessPassword(webappID)
	}
	in := runninghub.RunInput{
		Email:           acc.Email,
		Password:        acc.Password,
		AccessToken:     acc.AccessToken,
		AnonymousID:     acc.AnonymousID,
		FingerprintJSON: acc.FingerprintJSON,
		Prompt:          prompt,
		ImagePath:       imagePath,
		WebappID:        webappID,
		Duration:        duration,
		AspectRatio:     aspect,
		AccessPassword:  accessPassword,
		NodeInfoList:    nodeInfoList,
		ExtraImages:     extraImages,
		BaseURL:         s.cfg.BaseURL,
		OnProgress: func(p runninghub.Progress) {
			if p.TaskID != "" {
				_ = s.store.UpdateTaskRunning(id, acc.ID, acc.Email, p.TaskID)
			}
			_ = s.store.UpdateTaskProgress(id, p.RHStatus, p.ProgressText, p.QueueIndex, p.QueueCount)
			if p.AccessToken != "" {
				_ = s.store.UpdateAccountToken(acc.ID, p.AccessToken)
			}
		},
	}
	egSess, egErr := s.egress.NewClientForAccount(acc, s.cfg.BaseURL)
	if egErr != nil {
		_ = s.pool.ReleaseFail(acc.ID, "EGRESS", egErr.Error())
		_ = s.store.RequeueTask(id, "EGRESS", egErr.Error())
		writeJSON(w, http.StatusAccepted, map[string]any{
			"id": id, "status": "queued", "rh_task_id": "",
			"webapp_id": webappID, "error_code": "EGRESS", "error": egErr.Error(),
			"message": "egress bind failed; requeued",
		})
		return
	}
	egress.AttachHTTP(&in, egSess)
	sess, submitErr := runninghub.SubmitWebappVideo(in)
	if submitErr != nil && egress.IsConnectivityError(submitErr) && egSess.Node != nil {
		if eg2, reErr := s.egress.Rebind(acc, s.cfg.BaseURL, egSess.Node, submitErr.Error()); reErr == nil {
			egress.AttachHTTP(&in, eg2)
			sess, submitErr = runninghub.SubmitWebappVideo(in)
			if submitErr == nil {
				s.egress.MarkOK(eg2)
			}
		}
	} else if submitErr == nil {
		s.egress.MarkOK(egSess)
	}
	if submitErr != nil {
		code := "RH"
		msg := submitErr.Error()
		low := strings.ToLower(msg)
		switch {
		case strings.Contains(low, "login"), strings.Contains(low, "token_invalid"), strings.Contains(low, "user_not_found"):
			code = "AUTH"
		case strings.Contains(low, "create_max_tasks_limit"), strings.Contains(low, "423"):
			code = "BUSY"
		case strings.Contains(low, "prompt"), strings.Contains(low, "sensitive"):
			code = "PROMPT"
		}
		_ = s.pool.ReleaseFail(acc.ID, code, msg)
		if code == "BUSY" || code == "AUTH" {
			_ = s.store.RequeueTask(id, code, msg)
			writeJSON(w, http.StatusAccepted, map[string]any{
				"id": id, "status": "queued", "rh_task_id": "",
				"webapp_id": webappID, "error_code": code, "error": msg,
				"message": "submit deferred; poll GET /v1/videos/{id} for rh_task_id",
			})
			return
		}
		_ = s.store.FinishTaskFail(id, code, msg)
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"id": id, "status": "failed", "rh_task_id": "", "webapp_id": webappID, "error_code": code, "error": msg,
		})
		return
	}

	_ = s.store.UpdateTaskRunning(id, acc.ID, acc.Email, sess.TaskID)
	_ = s.store.UpdateTaskProgress(id, "QUEUED", "已提交 RH，等待排队", 0, 0)
	task.Status = "running"
	task.AccountID = acc.ID
	task.AccountEmail = acc.Email
	task.RHTaskID = sess.TaskID
	task.Attempts = 1

	if s.worker != nil {
		go s.worker.FinishAfterSubmit(task, acc, sess, in)
	} else {
		log.Printf("[task %s] no worker; rh_task_id=%s left without poller", id, sess.TaskID)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":            id,
		"status":        "running",
		"rh_task_id":    sess.TaskID,
		"webapp_id":     webappID,
		"template_id":   templateID,
		"account_id":    acc.ID,
		"account_email": acc.Email,
	})
}

func (s *Server) handleVideoByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/videos/")
	id = strings.Trim(id, "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing id"})
		return
	}
	if strings.HasSuffix(id, "/cancel") && r.Method == http.MethodPost {
		realID := strings.TrimSuffix(id, "/cancel")
		ok, err := s.store.CancelTask(realID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": realID, "cancelled": ok})
		return
	}
	if (strings.HasSuffix(id, "/sync") || strings.HasSuffix(id, "/query")) && r.Method == http.MethodPost {
		realID := strings.TrimSuffix(strings.TrimSuffix(id, "/sync"), "/query")
		realID = strings.TrimSuffix(realID, "/")
		if s.worker == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "worker unavailable"})
			return
		}
		if err := s.worker.SyncAndMaybePoll(realID, false); err != nil {
			// still return latest task row so UI can refresh
			task, _ := s.store.GetTask(realID)
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "task": task})
			return
		}
		task, err := s.store.GetTask(realID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "task": task})
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	task, err := s.store.GetTask(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleAdminAccounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.store.ListAccounts()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": list})
	case http.MethodPost:
		s.handleCreateAccount(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var body store.Account
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	body.Email = strings.TrimSpace(body.Email)
	if body.Email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "email required"})
		return
	}
	if strings.TrimSpace(body.Password) == "" && strings.TrimSpace(body.AccessToken) == "" && strings.TrimSpace(body.APIKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "password / access_token / api_key 至少填一项"})
		return
	}
	if body.Source == "" {
		body.Source = "manual"
	}
	body.Source = normalizeAccountSource(body.Source)
	if body.Status == "" {
		body.Status = "active"
	}
	acc, err := s.store.UpsertAccount(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	acc.PublicView()
	writeJSON(w, http.StatusOK, acc)
}

func (s *Server) handleAdminAccountByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin/accounts/")
	path = strings.Trim(path, "/")
	if path == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if path == "import" {
		s.handleImportAccounts(w, r)
		return
	}
	parts := strings.Split(path, "/")
	idStr := parts[0]
	var id int64
	if _, err := fmt.Sscan(idStr, &id); err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid id"})
		return
	}
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	switch {
	case action == "" && r.Method == http.MethodGet:
		acc, err := s.store.GetAccount(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Pragma", "no-cache")
		acc.AdminView()
		writeJSON(w, http.StatusOK, acc)
	case action == "" && r.Method == http.MethodDelete:
		if err := s.store.DeleteAccount(id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
				return
			}
			if errors.Is(err, store.ErrAccountBusy) {
				writeJSON(w, http.StatusConflict, map[string]any{"error": "账号占用中，无法删除"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
	case action == "refresh" && r.Method == http.MethodPost:
		s.refreshAccount(w, id)
	case action == "ensure-api-key" && r.Method == http.MethodPost:
		force := false
		var body struct {
			Force bool `json:"force"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		force = body.Force
		acc, result, err := s.ensureAccountAPIKey(id, force)
		if err != nil {
			status := http.StatusBadGateway
			if errors.Is(err, store.ErrNotFound) {
				status = http.StatusNotFound
			} else if strings.Contains(err.Error(), "缺少") || strings.Contains(err.Error(), "已有") {
				status = http.StatusBadRequest
			}
			writeJSON(w, status, map[string]any{"error": err.Error()})
			return
		}
		acc.AdminView()
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"result": result,
			"account": acc,
		})
	case action == "disable" && r.Method == http.MethodPost:
		_ = s.store.DisableAccount(id, "manual disable")
		acc, err := s.store.GetAccount(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		acc.PublicView()
		writeJSON(w, http.StatusOK, acc)
	case action == "enable" && r.Method == http.MethodPost:
		_ = s.store.EnableAccount(id)
		acc, err := s.store.GetAccount(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		acc.PublicView()
		writeJSON(w, http.StatusOK, acc)
	case action == "max-concurrency" && r.Method == http.MethodPost:
		var body struct {
			MaxConcurrency int `json:"max_concurrency"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
		if body.MaxConcurrency < 1 || body.MaxConcurrency > 64 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "max_concurrency 需在 1..64"})
			return
		}
		if err := s.store.SetAccountMaxConcurrency(id, body.MaxConcurrency); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		acc, err := s.store.GetAccount(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		acc.PublicView()
		writeJSON(w, http.StatusOK, acc)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) refreshAccount(w http.ResponseWriter, id int64) {
	acc, err := s.store.GetAccount(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	var st map[string]any
	err = s.egress.Do(acc, s.cfg.BaseURL, func(client *runninghub.Client) error {
		apiKey := strings.TrimSpace(acc.APIKey)
		if apiKey == "" {
			got, e := client.EnsureConsumerApiKey(acc.Email, acc.Password, acc.AccessToken)
			if e != nil {
				return fmt.Errorf("补 api_key 失败: %w", e)
			}
			apiKey = got.APIKey
			_ = s.store.UpdateAccountAPIKey(id, apiKey)
			_ = s.store.UpdateAccountAuthTokens(id, got.AccessToken, got.RefreshToken)
			acc.APIKey = apiKey
		}
		var e error
		st, e = client.AccountStatus(apiKey)
		if e != nil {
			got, e2 := client.EnsureConsumerApiKey(acc.Email, acc.Password, "")
			if e2 != nil {
				return e
			}
			apiKey = got.APIKey
			_ = s.store.UpdateAccountAPIKey(id, apiKey)
			_ = s.store.UpdateAccountAuthTokens(id, got.AccessToken, got.RefreshToken)
			acc.APIKey = apiKey
			st, e = client.AccountStatus(apiKey)
			return e
		}
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	data, _ := st["data"].(map[string]any)
	remain := ""
	counts := ""
	apiType := ""
	if data != nil {
		if v, ok := data["remainCoins"]; ok {
			remain = fmt.Sprint(v)
		}
		if v, ok := data["currentTaskCounts"]; ok {
			counts = fmt.Sprint(v)
		}
		if v, ok := data["apiType"]; ok {
			apiType = fmt.Sprint(v)
		}
	}
	_ = s.store.UpdateAccountBalance(id, remain, counts, apiType)
	acc, _ = s.store.GetAccount(id)
	acc.PublicView()
	writeJSON(w, http.StatusOK, acc)
}

func (s *Server) handleClaimDailyLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.dailyClaimer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "daily claimer unavailable"})
		return
	}
	var body struct {
		IDs   []int64 `json:"ids"`
		Force bool    `json:"force"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	results, err := s.dailyClaimer.ClaimAll(body.IDs, body.Force)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	ok, skipped, failed, gained := 0, 0, 0, 0
	for _, item := range results {
		if item.Skipped {
			skipped++
		} else if item.OK {
			ok++
			gained += item.Gained
		} else {
			failed++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"day":     accounts.ShanghaiDate(time.Now()),
		"success": ok,
		"skipped": skipped,
		"failed":  failed,
		"gained":  gained,
		"results": results,
	})
}

func (s *Server) handleEnsureAPIKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		IDs   []int64 `json:"ids"`
		Force bool    `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if len(body.IDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "ids required"})
		return
	}
	success, skipped, failed := 0, 0, 0
	results := make([]map[string]any, 0, len(body.IDs))
	for _, id := range body.IDs {
		acc, result, err := s.ensureAccountAPIKey(id, body.Force)
		item := map[string]any{"id": id}
		if acc.Email != "" {
			item["email"] = acc.Email
		}
		if err != nil {
			failed++
			item["ok"] = false
			item["error"] = err.Error()
			results = append(results, item)
			continue
		}
		if skippedFlag, _ := result["skipped"].(bool); skippedFlag {
			skipped++
		} else {
			success++
		}
		item["ok"] = true
		item["result"] = result
		item["has_api_key"] = true
		results = append(results, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": success,
		"skipped": skipped,
		"failed":  failed,
		"results": results,
	})
}

// ensureAccountAPIKey mirrors gpt-plus ensure-api-key: login → createConsumer → reset.
func (s *Server) ensureAccountAPIKey(id int64, force bool) (store.Account, map[string]any, error) {
	acc, err := s.store.GetAccount(id)
	if err != nil {
		return store.Account{}, nil, err
	}
	existing := strings.TrimSpace(acc.APIKey)
	if existing != "" && !force {
		return acc, map[string]any{
			"skipped":  true,
			"code":     "RH_API_KEY_EXISTS",
			"message":  "已有 API Key，跳过（可 force 重置）",
			"api_key":  existing,
			"source":   "existing",
		}, nil
	}
	if strings.TrimSpace(acc.Password) == "" && strings.TrimSpace(acc.AccessToken) == "" {
		return acc, nil, fmt.Errorf("缺少密码与 Access Token，无法登录拿 Key")
	}

	var got runninghub.EnsureKeyResult
	remain, counts, apiType := "", "", ""
	err = s.egress.Do(acc, s.cfg.BaseURL, func(client *runninghub.Client) error {
		var e error
		got, e = client.EnsureConsumerApiKey(acc.Email, acc.Password, acc.AccessToken)
		if e != nil {
			return e
		}
		_ = s.store.UpdateAccountAPIKey(id, got.APIKey)
		_ = s.store.UpdateAccountAuthTokens(id, got.AccessToken, got.RefreshToken)
		if st, e2 := client.AccountStatus(got.APIKey); e2 == nil {
			if data, _ := st["data"].(map[string]any); data != nil {
				if v, ok := data["remainCoins"]; ok {
					remain = fmt.Sprint(v)
				}
				if v, ok := data["currentTaskCounts"]; ok {
					counts = fmt.Sprint(v)
				}
				if v, ok := data["apiType"]; ok {
					apiType = fmt.Sprint(v)
				}
				_ = s.store.UpdateAccountBalance(id, remain, counts, apiType)
			}
		}
		return nil
	})
	if err != nil {
		return acc, nil, err
	}

	acc, _ = s.store.GetAccount(id)
	msg := "已补 Key"
	if force {
		msg = "已重置并更新 Key"
	}
	return acc, map[string]any{
		"skipped":           false,
		"code":              "OK",
		"message":           msg,
		"source":            got.Source,
		"relogged":          got.Relogged,
		"remain_coins":      remain,
		"api_type":          apiType,
		"has_api_key":       true,
		"has_access_token":  strings.TrimSpace(acc.AccessToken) != "",
		"has_refresh_token": strings.TrimSpace(acc.RefreshToken) != "",
	}, nil
}

func (s *Server) handleImportAccounts(w http.ResponseWriter, r *http.Request) {
	s.handlePushAccounts(w, r)
}

// handlePushAccounts upserts one or many accounts for console import / gpt-plus sync.
// Body accepts:
//   { "account": { ... } }
//   { "accounts": [ ... ] }
//   [ ... ]
func (s *Server) handlePushAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "read body failed"})
		return
	}
	accounts, defaultSource, err := parseAccountPushBody(raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if len(accounts) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "account/accounts required"})
		return
	}

	ok, fail := 0, 0
	out := make([]store.Account, 0, len(accounts))
	errs := make([]map[string]any, 0)
	for _, a := range accounts {
		if a.Source == "" {
			a.Source = defaultSource
		}
		a.Source = normalizeAccountSource(a.Source)
		if a.Status == "" {
			a.Status = "active"
		}
		if strings.TrimSpace(a.Email) == "" {
			fail++
			errs = append(errs, map[string]any{"email": a.Email, "error": "email required"})
			continue
		}
		acc, upsertErr := s.store.UpsertAccount(a)
		if upsertErr != nil {
			fail++
			errs = append(errs, map[string]any{"email": a.Email, "error": upsertErr.Error()})
			continue
		}
		acc.PublicView()
		out = append(out, acc)
		ok++
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"imported": ok,
		"failed":   fail,
		"data":     out,
		"errors":   errs,
	})
}

func parseAccountPushBody(raw []byte) ([]store.Account, string, error) {
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 {
		return nil, "", fmt.Errorf("empty body")
	}
	if trim[0] == '[' {
		var list []store.Account
		if err := json.Unmarshal(trim, &list); err != nil {
			return nil, "", fmt.Errorf("invalid accounts array")
		}
		return list, "sync", nil
	}
	var body struct {
		Account  *store.Account  `json:"account"`
		Accounts []store.Account `json:"accounts"`
		Source   string          `json:"source"`
		// flat single-account fields also accepted at top level
		Email           string `json:"email"`
		Password        string `json:"password"`
		AccessToken     string `json:"access_token"`
		RefreshToken    string `json:"refresh_token"`
		APIKey          string `json:"api_key"`
		Identify        string `json:"identify"`
		RHUserID        string `json:"rh_user_id"`
		RemainCoins     string `json:"remain_coins"`
		AnonymousID     string `json:"anonymous_id"`
		DeviceID        string `json:"device_id"`
		FingerprintJSON string `json:"fingerprint_json"`
		Status          string `json:"status"`
		ExternalID      string `json:"external_id"`
	}
	if err := json.Unmarshal(trim, &body); err != nil {
		return nil, "", fmt.Errorf("invalid json")
	}
	src := strings.TrimSpace(body.Source)
	if src == "" {
		src = "sync"
	}
	if body.Account != nil {
		return []store.Account{*body.Account}, src, nil
	}
	if len(body.Accounts) > 0 {
		return body.Accounts, src, nil
	}
	if strings.TrimSpace(body.Email) != "" {
		return []store.Account{{
			Email: body.Email, Password: body.Password, AccessToken: body.AccessToken,
			RefreshToken: body.RefreshToken, APIKey: body.APIKey, Identify: body.Identify,
			RHUserID: body.RHUserID, RemainCoins: body.RemainCoins,
			AnonymousID: body.AnonymousID, DeviceID: body.DeviceID, FingerprintJSON: body.FingerprintJSON,
			Status: body.Status, Source: src, ExternalID: body.ExternalID,
		}}, src, nil
	}
	return nil, src, fmt.Errorf("account/accounts required")
}

func (s *Server) handleAdminTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	list, err := s.store.ListTasks(200)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func normalizeAccountSource(src string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return "sync"
	}
	switch strings.ToLower(src) {
	case "gpt-plus", "gptplus", "gpt_plus":
		return "sync"
	default:
		return src
	}
}

func randomID(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}

func sanitizeName(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", ".")
	if name == "" {
		return "image.png"
	}
	return name
}

func decodeMaybeBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, ","); i >= 0 && strings.Contains(s[:i], "base64") {
		s = s[i+1:]
	}
	if raw, err := base64.StdEncoding.DecodeString(s); err == nil {
		return raw, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}

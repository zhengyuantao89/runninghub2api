package httpapi

import (
	"bytes"
	"crypto/rand"
	"database/sql"
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
	s.mux.HandleFunc("/api/register", s.handleRegister)
	s.mux.HandleFunc("/api/logout", s.withActor(s.handleLogout))
	s.mux.HandleFunc("/api/me", s.withActor(s.handleMe))
	s.mux.HandleFunc("/api/console", s.withActor(s.handleConsole))
	s.mux.HandleFunc("/api/keys", s.withActor(s.handleUserKeys))
	s.mux.HandleFunc("/api/keys/", s.withActor(s.handleUserKeyByID))
	s.mux.HandleFunc("/api/registration", s.handleRegistrationStatus)

	s.mux.HandleFunc("/admin/settings", s.withActor(s.handleAdminSettings))
	s.mux.HandleFunc("/admin/credentials", s.withAdmin(s.handleAdminCredentials))
	s.mux.HandleFunc("/admin/users", s.withAdmin(s.handleAdminUsers))
	s.mux.HandleFunc("/admin/users/", s.withAdmin(s.handleAdminUserByID))

	// OpenAI 兼容：模型列表 / 生图 / 生视频
	s.mux.HandleFunc("/v1/models", s.withUserSubmit(s.handleModels))
	s.mux.HandleFunc("/v1/models/", s.withUserSubmit(s.handleModels))
	s.mux.HandleFunc("/v1/images/generations", s.withUserSubmit(s.handleImages))
	s.mux.HandleFunc("/v1/images/", s.withUserSubmit(s.handleImages))
	s.mux.HandleFunc("/v1/videos", s.withUserSubmit(s.handleVideos))
	s.mux.HandleFunc("/v1/videos/", s.withUserSubmit(s.handleVideoByID))
	// Runow QC 模型仓（免费+付费）
	s.mux.HandleFunc("/v1/qc/catalog", s.withActor(s.handleQCCatalog))
	// 模板市场：登录可读；写权限在 handler 内区分
	s.mux.HandleFunc("/v1/webapps/", s.withActor(s.handleWebappByID))
	s.mux.HandleFunc("/admin/accounts/import", s.withActor(s.handleImportAccounts))
	s.mux.HandleFunc("/admin/accounts/ensure-api-key", s.withActor(s.handleEnsureAPIKeys))
	s.mux.HandleFunc("/admin/accounts/claim-daily-login", s.withActor(s.handleClaimDailyLogin))
	s.mux.HandleFunc("/admin/accounts/", s.withActor(s.handleAdminAccountByID))
	s.mux.HandleFunc("/admin/accounts", s.withActor(s.handleAdminAccounts))
	s.mux.HandleFunc("/admin/tasks", s.withActor(s.handleAdminTasks))
	s.mux.HandleFunc("/admin/templates", s.withActor(s.handleAdminTemplates))
	s.mux.HandleFunc("/admin/templates/", s.withActor(s.handleAdminTemplateByID))
	s.mux.HandleFunc("/admin/proxies/import", s.withAdmin(s.handleImportProxies))
	s.mux.HandleFunc("/admin/proxies/groups", s.withAdmin(s.handleProxyGroups))
	s.mux.HandleFunc("/admin/proxies/subscriptions/refresh", s.withAdmin(s.handleProxySubscriptionRefresh))
	s.mux.HandleFunc("/admin/proxies/subscriptions", s.withAdmin(s.handleProxySubscriptions))
	s.mux.HandleFunc("/admin/proxies/", s.withAdmin(s.handleAdminProxyByID))
	s.mux.HandleFunc("/admin/proxies", s.withAdmin(s.handleAdminProxies))

	// 普通用户可用个人 API Key 推送到自己的号池；管理员可指定 user_id。
	s.mux.HandleFunc("/admin/sync/accounts", s.withActor(s.handlePushAccounts))
	s.mux.HandleFunc("/api/accounts/push", s.withActor(s.handlePushAccounts))
	s.mux.HandleFunc("/api/proxies/push", s.withAdmin(s.handlePushProxies))
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

func (s *Server) handleRegistrationStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": s.store.RegistrationEnabled()})
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
	u, err := s.store.GetUserByUsername(user)
	if err != nil || !store.CheckPassword(u.PasswordHash, pass) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "账号或密码错误"})
		return
	}
	if u.Status != store.UserActive {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "账号已禁用"})
		return
	}
	token := s.sessions.Create(u)
	writeJSON(w, http.StatusOK, map[string]any{
		"token":    token,
		"username": u.Username,
		"role":     u.Role,
		"user_id":  u.ID,
	})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.store.RegistrationEnabled() {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "公开注册已关闭，请联系管理员"})
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
	if len(user) < 3 || len(pass) < 6 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "用户名至少 3 位，密码至少 6 位"})
		return
	}
	hash, err := store.HashPassword(pass)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	u, err := s.store.CreateUser(user, hash, store.RoleUser)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	token := s.sessions.Create(u)
	writeJSON(w, http.StatusOK, map[string]any{
		"token":    token,
		"username": u.Username,
		"role":     u.Role,
		"user_id":  u.ID,
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
	a, _ := actorFromRequest(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":  a.UserID,
		"username": a.Username,
		"role":     a.Role,
		"via":      a.Via,
	})
}

func (s *Server) handleConsole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	a, _ := actorFromRequest(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":               a.UserID,
		"username":              a.Username,
		"role":                  a.Role,
		"poll_auth":             s.resolvePollAuth(),
		"registration_enabled":  s.store.RegistrationEnabled(),
		"hint":                  "请在「API Key」页创建个人 Key 调用 /v1/videos；管理员与普通用户均可提交。",
	})
}

func (s *Server) resolvePollAuth() string {
	if v, err := s.store.GetSetting(store.SettingPollAuth); err == nil && strings.TrimSpace(v) != "" {
		return config.NormalizePollAuth(v)
	}
	return config.NormalizePollAuth(s.cfg.PollAuth)
}

func (s *Server) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	a, _ := actorFromRequest(r)
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"poll_auth":               s.resolvePollAuth(),
			"poll_auth_default":       config.NormalizePollAuth(s.cfg.PollAuth),
			"registration_enabled":    s.store.RegistrationEnabled(),
			"upload_keep_hours":       s.store.UploadKeepHours(s.cfg.UploadKeepHours),
			"upload_keep_hours_default": store.NormalizeUploadKeepHours(s.cfg.UploadKeepHours),
			"role":                    a.Role,
		})
	case http.MethodPut, http.MethodPost:
		var body struct {
			PollAuth            *string `json:"poll_auth"`
			RegistrationEnabled *bool   `json:"registration_enabled"`
			UploadKeepHours     *int    `json:"upload_keep_hours"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
		out := map[string]any{"ok": true}
		if body.PollAuth != nil {
			mode := config.NormalizePollAuth(*body.PollAuth)
			if err := s.store.SetSetting(store.SettingPollAuth, mode); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			out["poll_auth"] = mode
		}
		if body.RegistrationEnabled != nil {
			if !a.IsAdmin() {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "仅管理员可开关公开注册"})
				return
			}
			if err := s.store.SetRegistrationEnabled(*body.RegistrationEnabled); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			out["registration_enabled"] = *body.RegistrationEnabled
		}
		if body.UploadKeepHours != nil {
			if !a.IsAdmin() {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "仅管理员可设置上传清理"})
				return
			}
			hours := store.NormalizeUploadKeepHours(*body.UploadKeepHours)
			if err := s.store.SetUploadKeepHours(hours); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			out["upload_keep_hours"] = hours
		}
		writeJSON(w, http.StatusOK, out)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleVideos(w http.ResponseWriter, r *http.Request) {
	a, _ := actorFromRequest(r)
	switch r.Method {
	case http.MethodGet:
		list, err := s.store.ListTasks(a.UserID, 100)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": list})
	case http.MethodPost:
		// OpenAI 风格免费视频 / 显式 sync|async → 统一包装；模板异步创建仍走原逻辑。
		if s.maybeOpenAIVideoResponse(w, r, s.createVideo) {
			return
		}
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
	resolution := ""
	accessPassword := ""
	submitMode := ""
	bindingID := ""
	skuID := ""
	categoryID := ""
	creationMode := ""
	allowPaid := false
	qcParams := map[string]any{}
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
		submitMode = strings.TrimSpace(r.FormValue("submit_mode"))
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
		if v := strings.TrimSpace(r.FormValue("resolution")); v != "" {
			resolution = v
		}
		bindingID = strings.TrimSpace(r.FormValue("binding_id"))
		skuID = strings.TrimSpace(r.FormValue("sku_id"))
		categoryID = strings.TrimSpace(r.FormValue("category_id"))
		creationMode = strings.TrimSpace(r.FormValue("creation_mode"))
		if creationMode == "" {
			creationMode = strings.TrimSpace(r.FormValue("free_image_model"))
		}
		if creationMode == "" {
			creationMode = strings.TrimSpace(r.FormValue("free_video_mode"))
		}
		allowPaid = true // generic qc defaults to allowing paid unless explicitly disabled
		if r.MultipartForm != nil {
			if _, ok := r.MultipartForm.Value["allow_paid"]; ok {
				allowPaid = parseTruthy(r.FormValue("allow_paid"))
			}
		} else if _, ok := r.Form["allow_paid"]; ok {
			allowPaid = parseTruthy(r.FormValue("allow_paid"))
		}
		if raw := strings.TrimSpace(r.FormValue("qc_params")); raw != "" {
			_ = json.Unmarshal([]byte(raw), &qcParams)
		}
		if v := strings.TrimSpace(r.FormValue("free_video_mode")); v != "" {
			if qcParams == nil {
				qcParams = map[string]any{}
			}
			qcParams["free_video_mode"] = v
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
			Resolution     string                 `json:"resolution"`
			AccessPassword string                 `json:"access_password"`
			SubmitMode     string                 `json:"submit_mode"`
			BindingID      string                 `json:"binding_id"`
			SkuID          string                 `json:"sku_id"`
			CategoryID     string                 `json:"category_id"`
			CreationMode   string                 `json:"creation_mode"`
			AllowPaid      *bool                  `json:"allow_paid"`
			QCParams       map[string]any         `json:"qc_params"`
			NodeInfoList   []runninghub.NodeInput `json:"node_info_list"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
		prompt = strings.TrimSpace(body.Prompt)
		submitMode = strings.TrimSpace(body.SubmitMode)
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
		if strings.TrimSpace(body.Resolution) != "" {
			resolution = strings.TrimSpace(body.Resolution)
		}
		bindingID = strings.TrimSpace(body.BindingID)
		skuID = strings.TrimSpace(body.SkuID)
		categoryID = strings.TrimSpace(body.CategoryID)
		creationMode = strings.TrimSpace(body.CreationMode)
		if body.AllowPaid != nil {
			allowPaid = *body.AllowPaid
		} else {
			allowPaid = true
		}
		if body.QCParams != nil {
			qcParams = body.QCParams
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

	// Per-request submit_mode overrides env; qc_free / qc_free_image = Runow official free (not our webapp).
	resolvedMode := config.NormalizeSubmitMode(s.cfg.SubmitMode)
	if strings.TrimSpace(submitMode) != "" {
		resolvedMode = config.NormalizeSubmitMode(submitMode)
	}
	useQCVideo := resolvedMode == runninghub.SubmitModeQCFree
	useQCImage := resolvedMode == runninghub.SubmitModeQCFreeImage
	useQCGeneric := resolvedMode == runninghub.SubmitModeQC
	useQC := useQCVideo || useQCImage || useQCGeneric

	if useQCGeneric {
		nodeInfoList = nil
		templateID = ""
		if categoryID == "" {
			categoryID = "VIDEO"
		}
		if bindingID == "" || skuID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "qc 提交需要 binding_id + sku_id"})
			return
		}
		if prompt == "" && strings.TrimSpace(fmt.Sprint(qcParams["prompt"])) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "qc 提交需要 prompt"})
			return
		}
		prefix := "qc:"
		if strings.EqualFold(categoryID, "IMAGE") {
			prefix = "qcimg:"
		}
		webappID = prefix + skuID
		// Persist QC meta for queued worker.
		meta := runninghub.QCTaskMeta{
			QC:           true,
			BindingID:    bindingID,
			SkuID:        skuID,
			CategoryID:   categoryID,
			CreationMode: creationMode,
			AllowPaid:    allowPaid,
			Params:       qcParams,
		}
		if nodes, err := runninghub.EncodeQCTaskMetaNode(meta); err == nil {
			nodeInfoList = nodes
		}
		// Persist extra local images so queued worker can upload them (ExtraImages is request-only).
		if len(extraImages) > 0 {
			uploadsDir := filepath.Join(s.cfg.DataDir, "uploads")
			_ = os.MkdirAll(uploadsDir, 0o755)
			i := 0
			for key, raw := range extraImages {
				if len(raw) == 0 {
					continue
				}
				i++
				name := time.Now().Format("20060102_150405") + "_qc_" + sanitizeName(key) + ".png"
				dst := filepath.Join(uploadsDir, name)
				if err := os.WriteFile(dst, raw, 0o644); err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
					return
				}
				if imagePath == "" {
					imagePath = dst
					continue
				}
				nodeInfoList = append(nodeInfoList, runninghub.NodeInput{
					NodeID: fmt.Sprintf("qc_ref_%d", i), FieldName: "image", FieldType: "IMAGE", FieldValue: dst,
				})
			}
		}
	} else if useQCVideo {
		// Official free video: Minimax H3-RH Enhanced — multimodal / freeCreation.
		nodeInfoList = nil
		templateID = ""
		if strings.TrimSpace(duration) == "" {
			duration = "4"
		}
		if strings.TrimSpace(aspect) == "" {
			aspect = "3:2 (Photo)"
		}
		if strings.TrimSpace(resolution) == "" {
			resolution = "480p"
		}
		if prompt == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "官方免费视频需要 prompt"})
			return
		}
		mode := runninghub.NormalizeQCFreeVideoMode(creationMode)
		if mode == "" {
			mode = runninghub.NormalizeQCFreeVideoMode(fmt.Sprint(qcParams["free_video_mode"]))
		}
		imgCount := 0
		if imagePath != "" {
			imgCount++
		}
		imgCount += len(extraImages)
		if mode == runninghub.QCFreeVideoModeMultimodal && imgCount == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "多模态参考至少需要一张参考图"})
			return
		}
		bid, sid, resolvedMode := runninghub.ResolveQCFreeVideoModel(mode, imgCount)
		bindingID, skuID, creationMode = bid, sid, resolvedMode
		categoryID = "VIDEO"
		webappID = "qc:" + sid
		if qcParams == nil {
			qcParams = map[string]any{}
		}
		qcParams["free_video_mode"] = resolvedMode
		// Persist extra local images for multimodal / queued worker.
		if len(extraImages) > 0 {
			uploadsDir := filepath.Join(s.cfg.DataDir, "uploads")
			_ = os.MkdirAll(uploadsDir, 0o755)
			extraNodes := make([]runninghub.NodeInput, 0, len(extraImages))
			i := 0
			for key, raw := range extraImages {
				if len(raw) == 0 {
					continue
				}
				i++
				name := time.Now().Format("20060102_150405") + "_qcvideo_" + sanitizeName(key) + ".png"
				dst := filepath.Join(uploadsDir, name)
				if err := os.WriteFile(dst, raw, 0o644); err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
					return
				}
				if imagePath == "" {
					imagePath = dst
					continue
				}
				extraNodes = append(extraNodes, runninghub.NodeInput{
					NodeID: fmt.Sprintf("qc_ref_%d", i), FieldName: "image", FieldType: "IMAGE", FieldValue: dst,
				})
			}
			nodeInfoList = extraNodes
		}
	} else if useQCImage {
		// Official free image families: Nano Banana2 / GPT Image 2.0.
		// Same family: with refs → i2i sku, without → t2i sku.
		templateID = ""
		if strings.TrimSpace(aspect) == "" {
			aspect = "9:16"
		}
		if strings.TrimSpace(resolution) == "" {
			resolution = "1k"
		}
		if prompt == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "官方免费生图需要 prompt"})
			return
		}
		family := strings.TrimSpace(creationMode)
		if family == "" {
			family = strings.TrimSpace(fmt.Sprint(qcParams["free_image_model"]))
		}
		hasImg := imagePath != "" || len(extraImages) > 0
		bid, sid, fam := runninghub.ResolveQCFreeImageModel(bindingID, skuID, family, hasImg)
		bindingID, skuID, creationMode = bid, sid, fam
		categoryID = "IMAGE"
		webappID = "qcimg:" + sid
		// Persist extra local images into node_info_json so queued worker can upload them.
		if len(extraImages) > 0 {
			uploadsDir := filepath.Join(s.cfg.DataDir, "uploads")
			_ = os.MkdirAll(uploadsDir, 0o755)
			extraNodes := make([]runninghub.NodeInput, 0, len(extraImages))
			i := 0
			for key, raw := range extraImages {
				if len(raw) == 0 {
					continue
				}
				i++
				name := time.Now().Format("20060102_150405") + "_qcimg_" + sanitizeName(key) + ".png"
				dst := filepath.Join(uploadsDir, name)
				if err := os.WriteFile(dst, raw, 0o644); err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
					return
				}
				if imagePath == "" {
					imagePath = dst
					continue
				}
				extraNodes = append(extraNodes, runninghub.NodeInput{
					NodeID: fmt.Sprintf("qc_ref_%d", i), FieldName: "image", FieldType: "IMAGE", FieldValue: dst,
				})
			}
			nodeInfoList = extraNodes
		}
		if len(extraImages) == 0 && len(nodeInfoList) > 0 {
			// keep caller-provided nodes only if they look like image path carriers
		} else if len(extraImages) == 0 {
			nodeInfoList = nil
		}
	} else {
		useNodesWebapp := len(nodeInfoList) > 0
		if !useNodesWebapp && (prompt == "" || imagePath == "") {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "prompt+image 或 webapp_id+node_info_list 必填其一"})
			return
		}
		if useNodesWebapp && strings.TrimSpace(webappID) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "webapp_id required when using node_info_list"})
			return
		}
		if useNodesWebapp && prompt == "" {
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
	}

	// Persist node list for webapp templates, QC image refs, and QC generic meta.
	useNodes := len(nodeInfoList) > 0 && (!useQC || useQCImage || useQCGeneric || useQCVideo)
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
	actor, _ := actorFromRequest(r)

	// 未显式传 template_id 时，按 webapp_id 反查已保存模板
	if templateID == "" && strings.TrimSpace(webappID) != "" {
		if tpl, err := s.store.GetWebappTemplateByWebappID(webappID); err == nil {
			templateID = tpl.ID
		}
	}

	// 先占号再落库，避免 queued 窗口被 worker 重复领取。
	// 管理员可调度全站号池；普通用户仅本用户号池。
	acquireUID := actor.UserID
	if actor.IsAdmin() {
		acquireUID = 0
	}
	acc, acquireErr := s.pool.Acquire(acquireUID, id)
	status := "queued"
	if acquireErr == nil {
		status = "running"
	} else if !errors.Is(acquireErr, accounts.ErrUnavailable) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": acquireErr.Error()})
		return
	}

	task := store.VideoTask{
		ID: id, UserID: actor.UserID, Status: status, Prompt: prompt, ImagePath: imagePath,
		WebappID: webappID, TemplateID: templateID, Duration: duration, AspectRatio: aspect, Resolution: resolution,
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
		Resolution:      resolution,
		BindingID:       bindingID,
		SkuID:           skuID,
		CategoryID:      categoryID,
		CreationMode:    creationMode,
		QCParams:        qcParams,
		AllowPaid:       allowPaid,
		AccessPassword:  accessPassword,
		NodeInfoList:    nodeInfoList,
		ExtraImages:     extraImages,
		BaseURL:         s.cfg.BaseURL,
		OnProgress: func(p runninghub.Progress) {
			if p.TaskID != "" {
				_ = s.store.UpdateTaskRunning(id, acc.ID, acc.Email, p.TaskID)
			}
			_ = s.store.UpdateTaskProgress(id, p.RHStatus, p.ProgressText, p.QueueIndex, p.QueueCount)
			if p.AccessToken != "" || p.RefreshToken != "" {
				_ = s.store.UpdateAccountAuthTokens(acc.ID, p.AccessToken, p.RefreshToken)
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
	submit := runninghub.SubmitWebappVideo
	switch {
	case useQCGeneric:
		submit = runninghub.SubmitQCGeneric
	case useQCImage:
		submit = runninghub.SubmitQCFreeImage
	case useQCVideo:
		submit = runninghub.SubmitQCFreeVideo
	}
	sess, submitErr := submit(in)
	if submitErr != nil && egress.IsConnectivityError(submitErr) && egSess.Node != nil {
		if eg2, reErr := s.egress.Rebind(acc, s.cfg.BaseURL, egSess.Node, submitErr.Error()); reErr == nil {
			egress.AttachHTTP(&in, eg2)
			sess, submitErr = submit(in)
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
		case strings.Contains(low, "insufficient"), strings.Contains(low, "balance"),
			strings.Contains(low, "remain"), strings.Contains(low, "coin"),
			strings.Contains(low, "积分"), strings.Contains(low, "余额"),
			strings.Contains(low, "qc not free"), strings.Contains(low, "prepare not free"):
			code = "NO_COINS"
		case strings.Contains(low, "prompt"), strings.Contains(low, "sensitive"):
			code = "PROMPT"
		}
		_ = s.pool.ReleaseFail(acc.ID, code, msg)
		if code == "BUSY" || code == "AUTH" || code == "NO_COINS" {
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

	if sess.WebappID != "" {
		webappID = sess.WebappID
		task.WebappID = sess.WebappID
		_ = s.store.UpdateTaskWebappID(id, sess.WebappID)
	}
	if sess.Kind == runninghub.SessionKindQC || runninghub.IsQCWebappID(sess.WebappID) {
		accounts.RefreshFreeQuota(s.store, sess.Client, acc.ID)
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
	a, _ := actorFromRequest(r)
	id := strings.TrimPrefix(r.URL.Path, "/v1/videos/")
	id = strings.Trim(id, "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing id"})
		return
	}
	if strings.HasSuffix(id, "/cancel") && r.Method == http.MethodPost {
		realID := strings.TrimSuffix(id, "/cancel")
		if !s.canAccessTask(a, realID) {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "无权操作该任务"})
			return
		}
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
		if !s.canAccessTask(a, realID) {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "无权操作该任务"})
			return
		}
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
	if !a.IsAdmin() && task.UserID != a.UserID {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "无权查看该任务"})
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) canAccessTask(a Actor, taskID string) bool {
	if a.IsAdmin() {
		return true
	}
	task, err := s.store.GetTask(taskID)
	if err != nil {
		return false
	}
	return task.UserID == a.UserID
}

func (s *Server) handleAdminAccounts(w http.ResponseWriter, r *http.Request) {
	a, _ := actorFromRequest(r)
	switch r.Method {
	case http.MethodGet:
		uid := scopeUserID(a, parseUserIDQuery(r))
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		ownerName := strings.TrimSpace(r.URL.Query().Get("owner"))
		var ownerUserID int64
		if a.IsAdmin() && ownerName != "" {
			if u, err := s.store.GetUserByUsername(ownerName); err == nil {
				ownerUserID = u.ID
			} else {
				writeJSON(w, http.StatusOK, map[string]any{
					"data": []any{}, "total": 0, "page": 1, "page_size": 50, "pages": 0,
				})
				return
			}
		}
		// No page param → full list (dashboard / legacy). With page → paginate.
		if strings.TrimSpace(r.URL.Query().Get("page")) == "" && strings.TrimSpace(r.URL.Query().Get("page_size")) == "" {
			list, err := s.store.ListAccountsPage(uid, ownerUserID, q, 0, 0)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			s.attachAccountOwners(list)
			writeJSON(w, http.StatusOK, map[string]any{"data": list, "total": len(list)})
			return
		}
		page := queryIntDefault(r, "page", 1)
		pageSize := queryIntDefault(r, "page_size", 50)
		if page < 1 {
			page = 1
		}
		if pageSize < 1 {
			pageSize = 50
		}
		if pageSize > 200 {
			pageSize = 200
		}
		total, err := s.store.CountAccounts(uid, ownerUserID, q)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		pages := 0
		if total > 0 {
			pages = (total + pageSize - 1) / pageSize
		}
		if pages == 0 {
			page = 1
		} else if page > pages {
			page = pages
		}
		list, err := s.store.ListAccountsPage(uid, ownerUserID, q, pageSize, (page-1)*pageSize)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		s.attachAccountOwners(list)
		writeJSON(w, http.StatusOK, map[string]any{
			"data":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"pages":     pages,
		})
	case http.MethodPost:
		s.handleCreateAccount(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	a, _ := actorFromRequest(r)
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
	if a.IsAdmin() && body.UserID > 0 {
		// admin may assign to a target user
	} else {
		body.UserID = a.UserID
	}
	if err := s.rejectForeignAccountEmail(a, body.Email); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": err.Error()})
		return
	}
	acc, err := s.store.UpsertAccount(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	acc.PublicView()
	writeJSON(w, http.StatusOK, acc)
}

// rejectForeignAccountEmail blocks non-admins from upserting emails they do not own
// (including unassigned user_id=0 rows — those stay admin-only).
func (s *Server) rejectForeignAccountEmail(actor Actor, email string) error {
	if actor.IsAdmin() {
		return nil
	}
	existing, err := s.store.GetAccountByEmail(email)
	if err != nil {
		// 新邮箱：允许当前用户写入自己的号池（兼容旧版返回 sql.ErrNoRows）
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if !ownsAccount(actor, existing) {
		return fmt.Errorf("账号已属于其他用户")
	}
	return nil
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
		actor, _ := actorFromRequest(r)
		if !ownsAccount(actor, acc) {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "无权查看该账号"})
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Pragma", "no-cache")
		acc.AdminView()
		if names, err := s.store.UsernameByID(); err == nil {
			acc.Owner = names[acc.UserID]
		}
		writeJSON(w, http.StatusOK, acc)
	case action == "" && r.Method == http.MethodDelete:
		{
			acc, err := s.store.GetAccount(id)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
					return
				}
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			actor, _ := actorFromRequest(r)
			if !ownsAccount(actor, acc) {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "无权删除该账号"})
				return
			}
		}
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
		if !s.requireAccountOwner(w, r, id) {
			return
		}
		s.refreshAccount(w, id)
	case action == "ensure-api-key" && r.Method == http.MethodPost:
		if !s.requireAccountOwner(w, r, id) {
			return
		}
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
			"ok":      true,
			"result":  result,
			"account": acc,
		})
	case action == "disable" && r.Method == http.MethodPost:
		if !s.requireAccountOwner(w, r, id) {
			return
		}
		_ = s.store.DisableAccount(id, "manual disable")
		acc, err := s.store.GetAccount(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		acc.PublicView()
		writeJSON(w, http.StatusOK, acc)
	case action == "enable" && r.Method == http.MethodPost:
		if !s.requireAccountOwner(w, r, id) {
			return
		}
		_ = s.store.EnableAccount(id)
		acc, err := s.store.GetAccount(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		acc.PublicView()
		writeJSON(w, http.StatusOK, acc)
	case action == "max-concurrency" && r.Method == http.MethodPost:
		if !s.requireAccountOwner(w, r, id) {
			return
		}
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

func (s *Server) requireAccountOwner(w http.ResponseWriter, r *http.Request, id int64) bool {
	acc, err := s.store.GetAccount(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return false
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return false
	}
	actor, _ := actorFromRequest(r)
	if !ownsAccount(actor, acc) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "无权操作该账号"})
		return false
	}
	return true
}

func (s *Server) refreshAccount(w http.ResponseWriter, id int64) {
	acc, err := s.store.GetAccount(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	var wallet runninghub.Wallet
	err = s.egress.Do(acc, s.cfg.BaseURL, func(client *runninghub.Client) error {
		at := strings.TrimSpace(acc.AccessToken)
		if at != "" {
			client.SetAccessToken(at)
		}
		apiKey := strings.TrimSpace(acc.APIKey)
		if apiKey == "" && (at == "" || strings.EqualFold(acc.BillingVersion, runninghub.BillingOld) || acc.BillingVersion == "") {
			got, e := client.EnsureConsumerApiKey(acc.Email, acc.Password, at)
			if e == nil {
				apiKey = got.APIKey
				_ = s.store.UpdateAccountAPIKey(id, apiKey)
				_ = s.store.UpdateAccountAuthTokens(id, got.AccessToken, got.RefreshToken)
				acc.APIKey = apiKey
				if got.AccessToken != "" {
					client.SetAccessToken(got.AccessToken)
				}
			} else if at == "" {
				return fmt.Errorf("补 api_key 失败: %w", e)
			}
		}
		var e error
		wallet, e = client.FetchWallet(apiKey)
		if e != nil && apiKey != "" {
			got, e2 := client.EnsureConsumerApiKey(acc.Email, acc.Password, "")
			if e2 != nil {
				return e
			}
			apiKey = got.APIKey
			_ = s.store.UpdateAccountAPIKey(id, apiKey)
			_ = s.store.UpdateAccountAuthTokens(id, got.AccessToken, got.RefreshToken)
			if got.AccessToken != "" {
				client.SetAccessToken(got.AccessToken)
			}
			wallet, e = client.FetchWallet(apiKey)
		}
		if e == nil {
			accounts.RefreshFreeQuota(s.store, client, id)
		}
		return e
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	_ = s.store.UpdateAccountWallet(id, wallet.RemainCoins, wallet.RemainPoints, wallet.CurrentTaskCounts, wallet.APIType, wallet.BillingVersion)
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
	actor, _ := actorFromRequest(r)
	scopeUID := int64(0)
	if !actor.IsAdmin() {
		scopeUID = actor.UserID
	}
	results, err := s.dailyClaimer.ClaimScoped(scopeUID, body.IDs, body.Force)
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
	actor, _ := actorFromRequest(r)
	success, skipped, failed := 0, 0, 0
	results := make([]map[string]any, 0, len(body.IDs))
	for _, id := range body.IDs {
		item := map[string]any{"id": id}
		acc, getErr := s.store.GetAccount(id)
		if getErr != nil {
			failed++
			item["ok"] = false
			if errors.Is(getErr, store.ErrNotFound) {
				item["error"] = "not found"
			} else {
				item["error"] = getErr.Error()
			}
			results = append(results, item)
			continue
		}
		item["email"] = acc.Email
		if !ownsAccount(actor, acc) {
			failed++
			item["ok"] = false
			item["error"] = "无权操作该账号"
			results = append(results, item)
			continue
		}
		acc, result, err := s.ensureAccountAPIKey(id, body.Force)
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
		// Do not echo raw RH api_key in batch responses; owner can GET /admin/accounts/{id}.
		delete(result, "api_key")
		item["ok"] = true
		item["result"] = result
		item["has_api_key"] = strings.TrimSpace(acc.APIKey) != ""
		results = append(results, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": success,
		"skipped": skipped,
		"failed":  failed,
		"results": results,
	})
}

// ensureAccountAPIKey: login → createConsumer → reset.
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
	var wallet runninghub.Wallet
	err = s.egress.Do(acc, s.cfg.BaseURL, func(client *runninghub.Client) error {
		var e error
		got, e = client.EnsureConsumerApiKey(acc.Email, acc.Password, acc.AccessToken)
		if e != nil {
			return e
		}
		_ = s.store.UpdateAccountAPIKey(id, got.APIKey)
		_ = s.store.UpdateAccountAuthTokens(id, got.AccessToken, got.RefreshToken)
		if got.AccessToken != "" {
			client.SetAccessToken(got.AccessToken)
		}
		wallet, _ = client.FetchWallet(got.APIKey)
		if wallet.RemainCoins != "" || wallet.RemainPoints != "" || wallet.BillingVersion != "" {
			_ = s.store.UpdateAccountWallet(id, wallet.RemainCoins, wallet.RemainPoints, wallet.CurrentTaskCounts, wallet.APIType, wallet.BillingVersion)
		}
		accounts.RefreshFreeQuota(s.store, client, id)
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
		"remain_coins":      wallet.RemainCoins,
		"remain_points":     wallet.RemainPoints,
		"api_type":          wallet.APIType,
		"billing_version":   wallet.BillingVersion,
		"has_api_key":       true,
		"has_access_token":  strings.TrimSpace(acc.AccessToken) != "",
		"has_refresh_token": strings.TrimSpace(acc.RefreshToken) != "",
	}, nil
}

func (s *Server) handleImportAccounts(w http.ResponseWriter, r *http.Request) {
	s.handlePushAccounts(w, r)
}

// handlePushAccounts upserts one or many accounts for console import / sync.
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

	actor, _ := actorFromRequest(r)
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
		if !(actor.IsAdmin() && a.UserID > 0) {
			a.UserID = actor.UserID
		}
		email := strings.TrimSpace(a.Email)
		if email == "" {
			fail++
			errs = append(errs, map[string]any{"email": a.Email, "error": "email required"})
			continue
		}
		a.Email = email
		if err := s.rejectForeignAccountEmail(actor, email); err != nil {
			fail++
			errs = append(errs, map[string]any{"email": email, "error": err.Error()})
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
	a, _ := actorFromRequest(r)
	uid := scopeUserID(a, parseUserIDQuery(r))
	list, err := s.store.ListTasks(uid, 200)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	s.attachTaskOwners(list)
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (s *Server) attachAccountOwners(list []store.Account) {
	names, err := s.store.UsernameByID()
	if err != nil || len(list) == 0 {
		return
	}
	for i := range list {
		if name := names[list[i].UserID]; name != "" {
			list[i].Owner = name
		}
	}
}

func (s *Server) attachTaskOwners(list []store.VideoTask) {
	names, err := s.store.UsernameByID()
	if err != nil || len(list) == 0 {
		return
	}
	for i := range list {
		if name := names[list[i].UserID]; name != "" {
			list[i].Owner = name
		}
	}
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
	return src
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

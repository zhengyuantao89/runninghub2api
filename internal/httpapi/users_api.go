package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/trae/runninghub2api/internal/store"
)

func (s *Server) handleUserKeys(w http.ResponseWriter, r *http.Request) {
	a, _ := actorFromRequest(r)
	switch r.Method {
	case http.MethodGet:
		list, err := s.store.ListUserAPIKeys(a.UserID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": list})
	case http.MethodPost:
		var body struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		k, plain, err := s.store.CreateUserAPIKey(a.UserID, body.Name)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"key":  plain,
			"meta": k,
		})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUserKeyByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	a, _ := actorFromRequest(r)
	idStr := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/keys/"), "/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid id"})
		return
	}
	if err := s.store.DeleteUserAPIKey(a.UserID, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleAdminCredentials lets the logged-in admin change own username/password (DB-persisted).
func (s *Server) handleAdminCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	a, _ := actorFromRequest(r)
	var body struct {
		CurrentPassword string `json:"current_password"`
		Username        string `json:"username"`
		Password        string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	cur := strings.TrimSpace(body.CurrentPassword)
	if cur == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请填写当前密码以确认身份"})
		return
	}
	u, err := s.store.GetUser(a.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !store.CheckPassword(u.PasswordHash, cur) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "当前密码错误"})
		return
	}

	newUser := strings.TrimSpace(strings.ToLower(body.Username))
	newPass := strings.TrimSpace(body.Password)
	if newUser == "" && newPass == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请填写新用户名或新密码"})
		return
	}

	out := map[string]any{"ok": true, "user_id": u.ID, "username": u.Username}
	if newUser != "" && newUser != u.Username {
		if len(newUser) < 3 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "用户名至少 3 位"})
			return
		}
		if err := s.store.SetUserUsername(u.ID, newUser); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		out["username"] = newUser
	}
	if newPass != "" {
		if len(newPass) < 6 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "新密码至少 6 位"})
			return
		}
		hash, err := store.HashPassword(newPass)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		if err := s.store.SetUserPassword(u.ID, hash); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		out["password_updated"] = true
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.store.ListUsers()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": list})
	case http.MethodPost:
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
		hash, err := store.HashPassword(strings.TrimSpace(body.Password))
		if err != nil || len(strings.TrimSpace(body.Password)) < 6 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "密码至少 6 位"})
			return
		}
		role := store.RoleUser
		if body.Role == store.RoleAdmin {
			role = store.RoleAdmin
		}
		u, err := s.store.CreateUser(body.Username, hash, role)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, u)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAdminUserByID(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/admin/users/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid id"})
		return
	}
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	switch {
	case action == "" && r.Method == http.MethodGet:
		u, err := s.store.GetUser(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, u)
	case action == "disable" && r.Method == http.MethodPost:
		if err := s.store.SetUserStatus(id, store.UserDisabled); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case action == "enable" && r.Method == http.MethodPost:
		if err := s.store.SetUserStatus(id, store.UserActive); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case action == "password" && r.Method == http.MethodPost:
		var body struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
		if len(strings.TrimSpace(body.Password)) < 6 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "密码至少 6 位"})
			return
		}
		hash, err := store.HashPassword(strings.TrimSpace(body.Password))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		if err := s.store.SetUserPassword(id, hash); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": fmt.Sprintf("unknown action %q", action)})
	}
}

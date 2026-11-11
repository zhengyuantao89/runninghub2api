package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/trae/runninghub2api/internal/store"
)

type ctxKey int

const actorKey ctxKey = 1

// Actor is the authenticated principal for a request.
type Actor struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Via      string `json:"via"` // session | api_key | legacy_env
}

func (a Actor) IsAdmin() bool { return a.Role == store.RoleAdmin }
func (a Actor) IsUser() bool  { return a.Role == store.RoleUser }
func (a Actor) OK() bool      { return a.UserID > 0 && a.Role != "" }

func withActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey, a)
}

func actorFrom(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(actorKey).(Actor)
	return a, ok && a.OK()
}

func actorFromRequest(r *http.Request) (Actor, bool) {
	return actorFrom(r.Context())
}

func (s *Server) resolveActor(r *http.Request) (Actor, bool) {
	token := s.bearerToken(r)
	if token == "" {
		return Actor{}, false
	}
	if info, ok := s.sessions.Get(token); ok {
		u, err := s.store.GetUser(info.UserID)
		if err != nil || u.Status != store.UserActive {
			return Actor{}, false
		}
		return Actor{UserID: u.ID, Username: u.Username, Role: u.Role, Via: "session"}, true
	}
	if u, _, err := s.store.LookupUserByAPIKey(token); err == nil {
		if u.Status != store.UserActive {
			return Actor{}, false
		}
		return Actor{UserID: u.ID, Username: u.Username, Role: u.Role, Via: "api_key"}, true
	}
	// Legacy env keys: treat as admin manage-only (no video submit).
	for _, k := range s.cfg.ExternalKeys() {
		if token == k {
			adminID := s.store.FirstAdminID()
			if adminID <= 0 {
				return Actor{}, false
			}
			u, err := s.store.GetUser(adminID)
			if err != nil || u.Status != store.UserActive {
				return Actor{}, false
			}
			return Actor{UserID: u.ID, Username: u.Username, Role: store.RoleAdmin, Via: "legacy_env"}, true
		}
	}
	return Actor{}, false
}

// isBrowserNavigation detects top-level browser GETs (vs SPA fetch/XHR).
// Vue pages and JSON APIs share some /admin/* paths; navigations must get SPA/login.
func isBrowserNavigation(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	mode := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Mode")))
	if mode == "navigate" || mode == "nested-navigate" {
		return true
	}
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html")
}

func (s *Server) serveSPAIndex(w http.ResponseWriter, r *http.Request) {
	index := filepath.Join(s.cfg.WebDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":   true,
			"hint": "web UI not built; open /login after building web/dist",
		})
		return
	}
	http.ServeFile(w, r, index)
}

func (s *Server) redirectLogin(w http.ResponseWriter, r *http.Request) {
	redir := r.URL.RequestURI()
	if redir == "" {
		redir = "/"
	}
	http.Redirect(w, r, "/login?redirect="+url.QueryEscape(redir), http.StatusFound)
}

func (s *Server) withActor(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nav := isBrowserNavigation(r)
		a, ok := s.resolveActor(r)
		if !ok {
			if nav {
				s.redirectLogin(w, r)
				return
			}
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "未登录或登录已失效"})
			return
		}
		if nav && isConsolePagePath(r.URL.Path) {
			// Same path as Vue console pages (e.g. /admin/accounts): serve SPA, not JSON API.
			s.serveSPAIndex(w, r)
			return
		}
		next(w, r.WithContext(withActor(r.Context(), a)))
	}
}

func isConsolePagePath(path string) bool {
	return strings.HasPrefix(path, "/admin") || strings.HasPrefix(path, "/app")
}

func (s *Server) withAdmin(next http.HandlerFunc) http.HandlerFunc {
	return s.withActor(func(w http.ResponseWriter, r *http.Request) {
		a, _ := actorFromRequest(r)
		if !a.IsAdmin() {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "需要管理员权限"})
			return
		}
		next(w, r)
	})
}

func (s *Server) withUserSubmit(next http.HandlerFunc) http.HandlerFunc {
	return s.withActor(func(w http.ResponseWriter, r *http.Request) {
		a, _ := actorFromRequest(r)
		path := strings.TrimRight(r.URL.Path, "/")
		isCreate := r.Method == http.MethodPost && (path == "/v1/videos" || path == "/v1/images/generations")
		if isCreate && !a.IsAdmin() && !a.IsUser() {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "无权提交任务"})
			return
		}
		next(w, r)
	})
}

func scopeUserID(a Actor, queryUserID int64) int64 {
	if a.IsAdmin() {
		return queryUserID // 0 = all
	}
	return a.UserID
}

func ownsAccount(a Actor, acc store.Account) bool {
	return a.IsAdmin() || (a.UserID > 0 && acc.UserID == a.UserID)
}

func parseUserIDQuery(r *http.Request) int64 {
	v := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if v == "" {
		return 0
	}
	var id int64
	for _, c := range v {
		if c < '0' || c > '9' {
			return 0
		}
		id = id*10 + int64(c-'0')
	}
	return id
}

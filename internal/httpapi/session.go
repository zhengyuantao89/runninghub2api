package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/trae/runninghub2api/internal/store"
)

type sessionInfo struct {
	UserID    int64
	Username  string
	Role      string
	ExpiresAt time.Time
}

// sessionStore persists login tokens in SQLite so Docker/process restarts keep sessions.
type sessionStore struct {
	mu    sync.Mutex
	store *store.Store
	ttl   time.Duration
}

func newSessionStore(st *store.Store, ttl time.Duration) *sessionStore {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &sessionStore{store: st, ttl: ttl}
}

func (s *sessionStore) Create(u store.User) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	exp := time.Now().Add(s.ttl)
	_ = s.store.PurgeExpiredUserSessions()
	if err := s.store.CreateUserSession(id, u.ID, u.Username, u.Role, exp); err != nil {
		return ""
	}
	return id
}

func (s *sessionStore) Get(id string) (sessionInfo, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return sessionInfo{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	row, err := s.store.GetUserSession(id)
	if err != nil {
		return sessionInfo{}, false
	}
	now := time.Now()
	if now.After(row.ExpiresAt) {
		_ = s.store.DeleteUserSession(id)
		return sessionInfo{}, false
	}
	exp := now.Add(s.ttl)
	_ = s.store.TouchUserSession(id, exp)
	return sessionInfo{
		UserID:    row.UserID,
		Username:  row.Username,
		Role:      row.Role,
		ExpiresAt: exp,
	}, true
}

func (s *sessionStore) Valid(id string) bool {
	_, ok := s.Get(id)
	return ok
}

func (s *sessionStore) Revoke(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	_ = s.store.DeleteUserSession(id)
}

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

type sessionStore struct {
	mu   sync.Mutex
	byID map[string]sessionInfo
	ttl  time.Duration
}

func newSessionStore(ttl time.Duration) *sessionStore {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &sessionStore{byID: map[string]sessionInfo{}, ttl: ttl}
}

func (s *sessionStore) Create(u store.User) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked(time.Now())
	s.byID[id] = sessionInfo{
		UserID:    u.ID,
		Username:  u.Username,
		Role:      u.Role,
		ExpiresAt: time.Now().Add(s.ttl),
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
	now := time.Now()
	s.purgeLocked(now)
	info, ok := s.byID[id]
	if !ok {
		return sessionInfo{}, false
	}
	if now.After(info.ExpiresAt) {
		delete(s.byID, id)
		return sessionInfo{}, false
	}
	info.ExpiresAt = now.Add(s.ttl)
	s.byID[id] = info
	return info, true
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
	s.mu.Lock()
	delete(s.byID, id)
	s.mu.Unlock()
}

func (s *sessionStore) purgeLocked(now time.Time) {
	for id, exp := range s.byID {
		if now.After(exp.ExpiresAt) {
			delete(s.byID, id)
		}
	}
}

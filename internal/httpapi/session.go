package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

type sessionStore struct {
	mu   sync.Mutex
	byID map[string]time.Time
	ttl  time.Duration
}

func newSessionStore(ttl time.Duration) *sessionStore {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &sessionStore{byID: map[string]time.Time{}, ttl: ttl}
}

func (s *sessionStore) Create() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked(time.Now())
	s.byID[id] = time.Now().Add(s.ttl)
	return id
}

func (s *sessionStore) Valid(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.purgeLocked(now)
	exp, ok := s.byID[id]
	if !ok {
		return false
	}
	if now.After(exp) {
		delete(s.byID, id)
		return false
	}
	s.byID[id] = now.Add(s.ttl)
	return true
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
		if now.After(exp) {
			delete(s.byID, id)
		}
	}
}

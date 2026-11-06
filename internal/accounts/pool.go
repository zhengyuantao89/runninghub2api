package accounts

import (
	"errors"
	"sync"
	"time"

	"github.com/trae/runninghub2api/internal/store"
)

var ErrUnavailable = errors.New("no available accounts")

type Pool struct {
	store *store.Store
	mu    sync.Mutex
}

func New(s *store.Store) *Pool {
	return &Pool{store: s}
}

func (p *Pool) Acquire(taskID string) (store.Account, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	accounts, err := p.store.ListDispatchableAccounts()
	if err != nil {
		return store.Account{}, err
	}
	for _, a := range accounts {
		if err := p.store.LeaseAccount(a.ID, taskID, 30*time.Minute); err != nil {
			continue
		}
		return a, nil
	}
	return store.Account{}, ErrUnavailable
}

func (p *Pool) ReleaseOK(id int64) error {
	return p.store.ReleaseAccountOK(id)
}

func (p *Pool) ReleaseFail(id int64, code, msg string) error {
	cooldown := 2 * time.Minute
	switch code {
	case "AUTH", "LOGIN":
		cooldown = 30 * time.Minute
	case "BUSY":
		cooldown = 5 * time.Minute
	}
	return p.store.ReleaseAccountFail(id, code, msg, cooldown)
}

func (p *Pool) Disable(id int64, reason string) error {
	return p.store.DisableAccount(id, reason)
}

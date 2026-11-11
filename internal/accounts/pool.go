package accounts

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/trae/runninghub2api/internal/store"
)

var ErrUnavailable = errors.New("no available accounts")

type Pool struct {
	store     *store.Store
	submitGap time.Duration
	mu        sync.Mutex
}

func New(s *store.Store, submitGap time.Duration) *Pool {
	if submitGap < 0 {
		submitGap = 0
	}
	return &Pool{store: s, submitGap: submitGap}
}

func (p *Pool) Acquire(userID int64, taskID string) (store.Account, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	accounts, err := p.store.ListDispatchableAccounts(userID, p.submitGap)
	if err != nil {
		return store.Account{}, err
	}
	for _, a := range accounts {
		if userID > 0 && a.UserID != userID {
			continue
		}
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
	// RH 任务失败 / 忙 / 出口问题只释放并发，不冷却账号；上游本身只限制并发。
	// 真鉴权失效才冷却；FREQUENT_OPER 是登录限流，靠 submit gap 缓几秒即可。
	cooldown := time.Duration(0)
	low := strings.ToLower(msg)
	switch code {
	case "AUTH", "LOGIN":
		if strings.Contains(low, "frequent") {
			cooldown = 0
		} else {
			cooldown = 30 * time.Minute
		}
	}
	if err := p.store.ReleaseAccountFail(id, code, msg, cooldown); err != nil {
		return err
	}
	if code == "NO_COINS" {
		return p.store.DisableAccount(id, "NO_COINS: "+msg)
	}
	return nil
}

func (p *Pool) Disable(id int64, reason string) error {
	return p.store.DisableAccount(id, reason)
}

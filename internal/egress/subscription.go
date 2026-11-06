package egress

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/trae/runninghub2api/internal/store"
)

const (
	NodesGroup       = "nodes"
	fetchTimeout     = 60 * time.Second
	DefaultInterval  = 30
)

var AllowedIntervals = []int{10, 30, 60}

// RefreshSubscription pulls plain-text proxy list from URL and exact-syncs into nodes group.
func RefreshSubscription(st *store.Store) (map[string]any, error) {
	sub, err := st.GetEgressSubscription()
	if err != nil {
		return nil, fmt.Errorf("尚未配置订阅地址")
	}
	url := strings.TrimSpace(sub.URL)
	if url == "" {
		return nil, fmt.Errorf("尚未配置订阅地址")
	}
	text, err := fetchSubscriptionText(url)
	if err != nil {
		_ = st.MarkEgressSubscriptionSync(sub.ID, "error", err.Error(), store.SyncStats{})
		return nil, err
	}
	parsed := ParseProxies(text, sub.GroupName)
	if len(parsed) == 0 {
		err := fmt.Errorf("订阅内容为空或无法解析为代理节点")
		_ = st.MarkEgressSubscriptionSync(sub.ID, "error", err.Error(), store.SyncStats{})
		return nil, err
	}
	stats, err := st.ExactSyncEgressGroup(sub.GroupName, parsed, Fingerprint)
	if err != nil {
		_ = st.MarkEgressSubscriptionSync(sub.ID, "error", err.Error(), store.SyncStats{})
		return nil, err
	}
	_ = st.MarkEgressSubscriptionSync(sub.ID, "ok", "", stats)
	log.Printf("[egress-sub] sync ok total=%d added=%d updated=%d removed=%d", stats.Total, stats.Added, stats.Updated, stats.Removed)
	fresh, _ := st.GetEgressSubscription()
	return map[string]any{
		"subscription": fresh,
		"added":        stats.Added,
		"updated":      stats.Updated,
		"removed":      stats.Removed,
		"total":        stats.Total,
	}, nil
}

func fetchSubscriptionText(rawURL string) (string, error) {
	client := &http.Client{Timeout: fetchTimeout}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/plain, */*")
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("订阅拉取失败: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode >= 400 {
		return "", fmt.Errorf("订阅拉取失败 HTTP %d: %s", res.StatusCode, truncateRunes(string(body), 120))
	}
	return string(body), nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// SubscriptionPoller periodically refreshes due subscriptions.
type SubscriptionPoller struct {
	store    *store.Store
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

func NewSubscriptionPoller(st *store.Store, tick time.Duration) *SubscriptionPoller {
	if tick < time.Minute {
		tick = time.Minute
	}
	return &SubscriptionPoller{store: st, interval: tick, stopCh: make(chan struct{})}
}

func (p *SubscriptionPoller) Start() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		// first pass shortly after boot
		select {
		case <-time.After(15 * time.Second):
			p.tick()
		case <-p.stopCh:
			return
		}
		t := time.NewTicker(p.interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				p.tick()
			case <-p.stopCh:
				return
			}
		}
	}()
}

func (p *SubscriptionPoller) Stop() {
	close(p.stopCh)
	p.wg.Wait()
}

func (p *SubscriptionPoller) tick() {
	due, err := p.store.ListDueEgressSubscriptions(time.Now())
	if err != nil || len(due) == 0 {
		return
	}
	for range due {
		if _, err := RefreshSubscription(p.store); err != nil {
			log.Printf("[egress-sub] poll refresh: %v", err)
		}
	}
}

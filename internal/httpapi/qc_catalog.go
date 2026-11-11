package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/trae/runninghub2api/internal/provider/runninghub"
	"github.com/trae/runninghub2api/internal/store"
)

type qcCatalogCache struct {
	mu    sync.RWMutex
	byCat map[string]cachedQCCatalog
}

type cachedQCCatalog struct {
	cat     runninghub.QCCatalog
	expires time.Time
}

func (s *Server) qcCache() *qcCatalogCache {
	// lazily attach on Server — use package-level for simplicity
	return sharedQCCatalogCache
}

var sharedQCCatalogCache = &qcCatalogCache{byCat: map[string]cachedQCCatalog{}}

func (s *Server) handleQCCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	if category == "" {
		category = "VIDEO"
	}
	mode := strings.TrimSpace(r.URL.Query().Get("mode"))
	refresh := r.URL.Query().Get("refresh") == "1" || r.URL.Query().Get("refresh") == "true"

	cat, err := s.getQCCatalog(category, refresh)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	models := cat.Models
	if mode != "" {
		models = runninghub.FilterQCModelsByMode(models, mode)
	}
	// Enrich billing hints from bootstrap JSON when live has unknown.
	writeJSON(w, http.StatusOK, map[string]any{
		"category":   category,
		"modes":      cat.Modes,
		"models":     models,
		"source":     cat.Source,
		"fetched_at": cat.Fetched,
		"mode":       mode,
	})
}

func (s *Server) getQCCatalog(category string, force bool) (runninghub.QCCatalog, error) {
	category = strings.ToUpper(strings.TrimSpace(category))
	if category == "" {
		category = "VIDEO"
	}
	cache := s.qcCache()
	if !force {
		cache.mu.RLock()
		hit, ok := cache.byCat[category]
		cache.mu.RUnlock()
		if ok && time.Now().Before(hit.expires) {
			return hit.cat, nil
		}
	}

	live, liveErr := s.fetchLiveQCCatalog(category)
	if liveErr == nil && len(live.Models) > 0 {
		live.Fetched = time.Now().UTC().Format(time.RFC3339)
		cache.mu.Lock()
		cache.byCat[category] = cachedQCCatalog{cat: live, expires: time.Now().Add(30 * time.Minute)}
		cache.mu.Unlock()
		return live, nil
	}

	boot, bootErr := loadBootstrapQCCatalog(s.cfg.DataDir, category)
	if bootErr == nil && len(boot.Models) > 0 {
		boot.Source = "bootstrap"
		if liveErr != nil {
			boot.Source = "bootstrap; live_error=" + liveErr.Error()
		}
		cache.mu.Lock()
		cache.byCat[category] = cachedQCCatalog{cat: boot, expires: time.Now().Add(5 * time.Minute)}
		cache.mu.Unlock()
		return boot, nil
	}
	if liveErr != nil {
		return runninghub.QCCatalog{}, liveErr
	}
	return runninghub.QCCatalog{}, bootErr
}

func (s *Server) fetchLiveQCCatalog(category string) (runninghub.QCCatalog, error) {
	acc, err := s.pickCatalogAccount()
	if err != nil {
		return runninghub.QCCatalog{}, err
	}
	var out runninghub.QCCatalog
	err = s.egress.Do(acc, s.cfg.BaseURL, func(client *runninghub.Client) error {
		at := strings.TrimSpace(acc.AccessToken)
		if at != "" {
			client.SetAccessToken(at)
		} else if strings.TrimSpace(acc.Password) != "" {
			if _, e := client.EnsureConsumerApiKey(acc.Email, acc.Password, ""); e != nil {
				return e
			}
		}
		cat, e := client.FetchQCCatalog(category)
		if e != nil {
			return e
		}
		out = cat
		return nil
	})
	return out, err
}

func (s *Server) pickCatalogAccount() (store.Account, error) {
	list, err := s.store.ListAccounts(0)
	if err != nil {
		return store.Account{}, err
	}
	for _, a := range list {
		if a.Status != "active" {
			continue
		}
		full, err := s.store.GetAccount(a.ID)
		if err != nil {
			continue
		}
		if strings.TrimSpace(full.AccessToken) != "" || strings.TrimSpace(full.Password) != "" {
			return full, nil
		}
	}
	return store.Account{}, errString("no account available for QC catalog")
}

type errString string

func (e errString) Error() string { return string(e) }

type bootstrapModel struct {
	Name        string   `json:"name"`
	Group       string   `json:"group"`
	BindingID   string   `json:"bindingId"`
	SkuID       string   `json:"skuId"`
	Modes       []string `json:"modes"`
	BillingHint string   `json:"billingHint"`
	ParamKeys   []string `json:"paramKeysSeen"`
}

func loadBootstrapQCCatalog(dataDir, category string) (runninghub.QCCatalog, error) {
	// Prefer repo/docs snapshot copied next to binary data, then docs path for local.
	candidates := []string{
		filepath.Join(dataDir, "qc_video_models_from_har.json"),
		filepath.Join("docs", "superpowers", "specs", "2026-11-10-qc-video-models-from-har.json"),
		filepath.Join("..", "docs", "superpowers", "specs", "2026-11-10-qc-video-models-from-har.json"),
	}
	var raw []byte
	var err error
	for _, p := range candidates {
		raw, err = os.ReadFile(p)
		if err == nil && len(raw) > 0 {
			break
		}
	}
	if len(raw) == 0 {
		return runninghub.QCCatalog{}, errString("bootstrap catalog not found")
	}
	var list []bootstrapModel
	if err := json.Unmarshal(raw, &list); err != nil {
		return runninghub.QCCatalog{}, err
	}
	if !strings.EqualFold(category, "VIDEO") {
		return runninghub.QCCatalog{}, errString("bootstrap only has VIDEO")
	}
	modeSet := map[string]*runninghub.QCCreationMode{}
	models := make([]runninghub.QCModelEntry, 0, len(list))
	for _, m := range list {
		entry := runninghub.QCModelEntry{
			Category:    "VIDEO",
			Group:       m.Group,
			BindingID:   m.BindingID,
			SkuID:       m.SkuID,
			Name:        m.Name,
			Modes:       m.Modes,
			BillingHint: m.BillingHint,
			ParamKeys:   m.ParamKeys,
			SupportsFree: m.BillingHint == "free_seen" || m.BillingHint == "mixed_seen",
			SupportsPaid: m.BillingHint == "paid_seen" || m.BillingHint == "mixed_seen" || m.BillingHint == "unknown",
		}
		models = append(models, entry)
		for _, mk := range m.Modes {
			if modeSet[mk] == nil {
				modeSet[mk] = &runninghub.QCCreationMode{ModeKey: mk, Name: mk, BindingIDs: []string{}}
			}
			modeSet[mk].BindingIDs = append(modeSet[mk].BindingIDs, m.BindingID)
			modeSet[mk].BindingCount++
		}
	}
	modes := make([]runninghub.QCCreationMode, 0, len(modeSet))
	order := []string{"MULTIMODAL_REFERENCE", "SUBJECT_REFERENCE", "FREE_CREATION", "FIRST_LAST_FRAME"}
	for i, mk := range order {
		if m := modeSet[mk]; m != nil {
			m.Sort = i + 1
			modes = append(modes, *m)
			delete(modeSet, mk)
		}
	}
	for _, m := range modeSet {
		modes = append(modes, *m)
	}
	return runninghub.QCCatalog{Modes: modes, Models: models, Source: "bootstrap"}, nil
}

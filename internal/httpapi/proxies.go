package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/trae/runninghub2api/internal/egress"
	"github.com/trae/runninghub2api/internal/store"
)

func queryIntDefault(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

func (s *Server) handleAdminProxies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		group := strings.TrimSpace(r.URL.Query().Get("group"))
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
		total, err := s.store.CountEgress(group)
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
		offset := (page - 1) * pageSize
		list, err := s.store.ListEgressPage(group, pageSize, offset)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		// strip passwords in list
		for i := range list {
			list[i].Password = ""
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items":     list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"pages":     pages,
		})
	case http.MethodPost:
		s.importProxiesFromBody(w, r, "manual")
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleProxyGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	groups, err := s.store.ListEgressGroups()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

func (s *Server) handleImportProxies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.importProxiesFromBody(w, r, "manual")
}

func (s *Server) handlePushProxies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.importProxiesFromBody(w, r, "sync")
}

func (s *Server) importProxiesFromBody(w http.ResponseWriter, r *http.Request, defaultSource string) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "read body failed"})
		return
	}
	items, source, exactGroup, err := parseProxyPushBody(raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if source == "" {
		source = defaultSource
	}
	// plain-text / subscription-style import: exact sync into group when requested
	if exactGroup != "" {
		for i := range items {
			if items[i].Source == "" {
				items[i].Source = source
			}
			if items[i].GroupName == "" {
				items[i].GroupName = exactGroup
			}
		}
		stats, e := s.store.ExactSyncEgressGroup(exactGroup, items, egress.Fingerprint)
		if e != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": e.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"imported": stats.Total,
			"added":    stats.Added,
			"updated":  stats.Updated,
			"removed":  stats.Removed,
			"failed":   0,
			"group":    exactGroup,
		})
		return
	}
	imported := 0
	errs := make([]map[string]any, 0)
	out := make([]store.EgressNode, 0, len(items))
	for _, item := range items {
		if item.Source == "" {
			item.Source = source
		}
		if item.ExternalID == "" && item.ID > 0 {
			item.ExternalID = strconv.FormatInt(item.ID, 10)
			item.ID = 0
		}
		n, e := s.store.UpsertEgress(item)
		if e != nil {
			errs = append(errs, map[string]any{
				"host":  item.Host,
				"port":  item.Port,
				"error": e.Error(),
			})
			continue
		}
		n.Password = ""
		out = append(out, n)
		imported++
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"imported": imported,
		"failed":   len(errs),
		"data":     out,
		"errors":   errs,
	})
}

// parseProxyPushBody accepts:
//   - JSON array / {items|proxies}
//   - { text, group_name?, exact_sync? } plain-text lines (batch import)
// Returns exactGroup non-empty when caller should ExactSync.
func parseProxyPushBody(raw []byte) ([]store.EgressNode, string, string, error) {
	trim := strings.TrimSpace(string(raw))
	if trim == "" {
		return nil, "", "", errors.New("empty body")
	}
	if trim[0] == '[' {
		var list []store.EgressNode
		if err := json.Unmarshal([]byte(trim), &list); err != nil {
			return nil, "", "", errors.New("invalid proxies array")
		}
		return list, "sync", "", nil
	}
	var body struct {
		Items     []store.EgressNode `json:"items"`
		Proxies   []store.EgressNode `json:"proxies"`
		Source    string             `json:"source"`
		Text      string             `json:"text"`
		GroupName string             `json:"group_name"`
		ExactSync *bool              `json:"exact_sync"`
	}
	if err := json.Unmarshal([]byte(trim), &body); err != nil {
		return nil, "", "", errors.New("invalid json")
	}
	src := strings.TrimSpace(body.Source)
	group := strings.TrimSpace(body.GroupName)
	if group == "" {
		group = egress.NodesGroup
	}
	if text := strings.TrimSpace(body.Text); text != "" {
		list := egress.ParseProxies(text, group)
		if len(list) == 0 {
			return nil, "", "", errors.New("无法解析代理文本（支持 type://user:pass@host:port 等行）")
		}
		exact := true
		if body.ExactSync != nil {
			exact = *body.ExactSync
		}
		if exact {
			return list, firstNonEmpty(src, "text"), group, nil
		}
		return list, firstNonEmpty(src, "text"), "", nil
	}
	if len(body.Items) > 0 {
		return body.Items, src, "", nil
	}
	if len(body.Proxies) > 0 {
		return body.Proxies, src, "", nil
	}
	return nil, src, "", errors.New("items/proxies/text required")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (s *Server) handleProxySubscriptions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.store.ListEgressSubscriptions()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		sub, _ := s.store.GetEgressSubscription()
		writeJSON(w, http.StatusOK, map[string]any{
			"items":       list,
			"subscription": nilIfMissing(sub),
			"intervals":   egress.AllowedIntervals,
			"fixed_group": egress.NodesGroup,
		})
	case http.MethodPut:
		var body struct {
			URL             string `json:"url"`
			IntervalMinutes int    `json:"interval_minutes"`
			Enabled         *bool  `json:"enabled"`
			Name            string `json:"name"`
			RefreshNow      *bool  `json:"refresh_now"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
		enabled := true
		if body.Enabled != nil {
			enabled = *body.Enabled
		}
		sub, err := s.store.UpsertEgressSubscription(body.URL, body.IntervalMinutes, enabled, body.Name, egress.NodesGroup)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		refresh := true
		if body.RefreshNow != nil {
			refresh = *body.RefreshNow
		}
		if refresh && sub.Enabled {
			result, e := egress.RefreshSubscription(s.store)
			if e != nil {
				fresh, _ := s.store.GetEgressSubscription()
				writeJSON(w, http.StatusOK, map[string]any{
					"subscription": fresh,
					"sync_error":   e.Error(),
				})
				return
			}
			writeJSON(w, http.StatusOK, result)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"subscription": sub})
	case http.MethodDelete:
		clearGroup := r.URL.Query().Get("clear_group") == "1"
		var body struct {
			ClearGroup bool `json:"clear_group"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.ClearGroup {
			clearGroup = true
		}
		out, err := s.store.DeleteEgressSubscription(clearGroup)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, out)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleProxySubscriptionRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := egress.RefreshSubscription(s.store)
	if err != nil {
		sub, _ := s.store.GetEgressSubscription()
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":        err.Error(),
			"subscription": nilIfMissing(sub),
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func nilIfMissing(sub store.EgressSubscription) any {
	if sub.ID == 0 {
		return nil
	}
	return sub
}

func (s *Server) handleAdminProxyByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin/proxies/")
	path = strings.Trim(path, "/")
	if path == "" {
		s.handleAdminProxies(w, r)
		return
	}
	parts := strings.Split(path, "/")
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
		n, err := s.store.GetEgress(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		n.Password = ""
		writeJSON(w, http.StatusOK, n)
	case action == "" && r.Method == http.MethodDelete:
		if err := s.store.DeleteEgress(id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case action == "enable" && r.Method == http.MethodPost:
		_ = s.store.SetEgressStatus(id, "可用", "")
		n, _ := s.store.GetEgress(id)
		n.Password = ""
		writeJSON(w, http.StatusOK, n)
	case action == "disable" && r.Method == http.MethodPost:
		_ = s.store.SetEgressStatus(id, "禁用", "manual")
		n, _ := s.store.GetEgress(id)
		n.Password = ""
		writeJSON(w, http.StatusOK, n)
	case action == "test" && r.Method == http.MethodPost:
		n, err := s.store.GetEgress(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		lat, err := egress.Probe(n, s.cfg.BaseURL)
		if err != nil {
			_ = s.store.UpdateEgressProbe(id, false, lat, err.Error())
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": false, "latency": lat, "error": err.Error(),
			})
			return
		}
		_ = s.store.UpdateEgressProbe(id, true, lat, "")
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "latency": lat})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

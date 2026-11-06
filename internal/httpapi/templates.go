package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/trae/runninghub2api/internal/provider/runninghub"
	"github.com/trae/runninghub2api/internal/store"
)

// demoFromSavedTemplate maps a stored template into the same shape as FetchWebappDemo.
// Access password is NOT returned to downstream; gateway injects it on submit.
func demoFromSavedTemplate(tpl store.WebappTemplate) map[string]any {
	out := map[string]any{
		"webapp_id":           tpl.WebappID,
		"name":                tpl.Name,
		"description":         tpl.Description,
		"source":              "saved_template",
		"cached":              true,
		"template_id":         tpl.ID,
		"has_access_password": strings.TrimSpace(tpl.AccessPassword) != "",
		"node_info_list":      json.RawMessage("[]"),
	}
	if len(tpl.NodeInfoList) > 0 {
		out["node_info_list"] = tpl.NodeInfoList
	}
	if len(tpl.Example) > 0 {
		out["example"] = tpl.Example
	} else if strings.TrimSpace(tpl.ExampleJSON) != "" {
		out["example"] = json.RawMessage(tpl.ExampleJSON)
	}
	if tpl.Source != "" {
		out["saved_from"] = tpl.Source
	}
	return out
}

func (s *Server) handleAdminTemplates(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.store.ListWebappTemplates()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": list})
	case http.MethodPost:
		s.saveWebappTemplate(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAdminTemplateByID(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/admin/templates/"), "/")
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "template id required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		t, err := s.store.GetWebappTemplate(id)
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, t)
	case http.MethodDelete:
		if err := s.store.DeleteWebappTemplate(id); errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		} else if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) saveWebappTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WebappID       string          `json:"webapp_id"`
		Name           string          `json:"name"`
		Description    string          `json:"description"`
		Source         string          `json:"source"`
		Note           string          `json:"note"`
		AccessPassword string          `json:"access_password"`
		NodeInfoList   json.RawMessage `json:"node_info_list"`
		Example        json.RawMessage `json:"example"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	webappID := strings.TrimSpace(body.WebappID)
	if webappID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "webapp_id required"})
		return
	}

	nodeJSON := strings.TrimSpace(string(body.NodeInfoList))
	exampleJSON := strings.TrimSpace(string(body.Example))
	name := strings.TrimSpace(body.Name)
	desc := strings.TrimSpace(body.Description)
	source := strings.TrimSpace(body.Source)

	if nodeJSON == "" || nodeJSON == "null" {
		rhKey := ""
		if acc, err := s.store.FindAccountWithAPIKey(); err == nil {
			rhKey = strings.TrimSpace(acc.APIKey)
		}
		demo, err := runninghub.FetchWebappDemo(s.cfg.BaseURL, webappID, rhKey)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
		if name == "" {
			name = fmt.Sprint(demo["name"])
			if name == "<nil>" {
				name = ""
			}
		}
		if desc == "" {
			desc = fmt.Sprint(demo["description"])
			if desc == "<nil>" {
				desc = ""
			}
		}
		if source == "" {
			source = fmt.Sprint(demo["source"])
			if source == "<nil>" {
				source = ""
			}
		}
		if b, err := json.Marshal(demo["node_info_list"]); err == nil {
			nodeJSON = string(b)
		}
		if b, err := json.Marshal(demo["example"]); err == nil {
			exampleJSON = string(b)
		}
	}

	if nodeJSON == "" || nodeJSON == "null" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "node_info_list required"})
		return
	}
	if exampleJSON == "null" {
		exampleJSON = ""
	}

	saved, err := s.store.UpsertWebappTemplate(store.WebappTemplate{
		WebappID:       webappID,
		Name:           name,
		Description:    desc,
		Source:         source,
		Note:           strings.TrimSpace(body.Note),
		AccessPassword: strings.TrimSpace(body.AccessPassword),
		NodeInfoJSON:   nodeJSON,
		ExampleJSON:    exampleJSON,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrInvalidNodeInfo) ||
			errors.Is(err, store.ErrInvalidExample) ||
			strings.Contains(err.Error(), "webapp_id required") {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

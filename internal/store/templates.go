package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func randomHex(n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("randomHex: n must be > 0")
	}
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b)[:n], nil
}

type WebappTemplate struct {
	ID           string          `json:"id"`
	WebappID     string          `json:"webapp_id"`
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	Source       string          `json:"source,omitempty"`
	NodeInfoJSON string          `json:"-"`
	ExampleJSON  string          `json:"-"`
	NodeInfoList json.RawMessage `json:"node_info_list"`
	Example      json.RawMessage `json:"example,omitempty"`
	Note         string          `json:"note,omitempty"`
	NodeCount    int             `json:"node_count"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}

var (
	ErrInvalidNodeInfo = errors.New("node_info_json must be a JSON array")
	ErrInvalidExample  = errors.New("invalid example_json")
)

const templateSelect = `SELECT id, webapp_id, name, description, source, node_info_json, example_json, note, created_at, updated_at FROM webapp_templates`

func (t *WebappTemplate) hydrateJSON() {
	if strings.TrimSpace(t.NodeInfoJSON) == "" {
		t.NodeInfoList = json.RawMessage("[]")
		t.NodeCount = 0
	} else {
		t.NodeInfoList = json.RawMessage(t.NodeInfoJSON)
		var arr []any
		if err := json.Unmarshal(t.NodeInfoList, &arr); err == nil {
			t.NodeCount = len(arr)
		}
	}
	if strings.TrimSpace(t.ExampleJSON) != "" {
		t.Example = json.RawMessage(t.ExampleJSON)
	}
}

func validateNodeInfoJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "[]", nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return "", ErrInvalidNodeInfo
	}
	b, err := json.Marshal(arr)
	if err != nil {
		return "", ErrInvalidNodeInfo
	}
	return string(b), nil
}

func (s *Store) UpsertWebappTemplate(t WebappTemplate) (WebappTemplate, error) {
	t.WebappID = strings.TrimSpace(t.WebappID)
	if t.WebappID == "" {
		return WebappTemplate{}, fmt.Errorf("webapp_id required")
	}
	normalized, err := validateNodeInfoJSON(t.NodeInfoJSON)
	if err != nil {
		return WebappTemplate{}, err
	}
	t.NodeInfoJSON = normalized
	t.ExampleJSON = strings.TrimSpace(t.ExampleJSON)
	if t.ExampleJSON != "" && !json.Valid([]byte(t.ExampleJSON)) {
		return WebappTemplate{}, ErrInvalidExample
	}

	t.Name = strings.TrimSpace(t.Name)
	t.Note = strings.TrimSpace(t.Note)
	t.Source = strings.TrimSpace(t.Source)
	t.Description = strings.TrimSpace(t.Description)
	ts := now()

	existing, getErr := s.GetWebappTemplateByWebappID(t.WebappID)
	switch {
	case getErr == nil:
		t.ID = existing.ID
		if t.Name == "" {
			t.Name = existing.Name
		}
		if t.Description == "" {
			t.Description = existing.Description
		}
		if t.Source == "" {
			t.Source = existing.Source
		}
		if t.ExampleJSON == "" {
			t.ExampleJSON = existing.ExampleJSON
		}
		if t.Note == "" {
			t.Note = existing.Note
		}
	case errors.Is(getErr, ErrNotFound):
		if strings.TrimSpace(t.ID) == "" {
			hexID, genErr := randomHex(10)
			if genErr != nil {
				return WebappTemplate{}, genErr
			}
			t.ID = "tpl_" + hexID
		}
		if t.Name == "" {
			t.Name = t.WebappID
		}
	default:
		return WebappTemplate{}, getErr
	}

	_, err = s.db.Exec(`
INSERT INTO webapp_templates(id, webapp_id, name, description, source, node_info_json, example_json, note, created_at, updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(webapp_id) DO UPDATE SET
  name=CASE WHEN excluded.name='' THEN webapp_templates.name ELSE excluded.name END,
  description=CASE WHEN excluded.description='' THEN webapp_templates.description ELSE excluded.description END,
  source=CASE WHEN excluded.source='' THEN webapp_templates.source ELSE excluded.source END,
  node_info_json=excluded.node_info_json,
  example_json=CASE WHEN excluded.example_json='' THEN webapp_templates.example_json ELSE excluded.example_json END,
  note=CASE WHEN excluded.note='' THEN webapp_templates.note ELSE excluded.note END,
  updated_at=excluded.updated_at
`, t.ID, t.WebappID, t.Name, t.Description, t.Source, t.NodeInfoJSON, t.ExampleJSON, t.Note, ts, ts)
	if err != nil {
		return WebappTemplate{}, err
	}
	return s.GetWebappTemplateByWebappID(t.WebappID)
}

func (s *Store) GetWebappTemplate(id string) (WebappTemplate, error) {
	t, err := s.scanTemplate(s.db.QueryRow(templateSelect+` WHERE id=?`, strings.TrimSpace(id)))
	if errors.Is(err, sql.ErrNoRows) {
		return WebappTemplate{}, ErrNotFound
	}
	return t, err
}

func (s *Store) GetWebappTemplateByWebappID(webappID string) (WebappTemplate, error) {
	t, err := s.scanTemplate(s.db.QueryRow(templateSelect+` WHERE webapp_id=?`, strings.TrimSpace(webappID)))
	if errors.Is(err, sql.ErrNoRows) {
		return WebappTemplate{}, ErrNotFound
	}
	return t, err
}

func (s *Store) ListWebappTemplates() ([]WebappTemplate, error) {
	rows, err := s.db.Query(templateSelect + ` ORDER BY updated_at DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]WebappTemplate, 0)
	for rows.Next() {
		t, err := s.scanTemplateRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) DeleteWebappTemplate(id string) error {
	res, err := s.db.Exec(`DELETE FROM webapp_templates WHERE id=?`, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) scanTemplate(row rowScanner) (WebappTemplate, error) {
	var t WebappTemplate
	err := row.Scan(
		&t.ID, &t.WebappID, &t.Name, &t.Description, &t.Source,
		&t.NodeInfoJSON, &t.ExampleJSON, &t.Note, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return t, err
	}
	t.hydrateJSON()
	return t, nil
}

func (s *Store) scanTemplateRows(rows *sql.Rows) (WebappTemplate, error) {
	return s.scanTemplate(rows)
}

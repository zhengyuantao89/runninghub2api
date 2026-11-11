package runninghub

import (
	"fmt"
	"strings"
)

// QCCreationMode is one Runow quick-creation mode (HAR creation-modes).
type QCCreationMode struct {
	ID           string   `json:"id"`
	ModeKey      string   `json:"mode_key"`
	Name         string   `json:"name"`
	Sort         int      `json:"sort"`
	BindingCount int      `json:"binding_count"`
	BindingIDs   []string `json:"binding_ids"`
}

// QCModelEntry is a flattened VIDEO/IMAGE model binding from /api/qc/v2/models.
type QCModelEntry struct {
	Category      string   `json:"category"`
	Group         string   `json:"group"`
	BindingID     string   `json:"binding_id"`
	SkuID         string   `json:"sku_id"`
	Name          string   `json:"name"`
	NameEn        string   `json:"name_en,omitempty"`
	Modes         []string `json:"modes,omitempty"`
	BillingHint   string   `json:"billing_hint,omitempty"` // free_seen|paid_seen|mixed_seen|unknown
	ParamKeys     []string `json:"param_keys,omitempty"`
	SupportsFree  bool     `json:"supports_free,omitempty"`
	SupportsPaid  bool     `json:"supports_paid,omitempty"`
	ModelEntryKey string   `json:"model_entry_key,omitempty"`
}

type QCCatalog struct {
	Modes   []QCCreationMode `json:"modes"`
	Models  []QCModelEntry   `json:"models"`
	Source  string           `json:"source,omitempty"`
	Fetched string           `json:"fetched_at,omitempty"`
}

func (c *Client) QCCreationModes(categoryID string) ([]QCCreationMode, error) {
	categoryID = strings.TrimSpace(categoryID)
	if categoryID == "" {
		categoryID = "VIDEO"
	}
	raw, err := c.postJSON("/qc-service/api/qc/v2/creation-modes", map[string]any{
		"categoryId": categoryID,
	})
	if err != nil {
		return nil, err
	}
	arr, _ := raw["data"].([]any)
	out := make([]QCCreationMode, 0, len(arr))
	for _, it := range arr {
		m, _ := it.(map[string]any)
		if m == nil {
			continue
		}
		ids := make([]string, 0)
		for _, b := range asSlice(m["bindingIds"]) {
			if s := asString(b); s != "" {
				ids = append(ids, s)
			}
		}
		out = append(out, QCCreationMode{
			ID:           asString(m["id"]),
			ModeKey:      asString(m["modeKey"]),
			Name:         asString(m["name"]),
			Sort:         asInt(m["sort"]),
			BindingCount: asInt(m["bindingCount"]),
			BindingIDs:   ids,
		})
	}
	return out, nil
}

func (c *Client) QCModels(categoryIDs ...string) (map[string]any, error) {
	ids := make([]string, 0, len(categoryIDs))
	for _, id := range categoryIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		ids = []string{"VIDEO", "IMAGE"}
	}
	return c.postJSON("/qc-service/api/qc/v2/models", map[string]any{
		"categoryIds": ids,
	})
}

// ParseQCModelsCatalog flattens /api/qc/v2/models response for one category.
func ParseQCModelsCatalog(raw map[string]any, category string, modeIndex map[string][]string) []QCModelEntry {
	category = strings.TrimSpace(category)
	if category == "" {
		category = "VIDEO"
	}
	cats, _ := nested(raw, "data", "categories").(map[string]any)
	if cats == nil {
		cats, _ = raw["categories"].(map[string]any)
	}
	nodes, _ := cats[category].([]any)
	out := make([]QCModelEntry, 0)
	walkQCModelNodes(nodes, category, "", modeIndex, &out)
	return out
}

func walkQCModelNodes(nodes []any, category, group string, modeIndex map[string][]string, out *[]QCModelEntry) {
	for _, it := range nodes {
		n, _ := it.(map[string]any)
		if n == nil {
			continue
		}
		t := asString(n["type"])
		if t == "group" {
			g := firstNonEmpty(asString(n["groupName"]), asString(n["groupKey"]), group)
			children, _ := n["children"].([]any)
			walkQCModelNodes(children, category, g, modeIndex, out)
			continue
		}
		bid := asString(n["bindingId"])
		if bid == "" && t != "model" {
			continue
		}
		name := firstNonEmpty(asString(n["skuNameCn"]), asString(n["skuNameEn"]), asString(n["name"]))
		modes := []string(nil)
		if modeIndex != nil {
			modes = append([]string{}, modeIndex[bid]...)
		}
		*out = append(*out, QCModelEntry{
			Category:      category,
			Group:         group,
			BindingID:     bid,
			SkuID:         asString(n["skuId"]),
			Name:          name,
			NameEn:        firstNonEmpty(asString(n["skuNameEn"]), asString(n["name"])),
			Modes:         modes,
			ModelEntryKey: asString(n["modelEntryKey"]),
		})
	}
}

func BuildModeIndex(modes []QCCreationMode) map[string][]string {
	idx := map[string][]string{}
	for _, m := range modes {
		mk := strings.TrimSpace(m.ModeKey)
		if mk == "" {
			continue
		}
		for _, bid := range m.BindingIDs {
			bid = strings.TrimSpace(bid)
			if bid == "" {
				continue
			}
			list := idx[bid]
			found := false
			for _, x := range list {
				if x == mk {
					found = true
					break
				}
			}
			if !found {
				idx[bid] = append(list, mk)
			}
		}
	}
	return idx
}

func FilterQCModelsByMode(models []QCModelEntry, modeKey string) []QCModelEntry {
	modeKey = strings.TrimSpace(modeKey)
	if modeKey == "" {
		return models
	}
	out := make([]QCModelEntry, 0)
	for _, m := range models {
		for _, mk := range m.Modes {
			if mk == modeKey {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

func (c *Client) FetchQCCatalog(category string) (QCCatalog, error) {
	category = strings.TrimSpace(category)
	if category == "" {
		category = "VIDEO"
	}
	modes, err := c.QCCreationModes(category)
	if err != nil {
		return QCCatalog{}, err
	}
	raw, err := c.QCModels(category)
	if err != nil {
		return QCCatalog{}, err
	}
	idx := BuildModeIndex(modes)
	models := ParseQCModelsCatalog(raw, category, idx)
	if len(models) == 0 {
		return QCCatalog{}, fmt.Errorf("qc models empty for %s", category)
	}
	return QCCatalog{Modes: modes, Models: models, Source: "live"}, nil
}

func asSlice(v any) []any {
	if v == nil {
		return nil
	}
	if a, ok := v.([]any); ok {
		return a
	}
	return nil
}

// CreationModeParamValue maps UI/API mode keys to params.creationMode strings seen in HAR.
func CreationModeParamValue(modeKey string) string {
	switch strings.ToUpper(strings.TrimSpace(modeKey)) {
	case "MULTIMODAL_REFERENCE", "MULTIMODAL":
		return "multimodal"
	case "SUBJECT_REFERENCE", "SUBJECT":
		return "subjectRef"
	case "FREE_CREATION", "FREE":
		return "freeCreation"
	case "FIRST_LAST_FRAME", "FIRST_LAST":
		return "firstLastFrame"
	default:
		s := strings.TrimSpace(modeKey)
		if s == "" {
			return "freeCreation"
		}
		return s
	}
}

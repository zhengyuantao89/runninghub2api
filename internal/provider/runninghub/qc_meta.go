package runninghub

import (
	"encoding/json"
	"strings"
)

// QCTaskMeta is persisted into video_tasks.node_info_json for generic QC submits.
type QCTaskMeta struct {
	QC           bool           `json:"__qc"`
	BindingID    string         `json:"binding_id"`
	SkuID        string         `json:"sku_id"`
	CategoryID   string         `json:"category_id"`
	CreationMode string         `json:"creation_mode"`
	AllowPaid    bool           `json:"allow_paid"`
	Params       map[string]any `json:"params"`
}

func ParseQCTaskMetaJSON(raw string) (QCTaskMeta, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return QCTaskMeta{}, false
	}
	var nodes []NodeInput
	if err := json.Unmarshal([]byte(raw), &nodes); err == nil {
		for _, n := range nodes {
			if n.NodeID == "__qc_meta" || n.FieldName == "meta" {
				s := strings.TrimSpace(asString(n.FieldValue))
				if s == "" {
					if b, err := json.Marshal(n.FieldValue); err == nil {
						s = string(b)
					}
				}
				var env QCTaskMeta
				if err := json.Unmarshal([]byte(s), &env); err == nil && env.QC {
					return env, true
				}
			}
		}
	}
	var env QCTaskMeta
	if err := json.Unmarshal([]byte(raw), &env); err == nil && env.QC {
		return env, true
	}
	return QCTaskMeta{}, false
}

func EncodeQCTaskMetaNode(meta QCTaskMeta) ([]NodeInput, error) {
	meta.QC = true
	b, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	return []NodeInput{{
		NodeID: "__qc_meta", FieldName: "meta", FieldType: "STRING", FieldValue: string(b),
	}}, nil
}

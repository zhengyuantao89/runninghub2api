package runninghub

import (
	"encoding/json"
	"testing"
)

func TestEncodeParseQCTaskMeta(t *testing.T) {
	meta := QCTaskMeta{
		BindingID:    "b1",
		SkuID:        "s1",
		CategoryID:   "VIDEO",
		CreationMode: "FREE_CREATION",
		AllowPaid:    true,
		Params:       map[string]any{"prompt": "hi"},
	}
	nodes, err := EncodeQCTaskMetaNode(meta)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("encode: %v %#v", err, nodes)
	}
	b, _ := json.Marshal(nodes)
	got, ok := ParseQCTaskMetaJSON(string(b))
	if !ok || !got.QC || got.BindingID != "b1" || got.SkuID != "s1" || !got.AllowPaid {
		t.Fatalf("parse nodes: ok=%v got=%#v", ok, got)
	}
	raw, _ := json.Marshal(QCTaskMeta{QC: true, BindingID: "x", SkuID: "y"})
	got2, ok2 := ParseQCTaskMetaJSON(string(raw))
	if !ok2 || got2.BindingID != "x" {
		t.Fatalf("parse raw: ok=%v got=%#v", ok2, got2)
	}
}

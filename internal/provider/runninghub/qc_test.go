package runninghub

import (
	"testing"
)

func TestNormalizeSubmitMode(t *testing.T) {
	if NormalizeSubmitMode("qc") != SubmitModeQC {
		t.Fatal("expected qc")
	}
	if NormalizeSubmitMode("qc_free") != SubmitModeQCFree {
		t.Fatal("expected qc_free")
	}
	if NormalizeSubmitMode("qc_free_image") != SubmitModeQCFreeImage {
		t.Fatal("expected qc_free_image")
	}
	if NormalizeSubmitMode("webapp") != SubmitModeWebapp {
		t.Fatal("expected webapp")
	}
	if NormalizeSubmitMode("") != SubmitModeWebapp {
		t.Fatal("default webapp")
	}
}

func TestParseQCModelsCatalog(t *testing.T) {
	raw := map[string]any{
		"data": map[string]any{
			"categories": map[string]any{
				"VIDEO": []any{
					map[string]any{
						"type": "group", "groupName": "Demo",
						"children": []any{
							map[string]any{
								"type": "model", "bindingId": "b1", "skuId": "s1",
								"skuNameCn": "测试模型", "skuNameEn": "demo",
							},
						},
					},
				},
			},
		},
	}
	idx := map[string][]string{"b1": {"FREE_CREATION"}}
	models := ParseQCModelsCatalog(raw, "VIDEO", idx)
	if len(models) != 1 || models[0].BindingID != "b1" || models[0].Modes[0] != "FREE_CREATION" {
		t.Fatalf("%+v", models)
	}
}

func TestCreationModeParamValue(t *testing.T) {
	if CreationModeParamValue("SUBJECT_REFERENCE") != "subjectRef" {
		t.Fatal(CreationModeParamValue("SUBJECT_REFERENCE"))
	}
	if CreationModeParamValue("FREE_CREATION") != "freeCreation" {
		t.Fatal(CreationModeParamValue("FREE_CREATION"))
	}
}

func TestBuildQCFreeImageCreateRequest(t *testing.T) {
	t2i := buildQCFreeImageCreateRequest("一只猫", nil, "16:9", "2k", "p1", "page1", "req-1", "", "", DefaultQCFreeImageFamilyNano)
	if t2i["bindingId"] != DefaultQCFreeImageT2IBindingID || t2i["skuId"] != DefaultQCFreeImageT2ISkuID {
		t.Fatalf("nano t2i ids %+v", t2i)
	}
	p1, _ := t2i["params"].(map[string]any)
	if p1["resolution"] != "2k" || p1["aspectRatio"] != "16:9" {
		t.Fatalf("t2i params %+v", p1)
	}
	if _, ok := p1["imageUrls"]; ok {
		t.Fatal("t2i should omit imageUrls")
	}

	i2i := buildQCFreeImageCreateRequest("换装", []string{"https://cdn.example/a.png", "https://cdn.example/b.png"}, "9:16 (Portrait)", "1k", "p1", "page1", "req-2", "", "", DefaultQCFreeImageFamilyNano)
	if i2i["bindingId"] != DefaultQCFreeImageBindingID || i2i["skuId"] != DefaultQCFreeImageSkuID {
		t.Fatalf("nano i2i ids %+v", i2i)
	}
	p2, _ := i2i["params"].(map[string]any)
	urls, _ := p2["imageUrls"].([]string)
	if len(urls) != 2 || p2["aspectRatio"] != "9:16" || p2["resolution"] != "1k" {
		t.Fatalf("i2i params %+v", p2)
	}

	gptT2I := buildQCFreeImageCreateRequest("x", nil, "1:1", "1k", "p1", "page1", "req-3", "", "", DefaultQCFreeImageFamilyGPT)
	if gptT2I["bindingId"] != DefaultQCFreeGPTImageT2IBindingID || gptT2I["skuId"] != DefaultQCFreeGPTImageT2ISkuID {
		t.Fatalf("gpt t2i %+v", gptT2I)
	}
	gptI2I := buildQCFreeImageCreateRequest("x", []string{"https://cdn.example/a.png"}, "1:1", "1k", "p1", "page1", "req-4", "", "", DefaultQCFreeImageFamilyGPT)
	if gptI2I["bindingId"] != DefaultQCFreeGPTImageBindingID || gptI2I["skuId"] != DefaultQCFreeGPTImageSkuID {
		t.Fatalf("gpt i2i %+v", gptI2I)
	}
}

func TestParseQCFeePreviewFree(t *testing.T) {
	f := parseQCFeePreview(map[string]any{
		"free":               true,
		"welfareFree":        true,
		"passed":             true,
		"freeQuotaRemaining": float64(2),
		"skuId":              DefaultQCFreeSkuID,
	})
	if !f.IsFree() || f.FreeQuotaRemaining != 2 {
		t.Fatalf("got %+v", f)
	}
}

func TestParseQCFeePreviewPaid(t *testing.T) {
	f := parseQCFeePreview(map[string]any{
		"free":               false,
		"welfareFree":        false,
		"requiredPointUnit":  "988720",
		"insufficientType":   "rh",
		"freeQuotaRemaining": float64(0),
	})
	if f.IsFree() {
		t.Fatal("should not be free")
	}
	if f.RequiredPointUnit != "988720" {
		t.Fatalf("points=%s", f.RequiredPointUnit)
	}
}

func TestNormalizeQCFreeVideoMode(t *testing.T) {
	if NormalizeQCFreeVideoMode("multimodal") != QCFreeVideoModeMultimodal {
		t.Fatal("multimodal")
	}
	if NormalizeQCFreeVideoMode("多模态参考") != QCFreeVideoModeMultimodal {
		t.Fatal("cn multimodal")
	}
	if NormalizeQCFreeVideoMode("freeCreation") != QCFreeVideoModeFreeCreation {
		t.Fatal("freeCreation")
	}
	if NormalizeQCFreeVideoMode("i2v") != QCFreeVideoModeFreeCreation {
		t.Fatal("legacy i2v")
	}
}

func TestResolveQCFreeVideoModel(t *testing.T) {
	b, s, m := ResolveQCFreeVideoModel("multimodal", 2)
	if b != DefaultQCFreeMultiBindingID || s != DefaultQCFreeMultiSkuID || m != QCFreeVideoModeMultimodal {
		t.Fatalf("multi %s %s %s", b, s, m)
	}
	b, s, m = ResolveQCFreeVideoModel("freeCreation", 1)
	if b != DefaultQCFreeBindingID || s != DefaultQCFreeSkuID || m != QCFreeVideoModeFreeCreation {
		t.Fatalf("i2v %s %s %s", b, s, m)
	}
	b, s, m = ResolveQCFreeVideoModel("freeCreation", 0)
	if b != DefaultQCFreeT2VBindingID || s != DefaultQCFreeT2VSkuID || m != QCFreeVideoModeFreeCreation {
		t.Fatalf("t2v %s %s %s", b, s, m)
	}
}

func TestBuildQCFreeCreateRequest(t *testing.T) {
	req := buildQCFreeCreateRequest("人物动起来", []string{"https://cdn.example/a.png"}, "9:16", "4", "480p", QCFreeVideoModeFreeCreation, "p1", "page1", "req-1")
	if req["bindingId"] != DefaultQCFreeBindingID || req["skuId"] != DefaultQCFreeSkuID {
		t.Fatalf("i2v ids %+v", req)
	}
	params, _ := req["params"].(map[string]any)
	if params["creationMode"] != "freeCreation" {
		t.Fatalf("mode=%v", params["creationMode"])
	}
	if params["firstFrameUrl"] != "https://cdn.example/a.png" {
		t.Fatalf("url=%v", params["firstFrameUrl"])
	}
	if params["duration"] != 4 {
		t.Fatalf("duration=%v", params["duration"])
	}
	if params["aspectRatio"] != "9:16" {
		t.Fatalf("aspect=%v", params["aspectRatio"])
	}
	if params["resolution"] != "480p" || params["73##megapixels"] != "480p" {
		t.Fatalf("resolution=%v megapixels=%v", params["resolution"], params["73##megapixels"])
	}

	t2v := buildQCFreeCreateRequest("人物动起来", nil, "3:2 (Photo)", "5", "768p", QCFreeVideoModeFreeCreation, "p1", "page1", "req-2")
	if t2v["bindingId"] != DefaultQCFreeT2VBindingID || t2v["skuId"] != DefaultQCFreeT2VSkuID {
		t.Fatalf("t2v ids %+v", t2v)
	}
	p2, _ := t2v["params"].(map[string]any)
	if _, ok := p2["firstFrameUrl"]; ok {
		t.Fatal("t2v must omit firstFrameUrl")
	}
	if p2["duration"] != 5 || p2["resolution"] != "768p" {
		t.Fatalf("t2v params %+v", p2)
	}

	multi := buildQCFreeCreateRequest("人物动起来", []string{"https://cdn.example/a.png", "https://cdn.example/b.png"}, "1:1 (Square)", "5", "1080p", QCFreeVideoModeMultimodal, "p1", "page1", "req-3")
	if multi["bindingId"] != DefaultQCFreeMultiBindingID || multi["skuId"] != DefaultQCFreeMultiSkuID {
		t.Fatalf("multi ids %+v", multi)
	}
	pm, _ := multi["params"].(map[string]any)
	if pm["creationMode"] != "multimodal" || pm["refImage1"] != "https://cdn.example/a.png" || pm["refImage2"] != "https://cdn.example/b.png" {
		t.Fatalf("multi params %+v", pm)
	}
	if _, ok := pm["firstFrameUrl"]; ok {
		t.Fatal("multimodal must omit firstFrameUrl")
	}
}

func TestAWS4SigningKeyStable(t *testing.T) {
	key := aws4SigningKey("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "20130524", "us-east-1", "s3")
	if len(key) != 32 {
		t.Fatalf("len=%d", len(key))
	}
}

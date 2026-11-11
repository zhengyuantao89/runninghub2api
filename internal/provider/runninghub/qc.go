package runninghub

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Runow QC free paths from HAR 2026-11-10.
const (
	// Video free family — official picker groupName: Minimax H3-RH Enhanced.
	// Modes: 多模态参考 (multimodal) / 自由创作 (freeCreation; with image → i2v, without → t2v).
	DefaultQCFreeBindingID       = "2103076348802752513" // Image-to-Video (freeCreation + firstFrame)
	DefaultQCFreeSkuID           = "2102602472381587457"
	DefaultQCFreeT2VBindingID    = "2103076432063881217" // Text-to-Video (freeCreation, no image)
	DefaultQCFreeT2VSkuID        = "2102602278114009090"
	DefaultQCFreeMultiBindingID  = "2103076210080342018" // Multi-Reference
	DefaultQCFreeMultiSkuID      = "2102602670923161601"
	DefaultQCFreeVideoName       = "Minimax H3-RH Enhanced"
	DefaultQCFreeVideoMaxRefs    = 9 // multimodal refImage1..9
	QCFreeVideoModeFreeCreation  = "freeCreation"
	QCFreeVideoModeMultimodal    = "multimodal"
	// Official free image families (HAR): same family, with refs → i2i sku, without → t2i sku.
	DefaultQCFreeImageFamilyNano = "nano_banana2"
	DefaultQCFreeImageFamilyGPT  = "gpt_image_2"
	// Nano Banana2 i2i / t2i.
	DefaultQCFreeImageBindingID    = "2043951705349238786"
	DefaultQCFreeImageSkuID        = "2027196343409463297"
	DefaultQCFreeImageT2IBindingID = "2043954826653806594"
	DefaultQCFreeImageT2ISkuID     = "2027192837726294017"
	DefaultQCFreeImageName         = "Nano Banana2-Low-Cost"
	// GPT Image 2.0 i2i / t2i.
	DefaultQCFreeGPTImageBindingID    = "2046586338670891009"
	DefaultQCFreeGPTImageSkuID        = "2046503667076751361"
	DefaultQCFreeGPTImageT2IBindingID = "2046581921695485954"
	DefaultQCFreeGPTImageT2ISkuID     = "2046514150500524033"
	DefaultQCFreeGPTImageName         = "GPT Image2.0-Low-Cost"
	// Official free imageUrls maxUploadCount from HAR models/binding.
	DefaultQCFreeImageMaxRefs = 10
	SessionKindWebapp              = "webapp"
	SessionKindQC                  = "qc"
	SubmitModeWebapp               = "webapp"
	SubmitModeQCFree               = "qc_free"
	SubmitModeQCFreeImage          = "qc_free_image"
	SubmitModeQC                   = "qc" // generic multi-model (free or paid)
)

// QCStorageCreds is temporary COS credentials from /api/storage/getCredentials.
type QCStorageCreds struct {
	Endpoint    string
	Region      string
	Bucket      string
	AccessKey   string
	SecretKey   string
	SessionToken string
	CDNDomain   string
	Identify    string
	Type        string
}

// QCFeePreview summarizes fee-preview / prepare fee fields.
type QCFeePreview struct {
	Free               bool
	WelfareFree        bool
	Passed             bool
	FreeQuotaRemaining int
	RequiredPointUnit  string
	RequiredCashAmount string
	SkuID              string
	InsufficientType   string
}

func (f QCFeePreview) IsFree() bool {
	return f.Free || f.WelfareFree
}

// NormalizeSubmitMode returns webapp | qc | qc_free | qc_free_image.
func NormalizeSubmitMode(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "qc_generic", "qc-generic", "runow_qc", "qc_paid", "qc-paid":
		return SubmitModeQC
	case "qc", "runow":
		// bare "qc" = generic multi-model (free+paid)
		return SubmitModeQC
	case "qc_free", "qc-free", "free", "qc_free_video", "qc-free-video":
		return SubmitModeQCFree
	case "qc_free_image", "qc-free-image", "qc_image", "free_image", "image_free":
		return SubmitModeQCFreeImage
	default:
		return SubmitModeWebapp
	}
}

func IsQCWebappID(webappID string) bool {
	id := strings.TrimSpace(webappID)
	return strings.HasPrefix(id, "qc:") || strings.HasPrefix(id, "qcimg:")
}

func IsQCImageWebappID(webappID string) bool {
	return strings.HasPrefix(strings.TrimSpace(webappID), "qcimg:")
}

func normalizeQCImageResolution(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	switch s {
	case "", "1", "1k", "1024":
		return "1k"
	case "2", "2k", "2048":
		return "2k"
	case "4", "4k", "4096":
		return "4k"
	default:
		return s
	}
}

func normalizeQCImageAspect(v string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return "9:16"
	}
	// Accept labeled forms by taking the ratio prefix.
	if i := strings.IndexByte(s, ' '); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

func qcFreeImageFromWebappID(webappID string) (bindingID, skuID, family string, hasI2I bool) {
	id := strings.TrimSpace(webappID)
	if !strings.HasPrefix(id, "qcimg:") {
		return "", "", "", false
	}
	sku := strings.TrimPrefix(id, "qcimg:")
	switch sku {
	case DefaultQCFreeImageSkuID:
		return DefaultQCFreeImageBindingID, sku, DefaultQCFreeImageFamilyNano, true
	case DefaultQCFreeImageT2ISkuID:
		return DefaultQCFreeImageT2IBindingID, sku, DefaultQCFreeImageFamilyNano, false
	case DefaultQCFreeGPTImageSkuID:
		return DefaultQCFreeGPTImageBindingID, sku, DefaultQCFreeImageFamilyGPT, true
	case DefaultQCFreeGPTImageT2ISkuID:
		return DefaultQCFreeGPTImageT2IBindingID, sku, DefaultQCFreeImageFamilyGPT, false
	default:
		return "", sku, "", false
	}
}

func normalizeQCFreeImageFamily(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	switch s {
	case DefaultQCFreeImageFamilyGPT, "gpt", "gpt_image", "gpt-image", "gpt-image-2", "gpt_image_2.0", "gptimage2":
		return DefaultQCFreeImageFamilyGPT
	case DefaultQCFreeImageFamilyNano, "nano", "banana", "nano_banana", "nano-banana", "nano-banana-2", "nanobanana2":
		return DefaultQCFreeImageFamilyNano
	// Legacy mistaken UI variants → default family; path still decided by hasImages.
	case "i2i", "t2i", "img2img", "txt2img", "image", "text":
		return DefaultQCFreeImageFamilyNano
	default:
		return ""
	}
}

func familyFromQCFreeImageIDs(bindingID, skuID string) string {
	switch strings.TrimSpace(bindingID) {
	case DefaultQCFreeImageBindingID, DefaultQCFreeImageT2IBindingID:
		return DefaultQCFreeImageFamilyNano
	case DefaultQCFreeGPTImageBindingID, DefaultQCFreeGPTImageT2IBindingID:
		return DefaultQCFreeImageFamilyGPT
	}
	switch strings.TrimSpace(skuID) {
	case DefaultQCFreeImageSkuID, DefaultQCFreeImageT2ISkuID:
		return DefaultQCFreeImageFamilyNano
	case DefaultQCFreeGPTImageSkuID, DefaultQCFreeGPTImageT2ISkuID:
		return DefaultQCFreeImageFamilyGPT
	}
	return ""
}

// ResolveQCFreeImageModel picks free image binding/sku for a model family.
// Family is Nano Banana2 / GPT Image 2.0; hasImages chooses that family's i2i vs t2i sku.
func ResolveQCFreeImageModel(bindingID, skuID, family string, hasImages bool) (bid, sid, fam string) {
	fam = normalizeQCFreeImageFamily(family)
	if fam == "" {
		fam = familyFromQCFreeImageIDs(bindingID, skuID)
	}
	if fam == "" {
		fam = DefaultQCFreeImageFamilyNano
	}
	switch fam {
	case DefaultQCFreeImageFamilyGPT:
		if hasImages {
			return DefaultQCFreeGPTImageBindingID, DefaultQCFreeGPTImageSkuID, fam
		}
		return DefaultQCFreeGPTImageT2IBindingID, DefaultQCFreeGPTImageT2ISkuID, fam
	default:
		if hasImages {
			return DefaultQCFreeImageBindingID, DefaultQCFreeImageSkuID, fam
		}
		return DefaultQCFreeImageT2IBindingID, DefaultQCFreeImageT2ISkuID, fam
	}
}

func buildQCFreeImageCreateRequest(prompt string, imageURLs []string, aspect, resolution, projectID, pageID, clientReqID, bindingID, skuID, family string) map[string]any {
	prompt = strings.TrimSpace(strings.TrimPrefix(prompt, "\ufeff"))
	aspect = normalizeQCImageAspect(aspect)
	res := normalizeQCImageResolution(resolution)
	params := map[string]any{
		"prompt":      prompt,
		"aspectRatio": aspect,
		"resolution":  res,
	}
	hasImages := len(imageURLs) > 0
	bid, sid, _ := ResolveQCFreeImageModel(bindingID, skuID, family, hasImages)
	if hasImages {
		params["imageUrls"] = imageURLs
	}
	return map[string]any{
		"categoryId":      "IMAGE",
		"params":          params,
		"bindingId":       bid,
		"skuId":           sid,
		"generateCount":   1,
		"projectId":       projectID,
		"pageId":          pageID,
		"clientRequestId": clientReqID,
	}
}

func (c *Client) StorageGetCredentials(scene string) (QCStorageCreds, error) {
	var out QCStorageCreds
	if strings.TrimSpace(scene) == "" {
		scene = "qc"
	}
	raw, err := c.postJSON("/api/storage/getCredentials", map[string]any{"scene": scene})
	if err != nil {
		return out, err
	}
	data, _ := nested(raw, "data").(map[string]any)
	if data == nil {
		return out, fmt.Errorf("getCredentials missing data")
	}
	out.Endpoint = asString(data["endpoint"])
	out.Region = asString(data["region"])
	out.Bucket = asString(data["bucket"])
	out.AccessKey = asString(data["accessKey"])
	out.SecretKey = asString(data["secretKey"])
	out.SessionToken = asString(data["sessionToken"])
	out.CDNDomain = asString(data["cdnDomain"])
	out.Identify = asString(data["identify"])
	out.Type = asString(data["type"])
	if out.Bucket == "" || out.Region == "" || out.Identify == "" {
		return out, fmt.Errorf("getCredentials incomplete: bucket/region/identify")
	}
	return out, nil
}

// UploadQCImage uploads to QC COS and returns the public CDN URL.
func (c *Client) UploadQCImage(data []byte, filename string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("image bytes required")
	}
	cred, err := c.StorageGetCredentials("qc")
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = ".png"
	}
	sum := md5.Sum(data)
	key := fmt.Sprintf("%s/uploads/%s%s", cred.Identify, hex.EncodeToString(sum[:]), ext)
	ct := guessContentType(filename)
	if err := putCOSObject(c.HTTP, cred, key, data, ct); err != nil {
		return "", err
	}
	cdn := strings.TrimRight(cred.CDNDomain, "/")
	if cdn == "" {
		cdn = cred.Bucket + ".cos." + cred.Region + ".myqcloud.com"
	}
	if !strings.HasPrefix(cdn, "http") {
		cdn = "https://" + cdn
	}
	return cdn + "/" + key, nil
}

func (c *Client) QCFeePreview(body map[string]any) (QCFeePreview, map[string]any, error) {
	raw, err := c.postJSON("/qc-service/task/quick-creation/fee-preview", body)
	if err != nil {
		return QCFeePreview{}, nil, err
	}
	data, _ := nested(raw, "data").(map[string]any)
	return parseQCFeePreview(data), data, nil
}

func (c *Client) QCProjectCreate(name string) (projectID string, err error) {
	raw, err := c.postJSON("/qc-service/task/quick-creation/v2/canvas/project/create", map[string]any{
		"name": firstNonEmpty(strings.TrimSpace(name), "qc"),
	})
	if err != nil {
		return "", err
	}
	id := asString(nested(raw, "data", "projectId"))
	if id == "" {
		return "", fmt.Errorf("project/create missing projectId")
	}
	return id, nil
}

func (c *Client) QCPageList(projectID string) (pageID string, err error) {
	raw, err := c.postJSON("/qc-service/task/quick-creation/v2/canvas/page/list", map[string]any{
		"projectId": projectID,
	})
	if err != nil {
		return "", err
	}
	pages, _ := nested(raw, "data", "pages").([]any)
	for _, it := range pages {
		item, _ := it.(map[string]any)
		if item == nil {
			continue
		}
		id := asString(item["pageId"])
		if id == "" {
			continue
		}
		if asBool(item["isDefault"]) || pageID == "" {
			pageID = id
			if asBool(item["isDefault"]) {
				break
			}
		}
	}
	if pageID == "" {
		return "", fmt.Errorf("page/list missing pageId")
	}
	return pageID, nil
}

func (c *Client) QCPrepare(body map[string]any) (map[string]any, error) {
	return c.postJSON("/qc-service/task/quick-creation/prepare", body)
}

func (c *Client) QCCommit(prepareToken, quoteID string, createRequest map[string]any) (map[string]any, error) {
	return c.postJSON("/qc-service/task/quick-creation/commit", map[string]any{
		"prepareToken":  prepareToken,
		"quoteId":       firstNonEmpty(quoteID, prepareToken),
		"createRequest": createRequest,
	})
}

func (c *Client) QCBatchStatus(taskIDs ...string) ([]map[string]any, error) {
	ids := make([]string, 0, len(taskIDs))
	for _, id := range taskIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("taskIds required")
	}
	raw, err := c.postJSON("/qc-service/task/quick-creation/batchStatus", map[string]any{
		"taskIds": ids,
	})
	if err != nil {
		return nil, err
	}
	arr, _ := raw["data"].([]any)
	out := make([]map[string]any, 0, len(arr))
	for _, it := range arr {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

func parseQCFeePreview(data map[string]any) QCFeePreview {
	var f QCFeePreview
	if data == nil {
		return f
	}
	f.Free = asBool(data["free"])
	f.WelfareFree = asBool(data["welfareFree"])
	f.Passed = asBool(data["passed"])
	f.FreeQuotaRemaining = asInt(data["freeQuotaRemaining"])
	if f.FreeQuotaRemaining < 0 {
		f.FreeQuotaRemaining = 0
	}
	f.RequiredPointUnit = asString(data["requiredPointUnit"])
	f.RequiredCashAmount = asString(data["requiredCashAmount"])
	if f.RequiredCashAmount == "" {
		if v, ok := data["requiredCashAmount"].(float64); ok {
			f.RequiredCashAmount = fmt.Sprint(v)
		}
	}
	f.SkuID = asString(data["skuId"])
	f.InsufficientType = asString(data["insufficientType"])
	return f
}

func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return t != 0
	case int:
		return t != 0
	default:
		return false
	}
}

func normalizeQCResolution(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	switch s {
	case "480", "480p":
		return "480p"
	case "768", "768p":
		return "768p"
	case "1080", "1080p":
		return "1080p"
	default:
		if s == "" {
			return "480p"
		}
		return s
	}
}

func clampQCDuration(n int) int {
	if n < 4 {
		return 4
	}
	if n > 15 {
		return 15
	}
	return n
}

func NormalizeQCFreeVideoMode(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	switch s {
	case QCFreeVideoModeMultimodal, "multimodal_reference", "multi", "多模态", "多模态参考":
		return QCFreeVideoModeMultimodal
	case QCFreeVideoModeFreeCreation, "free_creation", "free-creation", "自由创作", "free":
		return QCFreeVideoModeFreeCreation
	default:
		// Legacy mistaken variants → freeCreation.
		if s == "i2v" || s == "t2v" || s == "" {
			return QCFreeVideoModeFreeCreation
		}
		return QCFreeVideoModeFreeCreation
	}
}

// ResolveQCFreeVideoModel picks Minimax H3-RH Enhanced binding/sku for mode + images.
func ResolveQCFreeVideoModel(mode string, imageCount int) (bindingID, skuID, creationMode string) {
	creationMode = NormalizeQCFreeVideoMode(mode)
	if creationMode == QCFreeVideoModeMultimodal {
		return DefaultQCFreeMultiBindingID, DefaultQCFreeMultiSkuID, creationMode
	}
	if imageCount > 0 {
		return DefaultQCFreeBindingID, DefaultQCFreeSkuID, creationMode
	}
	return DefaultQCFreeT2VBindingID, DefaultQCFreeT2VSkuID, creationMode
}

func buildQCFreeCreateRequest(prompt string, imageURLs []string, aspect, duration, resolution, mode, projectID, pageID, clientReqID string) map[string]any {
	prompt = strings.TrimSpace(strings.TrimPrefix(prompt, "\ufeff"))
	aspect = strings.TrimSpace(aspect)
	if aspect == "" {
		aspect = "9:16"
	}
	dur := 4
	if d := strings.TrimSpace(duration); d != "" {
		if n := asInt(d); n > 0 {
			dur = n
		}
	}
	dur = clampQCDuration(dur)
	res := normalizeQCResolution(resolution)
	urls := make([]string, 0, len(imageURLs))
	for _, u := range imageURLs {
		u = strings.TrimSpace(u)
		if u != "" {
			urls = append(urls, u)
		}
	}
	bindingID, skuID, creationMode := ResolveQCFreeVideoModel(mode, len(urls))
	params := map[string]any{
		"prompt":         prompt,
		"aspectRatio":    aspect,
		"resolution":     res,
		"duration":       dur,
		"audioMode":      "native",
		"73##megapixels": res,
		"creationMode":   creationMode,
	}
	if creationMode == QCFreeVideoModeMultimodal {
		max := DefaultQCFreeVideoMaxRefs
		if len(urls) < max {
			max = len(urls)
		}
		for i := 0; i < max; i++ {
			params[fmt.Sprintf("refImage%d", i+1)] = urls[i]
		}
	} else if len(urls) > 0 {
		params["firstFrameUrl"] = urls[0]
	}
	return map[string]any{
		"categoryId":      "VIDEO",
		"params":          params,
		"bindingId":       bindingID,
		"skuId":           skuID,
		"generateCount":   1,
		"projectId":       projectID,
		"pageId":          pageID,
		"clientRequestId": clientReqID,
	}
}

// SubmitQCFreeVideo creates a Runow QC free video task (prepare→commit). Refuses non-free quotes.
// Modes: multimodal (refImage1..N) or freeCreation (with image → i2v, without → t2v).
func SubmitQCFreeVideo(in RunInput) (*Session, error) {
	client, err := loginClient(in)
	if err != nil {
		return nil, err
	}
	emit := func(p Progress) {
		p.AccessToken = client.AccessToken
		if in.OnProgress != nil {
			in.OnProgress(p)
		}
	}

	prompt := strings.TrimSpace(strings.TrimPrefix(in.Prompt, "\ufeff"))
	if prompt == "" {
		prompt = firstPromptText(in)
	}
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	mode := strings.TrimSpace(in.CreationMode)
	if mode == "" && in.QCParams != nil {
		mode = strings.TrimSpace(asString(in.QCParams["free_video_mode"]))
	}
	if mode == "" {
		// Queued worker may only have webapp_id = qc:<sku>; infer multimodal from Multi sku.
		sku := strings.TrimSpace(in.SkuID)
		if sku == "" {
			sku = strings.TrimPrefix(strings.TrimSpace(in.WebappID), "qc:")
		}
		if sku == DefaultQCFreeMultiSkuID {
			mode = QCFreeVideoModeMultimodal
		}
	}
	mode = NormalizeQCFreeVideoMode(mode)

	imageURLs := make([]string, 0, 8)
	seen := map[string]struct{}{}
	addURL := func(u string) {
		u = strings.TrimSpace(u)
		if u == "" {
			return
		}
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		imageURLs = append(imageURLs, u)
	}
	if u := strings.TrimSpace(in.FirstFrameURL); u != "" {
		addURL(u)
	}
	type namedBytes struct {
		name string
		data []byte
	}
	uploads := make([]namedBytes, 0, 8)
	if img, filename, err := loadDefaultImage(in); err == nil && len(img) > 0 {
		uploads = append(uploads, namedBytes{name: filename, data: img})
	}
	for key, raw := range in.ExtraImages {
		if len(raw) == 0 {
			continue
		}
		name := strings.TrimSpace(key) + ".png"
		if name == ".png" {
			name = "ref.png"
		}
		uploads = append(uploads, namedBytes{name: name, data: raw})
	}
	for _, n := range in.NodeInfoList {
		if strings.ToUpper(strings.TrimSpace(n.FieldType)) != "IMAGE" {
			continue
		}
		path := strings.TrimSpace(fmt.Sprint(n.FieldValue))
		if path == "" || path == "<nil>" {
			continue
		}
		if strings.HasPrefix(path, "http") {
			addURL(path)
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil || len(raw) == 0 {
			continue
		}
		uploads = append(uploads, namedBytes{name: filepath.Base(path), data: raw})
	}
	maxUploads := 1
	if mode == QCFreeVideoModeMultimodal {
		maxUploads = DefaultQCFreeVideoMaxRefs
	}
	if len(uploads) > maxUploads {
		uploads = uploads[:maxUploads]
	}
	for _, u := range uploads {
		cdn, err := client.UploadQCImage(u.data, u.name)
		if err != nil {
			return nil, err
		}
		addURL(cdn)
	}
	if mode == QCFreeVideoModeMultimodal && len(imageURLs) == 0 {
		return nil, fmt.Errorf("多模态参考至少需要一张参考图")
	}
	if len(imageURLs) > maxUploads {
		imageURLs = imageURLs[:maxUploads]
	}

	projectID, err := client.QCProjectCreate(prompt)
	if err != nil {
		return nil, err
	}
	pageID, err := client.QCPageList(projectID)
	if err != nil {
		return nil, err
	}
	clientReqID := randomUUIDLike()
	createReq := buildQCFreeCreateRequest(prompt, imageURLs, in.AspectRatio, in.Duration, in.Resolution, mode, projectID, pageID, clientReqID)
	skuID := asString(createReq["skuId"])

	feeBody := map[string]any{
		"bindingId":     createReq["bindingId"],
		"skuId":         createReq["skuId"],
		"categoryId":    "VIDEO",
		"generateCount": 1,
		"params":        createReq["params"],
	}
	fee, _, err := client.QCFeePreview(feeBody)
	if err != nil {
		return nil, err
	}
	if !fee.IsFree() {
		return nil, fmt.Errorf("qc not free: freeQuotaRemaining=%d requiredPointUnit=%s insufficient=%s",
			fee.FreeQuotaRemaining, fee.RequiredPointUnit, fee.InsufficientType)
	}

	prepared, err := client.QCPrepare(createReq)
	if err != nil {
		return nil, err
	}
	prepareToken := asString(nested(prepared, "data", "prepareToken"))
	quoteID := asString(nested(prepared, "data", "quoteId"))
	if prepareToken == "" {
		return nil, fmt.Errorf("prepare missing prepareToken")
	}
	feeData, _ := nested(prepared, "data", "feePreview").(map[string]any)
	prepFee := parseQCFeePreview(feeData)
	if !prepFee.IsFree() {
		return nil, fmt.Errorf("qc prepare not free: freeQuotaRemaining=%d", prepFee.FreeQuotaRemaining)
	}

	committed, err := client.QCCommit(prepareToken, quoteID, createReq)
	if err != nil {
		return nil, err
	}
	taskID := asString(nested(committed, "data", "taskId"))
	if taskID == "" {
		return nil, fmt.Errorf("commit missing taskId")
	}
	label := "QC 免费自由创作"
	if mode == QCFreeVideoModeMultimodal {
		label = "QC 免费多模态参考"
	} else if len(imageURLs) > 0 {
		label = "QC 免费图生视频"
	} else {
		label = "QC 免费文生视频"
	}
	emit(Progress{TaskID: taskID, RHStatus: "QUEUED", ProgressText: "已提交" + label + "，等待排队"})
	return &Session{
		Client:   client,
		TaskID:   taskID,
		WebappID: "qc:" + skuID,
		Kind:     SessionKindQC,
	}, nil
}

// SubmitQCFreeImage creates a Runow QC free image task (prepare→commit). Refuses non-free quotes.
// With reference images → i2i sku; without → free t2i sku (HAR).
func SubmitQCFreeImage(in RunInput) (*Session, error) {
	client, err := loginClient(in)
	if err != nil {
		return nil, err
	}
	emit := func(p Progress) {
		p.AccessToken = client.AccessToken
		if in.OnProgress != nil {
			in.OnProgress(p)
		}
	}

	prompt := strings.TrimSpace(strings.TrimPrefix(in.Prompt, "\ufeff"))
	if prompt == "" {
		prompt = firstPromptText(in)
	}
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	imageURLs := make([]string, 0, 4)
	seen := map[string]struct{}{}
	addURL := func(u string) {
		u = strings.TrimSpace(u)
		if u == "" {
			return
		}
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		imageURLs = append(imageURLs, u)
	}
	if u := strings.TrimSpace(in.FirstFrameURL); u != "" {
		addURL(u)
	}
	type namedBytes struct {
		name string
		data []byte
	}
	uploads := make([]namedBytes, 0, 4)
	if img, filename, err := loadDefaultImage(in); err == nil && len(img) > 0 {
		uploads = append(uploads, namedBytes{name: filename, data: img})
	}
	for key, raw := range in.ExtraImages {
		if len(raw) == 0 {
			continue
		}
		name := strings.TrimSpace(key) + ".png"
		if name == ".png" {
			name = "ref.png"
		}
		uploads = append(uploads, namedBytes{name: name, data: raw})
	}
	for _, n := range in.NodeInfoList {
		ft := strings.ToUpper(strings.TrimSpace(n.FieldType))
		if ft != "IMAGE" {
			continue
		}
		path := strings.TrimSpace(fmt.Sprint(n.FieldValue))
		if path == "" || path == "<nil>" || strings.HasPrefix(path, "http") {
			if strings.HasPrefix(path, "http") {
				addURL(path)
			}
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil || len(raw) == 0 {
			continue
		}
		uploads = append(uploads, namedBytes{name: filepath.Base(path), data: raw})
	}
	if len(uploads) > DefaultQCFreeImageMaxRefs {
		uploads = uploads[:DefaultQCFreeImageMaxRefs]
	}
	for _, u := range uploads {
		cdn, err := client.UploadQCImage(u.data, u.name)
		if err != nil {
			return nil, err
		}
		addURL(cdn)
	}
	if len(imageURLs) > DefaultQCFreeImageMaxRefs {
		imageURLs = imageURLs[:DefaultQCFreeImageMaxRefs]
	}

	projectID, err := client.QCProjectCreate(prompt)
	if err != nil {
		return nil, err
	}
	pageID, err := client.QCPageList(projectID)
	if err != nil {
		return nil, err
	}
	clientReqID := randomUUIDLike()
	bindingID := strings.TrimSpace(in.BindingID)
	skuIDIn := strings.TrimSpace(in.SkuID)
	family := strings.TrimSpace(in.CreationMode)
	if family == "" && in.QCParams != nil {
		family = strings.TrimSpace(asString(in.QCParams["free_image_model"]))
	}
	// Queued worker only has webapp_id=qcimg:{sku}; recover family from it.
	if bindingID == "" || skuIDIn == "" || family == "" {
		if bid, sid, fam, _ := qcFreeImageFromWebappID(in.WebappID); bid != "" || fam != "" {
			if bindingID == "" {
				bindingID = bid
			}
			if skuIDIn == "" {
				skuIDIn = sid
			}
			if family == "" {
				family = fam
			}
		}
	}
	// Re-resolve by family + whether refs exist (do not trust stale i2i/t2i sku alone).
	bindingID, skuIDIn, family = ResolveQCFreeImageModel(bindingID, skuIDIn, family, len(imageURLs) > 0)
	createReq := buildQCFreeImageCreateRequest(prompt, imageURLs, in.AspectRatio, in.Resolution, projectID, pageID, clientReqID, bindingID, skuIDIn, family)
	skuID := asString(createReq["skuId"])

	feeBody := map[string]any{
		"bindingId":     createReq["bindingId"],
		"skuId":         createReq["skuId"],
		"categoryId":    "IMAGE",
		"generateCount": 1,
		"params":        createReq["params"],
	}
	fee, _, err := client.QCFeePreview(feeBody)
	if err != nil {
		return nil, err
	}
	if !fee.IsFree() {
		return nil, fmt.Errorf("qc image not free: freeQuotaRemaining=%d requiredPointUnit=%s insufficient=%s",
			fee.FreeQuotaRemaining, fee.RequiredPointUnit, fee.InsufficientType)
	}

	prepared, err := client.QCPrepare(createReq)
	if err != nil {
		return nil, err
	}
	prepareToken := asString(nested(prepared, "data", "prepareToken"))
	quoteID := asString(nested(prepared, "data", "quoteId"))
	if prepareToken == "" {
		return nil, fmt.Errorf("prepare missing prepareToken")
	}
	feeData, _ := nested(prepared, "data", "feePreview").(map[string]any)
	prepFee := parseQCFeePreview(feeData)
	if !prepFee.IsFree() {
		return nil, fmt.Errorf("qc image prepare not free: freeQuotaRemaining=%d", prepFee.FreeQuotaRemaining)
	}

	committed, err := client.QCCommit(prepareToken, quoteID, createReq)
	if err != nil {
		return nil, err
	}
	taskID := asString(nested(committed, "data", "taskId"))
	if taskID == "" {
		return nil, fmt.Errorf("commit missing taskId")
	}
	emit(Progress{TaskID: taskID, RHStatus: "QUEUED", ProgressText: "已提交 QC 免费生图，等待排队"})
	return &Session{
		Client:   client,
		TaskID:   taskID,
		WebappID: "qcimg:" + skuID,
		Kind:     SessionKindQC,
	}, nil
}

// SubmitQCGeneric creates a Runow QC task for an arbitrary binding/sku (free or paid).
// Uploads local images from ImagePath/ExtraImages/NodeInfoList into params when needed.
func SubmitQCGeneric(in RunInput) (*Session, error) {
	client, err := loginClient(in)
	if err != nil {
		return nil, err
	}
	emit := func(p Progress) {
		p.AccessToken = client.AccessToken
		if in.OnProgress != nil {
			in.OnProgress(p)
		}
	}

	bindingID := strings.TrimSpace(in.BindingID)
	skuID := strings.TrimSpace(in.SkuID)
	categoryID := strings.TrimSpace(in.CategoryID)
	if categoryID == "" {
		categoryID = "VIDEO"
	}
	if bindingID == "" || skuID == "" {
		return nil, fmt.Errorf("binding_id and sku_id required for qc submit")
	}

	params := map[string]any{}
	for k, v := range in.QCParams {
		if strings.TrimSpace(k) == "" || v == nil {
			continue
		}
		params[k] = v
	}
	prompt := strings.TrimSpace(strings.TrimPrefix(in.Prompt, "\ufeff"))
	if prompt == "" {
		prompt = firstPromptText(in)
	}
	if prompt != "" {
		if _, ok := params["prompt"]; !ok {
			params["prompt"] = prompt
		}
	}
	if mode := strings.TrimSpace(in.CreationMode); mode != "" {
		if _, ok := params["creationMode"]; !ok {
			params["creationMode"] = CreationModeParamValue(mode)
		}
	}
	if v := strings.TrimSpace(in.Duration); v != "" {
		if _, ok := params["duration"]; !ok {
			if n := asInt(v); n > 0 {
				params["duration"] = n
			} else {
				params["duration"] = v
			}
		}
	}
	if v := strings.TrimSpace(in.AspectRatio); v != "" {
		if _, ok := params["aspectRatio"]; !ok {
			params["aspectRatio"] = normalizeQCImageAspect(v)
		}
		if _, ok := params["ratio"]; !ok && strings.EqualFold(categoryID, "VIDEO") {
			// some models use ratio instead of aspectRatio
			if _, hasAR := params["aspectRatio"]; hasAR {
				// keep both only if caller didn't set ratio
			}
		}
	}
	if v := strings.TrimSpace(in.Resolution); v != "" {
		if _, ok := params["resolution"]; !ok {
			params["resolution"] = v
		}
	}

	// Upload local images and inject into common param keys if empty.
	uploaded := make([]string, 0, 8)
	addUpload := func(data []byte, name string) error {
		if len(data) == 0 {
			return nil
		}
		u, err := client.UploadQCImage(data, name)
		if err != nil {
			return err
		}
		uploaded = append(uploaded, u)
		return nil
	}
	if img, filename, err := loadDefaultImage(in); err == nil && len(img) > 0 {
		if err := addUpload(img, filename); err != nil {
			return nil, err
		}
	}
	for key, raw := range in.ExtraImages {
		if err := addUpload(raw, strings.TrimSpace(key)+".png"); err != nil {
			return nil, err
		}
	}
	for _, n := range in.NodeInfoList {
		if strings.ToUpper(strings.TrimSpace(n.FieldType)) != "IMAGE" {
			continue
		}
		path := strings.TrimSpace(fmt.Sprint(n.FieldValue))
		if path == "" || path == "<nil>" {
			continue
		}
		if strings.HasPrefix(path, "http") {
			uploaded = append(uploaded, path)
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil || len(raw) == 0 {
			continue
		}
		if err := addUpload(raw, filepath.Base(path)); err != nil {
			return nil, err
		}
	}
	if u := strings.TrimSpace(in.FirstFrameURL); u != "" {
		uploaded = append([]string{u}, uploaded...)
	}
	injectQCImageParams(params, uploaded)

	projectID, err := client.QCProjectCreate(fmt.Sprint(params["prompt"]))
	if err != nil {
		return nil, err
	}
	pageID, err := client.QCPageList(projectID)
	if err != nil {
		return nil, err
	}
	clientReqID := randomUUIDLike()
	createReq := map[string]any{
		"categoryId":      categoryID,
		"params":          params,
		"bindingId":       bindingID,
		"skuId":           skuID,
		"generateCount":   1,
		"projectId":       projectID,
		"pageId":          pageID,
		"clientRequestId": clientReqID,
	}

	feeBody := map[string]any{
		"bindingId":     bindingID,
		"skuId":         skuID,
		"categoryId":    categoryID,
		"generateCount": 1,
		"params":        params,
	}
	fee, _, err := client.QCFeePreview(feeBody)
	if err != nil {
		return nil, err
	}
	if fee.IsFree() {
		// ok
	} else if !in.AllowPaid {
		return nil, fmt.Errorf("qc not free (set allow_paid): points=%s insufficient=%s", fee.RequiredPointUnit, fee.InsufficientType)
	} else if !fee.Passed || fee.InsufficientType != "" {
		return nil, fmt.Errorf("qc paid quote rejected: passed=%v points=%s insufficient=%s", fee.Passed, fee.RequiredPointUnit, fee.InsufficientType)
	}

	prepared, err := client.QCPrepare(createReq)
	if err != nil {
		return nil, err
	}
	prepareToken := asString(nested(prepared, "data", "prepareToken"))
	quoteID := asString(nested(prepared, "data", "quoteId"))
	if prepareToken == "" {
		return nil, fmt.Errorf("prepare missing prepareToken")
	}
	feeData, _ := nested(prepared, "data", "feePreview").(map[string]any)
	prepFee := parseQCFeePreview(feeData)
	if prepFee.IsFree() {
		// ok
	} else if !in.AllowPaid {
		return nil, fmt.Errorf("qc prepare not free")
	} else if !prepFee.Passed || prepFee.InsufficientType != "" {
		return nil, fmt.Errorf("qc prepare paid rejected: points=%s insufficient=%s", prepFee.RequiredPointUnit, prepFee.InsufficientType)
	}

	committed, err := client.QCCommit(prepareToken, quoteID, createReq)
	if err != nil {
		return nil, err
	}
	taskID := asString(nested(committed, "data", "taskId"))
	if taskID == "" {
		return nil, fmt.Errorf("commit missing taskId")
	}
	label := "QC 任务"
	if fee.IsFree() {
		label = "QC 免费任务"
	} else {
		label = "QC 积分任务"
	}
	emit(Progress{TaskID: taskID, RHStatus: "QUEUED", ProgressText: "已提交" + label + "，等待排队"})
	prefix := "qc:"
	if strings.EqualFold(categoryID, "IMAGE") {
		prefix = "qcimg:"
	}
	return &Session{
		Client:   client,
		TaskID:   taskID,
		WebappID: prefix + skuID,
		Kind:     SessionKindQC,
	}, nil
}

func injectQCImageParams(params map[string]any, urls []string) {
	if len(urls) == 0 || params == nil {
		return
	}
	// Prefer filling empty common keys used across models.
	setIfEmpty := func(key string, val any) {
		if _, ok := params[key]; ok {
			return
		}
		params[key] = val
	}
	if len(urls) == 1 {
		setIfEmpty("firstFrameUrl", urls[0])
		setIfEmpty("firstImageUrl", urls[0])
		setIfEmpty("imageUrl", urls[0])
		setIfEmpty("imageUrls", urls)
		setIfEmpty("refImage1", urls[0])
		return
	}
	setIfEmpty("imageUrls", urls)
	setIfEmpty("firstFrameUrl", urls[0])
	if len(urls) > 1 {
		setIfEmpty("lastFrameUrl", urls[len(urls)-1])
	}
	for i, u := range urls {
		if i >= 10 {
			break
		}
		setIfEmpty(fmt.Sprintf("refImage%d", i+1), u)
	}
}

// LookupQCSnapshot queries batchStatus once.
func LookupQCSnapshot(client *Client, taskID string) (TaskSnapshot, error) {
	var snap TaskSnapshot
	if client == nil || strings.TrimSpace(taskID) == "" {
		return snap, fmt.Errorf("client and taskId required")
	}
	rows, err := client.QCBatchStatus(taskID)
	if err != nil {
		return snap, err
	}
	for _, item := range rows {
		if asString(item["taskId"]) != taskID {
			continue
		}
		status := strings.ToUpper(asString(item["taskStatus"]))
		snap.RHStatus = status
		switch {
		case status == "QUEUED":
			snap.ProgressText = "排队中"
		case status == "RUNNING":
			snap.ProgressText = "生成中"
		case status == "SUCCESS":
			snap.ProgressText = "完成"
		default:
			snap.ProgressText = status
		}
		if strings.Contains(status, "FAIL") || strings.Contains(status, "ERROR") || strings.Contains(status, "CANCEL") {
			snap.Failed = true
			snap.FailMsg = firstNonEmpty(asString(item["taskResultDesc"]), status)
			return snap, nil
		}
		outputs, _ := item["outputList"].([]any)
		for _, it := range outputs {
			out, _ := it.(map[string]any)
			if out == nil {
				continue
			}
			fileURL := asString(out["fileUrl"])
			if fileURL == "" {
				continue
			}
			if status == "SUCCESS" || status == "" {
				snap.FileURL = fileURL
				snap.PreviewURL = firstNonEmpty(asString(out["filePreviewUrl"]), fileURL)
				snap.Done = true
				snap.RHStatus = "SUCCESS"
				snap.ProgressText = "完成"
				return snap, nil
			}
		}
		return snap, nil
	}
	return snap, fmt.Errorf("task %s not found in batchStatus", taskID)
}

// PollQC waits for QC batchStatus SUCCESS + fileUrl.
func PollQC(client *Client, taskID string, in RunInput) (*RunResult, error) {
	if client == nil || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("client and taskId required")
	}
	pollEvery := in.PollEvery
	if pollEvery <= 0 {
		pollEvery = 5 * time.Second
	}
	timeout := in.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snap, err := LookupQCSnapshot(client, taskID)
		if err != nil {
			return nil, err
		}
		if in.OnProgress != nil {
			in.OnProgress(Progress{
				TaskID:       taskID,
				RHStatus:     snap.RHStatus,
				ProgressText: snap.ProgressText,
				AccessToken:  client.AccessToken,
			})
		}
		if snap.Failed {
			return nil, fmt.Errorf("task failed: %s", firstNonEmpty(snap.FailMsg, snap.RHStatus))
		}
		if snap.Done {
			return &RunResult{
				TaskID:      taskID,
				FileURL:     snap.FileURL,
				PreviewURL:  snap.PreviewURL,
				Status:      "SUCCESS",
				WebappID:    firstNonEmpty(in.WebappID, "qc:"+DefaultQCFreeSkuID),
				UserID:      DecodeJWTSub(client.AccessToken),
				AccessToken: client.AccessToken,
			}, nil
		}
		time.Sleep(pollEvery)
	}
	return nil, fmt.Errorf("timeout waiting QC fileUrl for task %s", taskID)
}

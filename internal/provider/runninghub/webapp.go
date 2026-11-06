package runninghub

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultWebappID = "2099427470494662657"

// NodeInput is one RH webapp input (same shape as RH nodeInfoList / inputs).
type NodeInput struct {
	NodeID      string `json:"nodeId"`
	NodeName    string `json:"nodeName,omitempty"`
	FieldName   string `json:"fieldName"`
	FieldValue  any    `json:"fieldValue"`
	Description string `json:"description,omitempty"`
	FieldType   string `json:"fieldType,omitempty"`
}

type RunInput struct {
	Email           string
	Password        string
	AccessToken     string
	AnonymousID     string
	FingerprintJSON string
	Prompt          string
	ImagePath       string
	ImageBytes      []byte
	Filename        string
	WebappID        string
	Duration        string
	AspectRatio     string
	// NodeInfoList when set, bypasses the hardcoded default-app mapping.
	NodeInfoList []NodeInput
	// ExtraImages maps nodeId (or "default") -> local image bytes for IMAGE fields.
	ExtraImages map[string][]byte
	// AccessPassword for RH apps with encrypted access (accessEncrypted).
	AccessPassword string
	PollEvery      time.Duration
	Timeout        time.Duration
	BaseURL        string
	// HTTP when set, used as the RH client's transport (sticky egress proxy).
	HTTP       *http.Client
	OnProgress func(Progress)
}

type Progress struct {
	TaskID       string
	RHStatus     string
	QueueIndex   int
	QueueCount   int
	ProgressText string
	AccessToken  string
}

type RunResult struct {
	TaskID      string `json:"task_id"`
	FileURL     string `json:"file_url"`
	PreviewURL  string `json:"preview_url"`
	OutputName  string `json:"output_name,omitempty"`
	Status      string `json:"status"`
	WebappID    string `json:"webapp_id"`
	UserID      string `json:"user_id,omitempty"`
	AccessToken string `json:"-"`
}

// Session holds auth + RH task id after SubmitWebappVideo succeeds.
type Session struct {
	Client   *Client
	TaskID   string
	WebappID string
}

func RunWebappVideo(in RunInput) (*RunResult, error) {
	sess, err := SubmitWebappVideo(in)
	if err != nil {
		return nil, err
	}
	return sess.Poll(in)
}

func (in RunInput) resolveWebappID() string {
	if strings.TrimSpace(in.WebappID) != "" {
		return strings.TrimSpace(in.WebappID)
	}
	return DefaultWebappID
}

func loginClient(in RunInput) (*Client, error) {
	base := in.BaseURL
	if base == "" {
		base = "https://www.runninghub.ai"
	}
	client := NewClient(base)
	if in.HTTP != nil {
		client.HTTP = in.HTTP
	}
	client.ApplyIdentity(in.AnonymousID, in.FingerprintJSON)

	if strings.TrimSpace(in.Email) != "" && strings.TrimSpace(in.Password) != "" {
		login, err := client.PwdLogin(in.Email, in.Password)
		if err != nil {
			return nil, err
		}
		token := asString(nested(login, "data", "access_token"))
		if token == "" {
			token = asString(nested(login, "data", "accessToken"))
		}
		if token == "" {
			return nil, fmt.Errorf("login missing access_token")
		}
		client.SetAccessToken(token)
		return client, nil
	}
	if strings.TrimSpace(in.AccessToken) != "" {
		client.SetAccessToken(in.AccessToken)
		return client, nil
	}
	return nil, fmt.Errorf("email+password or access_token required")
}

func loadDefaultImage(in RunInput) ([]byte, string, error) {
	img := in.ImageBytes
	filename := in.Filename
	if len(img) == 0 && in.ImagePath != "" {
		b, err := os.ReadFile(in.ImagePath)
		if err != nil {
			return nil, "", err
		}
		img = b
		if filename == "" {
			filename = filepath.Base(in.ImagePath)
		}
	}
	if filename == "" {
		filename = "image.png"
	}
	return img, filename, nil
}

func isImageNode(n NodeInput) bool {
	ft := strings.ToUpper(strings.TrimSpace(n.FieldType))
	if ft == "IMAGE" {
		return true
	}
	fn := strings.ToLower(strings.TrimSpace(n.FieldName))
	if fn == "image" || strings.Contains(fn, "image") {
		return true
	}
	nn := strings.ToLower(strings.TrimSpace(n.NodeName))
	return strings.Contains(nn, "loadimage") || strings.Contains(nn, "load_image")
}

func needsUpload(v any) bool {
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return true
	}
	low := strings.ToLower(s)
	return low == "__upload__" || low == "__image__" || low == "@image" || strings.HasPrefix(low, "upload:")
}

// SubmitWebappVideo logs in, uploads, creates the RH webapp task and returns as soon as taskId is known.
func SubmitWebappVideo(in RunInput) (*Session, error) {
	webappID := in.resolveWebappID()
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

	var inputs []map[string]any
	if len(in.NodeInfoList) > 0 {
		inputs, err = buildDynamicInputs(client, in, webappID)
		if err != nil {
			return nil, err
		}
	} else {
		inputs, err = buildLegacyInputs(client, in, webappID)
		if err != nil {
			return nil, err
		}
	}

	if _, err := client.WorkValid(webappID); err != nil {
		return nil, err
	}
	if text := firstPromptText(in); text != "" {
		if _, err := client.SensitiveCheck(text); err != nil {
			return nil, err
		}
	}

	createBody := map[string]any{
		"webappId":         webappID,
		"inputs":           inputs,
		"clientId":         makeClientID(),
		"usePersonalQueue": false,
	}
	if pwd := strings.TrimSpace(in.AccessPassword); pwd != "" {
		// RH OpenAPI 字段名为 accessPassword；网页 create 同步带上
		createBody["accessPassword"] = pwd
		createBody["password"] = pwd
	}
	created, err := client.WebappCreate(createBody)
	if err != nil {
		return nil, err
	}
	taskID := asString(nested(created, "data", "taskId"))
	if taskID == "" {
		return nil, fmt.Errorf("create missing taskId")
	}
	emit(Progress{TaskID: taskID, RHStatus: "QUEUED", ProgressText: "已提交 RH，等待排队"})
	return &Session{Client: client, TaskID: taskID, WebappID: webappID}, nil
}

func buildLegacyInputs(client *Client, in RunInput, webappID string) ([]map[string]any, error) {
	prompt := strings.TrimSpace(strings.TrimPrefix(in.Prompt, "\ufeff"))
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}
	img, filename, err := loadDefaultImage(in)
	if err != nil {
		return nil, err
	}
	if len(img) == 0 {
		return nil, fmt.Errorf("image required")
	}
	uploaded, err := client.UploadImage(img, filename, webappID)
	if err != nil {
		return nil, err
	}
	imageName := asString(uploaded["name"])
	if imageName == "" {
		return nil, fmt.Errorf("upload missing name: %v", uploaded)
	}
	duration := in.Duration
	if duration == "" {
		duration = "8"
	}
	aspect := in.AspectRatio
	if aspect == "" {
		aspect = "3:4 (Portrait Standard)"
	}
	return []map[string]any{
		{"nodeId": "266", "nodeName": "LoadImage", "fieldName": "image", "fieldValue": imageName, "description": "image"},
		{"nodeId": "285", "nodeName": "PrimitiveFloat", "fieldName": "value", "fieldValue": duration, "description": "value"},
		{"nodeId": "286", "nodeName": "ResolutionSelector", "fieldName": "aspect_ratio", "fieldValue": aspect, "description": "aspect_ratio"},
		{"nodeId": "249", "nodeName": "PrimitiveStringMultiline", "fieldName": "value", "fieldValue": prompt, "description": "value"},
	}, nil
}

func buildDynamicInputs(client *Client, in RunInput, webappID string) ([]map[string]any, error) {
	defaultImg, defaultName, err := loadDefaultImage(in)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(in.NodeInfoList))
	uploadedOnce := ""
	for _, n := range in.NodeInfoList {
		nodeID := strings.TrimSpace(n.NodeID)
		fieldName := strings.TrimSpace(n.FieldName)
		if nodeID == "" || fieldName == "" {
			return nil, fmt.Errorf("nodeId and fieldName required in node_info_list")
		}
		val := n.FieldValue
		if isImageNode(n) && needsUpload(val) {
			img := defaultImg
			filename := defaultName
			if in.ExtraImages != nil {
				if b := in.ExtraImages[nodeID]; len(b) > 0 {
					img = b
					filename = nodeID + ".png"
				} else if b := in.ExtraImages["default"]; len(b) > 0 {
					img = b
				}
			}
			if len(img) == 0 {
				return nil, fmt.Errorf("image required for node %s field %s", nodeID, fieldName)
			}
			if uploadedOnce != "" && len(in.ExtraImages) == 0 {
				val = uploadedOnce
			} else {
				uploaded, err := client.UploadImage(img, filename, webappID)
				if err != nil {
					return nil, err
				}
				name := asString(uploaded["name"])
				if name == "" {
					return nil, fmt.Errorf("upload missing name: %v", uploaded)
				}
				val = name
				if uploadedOnce == "" {
					uploadedOnce = name
				}
			}
		}
		item := map[string]any{
			"nodeId":     nodeID,
			"fieldName":  fieldName,
			"fieldValue": val,
		}
		if strings.TrimSpace(n.NodeName) != "" {
			item["nodeName"] = n.NodeName
		}
		if strings.TrimSpace(n.Description) != "" {
			item["description"] = n.Description
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("node_info_list required")
	}
	return out, nil
}

func firstPromptText(in RunInput) string {
	if p := strings.TrimSpace(strings.TrimPrefix(in.Prompt, "\ufeff")); p != "" {
		return p
	}
	for _, n := range in.NodeInfoList {
		if isImageNode(n) {
			continue
		}
		ft := strings.ToUpper(strings.TrimSpace(n.FieldType))
		if ft != "" && ft != "STRING" && ft != "TEXT" {
			continue
		}
		s := strings.TrimSpace(fmt.Sprint(n.FieldValue))
		if s == "" || s == "<nil>" {
			continue
		}
		if len(s) >= 4 {
			return s
		}
	}
	return ""
}

// FetchWebappDemo returns normalized demo payload for downstream.
// When rhAPIKey is set, prefer official GET /api/webapp/apiCallDemo; otherwise public /api/webapp/detail.
func FetchWebappDemo(baseURL, webappID, rhAPIKey string) (map[string]any, error) {
	webappID = strings.TrimSpace(webappID)
	if webappID == "" {
		return nil, fmt.Errorf("webapp_id required")
	}
	if baseURL == "" {
		baseURL = "https://www.runninghub.ai"
	}
	client := NewClient(baseURL)

	if key := strings.TrimSpace(rhAPIKey); key != "" {
		raw, err := client.ApiCallDemo(key, webappID)
		if err == nil {
			if out, ok := normalizeDemoFromApiCall(webappID, raw); ok {
				return out, nil
			}
		}
		// fall through to public detail
	}

	raw, err := client.WebappDetail(webappID)
	if err != nil {
		return nil, err
	}
	data, _ := nested(raw, "data").(map[string]any)
	if data == nil {
		return nil, fmt.Errorf("webapp detail empty")
	}
	nodesRaw, _ := data["inputNodes"].([]any)
	return buildDemoPayload(webappID, asString(data["name"]), asString(data["description"]), nodesRaw, "webapp/detail", ""), nil
}

func normalizeDemoFromApiCall(webappID string, raw map[string]any) (map[string]any, bool) {
	data, _ := nested(raw, "data").(map[string]any)
	if data == nil {
		return nil, false
	}
	nodesRaw, _ := data["nodeInfoList"].([]any)
	if len(nodesRaw) == 0 {
		return nil, false
	}
	name := firstNonEmpty(asString(data["webappName"]), asString(data["name"]))
	out := buildDemoPayload(webappID, name, asString(data["description"]), nodesRaw, "apiCallDemo", asString(data["curl"]))
	if enc, ok := data["accessEncrypted"].(bool); ok && enc {
		out["access_encrypted"] = true
	} else if s := strings.ToLower(asString(data["accessEncrypted"])); s == "true" || s == "1" {
		out["access_encrypted"] = true
	}
	return out, true
}

func buildDemoPayload(webappID, name, description string, nodesRaw []any, source, rhCurl string) map[string]any {
	nodes := make([]map[string]any, 0, len(nodesRaw))
	exampleNodes := make([]map[string]any, 0, len(nodesRaw))
	for _, it := range nodesRaw {
		n, _ := it.(map[string]any)
		if n == nil {
			continue
		}
		node := map[string]any{
			"nodeId":      asString(n["nodeId"]),
			"nodeName":    asString(n["nodeName"]),
			"fieldName":   asString(n["fieldName"]),
			"fieldType":   asString(n["fieldType"]),
			"fieldValue":  n["fieldValue"],
			"description": firstNonEmpty(asString(n["descriptionCn"]), asString(n["description"]), asString(n["descriptionEn"])),
		}
		if fd := asString(n["fieldData"]); fd != "" {
			node["fieldData"] = fd
		}
		nodes = append(nodes, node)

		exVal := n["fieldValue"]
		ft := strings.ToUpper(asString(n["fieldType"]))
		if ft == "IMAGE" {
			exVal = "__upload__"
		}
		exampleNodes = append(exampleNodes, map[string]any{
			"nodeId":      node["nodeId"],
			"nodeName":    node["nodeName"],
			"fieldName":   node["fieldName"],
			"fieldType":   node["fieldType"],
			"fieldValue":  exVal,
			"description": node["description"],
		})
	}
	out := map[string]any{
		"webapp_id":      webappID,
		"name":           name,
		"description":    description,
		"source":         source,
		"node_info_list": nodes,
		"example": map[string]any{
			"method": "POST",
			"path":   "/v1/videos",
			"headers": map[string]string{
				"Authorization": "Bearer <RH2API_API_KEY>",
				"Content-Type":  "application/json",
			},
			"body": map[string]any{
				"webapp_id":      webappID,
				"node_info_list": exampleNodes,
				"image_base64":   "<可选：当 IMAGE 节点 fieldValue=__upload__ 时提供>",
			},
			"multipart_hint": "也可用 multipart：字段 node_info_list(JSON字符串) + image(文件)；IMAGE 节点 fieldValue 填 __upload__",
		},
	}
	if rhCurl != "" {
		out["rh_official_curl"] = rhCurl
	}
	return out
}

// Poll waits for SUCCESS fileUrl (or failure/timeout) for an already-submitted RH task.
func (s *Session) Poll(in RunInput) (*RunResult, error) {
	if s == nil || s.Client == nil || s.TaskID == "" {
		return nil, fmt.Errorf("invalid session")
	}
	webappID := s.WebappID
	if webappID == "" {
		webappID = DefaultWebappID
	}
	taskID := s.TaskID
	client := s.Client

	emit := func(p Progress) {
		p.AccessToken = client.AccessToken
		if in.OnProgress != nil {
			in.OnProgress(p)
		}
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
		if st, qi, qc, ok := lookupTaskList(client, taskID); ok {
			text := st
			if strings.EqualFold(st, "QUEUED") && qc > 0 {
				text = fmt.Sprintf("排队中 %d/%d", qi, qc)
			} else if strings.EqualFold(st, "RUNNING") {
				text = "生成中"
			}
			emit(Progress{TaskID: taskID, RHStatus: st, QueueIndex: qi, QueueCount: qc, ProgressText: text})
			upper := strings.ToUpper(st)
			if strings.Contains(upper, "FAIL") || strings.Contains(upper, "ERROR") || strings.Contains(upper, "CANCEL") {
				return nil, fmt.Errorf("task failed: %s", st)
			}
		}

		hist, err := client.OutputHistoryV2(map[string]any{
			"size": 20, "current": 1,
			"taskType": []string{"WORKFLOW", "WEBAPP"},
			"webappId": webappID,
			"fromId":   "",
		})
		if err != nil {
			return nil, err
		}
		items, _ := nested(hist, "data").([]any)
		for _, it := range items {
			item, _ := it.(map[string]any)
			if item == nil {
				continue
			}
			if asString(item["taskId"]) != taskID && promptID(item) != taskID {
				continue
			}
			status := strings.ToUpper(asString(item["taskStatus"]))
			fileURL := asString(item["fileUrl"])
			if fileURL != "" && (status == "SUCCESS" || status == "") {
				emit(Progress{TaskID: taskID, RHStatus: "SUCCESS", ProgressText: "完成"})
				return &RunResult{
					TaskID:      taskID,
					FileURL:     fileURL,
					PreviewURL:  firstNonEmpty(asString(item["filePreviewUrl"]), fileURL),
					OutputName:  asString(item["outputName"]),
					Status:      firstNonEmpty(status, "SUCCESS"),
					WebappID:    webappID,
					UserID:      DecodeJWTSub(client.AccessToken),
					AccessToken: client.AccessToken,
				}, nil
			}
			if strings.Contains(status, "FAIL") || strings.Contains(status, "ERROR") || strings.Contains(status, "CANCEL") {
				return nil, fmt.Errorf("task failed: %s %s", status, asString(item["taskResultDesc"]))
			}
		}
		time.Sleep(pollEvery)
	}
	return nil, fmt.Errorf("timeout waiting fileUrl for task %s", taskID)
}

// TaskSnapshot is a one-shot RH status lookup (for resume / manual sync after restart).
type TaskSnapshot struct {
	RHStatus     string
	ProgressText string
	QueueIndex   int
	QueueCount   int
	FileURL      string
	PreviewURL   string
	Done         bool // SUCCESS with file
	Failed       bool
	FailMsg      string
}

// LookupTaskSnapshot checks task list + output history once.
func LookupTaskSnapshot(client *Client, webappID, taskID string) (TaskSnapshot, error) {
	var snap TaskSnapshot
	if client == nil || strings.TrimSpace(taskID) == "" {
		return snap, fmt.Errorf("client and taskId required")
	}
	if webappID == "" {
		webappID = DefaultWebappID
	}
	if st, qi, qc, ok := lookupTaskList(client, taskID); ok {
		snap.RHStatus = st
		snap.QueueIndex = qi
		snap.QueueCount = qc
		if strings.EqualFold(st, "QUEUED") && qc > 0 {
			snap.ProgressText = fmt.Sprintf("排队中 %d/%d", qi, qc)
		} else if strings.EqualFold(st, "RUNNING") {
			snap.ProgressText = "生成中"
		} else {
			snap.ProgressText = st
		}
		upper := strings.ToUpper(st)
		if strings.Contains(upper, "FAIL") || strings.Contains(upper, "ERROR") || strings.Contains(upper, "CANCEL") {
			snap.Failed = true
			snap.FailMsg = st
			return snap, nil
		}
	}

	hist, err := client.OutputHistoryV2(map[string]any{
		"size": 20, "current": 1,
		"taskType": []string{"WORKFLOW", "WEBAPP"},
		"webappId": webappID,
		"fromId":   "",
	})
	if err != nil {
		return snap, err
	}
	items, _ := nested(hist, "data").([]any)
	for _, it := range items {
		item, _ := it.(map[string]any)
		if item == nil {
			continue
		}
		if asString(item["taskId"]) != taskID && promptID(item) != taskID {
			continue
		}
		status := strings.ToUpper(asString(item["taskStatus"]))
		fileURL := asString(item["fileUrl"])
		if fileURL != "" && (status == "SUCCESS" || status == "") {
			snap.RHStatus = "SUCCESS"
			snap.ProgressText = "完成"
			snap.FileURL = fileURL
			snap.PreviewURL = firstNonEmpty(asString(item["filePreviewUrl"]), fileURL)
			snap.Done = true
			return snap, nil
		}
		if strings.Contains(status, "FAIL") || strings.Contains(status, "ERROR") || strings.Contains(status, "CANCEL") {
			snap.RHStatus = status
			snap.Failed = true
			snap.FailMsg = firstNonEmpty(asString(item["taskResultDesc"]), status)
			snap.ProgressText = snap.FailMsg
			return snap, nil
		}
		if status != "" && snap.RHStatus == "" {
			snap.RHStatus = status
			if strings.EqualFold(status, "RUNNING") {
				snap.ProgressText = "生成中"
			} else if snap.ProgressText == "" {
				snap.ProgressText = status
			}
		}
	}
	return snap, nil
}

func lookupTaskList(client *Client, taskID string) (status string, queueIndex, queueCount int, ok bool) {
	for _, body := range []map[string]any{
		{"size": 50, "current": 1, "taskStatus": []string{"RUNNING", "QUEUED"}, "taskType": []string{"WORKFLOW", "WEBAPP"}},
		{"size": 50, "current": 1, "taskType": []string{"WORKFLOW", "WEBAPP"}},
	} {
		lst, err := client.TaskList(body)
		if err != nil {
			continue
		}
		data, _ := nested(lst, "data").(map[string]any)
		records, _ := data["records"].([]any)
		for _, raw := range records {
			item, _ := raw.(map[string]any)
			if item == nil || asString(item["taskId"]) != taskID {
				continue
			}
			status = asString(item["taskStatus"])
			if share, _ := item["shareQueueInfo"].(map[string]any); share != nil {
				queueIndex = asInt(share["shareQueueIndex"])
				queueCount = asInt(share["shareQueueCount"])
			}
			return status, queueIndex, queueCount, true
		}
	}
	return "", 0, 0, false
}

func promptID(item map[string]any) string {
	raw := asString(item["promptResponse"])
	if raw == "" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) == nil {
		return asString(m["prompt_id"])
	}
	return ""
}

func makeClientID() string {
	sum := md5.Sum([]byte(fmt.Sprintf("%d-client", time.Now().UnixNano())))
	return hex.EncodeToString(sum[:])
}

package tasks

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/trae/runninghub2api/internal/accounts"
	"github.com/trae/runninghub2api/internal/config"
	"github.com/trae/runninghub2api/internal/egress"
	"github.com/trae/runninghub2api/internal/provider/runninghub"
	"github.com/trae/runninghub2api/internal/store"
)

type Worker struct {
	store  *store.Store
	pool   *accounts.Pool
	cfg    config.Config
	egress *egress.Manager
	stopCh chan struct{}
}

func NewWorker(s *store.Store, pool *accounts.Pool, cfg config.Config) *Worker {
	return &Worker{store: s, pool: pool, cfg: cfg, egress: egress.NewManager(s), stopCh: make(chan struct{})}
}

func (w *Worker) Start(n int) {
	for i := 0; i < n; i++ {
		go w.loop(i)
	}
	go w.resumeOrphanRunning()
}

// resumeOrphanRunning re-attaches pollers for running tasks that already have rh_task_id
// (e.g. process restarted mid-poll).
func (w *Worker) resumeOrphanRunning() {
	list, err := w.store.ListRunningWithRHTask()
	if err != nil {
		log.Printf("[worker] list orphan running: %v", err)
		return
	}
	for _, task := range list {
		t := task
		log.Printf("[task %s] resume poll rh=%s after restart", t.ID, t.RHTaskID)
		go func() {
			if err := w.SyncAndMaybePoll(t.ID, true); err != nil {
				log.Printf("[task %s] resume poll: %v", t.ID, err)
			}
		}()
		time.Sleep(200 * time.Millisecond)
	}
}

// ResolvePollAuth returns runtime setting, falling back to env default.
func (w *Worker) ResolvePollAuth() string {
	if v, err := w.store.GetSetting(store.SettingPollAuth); err == nil && strings.TrimSpace(v) != "" {
		return config.NormalizePollAuth(v)
	}
	return config.NormalizePollAuth(w.cfg.PollAuth)
}

// SyncAndMaybePoll queries RH once; if still in progress and continuePoll, keeps polling until done.
func (w *Worker) SyncAndMaybePoll(taskID string, continuePoll bool) error {
	task, err := w.store.GetTask(taskID)
	if err != nil {
		return err
	}
	if task.Status != "running" && task.Status != "queued" {
		return fmt.Errorf("task status is %s, nothing to sync", task.Status)
	}
	if strings.TrimSpace(task.RHTaskID) == "" {
		return fmt.Errorf("no rh_task_id yet; wait for submit or requeue")
	}
	acc, err := w.store.GetAccount(task.AccountID)
	if err != nil {
		return fmt.Errorf("account missing for task (id=%d)", task.AccountID)
	}
	hasATCreds := strings.TrimSpace(acc.Password) != "" || strings.TrimSpace(acc.AccessToken) != ""
	hasAPIKey := strings.TrimSpace(acc.APIKey) != ""
	if !hasATCreds && !hasAPIKey {
		return fmt.Errorf("account missing credentials for task (id=%d)", task.AccountID)
	}

	// QC free tasks only have batchStatus under AT; skip openapi apikey path.
	if isQCTask(task) {
		return w.syncAndMaybePollQC(task, acc, continuePoll)
	}
	mode := w.ResolvePollAuth()
	if mode == "apikey" {
		if err := w.syncAndMaybePollAPIKey(task, acc, continuePoll); err != nil {
			if hasATCreds {
				log.Printf("[task %s] apiKey poll failed, fallback to AT: %v", task.ID, err)
				return w.syncAndMaybePollAT(task, acc, continuePoll)
			}
			return err
		}
		return nil
	}
	return w.syncAndMaybePollAT(task, acc, continuePoll)
}

func isQCTask(task store.VideoTask) bool {
	return runninghub.IsQCWebappID(task.WebappID)
}

func (w *Worker) syncAndMaybePollQC(task store.VideoTask, acc store.Account, continuePoll bool) error {
	client, err := w.loginForTask(acc)
	if err != nil {
		return err
	}
	if client.AccessToken != "" && client.AccessToken != acc.AccessToken {
		_ = w.store.UpdateAccountToken(acc.ID, client.AccessToken)
	}
	snap, err := runninghub.LookupQCSnapshot(client, task.RHTaskID)
	if err != nil && egress.IsConnectivityError(err) && acc.EgressID > 0 {
		if n, e := w.store.GetEgress(acc.EgressID); e == nil {
			_, _ = w.egress.Rebind(acc, w.cfg.BaseURL, &n, err.Error())
			acc.EgressID = 0
		}
		client, err = w.loginForTask(acc)
		if err != nil {
			return err
		}
		snap, err = runninghub.LookupQCSnapshot(client, task.RHTaskID)
	}
	if err != nil {
		return err
	}
	if snap.RHStatus != "" || snap.ProgressText != "" {
		_ = w.store.UpdateTaskProgress(task.ID, snap.RHStatus, snap.ProgressText, snap.QueueIndex, snap.QueueCount)
	}
	if snap.Done {
		_ = w.pool.ReleaseOK(acc.ID)
		w.refreshAccountBalance(acc.ID)
		return w.store.FinishTaskOK(task.ID, snap.FileURL, snap.PreviewURL)
	}
	if snap.Failed {
		_ = w.pool.ReleaseFail(acc.ID, "RH", snap.FailMsg)
		w.refreshAccountBalance(acc.ID)
		return w.store.FinishTaskFail(task.ID, "RH", snap.FailMsg)
	}
	if !continuePoll {
		return nil
	}
	in := runninghub.RunInput{
		AccessToken:     client.AccessToken,
		AnonymousID:     acc.AnonymousID,
		FingerprintJSON: acc.FingerprintJSON,
		WebappID:        first(task.WebappID, "qc:"+runninghub.DefaultQCFreeSkuID),
		BaseURL:         w.cfg.BaseURL,
		OnProgress: func(p runninghub.Progress) {
			_ = w.store.UpdateTaskProgress(task.ID, p.RHStatus, p.ProgressText, p.QueueIndex, p.QueueCount)
			if p.AccessToken != "" || p.RefreshToken != "" {
				_ = w.store.UpdateAccountAuthTokens(acc.ID, p.AccessToken, p.RefreshToken)
			}
		},
	}
	result, runErr := runninghub.PollQC(client, task.RHTaskID, in)
	if runErr != nil {
		low := strings.ToLower(runErr.Error())
		if strings.Contains(low, "timeout") {
			_ = w.store.UpdateTaskProgress(task.ID, first(snap.RHStatus, "RUNNING"), "轮询超时，可点「查询状态」", snap.QueueIndex, snap.QueueCount)
			return runErr
		}
		if strings.Contains(low, "fail") || strings.Contains(low, "error") || strings.Contains(low, "cancel") {
			_ = w.pool.ReleaseFail(acc.ID, "RH", runErr.Error())
			w.refreshAccountBalance(acc.ID)
			return w.store.FinishTaskFail(task.ID, "RH", runErr.Error())
		}
		return runErr
	}
	if result.AccessToken != "" {
		_ = w.store.UpdateAccountToken(acc.ID, result.AccessToken)
	}
	_ = w.pool.ReleaseOK(acc.ID)
	w.refreshAccountBalance(acc.ID)
	return w.store.FinishTaskOK(task.ID, result.FileURL, result.PreviewURL)
}

func (w *Worker) ensureAccountAPIKey(acc store.Account) (string, store.Account, error) {
	if key := strings.TrimSpace(acc.APIKey); key != "" {
		return key, acc, nil
	}
	baseURL := w.cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://www.runninghub.ai"
	}
	var gotKey string
	err := w.egress.Do(acc, baseURL, func(client *runninghub.Client) error {
		got, e := client.EnsureConsumerApiKey(acc.Email, acc.Password, acc.AccessToken)
		if e != nil {
			return e
		}
		gotKey = got.APIKey
		_ = w.store.UpdateAccountAPIKey(acc.ID, got.APIKey)
		_ = w.store.UpdateAccountAuthTokens(acc.ID, got.AccessToken, got.RefreshToken)
		return nil
	})
	if err != nil {
		return "", acc, err
	}
	acc.APIKey = gotKey
	return gotKey, acc, nil
}

func (w *Worker) syncAndMaybePollAPIKey(task store.VideoTask, acc store.Account, continuePoll bool) error {
	apiKey, acc, err := w.ensureAccountAPIKey(acc)
	if err != nil {
		return err
	}
	baseURL := w.cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://www.runninghub.ai"
	}
	var snap runninghub.TaskSnapshot
	err = w.egress.Do(acc, baseURL, func(client *runninghub.Client) error {
		var e error
		snap, e = runninghub.LookupTaskSnapshotByAPIKey(client, apiKey, task.RHTaskID)
		return e
	})
	if err != nil && egress.IsConnectivityError(err) && acc.EgressID > 0 {
		if n, e := w.store.GetEgress(acc.EgressID); e == nil {
			_, _ = w.egress.Rebind(acc, w.cfg.BaseURL, &n, err.Error())
			acc.EgressID = 0
		}
		err = w.egress.Do(acc, baseURL, func(client *runninghub.Client) error {
			var e error
			snap, e = runninghub.LookupTaskSnapshotByAPIKey(client, apiKey, task.RHTaskID)
			return e
		})
	}
	if err != nil {
		return err
	}
	if snap.RHStatus != "" || snap.ProgressText != "" {
		_ = w.store.UpdateTaskProgress(task.ID, snap.RHStatus, snap.ProgressText, snap.QueueIndex, snap.QueueCount)
	}
	if snap.Done {
		_ = w.pool.ReleaseOK(acc.ID)
		w.refreshAccountBalance(acc.ID)
		return w.store.FinishTaskOK(task.ID, snap.FileURL, snap.PreviewURL)
	}
	if snap.Failed {
		_ = w.pool.ReleaseFail(acc.ID, "RH", snap.FailMsg)
		w.refreshAccountBalance(acc.ID)
		return w.store.FinishTaskFail(task.ID, "RH", snap.FailMsg)
	}
	if !continuePoll {
		return nil
	}
	webappID := first(task.WebappID, w.cfg.WebappID)
	in := runninghub.RunInput{
		WebappID: webappID,
		BaseURL:  w.cfg.BaseURL,
		OnProgress: func(p runninghub.Progress) {
			_ = w.store.UpdateTaskProgress(task.ID, p.RHStatus, p.ProgressText, p.QueueIndex, p.QueueCount)
		},
	}
	var result *runninghub.RunResult
	var runErr error
	err = w.egress.Do(acc, baseURL, func(client *runninghub.Client) error {
		result, runErr = runninghub.PollByAPIKey(client, apiKey, task.RHTaskID, in)
		return runErr
	})
	if err != nil {
		low := strings.ToLower(err.Error())
		if strings.Contains(low, "timeout") {
			_ = w.store.UpdateTaskProgress(task.ID, first(snap.RHStatus, "RUNNING"), "轮询超时，可点「查询状态」", snap.QueueIndex, snap.QueueCount)
			return err
		}
		if strings.Contains(low, "fail") || strings.Contains(low, "error") || strings.Contains(low, "cancel") {
			_ = w.pool.ReleaseFail(acc.ID, "RH", err.Error())
			w.refreshAccountBalance(acc.ID)
			return w.store.FinishTaskFail(task.ID, "RH", err.Error())
		}
		return err
	}
	_ = w.pool.ReleaseOK(acc.ID)
	w.refreshAccountBalance(acc.ID)
	return w.store.FinishTaskOK(task.ID, result.FileURL, result.PreviewURL)
}

func (w *Worker) syncAndMaybePollAT(task store.VideoTask, acc store.Account, continuePoll bool) error {
	if strings.TrimSpace(acc.Password) == "" && strings.TrimSpace(acc.AccessToken) == "" {
		return fmt.Errorf("account missing AT credentials for task (id=%d)", acc.ID)
	}
	client, err := w.loginForTask(acc)
	if err != nil {
		return err
	}
	if client.AccessToken != "" && client.AccessToken != acc.AccessToken {
		_ = w.store.UpdateAccountToken(acc.ID, client.AccessToken)
	}
	webappID := first(task.WebappID, w.cfg.WebappID)
	snap, err := runninghub.LookupTaskSnapshot(client, webappID, task.RHTaskID)
	if err != nil && egress.IsConnectivityError(err) && acc.EgressID > 0 {
		if n, e := w.store.GetEgress(acc.EgressID); e == nil {
			_, _ = w.egress.Rebind(acc, w.cfg.BaseURL, &n, err.Error())
			acc.EgressID = 0
		}
		client, err = w.loginForTask(acc)
		if err != nil {
			return err
		}
		snap, err = runninghub.LookupTaskSnapshot(client, webappID, task.RHTaskID)
	}
	if err != nil {
		return err
	}
	if snap.RHStatus != "" || snap.ProgressText != "" {
		_ = w.store.UpdateTaskProgress(task.ID, snap.RHStatus, snap.ProgressText, snap.QueueIndex, snap.QueueCount)
	}
	if snap.Done {
		_ = w.pool.ReleaseOK(acc.ID)
		w.refreshAccountBalance(acc.ID)
		return w.store.FinishTaskOK(task.ID, snap.FileURL, snap.PreviewURL)
	}
	if snap.Failed {
		_ = w.pool.ReleaseFail(acc.ID, "RH", snap.FailMsg)
		w.refreshAccountBalance(acc.ID)
		return w.store.FinishTaskFail(task.ID, "RH", snap.FailMsg)
	}
	if !continuePoll {
		return nil
	}
	in := runninghub.RunInput{
		AccessToken:     client.AccessToken,
		AnonymousID:     acc.AnonymousID,
		FingerprintJSON: acc.FingerprintJSON,
		WebappID:        webappID,
		BaseURL:         w.cfg.BaseURL,
		OnProgress: func(p runninghub.Progress) {
			_ = w.store.UpdateTaskProgress(task.ID, p.RHStatus, p.ProgressText, p.QueueIndex, p.QueueCount)
			if p.AccessToken != "" || p.RefreshToken != "" {
				_ = w.store.UpdateAccountAuthTokens(acc.ID, p.AccessToken, p.RefreshToken)
			}
		},
	}
	sess := &runninghub.Session{Client: client, TaskID: task.RHTaskID, WebappID: webappID}
	result, runErr := sess.Poll(in)
	if runErr != nil {
		low := strings.ToLower(runErr.Error())
		if strings.Contains(low, "timeout") {
			_ = w.store.UpdateTaskProgress(task.ID, first(snap.RHStatus, "RUNNING"), "轮询超时，可点「查询状态」", snap.QueueIndex, snap.QueueCount)
			return runErr
		}
		if strings.Contains(low, "fail") || strings.Contains(low, "error") || strings.Contains(low, "cancel") {
			_ = w.pool.ReleaseFail(acc.ID, "RH", runErr.Error())
			w.refreshAccountBalance(acc.ID)
			return w.store.FinishTaskFail(task.ID, "RH", runErr.Error())
		}
		return runErr
	}
	if result.AccessToken != "" {
		_ = w.store.UpdateAccountToken(acc.ID, result.AccessToken)
	}
	_ = w.pool.ReleaseOK(acc.ID)
	w.refreshAccountBalance(acc.ID)
	return w.store.FinishTaskOK(task.ID, result.FileURL, result.PreviewURL)
}

func (w *Worker) loginForTask(acc store.Account) (*runninghub.Client, error) {
	baseURL := w.cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://www.runninghub.ai"
	}
	var out *runninghub.Client
	var newAT, newRT string
	err := w.egress.Do(acc, baseURL, func(client *runninghub.Client) error {
		at := strings.TrimSpace(acc.AccessToken)
		if runninghub.AccessTokenFresh(at, 24*time.Hour) {
			client.SetAccessToken(at)
			out = client
			return nil
		}
		if strings.TrimSpace(acc.Email) == "" || strings.TrimSpace(acc.Password) == "" {
			if at != "" {
				client.SetAccessToken(at)
				out = client
				return nil
			}
			return fmt.Errorf("email+password or access_token required")
		}
		login, err := client.PwdLogin(acc.Email, acc.Password)
		if err != nil {
			if at != "" {
				client.SetAccessToken(at)
				out = client
				return nil
			}
			return err
		}
		data, _ := login["data"].(map[string]any)
		token := ""
		rt := ""
		if data != nil {
			if v, ok := data["access_token"].(string); ok {
				token = strings.TrimSpace(v)
			}
			if token == "" {
				if v, ok := data["accessToken"].(string); ok {
					token = strings.TrimSpace(v)
				}
			}
			if v, ok := data["refresh_token"].(string); ok {
				rt = strings.TrimSpace(v)
			}
			if rt == "" {
				if v, ok := data["refreshToken"].(string); ok {
					rt = strings.TrimSpace(v)
				}
			}
		}
		if token == "" {
			if at != "" {
				client.SetAccessToken(at)
				out = client
				return nil
			}
			return fmt.Errorf("login missing access_token")
		}
		client.SetAccessToken(token)
		newAT, newRT = token, rt
		out = client
		return nil
	})
	if err != nil {
		return nil, err
	}
	if newAT != "" {
		_ = w.store.UpdateAccountAuthTokens(acc.ID, newAT, newRT)
	}
	return out, nil
}

func (w *Worker) Stop() { close(w.stopCh) }

func (w *Worker) loop(idx int) {
	log.Printf("[worker-%d] started", idx)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.tick()
		}
	}
}

func (w *Worker) tick() {
	task, err := w.store.ClaimQueuedTask()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return
		}
		log.Printf("claim task: %v", err)
		return
	}
	log.Printf("[task %s] running attempt=%d", task.ID, task.Attempts)
	if err := w.runOne(task); err != nil {
		log.Printf("[task %s] failed: %v", task.ID, err)
	}
}

func (w *Worker) acquireScope(taskUserID int64) int64 {
	if taskUserID <= 0 {
		return 0
	}
	u, err := w.store.GetUser(taskUserID)
	if err == nil && u.Role == store.RoleAdmin {
		return 0 // admin tasks may use the global account pool
	}
	return taskUserID
}

func (w *Worker) runOne(task store.VideoTask) error {
	const maxAttempts = 3
	acc, err := w.pool.Acquire(w.acquireScope(task.UserID), task.ID)
	if err != nil {
		if errors.Is(err, accounts.ErrUnavailable) {
			_ = w.store.RequeueTask(task.ID, "NO_ACCOUNT", "no available accounts")
			time.Sleep(3 * time.Second)
			return err
		}
		_ = w.store.FinishTaskFail(task.ID, "POOL", err.Error())
		return err
	}

	_ = w.store.UpdateTaskRunning(task.ID, acc.ID, acc.Email, "")

	webappID := first(task.WebappID, w.cfg.WebappID)
	qcMeta, hasQCMeta := runninghub.ParseQCTaskMetaJSON(task.NodeInfoJSON)
	nodes := parseNodeInfoList(task.NodeInfoJSON)
	if hasQCMeta {
		// Drop meta-only node; image path carriers (if any) remain for free-image path.
		filtered := make([]runninghub.NodeInput, 0, len(nodes))
		for _, n := range nodes {
			if n.NodeID == "__qc_meta" {
				continue
			}
			filtered = append(filtered, n)
		}
		nodes = filtered
	}
	in := runninghub.RunInput{
		Email:           acc.Email,
		Password:        acc.Password,
		AccessToken:     acc.AccessToken,
		AnonymousID:     acc.AnonymousID,
		FingerprintJSON: acc.FingerprintJSON,
		Prompt:          task.Prompt,
		ImagePath:       task.ImagePath,
		WebappID:        webappID,
		Duration:        task.Duration,
		AspectRatio:     task.AspectRatio,
		Resolution:      task.Resolution,
		AccessPassword:  w.store.GetWebappAccessPassword(webappID),
		NodeInfoList:    nodes,
		BaseURL:         w.cfg.BaseURL,
		OnProgress: func(p runninghub.Progress) {
			if p.TaskID != "" {
				_ = w.store.UpdateTaskRunning(task.ID, acc.ID, acc.Email, p.TaskID)
			}
			_ = w.store.UpdateTaskProgress(task.ID, p.RHStatus, p.ProgressText, p.QueueIndex, p.QueueCount)
			if p.AccessToken != "" || p.RefreshToken != "" {
				_ = w.store.UpdateAccountAuthTokens(acc.ID, p.AccessToken, p.RefreshToken)
			}
		},
	}
	if hasQCMeta {
		in.BindingID = qcMeta.BindingID
		in.SkuID = qcMeta.SkuID
		in.CategoryID = qcMeta.CategoryID
		in.CreationMode = qcMeta.CreationMode
		in.QCParams = qcMeta.Params
		in.AllowPaid = qcMeta.AllowPaid
	}

	egSess, egErr := w.egress.NewClientForAccount(acc, w.cfg.BaseURL)
	if egErr != nil {
		return w.failOrRequeue(task, acc.ID, egErr, maxAttempts)
	}
	egress.AttachHTTP(&in, egSess)

	submit := runninghub.SubmitWebappVideo
	mode := config.NormalizeSubmitMode(w.cfg.SubmitMode)
	switch {
	case hasQCMeta || mode == runninghub.SubmitModeQC:
		submit = runninghub.SubmitQCGeneric
	case runninghub.IsQCImageWebappID(task.WebappID) || mode == runninghub.SubmitModeQCFreeImage:
		submit = runninghub.SubmitQCFreeImage
	case strings.HasPrefix(strings.TrimSpace(task.WebappID), "qc:") || mode == runninghub.SubmitModeQCFree:
		submit = runninghub.SubmitQCFreeVideo
	}
	sess, runErr := submit(in)
	if runErr != nil && egress.IsConnectivityError(runErr) && egSess.Node != nil {
		egSess2, reErr := w.egress.Rebind(acc, w.cfg.BaseURL, egSess.Node, runErr.Error())
		if reErr == nil {
			egress.AttachHTTP(&in, egSess2)
			sess, runErr = submit(in)
			if runErr == nil {
				w.egress.MarkOK(egSess2)
			}
		}
	} else if runErr == nil {
		w.egress.MarkOK(egSess)
	}
	if runErr != nil {
		return w.failOrRequeue(task, acc.ID, runErr, maxAttempts)
	}
	if sess != nil && sess.WebappID != "" {
		task.WebappID = sess.WebappID
		_ = w.store.UpdateTaskWebappID(task.ID, sess.WebappID)
	}
	if sess != nil && (sess.Kind == runninghub.SessionKindQC || runninghub.IsQCWebappID(sess.WebappID)) {
		accounts.RefreshFreeQuota(w.store, sess.Client, acc.ID)
	}
	_ = w.store.UpdateTaskRunning(task.ID, acc.ID, acc.Email, sess.TaskID)
	return w.finishPoll(task, acc, sess, in, maxAttempts)
}

// FinishAfterSubmit continues polling for a task already submitted by the HTTP create path.
func (w *Worker) FinishAfterSubmit(task store.VideoTask, acc store.Account, sess *runninghub.Session, in runninghub.RunInput) {
	const maxAttempts = 3
	if err := w.finishPoll(task, acc, sess, in, maxAttempts); err != nil {
		log.Printf("[task %s] poll failed: %v", task.ID, err)
	}
}

func (w *Worker) finishPoll(task store.VideoTask, acc store.Account, sess *runninghub.Session, in runninghub.RunInput, maxAttempts int) error {
	result, runErr := sess.Poll(in)
	if runErr != nil {
		return w.failOrRequeue(task, acc.ID, runErr, maxAttempts)
	}
	if result.AccessToken != "" {
		_ = w.store.UpdateAccountToken(acc.ID, result.AccessToken)
	}
	_ = w.store.UpdateTaskRunning(task.ID, acc.ID, acc.Email, result.TaskID)
	_ = w.pool.ReleaseOK(acc.ID)
	w.refreshAccountBalance(acc.ID)
	return w.store.FinishTaskOK(task.ID, result.FileURL, result.PreviewURL)
}

func (w *Worker) failOrRequeue(task store.VideoTask, accountID int64, runErr error, maxAttempts int) error {
	code := classifyErr(runErr)
	_ = w.pool.ReleaseFail(accountID, code, runErr.Error())
	w.refreshAccountBalance(accountID)
	if task.Attempts < maxAttempts && code != "PROMPT" {
		_ = w.store.RequeueTask(task.ID, code, runErr.Error())
		return runErr
	}
	_ = w.store.FinishTaskFail(task.ID, code, runErr.Error())
	return runErr
}

// refreshAccountBalance pulls wallet via NEW point/summary or OLD apiKey accountStatus.
func (w *Worker) refreshAccountBalance(accountID int64) {
	acc, err := w.store.GetAccount(accountID)
	if err != nil {
		return
	}
	var wallet runninghub.Wallet
	err = w.egress.Do(acc, w.cfg.BaseURL, func(client *runninghub.Client) error {
		at := strings.TrimSpace(acc.AccessToken)
		if at != "" {
			client.SetAccessToken(at)
		}
		apiKey := strings.TrimSpace(acc.APIKey)
		// NEW path only needs AT; OLD still needs consumer apiKey.
		if apiKey == "" && (at == "" || strings.EqualFold(acc.BillingVersion, runninghub.BillingOld) || acc.BillingVersion == "") {
			got, e := client.EnsureConsumerApiKey(acc.Email, acc.Password, at)
			if e == nil {
				apiKey = got.APIKey
				_ = w.store.UpdateAccountAPIKey(accountID, apiKey)
				_ = w.store.UpdateAccountAuthTokens(accountID, got.AccessToken, got.RefreshToken)
				acc.APIKey = apiKey
				if got.AccessToken != "" {
					client.SetAccessToken(got.AccessToken)
				}
			} else if at == "" {
				return e
			}
		}
		var e error
		wallet, e = client.FetchWallet(apiKey)
		if e != nil && apiKey != "" {
			got, e2 := client.EnsureConsumerApiKey(acc.Email, acc.Password, "")
			if e2 != nil {
				return e
			}
			apiKey = got.APIKey
			_ = w.store.UpdateAccountAPIKey(accountID, apiKey)
			_ = w.store.UpdateAccountAuthTokens(accountID, got.AccessToken, got.RefreshToken)
			if got.AccessToken != "" {
				client.SetAccessToken(got.AccessToken)
			}
			wallet, e = client.FetchWallet(apiKey)
		}
		if e == nil {
			accounts.RefreshFreeQuota(w.store, client, accountID)
		}
		return e
	})
	if err != nil {
		log.Printf("[account %d] refresh balance failed: %v", accountID, err)
		return
	}
	if err := w.store.UpdateAccountWallet(accountID, wallet.RemainCoins, wallet.RemainPoints, wallet.CurrentTaskCounts, wallet.APIType, wallet.BillingVersion); err != nil {
		log.Printf("[account %d] save balance failed: %v", accountID, err)
		return
	}
	log.Printf("[account %d] balance refreshed coins=%s points=%s spendable=%s counts=%s type=%s billing=%s",
		accountID, wallet.RemainCoins, wallet.RemainPoints, wallet.Spendable(), wallet.CurrentTaskCounts, wallet.APIType, wallet.BillingVersion)
}

func classifyErr(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "qc not free"), strings.Contains(msg, "prepare not free"):
		return "NO_COINS"
	case strings.Contains(msg, "login"), strings.Contains(msg, "pwdlogin"), strings.Contains(msg, "401"),
		strings.Contains(msg, "token_invalid"), strings.Contains(msg, "412"),
		strings.Contains(msg, "user_not_found"):
		return "AUTH"
	case strings.Contains(msg, "create_max_tasks_limit"), strings.Contains(msg, "423"):
		return "BUSY"
	case strings.Contains(msg, "insufficient"), strings.Contains(msg, "balance"),
		strings.Contains(msg, "remain"), strings.Contains(msg, "coin"),
		strings.Contains(msg, "积分"), strings.Contains(msg, "余额"):
		return "NO_COINS"
	case strings.Contains(msg, "prompt"), strings.Contains(msg, "sensitive"):
		return "PROMPT"
	case strings.Contains(msg, "timeout"):
		return "TIMEOUT"
	default:
		return "RH"
	}
}

func first(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseNodeInfoList(raw string) []runninghub.NodeInput {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var list []runninghub.NodeInput
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil
	}
	return list
}

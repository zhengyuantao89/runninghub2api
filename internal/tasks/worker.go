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
	if err != nil || (strings.TrimSpace(acc.Password) == "" && strings.TrimSpace(acc.AccessToken) == "") {
		return fmt.Errorf("account missing for task (id=%d)", task.AccountID)
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
			if p.AccessToken != "" {
				_ = w.store.UpdateAccountToken(acc.ID, p.AccessToken)
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
	err := w.egress.Do(acc, baseURL, func(client *runninghub.Client) error {
		if strings.TrimSpace(acc.AccessToken) != "" {
			client.SetAccessToken(acc.AccessToken)
			out = client
			return nil
		}
		if strings.TrimSpace(acc.Email) == "" || strings.TrimSpace(acc.Password) == "" {
			return fmt.Errorf("email+password or access_token required")
		}
		login, err := client.PwdLogin(acc.Email, acc.Password)
		if err != nil {
			return err
		}
		data, _ := login["data"].(map[string]any)
		token := ""
		if data != nil {
			if v, ok := data["access_token"].(string); ok {
				token = strings.TrimSpace(v)
			}
			if token == "" {
				if v, ok := data["accessToken"].(string); ok {
					token = strings.TrimSpace(v)
				}
			}
		}
		if token == "" {
			return fmt.Errorf("login missing access_token")
		}
		client.SetAccessToken(token)
		out = client
		return nil
	})
	if err != nil {
		return nil, err
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

func (w *Worker) runOne(task store.VideoTask) error {
	const maxAttempts = 3
	acc, err := w.pool.Acquire(task.ID)
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
		AccessPassword:  w.store.GetWebappAccessPassword(webappID),
		NodeInfoList:    parseNodeInfoList(task.NodeInfoJSON),
		BaseURL:         w.cfg.BaseURL,
		OnProgress: func(p runninghub.Progress) {
			if p.TaskID != "" {
				_ = w.store.UpdateTaskRunning(task.ID, acc.ID, acc.Email, p.TaskID)
			}
			_ = w.store.UpdateTaskProgress(task.ID, p.RHStatus, p.ProgressText, p.QueueIndex, p.QueueCount)
			if p.AccessToken != "" {
				_ = w.store.UpdateAccountToken(acc.ID, p.AccessToken)
			}
		},
	}

	egSess, egErr := w.egress.NewClientForAccount(acc, w.cfg.BaseURL)
	if egErr != nil {
		return w.failOrRequeue(task, acc.ID, egErr, maxAttempts)
	}
	egress.AttachHTTP(&in, egSess)

	sess, runErr := runninghub.SubmitWebappVideo(in)
	if runErr != nil && egress.IsConnectivityError(runErr) && egSess.Node != nil {
		egSess2, reErr := w.egress.Rebind(acc, w.cfg.BaseURL, egSess.Node, runErr.Error())
		if reErr == nil {
			egress.AttachHTTP(&in, egSess2)
			sess, runErr = runninghub.SubmitWebappVideo(in)
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

// refreshAccountBalance pulls remainCoins / currentTaskCounts via consumer apiKey after a task ends.
func (w *Worker) refreshAccountBalance(accountID int64) {
	acc, err := w.store.GetAccount(accountID)
	if err != nil {
		return
	}
	var st map[string]any
	err = w.egress.Do(acc, w.cfg.BaseURL, func(client *runninghub.Client) error {
		apiKey := strings.TrimSpace(acc.APIKey)
		if apiKey == "" {
			got, e := client.EnsureConsumerApiKey(acc.Email, acc.Password, acc.AccessToken)
			if e != nil {
				return e
			}
			apiKey = got.APIKey
			_ = w.store.UpdateAccountAPIKey(accountID, apiKey)
			_ = w.store.UpdateAccountAuthTokens(accountID, got.AccessToken, got.RefreshToken)
			acc.APIKey = apiKey
		}
		var e error
		st, e = client.AccountStatus(apiKey)
		if e != nil {
			got, e2 := client.EnsureConsumerApiKey(acc.Email, acc.Password, "")
			if e2 != nil {
				return e
			}
			apiKey = got.APIKey
			_ = w.store.UpdateAccountAPIKey(accountID, apiKey)
			_ = w.store.UpdateAccountAuthTokens(accountID, got.AccessToken, got.RefreshToken)
			acc.APIKey = apiKey
			st, e = client.AccountStatus(apiKey)
			return e
		}
		return nil
	})
	if err != nil {
		log.Printf("[account %d] refresh balance failed: %v", accountID, err)
		return
	}
	data, _ := st["data"].(map[string]any)
	remain, counts, apiType := "", "", ""
	if data != nil {
		if v, ok := data["remainCoins"]; ok {
			remain = fmt.Sprint(v)
		}
		if v, ok := data["currentTaskCounts"]; ok {
			counts = fmt.Sprint(v)
		}
		if v, ok := data["apiType"]; ok {
			apiType = fmt.Sprint(v)
		}
	}
	if err := w.store.UpdateAccountBalance(accountID, remain, counts, apiType); err != nil {
		log.Printf("[account %d] save balance failed: %v", accountID, err)
		return
	}
	log.Printf("[account %d] balance refreshed remain=%s counts=%s type=%s", accountID, remain, counts, apiType)
}

func classifyErr(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
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

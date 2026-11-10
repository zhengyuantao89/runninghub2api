package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/trae/runninghub2api/internal/accounts"
	"github.com/trae/runninghub2api/internal/config"
	"github.com/trae/runninghub2api/internal/egress"
	"github.com/trae/runninghub2api/internal/httpapi"
	"github.com/trae/runninghub2api/internal/store"
	"github.com/trae/runninghub2api/internal/tasks"
)

func main() {
	config.LoadDotEnv(".env")
	cfg := config.Load()
	st, err := store.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	pool := accounts.New(st, time.Duration(cfg.SubmitGapSec)*time.Second)
	worker := tasks.NewWorker(st, pool, cfg)
	worker.Start(cfg.Workers)
	defer worker.Stop()

	// 一天扫一两次即可；已领过的账号会按上海日期跳过，不会反复打 RH
	dailyClaimer := accounts.NewDailyLoginClaimer(st, cfg.BaseURL, 12*time.Hour)
	dailyClaimer.Start()
	defer dailyClaimer.Stop()

	subPoller := egress.NewSubscriptionPoller(st, time.Minute)
	subPoller.Start()
	defer subPoller.Stop()

	srv := httpapi.New(cfg, st, pool, worker, dailyClaimer)
	httpServer := &http.Server{Addr: cfg.Addr, Handler: srv.Handler()}

	go func() {
		log.Printf("runninghub2api listening on %s", cfg.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Printf("shutting down")
	_ = httpServer.Close()
}

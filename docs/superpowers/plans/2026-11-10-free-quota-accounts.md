# Free Image/Video Quota on Accounts — Implementation Plan

> **For agentic workers:** Execute task-by-task. Checkboxes track progress. Do **not** git commit unless the user asks.

**Goal:** Persist and show per-account daily free image/video quotas (`remaining/total`), refreshing from `/api/welfare/summary` on balance refresh, daily claim, and after QC free video submit.

**Architecture:** Parse `freeImage`/`freeVideo` from welfare summary into `FreeQuota`; store four ints (+ optional `last_free_quota_at`) on `accounts`; wire refresh helper used by wallet/claim/QC success; expose JSON + Accounts/Dashboard columns as `剩余/总额`.

**Tech Stack:** Go, SQLite migrate-on-open, Vue Accounts/Dashboard

**Spec:** `docs/superpowers/specs/2026-11-10-free-quota-accounts-design.md`

---

### Task 1: Parse FreeQuota + unit tests

**Files:**
- Create/Modify: `internal/provider/runninghub/wallet.go` (or new `welfare.go`)
- Create/Modify: `internal/provider/runninghub/wallet_test.go`

- [x] Add `FreeQuota` struct + `ParseFreeQuota(welfareSummary map[string]any) (FreeQuota, bool)`
- [x] Tests for remaining/total image+video from HAR-shaped JSON
- [x] `go test ./internal/provider/runninghub/ -count=1`

### Task 2: Store columns + UpdateAccountFreeQuota

**Files:**
- Modify: `internal/store/store.go` (Account struct, migrate, accountSelect, scan, updater)

- [x] Columns: `free_image_remaining`, `free_image_total`, `free_video_remaining`, `free_video_total`, `last_free_quota_at`
- [x] `UpdateAccountFreeQuota(id, imageRem, imageTot, videoRem, videoTot)`
- [x] JSON tags on Account

### Task 3: Refresh helper + wire wallet / daily claim / QC

**Files:**
- Modify: `internal/provider/runninghub/wallet.go` — optional FetchFreeQuota on Client
- Modify: `internal/tasks/worker.go` — after balance refresh
- Modify: `internal/accounts/daily_login.go` — after NEW claim
- Modify: `internal/httpapi/server.go` — after balance refresh API if any
- Modify: QC submit success paths (worker + httpapi) — refresh free quota for account

- [x] Helper that calls `WelfareSummary(false)` and `UpdateAccountFreeQuota`
- [x] Errors from welfare must not fail balance refresh (log + continue)
- [x] After `SubmitQCFreeVideo` success, refresh that account’s free quota

### Task 4: UI Accounts + Dashboard

**Files:**
- Modify: `web/src/views/Accounts.vue`
- Modify: `web/src/views/Dashboard.vue`

- [x] Columns 免费图 / 免费视频 showing `r/t` or `—` when total==0
- [x] Detail drawer fields

### Task 5: Local docker rebuild verify

- [x] Build linux binary + web → `docker_ctx`, `docker compose up -d --build`
- [x] Confirm accounts JSON includes new fields; health OK

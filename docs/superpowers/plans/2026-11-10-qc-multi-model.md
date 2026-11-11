# QC Multi-Model Implementation Plan

> **For agentic workers:** Implement task-by-task.

**Goal:** Generic Runow QC submit for all VIDEO/IMAGE models with free/paid fee-preview gating.

**Architecture:** Catalog cache + `SubmitQCGeneric`; HTTP `/v1/qc/*` + extend `/v1/videos` with `submit_mode=qc`; Studio Runow channel.

**Tech Stack:** Go, existing QC client, Vue studio.

## Tasks

- [ ] Task1: `qc_catalog.go` parse/fetch modes+models; unit test with fixture
- [ ] Task2: `SubmitQCGeneric` allow paid when fee.passed
- [ ] Task3: HTTP catalog + wire createVideo submit_mode=qc
- [ ] Task4: Studio Runow mode/model/params UI
- [ ] Task5: docker rebuild verify

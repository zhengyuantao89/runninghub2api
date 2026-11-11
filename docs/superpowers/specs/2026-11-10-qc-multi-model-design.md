# Design: Runow QC multi-model (free + paid)

Date: 2026-11-10  
Status: approved

## Goal

Support the full Runow QC video/image catalog — not only hard-coded free Minimax H3 — including paid models that spend account points. Distinguish free vs paid via live fee-preview.

## Approach

1. Catalog: cache `creation-modes` + `models` (and optional binding fields) via any pool account AT.
2. Submit: client sends `bindingId`, `skuId`, `categoryId`, `creationMode`, `params` (+ uploads). Server runs fee-preview → prepare → commit → batchStatus (existing QC poll).
3. Billing gate: if fee is free/welfareFree → OK; else require `passed` and no insufficient balance; spend points on RH side.
4. Keep `qc_free` / `qc_free_image` as shortcuts that pin known free bindings.

## Non-goals (this pass)

- Full parity with every obscure SKU field UI widget (start with common + JSON/advanced).
- VIDEO_EDIT category deep tooling.
- Changing webapp template path.

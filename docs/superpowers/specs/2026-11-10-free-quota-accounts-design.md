# Free Image/Video Quota on Accounts — Design

Date: 2026-11-10  
Status: approved approach B (pending user review of this spec)

## Goal

Show each account’s **daily free image / free video** quotas on the accounts list (and dashboard), and **refresh remaining counts from RH** after we successfully use free creation — so later we can schedule by free quota.

## Source of truth

`POST /api/welfare/summary` (`issue: false|true`) returns:

```json
{
  "freeImage": { "total": 3, "used": 0, "remaining": 3, "...": "..." },
  "freeVideo": { "total": 3, "used": 1, "remaining": 2, "...": "..." },
  "freePeriod": { "periodNo": 1, "endTime": "..." }
}
```

We persist **remaining + total** only (not used, not period end) for this iteration.

## Data model

Add columns on `accounts` (SQLite migrate-on-open, same pattern as `remain_points`):

| Column | Type | Meaning |
|--------|------|---------|
| `free_image_remaining` | INTEGER NOT NULL DEFAULT 0 | from `freeImage.remaining` |
| `free_image_total` | INTEGER NOT NULL DEFAULT 0 | from `freeImage.total` |
| `free_video_remaining` | INTEGER NOT NULL DEFAULT 0 | from `freeVideo.remaining` |
| `free_video_total` | INTEGER NOT NULL DEFAULT 0 | from `freeVideo.total` |

JSON on account APIs:

- `free_image_remaining`, `free_image_total`
- `free_video_remaining`, `free_video_total`

Unset / never synced: both remaining and total stay `0`; UI shows `—` when `total == 0` **and** never refreshed is ambiguous — rule:

- If `last_free_quota_at` is empty **or** both totals are 0 after a known OLD account without welfare: show `—`
- Simpler rule for v1: **show `remaining/total` when `total > 0`, else `—`**

Optional companion field (recommended for debug, cheap):

- `last_free_quota_at` TEXT — last successful welfare free-quota write (RFC3339)

## Refresh timing

1. **Balance refresh** (`FetchWallet` / worker `refreshAccountBalance` / admin refresh): for NEW billing (or whenever AT works), also call `WelfareSummary(false)` and write free fields.
2. **Daily claim** (`claimNewBilling`): after gift claim / summary, parse freeImage/freeVideo and persist (same write helper).
3. **QC free video commit success** (`SubmitQCFreeVideo` after commit returns taskId): caller (HTTP create path / worker) refreshes welfare summary for that account and updates DB.
4. **Do not** optimistic local decrement; always re-fetch summary (user choice #1).

OLD billing accounts: welfare may 401/empty — leave free fields at 0 / UI `—`, do not fail balance refresh.

## UI

**Accounts** table + detail drawer:

- Column **免费图**: `{{ remaining }}/{{ total }}` or `—`
- Column **免费视频**: same

**Dashboard** account strip/table: same two columns next to RH币 / 积分.

No scheduling change in this iteration (select-by-free-quota is follow-up).

## Code shape

- `runninghub.ParseFreeQuota(welfareSummary) (FreeQuota, bool)`
- `runninghub.FreeQuota { ImageRemaining, ImageTotal, VideoRemaining, VideoTotal }`
- `store.UpdateAccountFreeQuota(id, fq, at)`
- Wire into: wallet refresh, daily claim NEW path, QC submit success path

## Out of scope

- Dispatch / acquire prefer accounts with `free_video_remaining > 0`
- Persisting `used`, `freePeriod.endTime`, model entry keys
- Free image create client (only video QC exists today; refresh hook is shared)

## Test plan

- Unit: parse welfare JSON → FreeQuota
- Migrate: new columns exist; old rows default 0
- Manual local docker: after daily claim / balance refresh, Accounts show e.g. `3/3`; after one QC free submit, video becomes `2/3`

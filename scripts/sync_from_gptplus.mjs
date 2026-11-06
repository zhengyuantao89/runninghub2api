#!/usr/bin/env node
/**
 * 从 gpt-plus 导出 RH 账号并推入 runninghub2api。
 *
 * 用法:
 *   set GPTPLUS_URL=http://127.0.0.1:3000
 *   set GPTPLUS_TOKEN=...   # gpt-plus 登录 JWT
 *   set RH2API_URL=http://127.0.0.1:8088
 *   set RH2API_ADMIN_KEY=change-me-admin-key
 *   node scripts/sync_from_gptplus.mjs
 */
const gptPlus = (process.env.GPTPLUS_URL || 'http://127.0.0.1:3000').replace(/\/$/, '');
const gptToken = process.env.GPTPLUS_TOKEN || '';
const rhUrl = (process.env.RH2API_URL || 'http://127.0.0.1:8088').replace(/\/$/, '');
const adminKey = process.env.RH2API_ADMIN_KEY || process.env.RH2API_SYNC_TOKEN || '';

if (!gptToken || !adminKey) {
  console.error('需要 GPTPLUS_TOKEN 与 RH2API_ADMIN_KEY');
  process.exit(1);
}

const exported = await fetch(`${gptPlus}/api/runninghub/accounts/export-for-sync`, {
  method: 'POST',
  headers: {
    Authorization: `Bearer ${gptToken}`,
    'content-type': 'application/json',
  },
  body: JSON.stringify({}),
}).then((r) => r.json());

if (!Array.isArray(exported.accounts)) {
  console.error('export failed', exported);
  process.exit(1);
}

const pushed = await fetch(`${rhUrl}/admin/sync/accounts`, {
  method: 'POST',
  headers: {
    Authorization: `Bearer ${adminKey}`,
    'content-type': 'application/json',
  },
  body: JSON.stringify({ accounts: exported.accounts }),
}).then((r) => r.json());

console.log(JSON.stringify({ exported: exported.total, pushed }, null, 2));

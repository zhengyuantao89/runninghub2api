/**
 * 直接从 gpt-plus app.db 解密导出 RH 账号并导入 runninghub2api。
 * 用法: node scripts/_import_from_gptplus_db.mjs [limit]
 */
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const limit = Math.min(Math.max(Number(process.argv[2]) || 20, 1), 100);
const gptPlusRoot = process.env.GPTPLUS_ROOT || 'F:/Trae_projects/gpt-plus';
const dbPath = process.env.GPTPLUS_DB || path.join(gptPlusRoot, 'server/data/app.db');
const rhUrl = (process.env.RH2API_URL || 'http://127.0.0.1:8088').replace(/\/$/, '');
const adminKey = process.env.RH2API_ADMIN_KEY || 'change-me-admin-key';

// Load AES_KEY from gpt-plus .env into process.env before requiring crypto.
const envPath = path.join(gptPlusRoot, '.env');
for (const line of fs.readFileSync(envPath, 'utf8').split(/\r?\n/)) {
  const m = line.match(/^\s*([A-Z0-9_]+)\s*=\s*(.*)\s*$/);
  if (!m) continue;
  if (process.env[m[1]] == null) process.env[m[1]] = m[2];
}

const require = createRequire(path.join(gptPlusRoot, 'server/package.json'));
const Database = require('better-sqlite3');
const { decrypt } = require(path.join(gptPlusRoot, 'server/src/utils/crypto.js'));

const db = new Database(dbPath, { readonly: true });
const total = db.prepare('SELECT COUNT(*) AS n FROM rh_accounts').get().n;
const okCount = db
  .prepare(
    `SELECT COUNT(*) AS n FROM rh_accounts
     WHERE status='正常'
       AND registration_status IN ('已注册','success','registered','ok')`,
  )
  .get().n;

const rows = db
  .prepare(
    `SELECT * FROM rh_accounts
     WHERE status='正常'
       AND registration_status IN ('已注册','success','registered','ok')
     ORDER BY id DESC
     LIMIT ?`,
  )
  .all(limit);

const accounts = rows.map((row) => {
  let password = '';
  let accessToken = '';
  let refreshToken = '';
  try {
    password = decrypt(row.password_enc || '');
  } catch {
    /* ignore */
  }
  try {
    accessToken = decrypt(row.access_token_enc || '');
  } catch {
    /* ignore */
  }
  try {
    refreshToken = decrypt(row.refresh_token_enc || '');
  } catch {
    /* ignore */
  }
  return {
    email: row.email,
    password,
    access_token: accessToken,
    refresh_token: refreshToken,
    identify: row.identify || '',
    rh_user_id: row.rh_user_id || '',
    remain_coins: row.remain_coins || row.total_coin || '',
    status: 'active',
    source: 'gpt-plus',
    external_id: String(row.id),
  };
});

const withPassword = accounts.filter((a) => a.password).length;
console.log(
  JSON.stringify(
    {
      db: dbPath,
      total,
      okCount,
      exporting: accounts.length,
      withPassword,
      emails: accounts.map((a) => a.email),
    },
    null,
    2,
  ),
);

if (!accounts.length) {
  console.error('没有可导入的账号');
  process.exit(1);
}

const res = await fetch(`${rhUrl}/admin/sync/accounts`, {
  method: 'POST',
  headers: {
    Authorization: `Bearer ${adminKey}`,
    'content-type': 'application/json',
  },
  body: JSON.stringify({ accounts }),
});
const body = await res.json().catch(() => ({}));
console.log('import status', res.status, JSON.stringify(body, null, 2));

const list = await fetch(`${rhUrl}/admin/accounts`, {
  headers: { Authorization: `Bearer ${adminKey}` },
}).then((r) => r.json());
console.log('pool size', Array.isArray(list.data) ? list.data.length : list);

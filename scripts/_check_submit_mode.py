import json
import os

import paramiko

HOST = os.environ.get("RH2API_DEPLOY_HOST", "82.158.89.24")
PASS = os.environ.get("RH2API_DEPLOY_PASS", "")

REMOTE_PY = r'''
import sqlite3, json
conn=sqlite3.connect("file:/opt/runninghub2api/data/rh2api.db?mode=ro", uri=True)
cur=conn.cursor()
print("settings", cur.execute("SELECT key,value FROM app_settings").fetchall())
rows=cur.execute("""
SELECT id,status,account_email,rh_task_id,error_code,substr(error,1,100),created_at,finished_at
FROM video_tasks ORDER BY created_at DESC LIMIT 8
""").fetchall()
print("recent_tasks")
for r in rows:
    print(r)
acc=cur.execute("""
SELECT id,email,
  CASE WHEN IFNULL(access_token,'')!='' THEN 1 ELSE 0 END AS has_at,
  CASE WHEN IFNULL(api_key,'')!='' THEN 1 ELSE 0 END AS has_key,
  CASE WHEN IFNULL(password,'')!='' THEN 1 ELSE 0 END AS has_pwd,
  last_submit_at, status
FROM accounts
WHERE IFNULL(last_submit_at,'')!=''
ORDER BY last_submit_at DESC LIMIT 8
""").fetchall()
print("recent_submit_accounts")
for a in acc:
    print(a)
# how many accounts have AT
print("at_counts", cur.execute("""
SELECT
  SUM(CASE WHEN IFNULL(access_token,'')!='' THEN 1 ELSE 0 END),
  SUM(CASE WHEN IFNULL(api_key,'')!='' THEN 1 ELSE 0 END),
  COUNT(*)
FROM accounts WHERE status='active'
""").fetchone())
conn.close()
'''

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, 22, "root", PASS, timeout=30, allow_agent=False, look_for_keys=False)


def run(cmd: str) -> str:
    _, o, e = c.exec_command(cmd, timeout=60)
    out = o.read().decode("utf-8", "ignore")
    err = e.read().decode("utf-8", "ignore")
    print(">>>", cmd[:180])
    print((out or err)[:2500])
    return out


run("curl -sS http://127.0.0.1:8088/health")
login = run(
    "curl -sS -X POST http://127.0.0.1:8088/api/login "
    "-H 'Content-Type: application/json' "
    "-d '{\"username\":\"admin\",\"password\":\"admin123\"}'"
)
try:
    token = json.loads(login).get("token") or ""
except Exception:
    token = ""
if token:
    run(f"curl -sS http://127.0.0.1:8088/admin/settings -H 'Authorization: Bearer {token}'")

_, o, e = c.exec_command("python3 -", timeout=60)
o.channel.sendall(REMOTE_PY.encode())
o.channel.shutdown_write()
print(o.read().decode("utf-8", "ignore") or e.read().decode("utf-8", "ignore"))

run(
    "journalctl -u runninghub2api --since '6 hours ago' --no-pager "
    "| grep -Ei 'pwdLogin|FREQUENT|submit deferred|webapp/create|poll_auth|EnsureConsumer' "
    "| tail -n 40 || true"
)
c.close()

import json
import os
import sqlite3
import urllib.request

import paramiko

HOST = os.environ.get("RH2API_DEPLOY_HOST", "82.158.89.24")
PASS = os.environ["RH2API_DEPLOY_PASS"]
TASK_ID = os.environ.get("RH2API_TASK_NEEDLE", "vid_c1bc44acb6f7")

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, 22, "root", PASS, timeout=30, allow_agent=False, look_for_keys=False)

remote = r'''
import json, sqlite3, urllib.request
con = sqlite3.connect("/opt/runninghub2api/data/rh2api.db")
con.row_factory = sqlite3.Row
tid = "TASK_ID_PLACEHOLDER"
t = con.execute(
    "SELECT id,status,account_id,account_email,rh_task_id,rh_status,webapp_id,error,updated_at FROM video_tasks WHERE id=?",
    (tid,),
).fetchone()
print("task", dict(t) if t else None)
if not t:
    raise SystemExit(0)
acc = con.execute(
    "SELECT id,email,access_token,api_key,password,anonymous_id,fingerprint_json FROM accounts WHERE id=?",
    (t["account_id"],),
).fetchone()
print("account", {k: (acc[k][:20]+"..." if k in ("access_token","api_key","password","fingerprint_json") and acc[k] else acc[k]) for k in acc.keys()} if acc else None)

# local health / task API via loopback if possible
import subprocess, os
# Probe RH task list with AT if present
at = (acc["access_token"] or "").strip() if acc else ""
rh_tid = (t["rh_task_id"] or "").strip()
if at and rh_tid:
    body = json.dumps({"page":1,"pageSize":20}).encode()
    req = urllib.request.Request(
        "https://www.runninghub.ai/task/list",
        data=body,
        headers={
            "Content-Type":"application/json",
            "Authorization":"Bearer "+at,
            "client":"WEB",
            "user-language":"zh_CN",
            "x-rh-lang":"zh",
            "Origin":"https://www.runninghub.ai",
            "Referer":"https://www.runninghub.ai/zh-cn",
            "User-Agent":"Mozilla/5.0",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=40) as resp:
            raw = json.loads(resp.read().decode())
        print("task_list_code", raw.get("code"), raw.get("msg"))
        data = raw.get("data") or {}
        items = data.get("records") or data.get("list") or data.get("rows") or []
        if isinstance(data, list):
            items = data
        hit = None
        for it in items if isinstance(items, list) else []:
            sid = str(it.get("taskId") or it.get("id") or it.get("task_id") or "")
            if sid == rh_tid:
                hit = it
                break
        print("list_items", len(items) if isinstance(items, list) else type(items))
        print("hit_in_first_page", hit)
    except Exception as e:
        print("task_list_err", e)

# Also dump all running with same account
print("--- same account running ---")
for r in con.execute(
    "SELECT id,status,rh_task_id,rh_status,error_code,substr(error,1,100) e,updated_at FROM video_tasks WHERE account_id=? AND status='running'",
    (t["account_id"],),
):
    print(dict(r))
'''.replace("TASK_ID_PLACEHOLDER", TASK_ID)

sftp = c.open_sftp()
with sftp.file("/tmp/_check_rh_task.py", "w") as f:
    f.write(remote)
sftp.close()
_, out, err = c.exec_command("python3 /tmp/_check_rh_task.py", timeout=90)
print(out.read().decode("utf-8", "ignore"))
e = err.read().decode("utf-8", "ignore")
if e.strip():
    print("STDERR:", e)
c.close()

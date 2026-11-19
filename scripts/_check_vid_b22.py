#!/usr/bin/env python3
import os, sys, json, hashlib
from pathlib import Path
import paramiko

HOST = "82.158.89.24"
PASS = os.environ.get("RH2API_DEPLOY_PASS", "")
if not PASS:
    lines = Path("服务器.txt").read_text(encoding="utf-8").splitlines()
    for i, line in enumerate(lines):
        if HOST in line:
            for j in range(i, min(i + 5, len(lines))):
                if "密码" in lines[j]:
                    PASS = lines[j].split("：", 1)[-1].split(":", 1)[-1].strip()
                    break
            break

LOCAL_ID = "vid_b22f781ee84b"
c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, 22, "root", PASS, timeout=20, allow_agent=False, look_for_keys=False)
sftp = c.open_sftp()
helper = r'''
import json, sqlite3, urllib.request, hashlib, subprocess
from urllib.parse import quote
from datetime import datetime, timezone, timedelta

LOCAL_ID = "vid_b22f781ee84b"
BASE = "https://www.runninghub.ai"
con = sqlite3.connect("/opt/runninghub2api/data/rh2api.db")
con.row_factory = sqlite3.Row
task = dict(con.execute("SELECT * FROM video_tasks WHERE id=?", (LOCAL_ID,)).fetchone())
print("=== local ===")
for k in ["id","status","account_id","account_email","rh_task_id","rh_status","progress_text","error","error_code","attempts","webapp_id","created_at","updated_at","finished_at"]:
    print(f"  {k}: {task.get(k)}")

acc = dict(con.execute("SELECT * FROM accounts WHERE id=?", (task["account_id"],)).fetchone()) if task.get("account_id") else None
print("=== account ===", None if not acc else {k: acc.get(k) for k in ["id","email","status","inflight","lease_task_id","egress_id"]})
print("  has_at", bool((acc or {}).get("access_token")), "has_key", bool((acc or {}).get("api_key")), "has_pwd", bool((acc or {}).get("password")))

# nearby local tasks same account
print("=== nearby same account ===")
for r in con.execute(
    "SELECT id,status,rh_task_id,error,created_at,updated_at FROM video_tasks WHERE account_id=? AND created_at BETWEEN ? AND ? ORDER BY created_at",
    (task["account_id"], "2026-11-13T04:30:00Z", "2026-11-13T05:00:00Z"),
):
    print(dict(r))

# journal
print("=== journal ===")
j = subprocess.check_output(
    f"journalctl -u runninghub2api --since '2026-11-13 04:40:00' --until '2026-11-13 04:50:00' --no-pager | grep -E '{LOCAL_ID}|account=61|15360438361abutao' | tail -n 40",
    shell=True, text=True, stderr=subprocess.STDOUT,
)
print(j or "(none)")

if not acc or not (acc.get("password") or "").strip():
    raise SystemExit("no account/password")

eg = None
if acc.get("egress_id"):
    eg = dict(con.execute("SELECT * FROM egress_nodes WHERE id=?", (acc["egress_id"],)).fetchone())
handlers = []
if eg:
    user = quote((eg.get("username") or "").strip())
    pw = quote((eg.get("password") or "").strip())
    proxy_url = f"http://{user}:{pw}@{eg['host']}:{eg['port']}"
    handlers.append(urllib.request.ProxyHandler({"http": proxy_url, "https": proxy_url}))
    print("egress", eg["host"], eg["port"])
opener = urllib.request.build_opener(*handlers)

def post(path, body, headers=None):
    data = json.dumps(body).encode()
    h = {"Content-Type": "application/json", "User-Agent": "Mozilla/5.0"}
    if headers: h.update(headers)
    req = urllib.request.Request(BASE + path, data=data, headers=h, method="POST")
    with opener.open(req, timeout=60) as r:
        return json.loads(r.read().decode("utf-8", "ignore") or "{}")

resp = post("/uc/abroad/pwdLogin", {
    "email": acc["email"],
    "password": hashlib.md5((acc["password"] or "").encode()).hexdigest(),
})
print("pwdLogin", resp.get("code"), resp.get("msg"))
if resp.get("code") != 0:
    raise SystemExit("login fail")
data = resp.get("data") or {}
at = data.get("access_token") or data.get("accessToken") or ""
headers = {"Authorization": "Bearer " + at}
anon = (acc.get("anonymous_id") or "").strip()
if anon: headers["Anonymous-Id"] = anon

# local created 04:41:59Z = 12:41:59 CST
lst = post("/task/list", {"size": 50, "current": 1, "taskType": ["WORKFLOW", "WEBAPP"]}, headers)
records = ((lst.get("data") or {}).get("records") or [])
print("=== RH tasks around 12:40-12:50 CST / webapp match ===")
webapp = (task.get("webapp_id") or "").strip()
linked = {r["rh_task_id"] for r in con.execute("SELECT rh_task_id FROM video_tasks WHERE rh_task_id!=''")}
cands = []
for it in records:
    tid = str(it.get("taskId") or "")
    ct = str(it.get("createTime") or "")
    wid = str(it.get("webappId") or it.get("appId") or "")
    # show near window
    near = ct.startswith("2026-11-13 12:3") or ct.startswith("2026-11-13 12:4") or ct.startswith("2026-11-13 12:5")
    if near or (webapp and wid == webapp and ct.startswith("2026-11-13 12:")):
        row = {"taskId": tid, "status": it.get("taskStatus"), "webappId": wid, "createTime": ct, "linked_local": tid in linked}
        print(" ", row)
        if not row["linked_local"] and wid == webapp and near:
            cands.append(row)
print("orphan_cands", cands)

# also RUNNING/QUEUED
lst2 = post("/task/list", {"size": 50, "current": 1, "taskStatus": ["RUNNING", "QUEUED"], "taskType": ["WORKFLOW", "WEBAPP"]}, headers)
rec2 = ((lst2.get("data") or {}).get("records") or [])
print("RH running/queued", len(rec2))
for it in rec2[:10]:
    print(" ", {k: it.get(k) for k in ["taskId","taskStatus","webappId","createTime"]})
'''
with sftp.file("/tmp/check_vid_b22.py", "w") as f:
    f.write(helper)
sftp.close()
_, o, e = c.exec_command("python3 /tmp/check_vid_b22.py", timeout=120)
sys.stdout.write(o.read().decode("utf-8", "ignore"))
sys.stdout.write(e.read().decode("utf-8", "ignore"))
c.exec_command("rm -f /tmp/check_vid_b22.py")
c.close()

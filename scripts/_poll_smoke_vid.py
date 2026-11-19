#!/usr/bin/env python3
import os, sys, json, time
from pathlib import Path
import paramiko

HOST = "82.158.89.24"
TID = "vid_7320d3bd0f22"


def load_pass() -> str:
    lines = Path("服务器.txt").read_text(encoding="utf-8").splitlines()
    for i, line in enumerate(lines):
        if HOST in line:
            for j in range(i, min(i + 5, len(lines))):
                if "密码" in lines[j]:
                    return lines[j].split("：", 1)[-1].split(":", 1)[-1].strip()
    raise SystemExit("no pass")


REMOTE = f'''
import json, sqlite3, urllib.request
key=""
for line in open("/opt/runninghub2api/.env", encoding="utf-8", errors="ignore"):
    if line.startswith(("RH2API_API_KEY=","RH2API_ADMIN_KEY=","RH2API_AUTH_KEY=")):
        key=line.split("=",1)[1].strip().strip('"').strip("'")
        if key: break
req=urllib.request.Request("http://127.0.0.1:8088/v1/videos/{TID}", headers={{"Authorization":"Bearer "+key}})
with urllib.request.urlopen(req, timeout=30) as resp:
    cur=json.loads(resp.read().decode())
print(json.dumps({{k:cur.get(k) for k in ["id","status","rh_task_id","error","error_code","progress_text","file_url","rh_status"]}}, ensure_ascii=False))
con=sqlite3.connect("file:/opt/runninghub2api/data/rh2api.db?mode=ro", uri=True)
con.row_factory=sqlite3.Row
row=con.execute("SELECT id,status,error_code,substr(IFNULL(error,''),1,200) err,rh_task_id,progress_text FROM video_tasks WHERE id=?", ("{TID}",)).fetchone()
print("db", dict(row))
'''

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, 22, "root", load_pass(), timeout=30, allow_agent=False, look_for_keys=False)
_, o, e = c.exec_command("python3 -", timeout=60)
o.channel.sendall(REMOTE.encode())
o.channel.shutdown_write()
sys.stdout.write(o.read().decode("utf-8", "ignore"))
sys.stderr.write(e.read().decode("utf-8", "ignore"))
c.close()

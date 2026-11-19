#!/usr/bin/env python3
import os
import sys
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

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, 22, "root", PASS, timeout=20, allow_agent=False, look_for_keys=False)
remote = r"""
python3 <<'PY'
import json, urllib.request, sqlite3
con = sqlite3.connect('/opt/runninghub2api/data/rh2api.db')
con.row_factory = sqlite3.Row
print('=== running/queued ===')
rows = con.execute(
    "SELECT id,status,account_id,account_email,rh_task_id,rh_status,progress_text,error,attempts,updated_at "
    "FROM video_tasks WHERE status IN ('running','queued') ORDER BY updated_at DESC LIMIT 20"
).fetchall()
for r in rows:
    print(dict(r))
env=open('/opt/runninghub2api/.env',encoding='utf-8',errors='ignore').read().splitlines()
key=''
for line in env:
  if line.startswith(('ADMIN_API_KEY=','API_KEY=','RH2API_API_KEY=','AUTH_KEY=')):
    key=line.split('=',1)[1].strip().strip('"').strip("'"); break
h={'Authorization':'Bearer '+key}
req=urllib.request.Request('http://127.0.0.1:8088/admin/tasks?page=1&page_size=3',headers=h)
with urllib.request.urlopen(req,timeout=15) as r:
  data=json.loads(r.read())
print('=== api ===')
for t in data.get('data') or []:
  print({k:t.get(k) for k in ['id','status','rh_task_id','rh_status','progress_text']})
PY
journalctl -u runninghub2api -n 100 --no-pager | grep -E 'vid_|resume|orphan|poll' | tail -n 30
"""
_, o, e = c.exec_command(remote, timeout=40)
sys.stdout.write(o.read().decode("utf-8", "ignore"))
sys.stdout.write(e.read().decode("utf-8", "ignore"))
c.close()

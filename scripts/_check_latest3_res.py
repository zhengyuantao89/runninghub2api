#!/usr/bin/env python3
from pathlib import Path
import paramiko

HOST = "82.158.89.24"


def load_pass() -> str:
    lines = Path("服务器.txt").read_text(encoding="utf-8").splitlines()
    for i, line in enumerate(lines):
        if HOST in line:
            for j in range(i, min(i + 5, len(lines))):
                if "密码" in lines[j]:
                    return lines[j].split("：", 1)[-1].split(":", 1)[-1].strip()
    raise SystemExit("no pass")


REMOTE = r'''
import sqlite3, json, math
con=sqlite3.connect("file:/opt/runninghub2api/data/rh2api.db?mode=ro", uri=True)
con.row_factory=sqlite3.Row
rows=con.execute("""
SELECT id,status,resolution,aspect_ratio,duration,webapp_id,created_at,file_url,
       substr(IFNULL(prompt,''),1,30) prompt_head
FROM video_tasks
ORDER BY created_at DESC LIMIT 8
""").fetchall()
print("=== latest 8 ===")
for r in rows:
    d=dict(r)
    res=d.get("resolution") or ""
    mp={"480p":0.135164359862,"768p":0.346020761246,"1080p":0.684269571799}.get(res)
    d["mapped_megapixels"]=mp
    print(d)
print("=== latest 3 detail ===")
for r in rows[:3]:
    d=dict(r)
    print(f"{d['id']}  resolution={d['resolution']!r}  aspect={d['aspect_ratio']!r}  duration={d['duration']!r}  status={d['status']}")
con.close()
'''

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, 22, "root", load_pass(), timeout=30, allow_agent=False, look_for_keys=False)
_, o, e = c.exec_command("python3 -", timeout=60)
o.channel.sendall(REMOTE.encode())
o.channel.shutdown_write()
print(o.read().decode("utf-8", "ignore"))
err = e.read().decode("utf-8", "ignore")
if err:
    print(err)
c.close()

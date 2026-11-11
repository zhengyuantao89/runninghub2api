import os

import paramiko

HOST = os.environ.get("RH2API_DEPLOY_HOST", "82.158.89.24")
PASS = os.environ.get("RH2API_DEPLOY_PASS", "")

SCRIPT = r'''
import datetime as dt
import sqlite3
from pathlib import Path

up = Path("/opt/runninghub2api/data/uploads")
files = [p for p in up.iterdir() if p.is_file()] if up.exists() else []
files.sort(key=lambda p: p.stat().st_mtime)
print("disk_files", len(files))
if files:
    def fmt(p):
        return dt.datetime.utcfromtimestamp(p.stat().st_mtime).strftime("%Y-%m-%d %H:%M:%S UTC")
    total = sum(p.stat().st_size for p in files)
    print("oldest", files[0].name, fmt(files[0]))
    print("newest", files[-1].name, fmt(files[-1]))
    print("size_mb", round(total / 1024 / 1024, 2))

conn = sqlite3.connect("file:/opt/runninghub2api/data/rh2api.db?mode=ro", uri=True)
cur = conn.cursor()
rows = cur.execute(
    "SELECT status, COUNT(*), SUM(CASE WHEN image_path!='' THEN 1 ELSE 0 END) FROM video_tasks GROUP BY status ORDER BY status"
).fetchall()
print("tasks_by_status[status,count,with_image_path]", rows)
with_path = cur.execute("SELECT COUNT(*) FROM video_tasks WHERE image_path!=''").fetchone()[0]
print("tasks_with_image_path", with_path)
active = cur.execute(
    "SELECT COUNT(*) FROM video_tasks WHERE status IN ('queued','running') AND image_path!=''"
).fetchone()[0]
print("active_with_image_path", active)

def exists(path: str) -> bool:
    if not path:
        return False
    p = Path(path)
    if p.is_file():
        return True
    return (Path("/opt/runninghub2api") / path).is_file()

sample = cur.execute(
    "SELECT id,status,image_path,finished_at FROM video_tasks WHERE image_path!='' ORDER BY updated_at DESC LIMIT 8"
).fetchall()
print("sample_recent_with_path:")
for id_, st, path, fin in sample:
    print(f"  {id_} {st} exists={exists(path)} finished={fin} path={path}")

paths = [r[0] for r in cur.execute("SELECT DISTINCT image_path FROM video_tasks WHERE image_path!=''").fetchall()]
exist_n = sum(1 for p in paths if exists(p))
print("distinct_paths", len(paths), "exist", exist_n, "missing", len(paths) - exist_n)
bases = {Path(p).name for p in paths}
orphans = [p for p in files if p.name not in bases]
print("orphan_files", len(orphans))
conn.close()
'''

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, 22, "root", PASS, timeout=30, allow_agent=False, look_for_keys=False)
_, stdout, stderr = c.exec_command("python3 -", timeout=60)
stdout.channel.sendall(SCRIPT.encode())
stdout.channel.shutdown_write()
out = stdout.read().decode("utf-8", "ignore")
err = stderr.read().decode("utf-8", "ignore")
code = stdout.channel.recv_exit_status()
print(out or err)
if code != 0:
    raise SystemExit(err or f"exit {code}")
c.close()

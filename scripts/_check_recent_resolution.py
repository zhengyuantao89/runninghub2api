#!/usr/bin/env python3
"""Inspect recent video tasks' resolution fields on deploy host."""
from __future__ import annotations

import sys
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
import json, sqlite3, subprocess, os

con = sqlite3.connect("file:/opt/runninghub2api/data/rh2api.db?mode=ro", uri=True)
con.row_factory = sqlite3.Row
print("=== latest 12 tasks ===")
rows = con.execute("""
SELECT id, status, webapp_id, duration, aspect_ratio, resolution,
       substr(IFNULL(prompt,''),1,40) prompt_head,
       rh_task_id, account_email,
       substr(IFNULL(error,''),1,120) err,
       created_at, finished_at, file_url, progress_text
FROM video_tasks
ORDER BY created_at DESC LIMIT 12
""").fetchall()
for r in rows:
    d = dict(r)
    print(d)

print("=== binary mtime ===")
print(subprocess.getoutput("ls -la /opt/runninghub2api/runninghub2api | awk '{print $5,$6,$7,$8,$9}'"))
print("=== journal resolution/megapixel hints (2h) ===")
print(subprocess.getoutput(
    "journalctl -u runninghub2api --since '2 hours ago' --no-pager "
    "| grep -Ei 'resolution|megapixel|480p|768p|1080p|SubmitQC|vid_' "
    "| tail -n 40 || true"
))
con.close()
'''


def main() -> None:
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(HOST, 22, "root", load_pass(), timeout=30, allow_agent=False, look_for_keys=False)
    _, o, e = c.exec_command("python3 -", timeout=60)
    o.channel.sendall(REMOTE.encode())
    o.channel.shutdown_write()
    sys.stdout.write(o.read().decode("utf-8", "ignore"))
    err = e.read().decode("utf-8", "ignore")
    if err:
        sys.stderr.write(err)
    c.close()


if __name__ == "__main__":
    main()

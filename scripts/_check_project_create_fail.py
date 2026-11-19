#!/usr/bin/env python3
"""Probe 82.158.89.24 for QC project/create UNKNOWN_ERROR failures."""
from __future__ import annotations

import os
import sys
from pathlib import Path

import paramiko

HOST = os.environ.get("RH2API_DEPLOY_HOST", "82.158.89.24")
PASS = os.environ.get("RH2API_DEPLOY_PASS", "")


def load_pass() -> str:
    if PASS:
        return PASS
    p = Path("服务器.txt")
    if not p.exists():
        raise SystemExit("missing RH2API_DEPLOY_PASS / 服务器.txt")
    lines = p.read_text(encoding="utf-8").splitlines()
    for i, line in enumerate(lines):
        if HOST in line:
            for j in range(i, min(i + 5, len(lines))):
                if "密码" in lines[j]:
                    return lines[j].split("：", 1)[-1].split(":", 1)[-1].strip()
    raise SystemExit(f"password for {HOST} not found")


REMOTE = r'''
import json, sqlite3, os, subprocess

print("=== service ===")
print(subprocess.getoutput("systemctl is-active runninghub2api"))
print(subprocess.getoutput("systemctl show runninghub2api -p ActiveEnterTimestamp -p NRestarts --value --no-pager").replace("\n"," | "))

print("=== health ===")
print(subprocess.getoutput("curl -sS http://127.0.0.1:8088/health"))

print("=== binary/mtime ===")
print(subprocess.getoutput("ls -la /opt/runninghub2api/runninghub2api /opt/runninghub2api/.env 2>/dev/null"))

print("=== env (masked) ===")
for line in open("/opt/runninghub2api/.env", encoding="utf-8", errors="ignore"):
    line=line.strip()
    if not line or line.startswith("#"):
        continue
    k, _, v = line.partition("=")
    kl=k.upper()
    if any(x in kl for x in ("PASS","KEY","TOKEN","SECRET","PWD")):
        print(f"{k}=***")
    elif kl.startswith("RH2API_") or "SUBMIT" in kl or "BASE" in kl or "MODE" in kl:
        print(line)

print("=== recent failed tasks ===")
con=sqlite3.connect("file:/opt/runninghub2api/data/rh2api.db?mode=ro", uri=True)
con.row_factory=sqlite3.Row
rows=con.execute("""
SELECT id,status,account_email,webapp_id,error_code,
       substr(IFNULL(error,''),1,220) AS err,
       attempts,created_at,updated_at,finished_at
FROM video_tasks
WHERE IFNULL(error,'') LIKE '%project/create%'
   OR IFNULL(error,'') LIKE '%UNKNOWN_ERROR%'
   OR status='failed'
ORDER BY COALESCE(finished_at, updated_at, created_at) DESC
LIMIT 25
""").fetchall()
for r in rows:
    print(dict(r))

print("=== fail code counts (24h-ish latest 200 fails) ===")
rows=con.execute("""
SELECT IFNULL(error_code,'') AS code, COUNT(*) AS n
FROM (
  SELECT error_code FROM video_tasks
  WHERE status='failed'
  ORDER BY COALESCE(finished_at, updated_at, created_at) DESC
  LIMIT 200
)
GROUP BY code ORDER BY n DESC
""").fetchall()
for r in rows:
    print(dict(r))

print("=== UNKNOWN/project create sample counts ===")
print("project_create", con.execute("SELECT COUNT(*) FROM video_tasks WHERE error LIKE '%project/create%'").fetchone()[0])
print("unknown_error", con.execute("SELECT COUNT(*) FROM video_tasks WHERE error LIKE '%UNKNOWN_ERROR%'").fetchone()[0])

print("=== recent accounts used on those fails ===")
rows=con.execute("""
SELECT account_id, account_email, COUNT(*) n,
       MAX(COALESCE(finished_at, updated_at)) last_at
FROM video_tasks
WHERE error LIKE '%project/create%' OR error LIKE '%UNKNOWN_ERROR%'
GROUP BY account_id, account_email
ORDER BY n DESC LIMIT 15
""").fetchall()
for r in rows:
    print(dict(r))

print("=== active account snapshot ===")
rows=con.execute("""
SELECT status, COUNT(*) n,
  SUM(CASE WHEN IFNULL(access_token,'')!='' THEN 1 ELSE 0 END) has_at
FROM accounts GROUP BY status
""").fetchall()
for r in rows:
    print(dict(r))

con.close()

print("=== journal project/create|UNKNOWN_ERROR (12h) ===")
print(subprocess.getoutput(
    "journalctl -u runninghub2api --since '12 hours ago' --no-pager "
    "| grep -Ei 'project/create|UNKNOWN_ERROR|qc-service|submit failed|pwdLogin|FREQUENT|服务器繁忙' "
    "| tail -n 60 || true"
))
'''


def main() -> None:
    password = load_pass()
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(HOST, 22, "root", password, timeout=30, allow_agent=False, look_for_keys=False)
    _, o, e = c.exec_command("python3 -", timeout=90)
    o.channel.sendall(REMOTE.encode())
    o.channel.shutdown_write()
    out = o.read().decode("utf-8", "ignore")
    err = e.read().decode("utf-8", "ignore")
    sys.stdout.write(out)
    if err:
        sys.stderr.write(err)
    c.close()


if __name__ == "__main__":
    main()

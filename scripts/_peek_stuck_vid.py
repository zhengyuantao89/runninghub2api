import os

import paramiko

HOST = os.environ.get("RH2API_DEPLOY_HOST", "82.158.89.24")
PASS = os.environ["RH2API_DEPLOY_PASS"]
NEEDLE = os.environ.get("RH2API_TASK_NEEDLE", "vid_c1bc44acb6f7")

remote_py = r'''
import sqlite3
con = sqlite3.connect("/opt/runninghub2api/data/rh2api.db")
con.row_factory = sqlite3.Row
needle = "NEEDLE_PLACEHOLDER"

rows = con.execute(
    "SELECT id,status,account_id,account_email,rh_task_id,rh_status,queue_index,queue_count,"
    "progress_text,error_code,error,attempts,created_at,updated_at,finished_at,webapp_id "
    "FROM video_tasks WHERE id LIKE ? OR id=? ORDER BY created_at DESC",
    (needle + "%", needle),
).fetchall()
print("exact/prefix matches", len(rows))
for r in rows:
    print(dict(r))

print("--- running/queued ---")
run = con.execute(
    "SELECT id,status,account_email,rh_task_id,rh_status,progress_text,error_code,"
    "substr(error,1,160) AS err,attempts,updated_at,finished_at "
    "FROM video_tasks WHERE status IN ('running','queued','submitting','pending') "
    "ORDER BY updated_at DESC LIMIT 40"
).fetchall()
for r in run:
    print(dict(r))

print("--- status counts ---")
for r in con.execute("SELECT status, COUNT(1) AS c FROM video_tasks GROUP BY status ORDER BY c DESC"):
    print(dict(r))
'''.replace("NEEDLE_PLACEHOLDER", NEEDLE.replace('"', ""))

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, 22, "root", PASS, timeout=30, allow_agent=False, look_for_keys=False)
sftp = c.open_sftp()
with sftp.file("/tmp/_peek_stuck_vid.py", "w") as f:
    f.write(remote_py)
sftp.close()
_, out, err = c.exec_command("python3 /tmp/_peek_stuck_vid.py", timeout=60)
print(out.read().decode("utf-8", "ignore"))
e = err.read().decode("utf-8", "ignore")
if e.strip():
    print("STDERR:", e)
c.close()

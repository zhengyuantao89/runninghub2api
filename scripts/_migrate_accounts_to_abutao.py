import json
import os
import sqlite3
import tempfile

import paramiko

HOST = os.environ.get("RH2API_DEPLOY_HOST", "82.158.89.24")
PASS = os.environ.get("RH2API_DEPLOY_PASS", "")
REMOTE_DB = "/opt/runninghub2api/data/rh2api.db"
TARGET_USER = "abutao"
SOURCE_USER = "admin"


def main() -> None:
    if not PASS:
        raise SystemExit("RH2API_DEPLOY_PASS required")

    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(HOST, 22, "root", PASS, timeout=30, allow_agent=False, look_for_keys=False)

    def run(cmd: str, check: bool = True) -> str:
        print(">>>", cmd)
        _, stdout, stderr = c.exec_command(cmd, timeout=120)
        out = stdout.read().decode("utf-8", "ignore")
        err = stderr.read().decode("utf-8", "ignore")
        code = stdout.channel.recv_exit_status()
        if out.strip():
            print(out.rstrip())
        if err.strip():
            print(err.rstrip())
        if check and code != 0:
            raise SystemExit(f"cmd failed ({code}): {cmd}")
        return out

    # Stop service briefly for consistent sqlite copy, then migrate remotely with python.
    run("systemctl stop runninghub2api")
    try:
        script = r'''
import json, sqlite3
db="/opt/runninghub2api/data/rh2api.db"
conn=sqlite3.connect(db)
conn.row_factory=sqlite3.Row
cur=conn.cursor()
cur.execute("PRAGMA wal_checkpoint(TRUNCATE)")

def user(name):
    row=cur.execute("SELECT id,username,role,status FROM users WHERE username=? COLLATE NOCASE",(name,)).fetchone()
    if not row:
        raise SystemExit(f"user not found: {name}")
    return dict(row)

src=user("admin")
dst=user("abutao")
print("SRC", json.dumps(src, ensure_ascii=False))
print("DST", json.dumps(dst, ensure_ascii=False))
if src["id"]==dst["id"]:
    raise SystemExit("source and target are the same user")

# also move orphan/legacy rows still on user_id=0 if they were pre-multiuser under admin pool
acc_before=cur.execute("SELECT COUNT(*) FROM accounts WHERE user_id IN (?,0)", (src["id"],)).fetchone()[0]
task_before=cur.execute("SELECT COUNT(*) FROM video_tasks WHERE user_id IN (?,0)", (src["id"],)).fetchone()[0]
dst_acc_before=cur.execute("SELECT COUNT(*) FROM accounts WHERE user_id=?", (dst["id"],)).fetchone()[0]
print(f"admin(+orphan0) accounts={acc_before} tasks={task_before}; abutao accounts_before={dst_acc_before}")

cur.execute("UPDATE accounts SET user_id=?, updated_at=datetime('now') WHERE user_id IN (?,0)", (dst["id"], src["id"]))
acc_moved=cur.rowcount
cur.execute("UPDATE video_tasks SET user_id=? WHERE user_id IN (?,0)", (dst["id"], src["id"]))
task_moved=cur.rowcount
conn.commit()

acc_after_src=cur.execute("SELECT COUNT(*) FROM accounts WHERE user_id=?", (src["id"],)).fetchone()[0]
acc_after_dst=cur.execute("SELECT COUNT(*) FROM accounts WHERE user_id=?", (dst["id"],)).fetchone()[0]
task_after_dst=cur.execute("SELECT COUNT(*) FROM video_tasks WHERE user_id=?", (dst["id"],)).fetchone()[0]
print(json.dumps({
  "accounts_moved": acc_moved,
  "tasks_moved": task_moved,
  "admin_accounts_left": acc_after_src,
  "abutao_accounts": acc_after_dst,
  "abutao_tasks": task_after_dst,
}, ensure_ascii=False))
conn.close()
'''
        # write remote script
        sftp = c.open_sftp()
        with sftp.file("/tmp/migrate_accounts_to_abutao.py", "w") as f:
            f.write(script)
        sftp.close()
        run("python3 /tmp/migrate_accounts_to_abutao.py")
        run("chown rh2api:rh2api /opt/runninghub2api/data/rh2api.db /opt/runninghub2api/data/rh2api.db-wal /opt/runninghub2api/data/rh2api.db-shm 2>/dev/null || true")
        run("rm -f /opt/runninghub2api/data/runninghub2api.db")
    finally:
        run("systemctl start runninghub2api")
        run("systemctl is-active runninghub2api")
        run("curl -sS http://127.0.0.1:8088/health")

    run(
        "python3 - <<'PY'\n"
        "import sqlite3,json\n"
        "conn=sqlite3.connect('file:/opt/runninghub2api/data/rh2api.db?mode=ro', uri=True)\n"
        "cur=conn.cursor()\n"
        "rows=cur.execute('SELECT u.username,u.id,COUNT(a.id) FROM users u LEFT JOIN accounts a ON a.user_id=u.id GROUP BY u.id ORDER BY u.id').fetchall()\n"
        "print(json.dumps(rows, ensure_ascii=False))\n"
        "conn.close()\n"
        "PY"
    )
    c.close()
    print("MIGRATE_OK")


if __name__ == "__main__":
    main()

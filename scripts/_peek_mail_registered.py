import os
import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(
    os.environ.get("RH2API_DEPLOY_HOST", "82.158.89.24"),
    22,
    "root",
    os.environ["RH2API_DEPLOY_PASS"],
    timeout=30,
    allow_agent=False,
    look_for_keys=False,
)
cmd = r"""
python3 - <<'PY'
import sqlite3, glob, os
paths = []
for root, dirs, files in os.walk('/opt/runninghub2api'):
    for f in files:
        if f.endswith('.db'):
            paths.append(os.path.join(root, f))
for p in paths:
    print('DB', p)
    con = sqlite3.connect(p)
    try:
        rows = con.execute(
            "SELECT id,email,status,substr(ifnull(last_error,''),1,120) FROM mail_inboxes "
            "WHERE email LIKE '%rustier%' OR ifnull(last_error,'') LIKE '%已注册%' "
            "ORDER BY id DESC LIMIT 30"
        ).fetchall()
        for r in rows:
            print(r)
        if not rows:
            print('(no matching rows)')
    except Exception as e:
        print('err', e)
    finally:
        con.close()
PY
"""
_, out, err = c.exec_command(cmd, timeout=60)
print(out.read().decode("utf-8", "ignore"))
print(err.read().decode("utf-8", "ignore"))

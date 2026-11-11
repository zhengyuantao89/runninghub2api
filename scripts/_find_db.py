import os
import paramiko

HOST = os.environ.get("RH2API_DEPLOY_HOST", "82.158.89.24")
PASS = os.environ.get("RH2API_DEPLOY_PASS", "")

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, 22, "root", PASS, timeout=30, allow_agent=False, look_for_keys=False)


def run(cmd: str) -> None:
    print(">>>", cmd)
    _, o, e = c.exec_command(cmd, timeout=60)
    print(o.read().decode("utf-8", "ignore") or e.read().decode("utf-8", "ignore"))


run("ls -la /opt/runninghub2api/")
run("ls -la /opt/runninghub2api/data/ || true")
run("grep -E 'DB|DATA|SQLITE|WEB' /opt/runninghub2api/.env || true")
run("systemctl cat runninghub2api | sed -n '1,80p'")
run("find /opt/runninghub2api -maxdepth 3 -type f \\( -name '*.db' -o -name '*.sqlite*' \\) -ls")
run("python3 - <<'PY'\nfrom pathlib import Path\nimport sqlite3\nroot=Path('/opt/runninghub2api')\nfor p in root.rglob('*'):\n  if not p.is_file():\n    continue\n  if p.suffix.lower() not in {'.db','.sqlite','.sqlite3'} and 'db' not in p.name.lower():\n    continue\n  try:\n    conn=sqlite3.connect(f'file:{p}?mode=ro', uri=True)\n    tables=[r[0] for r in conn.execute(\"SELECT name FROM sqlite_master WHERE type='table' ORDER BY 1\")]\n    conn.close()\n    print(p, 'tables=', tables[:30])\n  except Exception as e:\n    print(p, 'ERR', e)\nPY")
c.close()

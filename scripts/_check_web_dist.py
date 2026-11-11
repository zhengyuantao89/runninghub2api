import os
import paramiko

HOST = os.environ.get("RH2API_DEPLOY_HOST", "82.158.89.24")
PASS = os.environ.get("RH2API_DEPLOY_PASS", "")

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, 22, "root", PASS, timeout=30, allow_agent=False, look_for_keys=False)


def run(cmd: str) -> None:
    print(">>>", cmd)
    _, o, e = c.exec_command(cmd, timeout=40)
    out = o.read().decode("utf-8", "ignore")
    err = e.read().decode("utf-8", "ignore")
    print(out or err)


run("ls -la /opt/runninghub2api/web/dist/assets")
run("ls -la /opt/runninghub2api/web/dist/index.html")
run("python3 - <<'PY'\nfrom pathlib import Path\np=Path('/opt/runninghub2api/web/dist/assets')\nfor f in p.glob('*.js'):\n t=f.read_text(encoding='utf-8',errors='ignore')\n print(f.name, 'users' in t, '用户管理' in t, '/admin/users' in t, '管理控制台' in t, '试跑工作室' in t, '出口池' in t)\nPY")
run("curl -sS http://127.0.0.1:8088/ | head -c 500")
c.close()

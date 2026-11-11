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
    print(o.read().decode("utf-8", "ignore") or e.read().decode("utf-8", "ignore"))


run("systemctl is-active runninghub2api")
run("curl -sS http://127.0.0.1:8088/health")
run("ls /opt/runninghub2api/data/uploads 2>/dev/null | wc -l")
run("journalctl -u runninghub2api -n 40 --no-pager | grep -E 'upload cleaner|listening|Started' || true")
c.close()

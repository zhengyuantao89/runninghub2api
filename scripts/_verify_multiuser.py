import json
import os

import paramiko

HOST = os.environ.get("RH2API_DEPLOY_HOST", "82.158.89.24")
USER = os.environ.get("RH2API_DEPLOY_USER", "root")
PASS = os.environ.get("RH2API_DEPLOY_PASS", "")

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, 22, USER, PASS, timeout=30, allow_agent=False, look_for_keys=False)


def run(cmd: str) -> str:
    _, stdout, stderr = c.exec_command(cmd, timeout=45)
    out = stdout.read().decode("utf-8", "ignore")
    err = stderr.read().decode("utf-8", "ignore")
    code = stdout.channel.recv_exit_status()
    print(">>>", cmd)
    text = out if out else err
    print(text[:500])
    if code != 0 and not out:
        raise SystemExit(f"failed ({code}): {cmd}")
    return out


run("curl -sS http://127.0.0.1:8088/api/registration")
out = run(
    "curl -sS -X POST http://127.0.0.1:8088/api/login "
    "-H 'Content-Type: application/json' "
    "-d '{\"username\":\"admin\",\"password\":\"admin123\"}'"
)
data = json.loads(out)
token = data.get("token") or ""
print("login role=", data.get("role"), "token_len=", len(token))
if not token:
    raise SystemExit("login failed")

run(f"curl -sS http://127.0.0.1:8088/api/me -H 'Authorization: Bearer {token}'")
run(
    "curl -sS -X POST http://127.0.0.1:8088/v1/videos "
    f"-H 'Authorization: Bearer {token}' -F 'prompt=x'"
)
run(f"curl -sS http://127.0.0.1:8088/admin/tasks -H 'Authorization: Bearer {token}' | head -c 240")
run(f"curl -sS http://127.0.0.1:8088/admin/settings -H 'Authorization: Bearer {token}'")
c.close()
print("VERIFY_OK")

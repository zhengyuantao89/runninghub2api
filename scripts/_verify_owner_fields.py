import json
import os

import paramiko

HOST = os.environ.get("RH2API_DEPLOY_HOST", "82.158.89.24")
PASS = os.environ.get("RH2API_DEPLOY_PASS", "")

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, 22, "root", PASS, timeout=30, allow_agent=False, look_for_keys=False)


def run(cmd: str) -> str:
    _, stdout, stderr = c.exec_command(cmd, timeout=45)
    out = stdout.read().decode("utf-8", "ignore")
    err = stderr.read().decode("utf-8", "ignore")
    code = stdout.channel.recv_exit_status()
    print(">>>", cmd)
    print((out or err)[:800])
    if code != 0 and not out:
        raise SystemExit(f"failed ({code})")
    return out


login = run(
    "curl -sS -X POST http://127.0.0.1:8088/api/login "
    "-H 'Content-Type: application/json' "
    "-d '{\"username\":\"admin\",\"password\":\"admin123\"}'"
)
token = json.loads(login).get("token") or ""
if not token:
    raise SystemExit("admin login failed")

acc = run(f"curl -sS http://127.0.0.1:8088/admin/accounts -H 'Authorization: Bearer {token}'")
tasks = run(f"curl -sS http://127.0.0.1:8088/admin/tasks -H 'Authorization: Bearer {token}'")
acc_data = json.loads(acc).get("data") or []
task_data = json.loads(tasks).get("data") or []
print(
    "accounts_sample=",
    [
        {"email": x.get("email"), "owner": x.get("owner"), "user_id": x.get("user_id")}
        for x in acc_data[:3]
    ],
)
print(
    "tasks_sample=",
    [
        {"id": x.get("id"), "owner": x.get("owner"), "user_id": x.get("user_id")}
        for x in task_data[:3]
    ],
)
print("has_owner_accounts=", any(x.get("owner") for x in acc_data))
print("has_owner_tasks=", any(x.get("owner") for x in task_data))
print("js=", run("ls /opt/runninghub2api/web/dist/assets/*.js"))
c.close()
print("VERIFY_OK")

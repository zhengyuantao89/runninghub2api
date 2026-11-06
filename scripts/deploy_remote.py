#!/usr/bin/env python3
"""One-shot remote deploy for runninghub2api via systemd. Do not commit secrets.

Prepare stage first (Windows cross-compile example):
  set CGO_ENABLED=0& set GOOS=linux& set GOARCH=amd64
  go build -o deploy_stage\\runninghub2api .\\cmd\\runninghub2api
  cd web && npm run build && xcopy /E /I /Y dist ..\\deploy_stage\\web\\dist
"""

from __future__ import annotations

import os
import sys
from pathlib import Path

import paramiko

HOST = os.environ.get("RH2API_DEPLOY_HOST", "")
USER = os.environ.get("RH2API_DEPLOY_USER", "root")
PASS = os.environ.get("RH2API_DEPLOY_PASS", "")
PORT = int(os.environ.get("RH2API_DEPLOY_PORT", "22"))
_REPO_ROOT = Path(__file__).resolve().parents[1]
LOCAL = Path(os.environ.get("RH2API_DEPLOY_STAGE", str(_REPO_ROOT / "deploy_stage")))
REMOTE_TMP = "/tmp/rh2api_release"


def main() -> None:
    if not HOST:
        print("set RH2API_DEPLOY_HOST", file=sys.stderr)
        sys.exit(2)
    if not PASS:
        print("set RH2API_DEPLOY_PASS", file=sys.stderr)
        sys.exit(2)
    if not (LOCAL / "runninghub2api").exists():
        print(f"missing binary: {LOCAL / 'runninghub2api'}", file=sys.stderr)
        sys.exit(2)

    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(HOST, PORT, USER, PASS, timeout=30, allow_agent=False, look_for_keys=False)

    def run(cmd: str, timeout: int = 120) -> str:
        print(">>>", cmd)
        _stdin, stdout, stderr = c.exec_command(cmd, timeout=timeout)
        out = stdout.read().decode("utf-8", "ignore")
        err = stderr.read().decode("utf-8", "ignore")
        code = stdout.channel.recv_exit_status()
        if out:
            print(out.rstrip())
        if err:
            print(err.rstrip())
        if code != 0:
            raise SystemExit(f"cmd failed ({code}): {cmd}")
        return out

    run(f"rm -rf {REMOTE_TMP} && mkdir -p {REMOTE_TMP}/web {REMOTE_TMP}/deploy")

    sftp = c.open_sftp()

    def ensure_dir(remote: str) -> None:
        try:
            sftp.stat(remote)
        except FileNotFoundError:
            sftp.mkdir(remote)

    def put_dir(local: Path, remote: str) -> None:
        ensure_dir(remote)
        for root, _dirs, files in os.walk(local):
            rel = os.path.relpath(root, local).replace("\\", "/")
            rdir = remote if rel == "." else f"{remote}/{rel}"
            ensure_dir(rdir)
            for name in files:
                lp = Path(root) / name
                rp = f"{rdir}/{name}"
                print(f"upload {lp} -> {rp} ({lp.stat().st_size})")
                sftp.put(str(lp), rp)

    print("uploading binary...")
    sftp.put(str(LOCAL / "runninghub2api"), f"{REMOTE_TMP}/runninghub2api")
    sftp.put(str(LOCAL / ".env"), f"{REMOTE_TMP}/.env")
    sftp.put(str(LOCAL / "deploy" / "install.sh"), f"{REMOTE_TMP}/deploy/install.sh")
    sftp.put(
        str(LOCAL / "deploy" / "runninghub2api.service"),
        f"{REMOTE_TMP}/deploy/runninghub2api.service",
    )
    put_dir(LOCAL / "web" / "dist", f"{REMOTE_TMP}/web/dist")
    sftp.close()

    run(f"chmod +x {REMOTE_TMP}/runninghub2api {REMOTE_TMP}/deploy/install.sh")
    run(
        f'sed -i "s/\\r$//" {REMOTE_TMP}/deploy/install.sh '
        f"{REMOTE_TMP}/.env {REMOTE_TMP}/deploy/runninghub2api.service"
    )
    run(f"cd {REMOTE_TMP} && bash deploy/install.sh", timeout=180)

    ufw = run("command -v ufw >/dev/null && ufw status || echo NO_UFW")
    if "Status: active" in ufw:
        run("ufw allow 8088/tcp || true")

    run("systemctl is-active runninghub2api")
    run("curl -sS http://127.0.0.1:8088/health")
    api_key = os.environ.get("RH2API_API_KEY") or os.environ.get("RH2API_ADMIN_KEY") or ""
    if api_key:
        run(
            'curl -sS -o /tmp/rh2_accounts.json -w "%{http_code}\\n" '
            f'-H "Authorization: Bearer {api_key}" http://127.0.0.1:8088/admin/accounts'
        )
    print("DONE")
    c.close()


if __name__ == "__main__":
    main()

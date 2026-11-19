#!/usr/bin/env python3
from pathlib import Path
import os
import paramiko

HOST = "82.158.89.24"
REPO = Path(__file__).resolve().parents[1]
DIST = REPO / "web" / "dist"
REMOTE = "/opt/runninghub2api/web/dist"


def load_pass() -> str:
    lines = (REPO / "服务器.txt").read_text(encoding="utf-8").splitlines()
    for i, line in enumerate(lines):
        if HOST in line:
            for j in range(i, min(i + 5, len(lines))):
                if "密码" in lines[j]:
                    return lines[j].split("：", 1)[-1].split(":", 1)[-1].strip()
    raise SystemExit("no pass")


def main() -> None:
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(HOST, 22, "root", load_pass(), timeout=30, allow_agent=False, look_for_keys=False)
    _, o, e = c.exec_command(f"rm -rf {REMOTE} && mkdir -p {REMOTE}", timeout=30)
    o.channel.recv_exit_status()
    sftp = c.open_sftp()

    def ensure(remote: str) -> None:
        try:
            sftp.stat(remote)
        except FileNotFoundError:
            sftp.mkdir(remote)

    for root, _dirs, files in os.walk(DIST):
        rel = os.path.relpath(root, DIST).replace("\\", "/")
        rdir = REMOTE if rel == "." else f"{REMOTE}/{rel}"
        ensure(rdir)
        for name in files:
            lp = Path(root) / name
            rp = f"{rdir}/{name}"
            print("put", rp)
            sftp.put(str(lp), rp)
    sftp.close()
    _, o, e = c.exec_command("chown -R rh2api:rh2api /opt/runninghub2api/web/dist && curl -sS http://127.0.0.1:8088/ | head -c 200", timeout=30)
    print(o.read().decode("utf-8", "ignore"))
    print(e.read().decode("utf-8", "ignore"))
    c.close()
    print("DONE")


if __name__ == "__main__":
    main()

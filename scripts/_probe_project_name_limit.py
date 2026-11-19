#!/usr/bin/env python3
"""Binary-search RH QC project name length limit from deploy host."""
from __future__ import annotations

import os
import sys
from pathlib import Path

import paramiko

HOST = os.environ.get("RH2API_DEPLOY_HOST", "82.158.89.24")


def load_pass() -> str:
    p = os.environ.get("RH2API_DEPLOY_PASS", "")
    if p:
        return p
    lines = Path("服务器.txt").read_text(encoding="utf-8").splitlines()
    for i, line in enumerate(lines):
        if HOST in line:
            for j in range(i, min(i + 5, len(lines))):
                if "密码" in lines[j]:
                    return lines[j].split("：", 1)[-1].split(":", 1)[-1].strip()
    raise SystemExit("no pass")


REMOTE = r'''
import json, sqlite3, urllib.request, hashlib, uuid

BASE = open("/opt/runninghub2api/.env", encoding="utf-8", errors="ignore").read()
base_url = "https://www.runninghub.ai"
for line in BASE.splitlines():
    if line.startswith("RH2API_BASE_URL="):
        base_url = line.split("=",1)[1].strip().strip('"').strip("'")

con = sqlite3.connect("file:/opt/runninghub2api/data/rh2api.db?mode=ro", uri=True)
con.row_factory = sqlite3.Row
acc = con.execute("SELECT email,password,access_token,anonymous_id FROM accounts WHERE id=58").fetchone()
prompt = con.execute("SELECT prompt FROM video_tasks WHERE id='vid_4abb211cc261'").fetchone()["prompt"]
con.close()

anon = acc["anonymous_id"] or str(uuid.uuid4())
md5 = hashlib.md5(acc["password"].encode()).hexdigest()

def post(path, body, token=""):
    data=json.dumps(body).encode()
    req=urllib.request.Request(base_url+path, data=data, method="POST")
    req.add_header("Content-Type","application/json")
    req.add_header("Accept","application/json")
    req.add_header("client","WEB")
    req.add_header("x-rh-anonymous-id", anon)
    req.add_header("Origin", base_url)
    req.add_header("Referer", base_url+"/zh-cn")
    req.add_header("User-Agent","Mozilla/5.0")
    if token:
        req.add_header("Authorization","Bearer "+token)
    with urllib.request.urlopen(req, timeout=40) as resp:
        return json.loads(resp.read().decode())

login = post("/uc/abroad/pwdLogin", {"email": acc["email"], "password": md5})
token = (login.get("data") or {}).get("access_token") or (login.get("data") or {}).get("accessToken")
print("login", login.get("code"), bool(token))

def try_name(name):
    j = post("/qc-service/task/quick-creation/v2/canvas/project/create", {"name": name}, token=token)
    return j.get("code"), j.get("msg"), (j.get("data") or {}).get("projectId")

# binary search on char length using prompt prefix
lo, hi = 1, min(len(prompt), 400)
best = 0
while lo <= hi:
    mid = (lo+hi)//2
    name = prompt[:mid]
    code, msg, pid = try_name(name)
    ok = code == 0
    print(f"len={mid} code={code} msg={msg} pid={bool(pid)}")
    if ok:
        best = mid
        lo = mid+1
    else:
        hi = mid-1

print("max_ok_len", best)

# also test ascii-only and with newlines stripped
for label, name in [
    ("ascii64", "a"*64),
    ("ascii128", "a"*128),
    ("ascii200", "a"*200),
    ("ascii256", "a"*256),
    ("no_crlf_120", prompt.replace("\r","").replace("\n"," ")[:120]),
    ("no_crlf_200", prompt.replace("\r","").replace("\n"," ")[:200]),
]:
    code, msg, pid = try_name(name)
    print(label, "len", len(name), "code", code, "msg", msg)
'''


def main() -> None:
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(HOST, 22, "root", load_pass(), timeout=30, allow_agent=False, look_for_keys=False)
    _, o, e = c.exec_command("python3 -", timeout=180)
    o.channel.sendall(REMOTE.encode())
    o.channel.shutdown_write()
    sys.stdout.write(o.read().decode("utf-8", "ignore"))
    err = e.read().decode("utf-8", "ignore")
    if err:
        sys.stderr.write(err)
    c.close()


if __name__ == "__main__":
    main()

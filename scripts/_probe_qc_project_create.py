#!/usr/bin/env python3
"""Live-probe RH QC project/create from the deploy host using a recent fail account."""
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
import json, sqlite3, urllib.request, ssl, hashlib, uuid

BASE = open("/opt/runninghub2api/.env", encoding="utf-8", errors="ignore").read()
base_url = "https://www.runninghub.ai"
for line in BASE.splitlines():
    if line.startswith("RH2API_BASE_URL="):
        base_url = line.split("=",1)[1].strip().strip('"').strip("'")

con = sqlite3.connect("file:/opt/runninghub2api/data/rh2api.db?mode=ro", uri=True)
con.row_factory = sqlite3.Row

print("=== video_tasks columns ===")
cols = [r[1] for r in con.execute("PRAGMA table_info(video_tasks)").fetchall()]
print(cols)

print("=== failed task payloads ===")
sel = [c for c in [
    "id","webapp_id","prompt","duration","aspect_ratio","resolution","image_path",
    "nodes_json","node_info_json","extra","meta","account_id","account_email","error","created_at"
] if c in cols]
rows = con.execute(f"""
SELECT {", ".join(sel)}
FROM video_tasks
WHERE error LIKE '%project/create%'
ORDER BY created_at DESC LIMIT 4
""").fetchall()
for r in rows:
    d = dict(r)
    d["prompt_len"] = len(d.get("prompt") or "")
    d["prompt_head"] = (d.get("prompt") or "")[:120]
    d["has_image"] = bool(d.get("image_path"))
    print(d)

print("=== app settings ===")
try:
    for r in con.execute("SELECT key,value FROM app_settings").fetchall():
        print(dict(r))
except Exception as e:
    print("settings err", e)

email = rows[0]["account_email"] if rows else ""
acc = con.execute("""
SELECT id, email, password, access_token, anonymous_id, status
FROM accounts WHERE email=? LIMIT 1
""", (email,)).fetchone()
print("=== probe account ===", dict(acc) if acc else None)

def post(host, path, body, token="", anon=""):
    data = json.dumps(body).encode()
    req = urllib.request.Request(host + path, data=data, method="POST")
    req.add_header("Content-Type", "application/json")
    req.add_header("Accept", "application/json, text/plain, */*")
    req.add_header("client", "WEB")
    req.add_header("user-language", "zh_CN")
    req.add_header("x-rh-lang", "zh")
    req.add_header("Origin", host)
    req.add_header("Referer", host + "/zh-cn")
    req.add_header("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36")
    req.add_header("x-rh-anonymous-id", anon or str(uuid.uuid4()))
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=45) as resp:
            return resp.status, resp.read().decode("utf-8", "ignore")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf-8", "ignore")
    except Exception as e:
        return 0, repr(e)

anon = (acc["anonymous_id"] or "").strip() or str(uuid.uuid4())
token = (acc["access_token"] or "").strip()
pwd = (acc["password"] or "").strip()
md5 = hashlib.md5(pwd.encode()).hexdigest() if pwd else ""

print("=== pwdLogin", base_url, "===")
st, raw = post(base_url, "/uc/abroad/pwdLogin", {"email": acc["email"], "password": md5}, anon=anon)
print("http", st, raw[:500])
try:
    j = json.loads(raw)
    token2 = ((j.get("data") or {}).get("accessToken") or "")
    if token2:
        token = token2
        print("got fresh AT")
except Exception as e:
    print("login parse", e)

print("=== project/create short ===")
st, raw = post(base_url, "/qc-service/task/quick-creation/v2/canvas/project/create", {"name": "qc-probe"}, token=token, anon=anon)
print("http", st, raw[:600])

print("=== project/create prompt-like ===")
long_name = (rows[0]["prompt"] if rows else "probe") or "probe"
long_name = long_name[:500]
st, raw = post(base_url, "/qc-service/task/quick-creation/v2/canvas/project/create", {"name": long_name}, token=token, anon=anon)
print("http", st, "name_len", len(long_name), raw[:600])

print("=== project/create on runninghub.cn ===")
st, raw = post("https://www.runninghub.cn", "/qc-service/task/quick-creation/v2/canvas/project/create", {"name": "qc-probe-cn"}, token=token, anon=anon)
print("http", st, raw[:600])

print("=== egress sample ===")
try:
    for r in con.execute("SELECT id,status,host,fail_count,last_error FROM egress_nodes ORDER BY id DESC LIMIT 8").fetchall():
        print(dict(r))
except Exception as e:
    print("no egress table or err", e)

con.close()
'''


def main() -> None:
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(HOST, 22, "root", load_pass(), timeout=30, allow_agent=False, look_for_keys=False)
    _, o, e = c.exec_command("python3 -", timeout=120)
    o.channel.sendall(REMOTE.encode())
    o.channel.shutdown_write()
    sys.stdout.write(o.read().decode("utf-8", "ignore"))
    err = e.read().decode("utf-8", "ignore")
    if err:
        sys.stderr.write(err)
    c.close()


if __name__ == "__main__":
    main()

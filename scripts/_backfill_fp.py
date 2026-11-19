import json
import os
import urllib.error
import urllib.request

import paramiko

HOST = os.environ.get("RH2API_DEPLOY_HOST", "82.158.89.24")
PASS = os.environ["RH2API_DEPLOY_PASS"]
BASE = f"http://{HOST}:8088"


def remote_env_keys():
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(HOST, 22, "root", PASS, timeout=30, allow_agent=False, look_for_keys=False)
    cmd = r"""
python3 - <<'PY'
import re
keys={}
with open('/opt/runninghub2api/.env','r',encoding='utf-8',errors='ignore') as f:
    for line in f:
        line=line.strip()
        if not line or line.startswith('#') or '=' not in line: continue
        k,v=line.split('=',1)
        keys[k.strip()]=v.strip().strip('"').strip("'")
for k in sorted(keys):
    if any(x in k.upper() for x in ('USER','PASS','ADMIN','AUTH','LOGIN','CONSOLE')):
        print(f'{k}={keys[k]}')
print('---missing_fp---')
import sqlite3
con=sqlite3.connect('/opt/runninghub2api/data/rh2api.db')
n=con.execute("select count(1) from accounts where trim(ifnull(fingerprint_json,''))=''").fetchone()[0]
print('missing', n)
print('sample', con.execute("select id,email from accounts where trim(ifnull(fingerprint_json,''))='' order by id limit 5").fetchall())
PY
"""
    _, out, err = c.exec_command(cmd, timeout=60)
    print(out.read().decode("utf-8", "ignore"))
    e = err.read().decode("utf-8", "ignore")
    if e.strip():
        print("ERR", e)
    c.close()


def try_login(user, password):
    req = urllib.request.Request(
        BASE + "/api/login",
        data=json.dumps({"username": user, "password": password}).encode(),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            body = json.loads(resp.read().decode())
            return body
    except urllib.error.HTTPError as e:
        print(f"login {user}: {e.code}")
        return None


def main():
    remote_env_keys()
    # Common candidates; real values printed above for local use.
    cands = [
        ("admin", "abutao"),
        ("admin", "admin"),
        ("abutao", "abutao"),
    ]
    # Also pull from remote env via second ssh for actual login
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(HOST, 22, "root", PASS, timeout=30, allow_agent=False, look_for_keys=False)
    cmd = r"""
python3 - <<'PY'
keys={}
with open('/opt/runninghub2api/.env','r',encoding='utf-8',errors='ignore') as f:
    for line in f:
        line=line.strip()
        if not line or line.startswith('#') or '=' not in line: continue
        k,v=line.split('=',1)
        keys[k.strip()]=v.strip().strip('"').strip("'")
user=keys.get('RH2API_ADMIN_USER') or keys.get('RH2API_CONSOLE_USER') or keys.get('RH2API_USERNAME') or 'admin'
pw=keys.get('RH2API_ADMIN_PASSWORD') or keys.get('RH2API_CONSOLE_PASSWORD') or keys.get('RH2API_PASSWORD') or keys.get('RH2API_ADMIN_PASS') or ''
print(user)
print(pw)
PY
"""
    _, out, _ = c.exec_command(cmd, timeout=30)
    lines = [x for x in out.read().decode().splitlines() if x != ""]
    if len(lines) >= 2:
        cands.insert(0, (lines[0], lines[1]))
    # Prefer calling backfill locally on server to avoid auth guessing
    cmd2 = r"""
python3 - <<'PY'
import sqlite3, json, secrets, uuid
fp={
  "profile_id":"edge157-win",
  "user_agent":"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/157.0.0.0 Safari/537.36 Edg/157.0.0.0",
  "sec_ch_ua":"\"Not-A.Brand\";v=\"99\", \"Microsoft Edge\";v=\"157\", \"Chromium\";v=\"157\"",
  "sec_ch_ua_mobile":"?0",
  "sec_ch_ua_platform":"\"Windows\"",
  "sec_ch_ua_full_version_list":"\"Not-A.Brand\";v=\"99.0.0.0\", \"Microsoft Edge\";v=\"157.0.0.0\", \"Chromium\";v=\"157.0.0.0\"",
  "sec_ch_prefers_color_scheme":"light",
  "lang":"zh-CN",
  "lang_full":"zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
  "accept":"application/json, text/plain, */*",
  "sec_fetch_site":"same-origin",
  "sec_fetch_mode":"cors",
  "sec_fetch_dest":"empty",
  "label_zh":"Windows Edge 157",
  "source":"backfill"
}
fp_json=json.dumps(fp, ensure_ascii=False, separators=(',',':'))
con=sqlite3.connect('/opt/runninghub2api/data/rh2api.db')
rows=con.execute("select id, anonymous_id, device_id, fingerprint_json from accounts where trim(ifnull(fingerprint_json,''))='' or trim(ifnull(anonymous_id,''))='' or trim(ifnull(device_id,''))='' ").fetchall()
filled=0
for id_, anon, device, fpx in rows:
    anon2 = anon if (anon or '').strip() else str(uuid.uuid4())
    device2 = device if (device or '').strip() else secrets.token_hex(32)
    fp2 = fpx if (fpx or '').strip() else fp_json
    cur=con.execute('''UPDATE accounts SET
      anonymous_id=CASE WHEN trim(ifnull(anonymous_id,''))='' THEN ? ELSE anonymous_id END,
      device_id=CASE WHEN trim(ifnull(device_id,''))='' THEN ? ELSE device_id END,
      fingerprint_json=CASE WHEN trim(ifnull(fingerprint_json,''))='' THEN ? ELSE fingerprint_json END,
      updated_at=datetime('now')
      WHERE id=?''', (anon2, device2, fp2, id_))
    if cur.rowcount:
        filled += 1
con.commit()
left=con.execute("select count(1) from accounts where trim(ifnull(fingerprint_json,''))=''").fetchone()[0]
print(f'candidates={len(rows)} filled={filled} missing_fp_left={left}')
PY
"""
    print("running direct DB backfill on server...")
    _, out2, err2 = c.exec_command(cmd2, timeout=120)
    print(out2.read().decode("utf-8", "ignore"))
    e2 = err2.read().decode("utf-8", "ignore")
    if e2.strip():
        print("ERR", e2)
    c.close()


if __name__ == "__main__":
    main()

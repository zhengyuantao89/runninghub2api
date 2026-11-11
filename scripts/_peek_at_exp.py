import base64
import json
import os
import sqlite3
from datetime import datetime, timezone

import paramiko

HOST = os.environ.get("RH2API_DEPLOY_HOST", "82.158.89.24")
PASS = os.environ.get("RH2API_DEPLOY_PASS", "")

REMOTE = r'''
import base64, json, sqlite3
from datetime import datetime, timezone

def b64url(s):
    s += "=" * ((4 - len(s) % 4) % 4)
    return base64.urlsafe_b64decode(s.encode())

def jwt_payload(tok):
    parts = tok.split(".")
    if len(parts) < 2:
        return None
    try:
        return json.loads(b64url(parts[1]))
    except Exception as e:
        return {"_err": str(e)}

conn = sqlite3.connect("file:/opt/runninghub2api/data/rh2api.db?mode=ro", uri=True)
cur = conn.cursor()
rows = cur.execute("""
SELECT id, email, access_token, refresh_token
FROM accounts
WHERE IFNULL(access_token,'')!=''
ORDER BY last_submit_at DESC
LIMIT 5
""").fetchall()
now = int(datetime.now(timezone.utc).timestamp())
for id_, email, at, rt in rows:
    p = jwt_payload(at or "")
    exp = p.get("exp") if isinstance(p, dict) else None
    iat = p.get("iat") if isinstance(p, dict) else None
    print({
        "id": id_,
        "email": email,
        "at_len": len(at or ""),
        "has_rt": bool(rt),
        "jwt_keys": sorted([k for k in (p or {}).keys() if not str(k).startswith("_")])[:20] if isinstance(p, dict) else None,
        "iat": iat,
        "iat_utc": datetime.utcfromtimestamp(iat).isoformat()+"Z" if isinstance(iat,(int,float)) else None,
        "exp": exp,
        "exp_utc": datetime.utcfromtimestamp(exp).isoformat()+"Z" if isinstance(exp,(int,float)) else None,
        "ttl_sec": (exp - now) if isinstance(exp,(int,float)) else None,
        "expired": (exp < now) if isinstance(exp,(int,float)) else None,
    })
conn.close()
'''

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, 22, "root", PASS, timeout=30, allow_agent=False, look_for_keys=False)
_, o, e = c.exec_command("python3 -", timeout=60)
o.channel.sendall(REMOTE.encode())
o.channel.shutdown_write()
print(o.read().decode() or e.read().decode())
c.close()

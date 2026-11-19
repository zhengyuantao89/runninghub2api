import os

import paramiko

HOST = os.environ.get("RH2API_DEPLOY_HOST", "82.158.89.24")
PASS = os.environ["RH2API_DEPLOY_PASS"]

remote = r'''
import json, sqlite3, urllib.request

con = sqlite3.connect("/opt/runninghub2api/data/rh2api.db")
con.row_factory = sqlite3.Row

def post(url, body, at=None, api_key=None):
    headers = {
        "Content-Type": "application/json",
        "client": "WEB",
        "user-language": "zh_CN",
        "x-rh-lang": "zh",
        "Origin": "https://www.runninghub.ai",
        "Referer": "https://www.runninghub.ai/zh-cn",
        "User-Agent": "Mozilla/5.0",
        "Accept": "application/json, text/plain, */*",
    }
    if at:
        headers["Authorization"] = "Bearer " + at
    raw = json.dumps(body).encode()
    req = urllib.request.Request(url, data=raw, headers=headers, method="POST")
    with urllib.request.urlopen(req, timeout=40) as resp:
        return json.loads(resp.read().decode())

running = con.execute(
    "SELECT t.id,t.status,t.account_id,t.account_email,t.rh_task_id,t.rh_status,t.webapp_id,t.updated_at,"
    "a.access_token,a.api_key "
    "FROM video_tasks t LEFT JOIN accounts a ON a.id=t.account_id "
    "WHERE t.status='running' ORDER BY t.updated_at DESC"
).fetchall()
print("running_count", len(running))

for t in running:
    tid = t["id"]
    rh = (t["rh_task_id"] or "").strip()
    at = (t["access_token"] or "").strip()
    key = (t["api_key"] or "").strip()
    print("=" * 60)
    print(tid, "rh=", rh, "local_rh_status=", t["rh_status"], "email=", t["account_email"])

    if key and rh:
        try:
            st = post("https://www.runninghub.ai/task/openapi/status", {"apiKey": key, "taskId": rh})
            print("openapi_status", st.get("code"), st.get("msg"), "data=", st.get("data"))
        except Exception as e:
            print("openapi_status_err", e)
        try:
            out = post("https://www.runninghub.ai/task/openapi/outputs", {"apiKey": key, "taskId": rh})
            data = out.get("data")
            preview = str(data)[:300] if data is not None else None
            print("openapi_outputs", out.get("code"), out.get("msg"), "data=", preview)
        except Exception as e:
            print("openapi_outputs_err", e)

    if at and rh:
        try:
            hist = post(
                "https://www.runninghub.ai/api/user/output/history/v2",
                {"pageNum": 1, "pageSize": 20, "taskIds": [rh]},
                at=at,
            )
            print("history_v2", hist.get("code"), hist.get("msg"), "data_keys=", list((hist.get("data") or {}).keys()) if isinstance(hist.get("data"), dict) else type(hist.get("data")))
            print("history_snip", json.dumps(hist, ensure_ascii=False)[:500])
        except Exception as e:
            print("history_err", e)
        try:
            # try common task list body shapes
            for body in (
                {"page": 1, "pageSize": 50},
                {"pageNum": 1, "pageSize": 50},
                {"current": 1, "size": 50},
            ):
                lst = post("https://www.runninghub.ai/task/list", body, at=at)
                data = lst.get("data")
                items = []
                if isinstance(data, dict):
                    items = data.get("records") or data.get("list") or data.get("rows") or data.get("data") or []
                elif isinstance(data, list):
                    items = data
                hit = None
                for it in items if isinstance(items, list) else []:
                    sid = str(it.get("taskId") or it.get("id") or it.get("task_id") or "")
                    if sid == rh:
                        hit = {k: it.get(k) for k in list(it.keys())[:12]}
                        break
                print("task_list", body, "code", lst.get("code"), "n", len(items) if isinstance(items, list) else items, "hit", hit)
                if hit:
                    break
        except Exception as e:
            print("task_list_err", e)
'''

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, 22, "root", PASS, timeout=30, allow_agent=False, look_for_keys=False)
sftp = c.open_sftp()
with sftp.file("/tmp/_check_rh_task2.py", "w") as f:
    f.write(remote)
sftp.close()
_, out, err = c.exec_command("python3 /tmp/_check_rh_task2.py", timeout=180)
print(out.read().decode("utf-8", "ignore"))
e = err.read().decode("utf-8", "ignore")
if e.strip():
    print("STDERR:", e[:2000])
c.close()

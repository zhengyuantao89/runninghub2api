#!/usr/bin/env python3
"""Deploy binary to 82.158.89.24 and submit a long-prompt QC free video smoke test."""
from __future__ import annotations

import json
import os
import sys
import time
import uuid
from pathlib import Path

import paramiko

HOST = os.environ.get("RH2API_DEPLOY_HOST", "82.158.89.24")
REPO = Path(__file__).resolve().parents[1]
STAGE = REPO / "deploy_stage"
REMOTE_TMP = "/tmp/rh2api_release"


def load_pass() -> str:
    p = os.environ.get("RH2API_DEPLOY_PASS", "")
    if p:
        return p
    lines = (REPO / "服务器.txt").read_text(encoding="utf-8").splitlines()
    for i, line in enumerate(lines):
        if HOST in line:
            for j in range(i, min(i + 5, len(lines))):
                if "密码" in lines[j]:
                    return lines[j].split("：", 1)[-1].split(":", 1)[-1].strip()
    raise SystemExit("no password")


def main() -> None:
    password = load_pass()
    binary = STAGE / "runninghub2api"
    if not binary.exists():
        raise SystemExit(f"missing {binary}")

    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(HOST, 22, "root", password, timeout=30, allow_agent=False, look_for_keys=False)

    def run(cmd: str, timeout: int = 180) -> str:
        print(">>>", cmd[:200])
        _, o, e = c.exec_command(cmd, timeout=timeout)
        out = o.read().decode("utf-8", "ignore")
        err = e.read().decode("utf-8", "ignore")
        code = o.channel.recv_exit_status()
        if out:
            print(out.rstrip()[:3000])
        if err:
            print(err.rstrip()[:1000])
        if code != 0:
            raise SystemExit(f"cmd failed ({code}): {cmd}")
        return out

    print("=== upload release ===")
    run(f"rm -rf {REMOTE_TMP} && mkdir -p {REMOTE_TMP}/web {REMOTE_TMP}/deploy")
    sftp = c.open_sftp()
    sftp.put(str(binary), f"{REMOTE_TMP}/runninghub2api")
    sftp.put(str(STAGE / ".env"), f"{REMOTE_TMP}/.env")
    sftp.put(str(STAGE / "deploy" / "install.sh"), f"{REMOTE_TMP}/deploy/install.sh")
    sftp.put(str(STAGE / "deploy" / "runninghub2api.service"), f"{REMOTE_TMP}/deploy/runninghub2api.service")

    def ensure_dir(remote: str) -> None:
        try:
            sftp.stat(remote)
        except FileNotFoundError:
            sftp.mkdir(remote)

    dist = STAGE / "web" / "dist"
    if dist.exists():
        ensure_dir(f"{REMOTE_TMP}/web/dist")
        for root, _dirs, files in os.walk(dist):
            rel = os.path.relpath(root, dist).replace("\\", "/")
            rdir = f"{REMOTE_TMP}/web/dist" if rel == "." else f"{REMOTE_TMP}/web/dist/{rel}"
            ensure_dir(rdir)
            for name in files:
                sftp.put(str(Path(root) / name), f"{rdir}/{name}")
    sftp.close()

    run(f"chmod +x {REMOTE_TMP}/runninghub2api {REMOTE_TMP}/deploy/install.sh")
    run(
        f'sed -i "s/\\r$//" {REMOTE_TMP}/deploy/install.sh '
        f"{REMOTE_TMP}/.env {REMOTE_TMP}/deploy/runninghub2api.service"
    )
    run(f"cd {REMOTE_TMP} && bash deploy/install.sh", timeout=180)
    run("systemctl is-active runninghub2api")
    run("curl -sS http://127.0.0.1:8088/health")

    print("=== smoke test long prompt QC free i2v ===")
    remote_test = r'''
import json, os, sqlite3, urllib.request, urllib.error, time, base64, mimetypes

# load api key from env
key=""
for line in open("/opt/runninghub2api/.env", encoding="utf-8", errors="ignore"):
    line=line.strip()
    if line.startswith(("RH2API_API_KEY=","RH2API_ADMIN_KEY=","RH2API_AUTH_KEY=")):
        key=line.split("=",1)[1].strip().strip('"').strip("'")
        if key:
            break
if not key:
    raise SystemExit("no api key")

# reuse an existing uploaded first-frame if present; else create tiny png
img=None
for cand in [
    "/opt/runninghub2api/data/uploads/20261119_062621_40258383797-1.png",
    "/opt/runninghub2api/data/uploads/20261119_062616_40258383797-1.png",
]:
    if os.path.exists(cand):
        img=cand
        break
if not img:
    # 1x1 png
    raw=base64.b64decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")
    os.makedirs("/tmp", exist_ok=True)
    img="/tmp/rh2api_smoke.png"
    open(img,"wb").write(raw)

prompt = """【全局强制约束】
2K高清画质，严格沿用首帧参考图片的人物外貌、发型、服装款式、服装颜色、服装材质，全程不允许修改服装，服装的颜色、剪裁、面料、配饰全程和首帧图片保持完全一致。
全程不允许凭空新增任何物体、道具、装饰元素。
人物五官、身材比例全程保持不变。
背景环境、光线色调全程保持统一。
只改变人物身体姿态与肢体动作，人物位置在画面始终居中，镜头运动严格遵循下面描述。

保持首帧中的人物面部五官、发型、服装款式与颜色、背景场景、光线方向全程不变。

一段连续流畅的固定全身长镜头，人物居中正面窄距站立。她首先右手举至脸颊旁比出 V 字手势，左手自然下垂，重心微偏右腿，面带微笑直视镜头，保持稳定姿态。随后人物匀速放下右手，另一只手轻放至面部，身体小幅左右轻晃，动作过渡顺滑。接着人物保持站位，双手交叉轻搭小腹前，双腿交叉站立，头部微微歪斜，姿态柔和优雅。之后人物缓慢调整身体重心，校准体态，平稳过渡。随后人物重心切换至左腿，右腿屈膝、脚尖轻点地面，双脚保持窄距；左手轻贴身侧，右手抬起梳理耳边发丝，眼神温柔看向镜头。接着人物维持正面站立体态，右手抬至额头前做出舒展姿势，左手收于腰侧，站姿松弛随性。随后人物以自身为轴心向右侧顺时针中速转体约 90 度，停稳侧面站位，头部回转看向镜头，动作流畅连贯。最后人物逆时针中速转回正面，右手维持舒展姿势，左手贴身侧，身体轻微平缓起伏，面带自信微笑直视镜头，静止定格收尾。

全程固定全身取景，机位保持不变，无切镜、无剪辑、无镜头切换。

高质量，写实风格，光线稳定一致，动作连续流畅，面料质感清晰，人物结构自然，无闪烁、无抖动、无变形、无肢体错位、无画面突变。

Negative Prompt: AI 感，塑料感，卡通渲染，CG 画风，绘画感，肢体畸形，手指扭曲，面部失真，五官模糊，皮肤光滑假白，发丝结块，画面闪烁，帧不一致，背景漂移，画面抖动，过曝，死黑，水印文字，比例失调，解剖结构错误，动作僵硬卡顿，夸张不自然动作，廉价质感"""

boundary="----rh2smoke"+str(int(time.time()))
parts=[]
def add(name, value, filename=None, ctype=None):
    if filename is None:
        parts.append(f"--{boundary}\r\nContent-Disposition: form-data; name=\"{name}\"\r\n\r\n{value}\r\n".encode())
    else:
        data=open(filename,"rb").read()
        ctype=ctype or mimetypes.guess_type(filename)[0] or "application/octet-stream"
        parts.append(
            f"--{boundary}\r\nContent-Disposition: form-data; name=\"{name}\"; filename=\"{os.path.basename(filename)}\"\r\nContent-Type: {ctype}\r\n\r\n".encode()
            + data + b"\r\n"
        )
add("prompt", prompt)
add("webapp_id", "qc:2102602472381587457")
add("submit_mode", "qc_free")
add("aspect_ratio", "3:4 (Portrait Standard)")
add("duration", "5")
add("resolution", "768p")
add("image", None, filename=img, ctype="image/png")
body=b"".join(parts)+f"--{boundary}--\r\n".encode()

req=urllib.request.Request(
    "http://127.0.0.1:8088/v1/videos",
    data=body,
    method="POST",
    headers={
        "Authorization":"Bearer "+key,
        "Content-Type": f"multipart/form-data; boundary={boundary}",
    },
)
try:
    with urllib.request.urlopen(req, timeout=180) as resp:
        raw=resp.read().decode()
        print("submit_http", resp.status)
        print(raw[:2000])
        data=json.loads(raw)
except urllib.error.HTTPError as e:
    raw=e.read().decode("utf-8","ignore")
    print("submit_http", e.code)
    print(raw[:2000])
    data=json.loads(raw) if raw.startswith("{") else {"error": raw}

tid=data.get("id") or ""
print("task_id", tid, "status", data.get("status"), "error", data.get("error"))
if not tid:
    raise SystemExit("no task id")

# poll a bit
for i in range(20):
    time.sleep(3)
    req=urllib.request.Request(f"http://127.0.0.1:8088/v1/videos/{tid}", headers={"Authorization":"Bearer "+key})
    with urllib.request.urlopen(req, timeout=30) as resp:
        cur=json.loads(resp.read().decode())
    print("poll", i, cur.get("status"), cur.get("rh_task_id"), (cur.get("error") or "")[:180], (cur.get("progress_text") or "")[:80])
    if cur.get("status") in ("succeeded","failed","cancelled"):
        break

con=sqlite3.connect("file:/opt/runninghub2api/data/rh2api.db?mode=ro", uri=True)
con.row_factory=sqlite3.Row
row=con.execute("SELECT id,status,error_code,substr(IFNULL(error,''),1,240) err,rh_task_id,webapp_id,attempts FROM video_tasks WHERE id=?", (tid,)).fetchone()
print("db", dict(row) if row else None)
con.close()
'''
    _, o, e = c.exec_command("python3 -", timeout=300)
    o.channel.sendall(remote_test.encode())
    o.channel.shutdown_write()
    sys.stdout.write(o.read().decode("utf-8", "ignore"))
    err = e.read().decode("utf-8", "ignore")
    if err:
        sys.stderr.write(err)
    c.close()


if __name__ == "__main__":
    main()

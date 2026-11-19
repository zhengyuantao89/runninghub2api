#!/usr/bin/env python3
"""Probe output video dimensions for recent tasks."""
from __future__ import annotations

import sys
from pathlib import Path

import paramiko

HOST = "82.158.89.24"


def load_pass() -> str:
    lines = Path("服务器.txt").read_text(encoding="utf-8").splitlines()
    for i, line in enumerate(lines):
        if HOST in line:
            for j in range(i, min(i + 5, len(lines))):
                if "密码" in lines[j]:
                    return lines[j].split("：", 1)[-1].split(":", 1)[-1].strip()
    raise SystemExit("no pass")


REMOTE = r'''
import json, sqlite3, subprocess, urllib.request, os, tempfile

ids = [
  "vid_3e220ea8291f",
  "vid_de072dae6a99",
  "vid_c96790f0e88f",
  "vid_7320d3bd0f22",
]
con = sqlite3.connect("file:/opt/runninghub2api/data/rh2api.db?mode=ro", uri=True)
con.row_factory = sqlite3.Row
has_ff = subprocess.getoutput("command -v ffprobe || true").strip()
print("ffprobe", has_ff or "MISSING")

for tid in ids:
    row = con.execute(
        "SELECT id,resolution,aspect_ratio,duration,file_url,status FROM video_tasks WHERE id=?",
        (tid,),
    ).fetchone()
    d = dict(row)
    print("="*60)
    print(d)
    url = (d.get("file_url") or "").strip()
    if not url:
        continue
    if has_ff:
        cmd = [
            "ffprobe","-v","error","-select_streams","v:0",
            "-show_entries","stream=width,height,codec_name,bit_rate",
            "-of","json", url
        ]
        out = subprocess.getoutput(" ".join(cmd))
        try:
            j = json.loads(out)
            s = (j.get("streams") or [{}])[0]
            print("ffprobe", s)
        except Exception:
            print("ffprobe raw", out[:300])
    else:
        # fallback: download head and use mediainfo/python? just print mapping expectation
        print("expected megapixels for", d["resolution"], end=": ")
        m = {
          "480p": 0.135164359862,
          "768p": 0.346020761246,
          "1080p": 0.684269571799,
        }.get(d["resolution"])
        print(m)

# Also show what 3:4 * megapixels implies roughly
import math
for label, mp in [("480p",0.135164359862),("768p",0.346020761246),("1080p",0.684269571799)]:
    # aspect 3:4 => w/h = 3/4, w*h = mp*1e6
    # w = sqrt(mp*1e6 * 3/4), h = w*4/3
    area = mp * 1_000_000
    w = math.sqrt(area * 3/4)
    h = w * 4/3
    print(f"approx 3:4 {label}: {round(w)}x{round(h)}")
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

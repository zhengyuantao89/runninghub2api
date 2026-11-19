#!/usr/bin/env python3
import json, re
from pathlib import Path
text = Path(r"F:\Trae_projects\runninghub2api\参考\runninghub11.19.devtools").read_text(encoding="utf-8")
# find resolution options near free video model
for m in re.finditer(r'"resolution"[^\]\{]{0,200}', text):
    s = m.group(0)
    if "480" in s or "720" in s or "1080" in s or "option" in s.lower() or "0.34" in s:
        print(s[:220])
        print("---")
# look in models response for sku 2102602472381587457
har = json.loads(text) if text.startswith("{") else None
if har:
    for e in har["log"]["entries"]:
        url = e["request"]["url"]
        if "/qc/v2/models" not in url and "binding" not in url:
            continue
        resp = (e["response"].get("content") or {}).get("text") or ""
        if "2102602472381587457" not in resp:
            continue
        print("FOUND models/binding url", url)
        # extract snippet around sku
        i = resp.find("2102602472381587457")
        print(resp[max(0,i-500):i+800])
        print("="*40)

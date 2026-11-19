#!/usr/bin/env python3
import json
from pathlib import Path

har = json.loads(Path(r"F:\Trae_projects\runninghub2api\参考\runninghub11.19.devtools").read_text(encoding="utf-8"))
sku = "2102602472381587457"
for e in har["log"]["entries"]:
    resp = (e["response"].get("content") or {}).get("text") or ""
    if sku not in resp or "fields" not in resp:
        continue
    try:
        data = json.loads(resp)
    except Exception:
        continue
    # walk for the model
    def walk(o):
        if isinstance(o, dict):
            if str(o.get("skuId")) == sku:
                yield o
            for v in o.values():
                yield from walk(v)
        elif isinstance(o, list):
            for it in o:
                yield from walk(it)
    for model in walk(data):
        print("model", model.get("name"), model.get("bindingId"), model.get("skuId"))
        for f in model.get("fields") or []:
            key = f.get("fieldKey") or f.get("mappedApiParamKey")
            if key in ("prompt", "aspectRatio", "duration", "resolution", "audioMode", "firstFrameUrl", "creationMode"):
                print("-", key, "type=", f.get("fieldType"), "default=", f.get("defaultValue"))
                opts = f.get("options") or f.get("enumOptions") or f.get("selectOptions")
                if opts:
                    print("  options=", json.dumps(opts, ensure_ascii=False)[:500])
                extra = f.get("skuInputExtraJson") or f.get("extraJson")
                if extra:
                    print("  extra=", str(extra)[:300])
        print("=" * 40)

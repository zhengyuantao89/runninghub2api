#!/usr/bin/env python3
import json
from pathlib import Path

har = json.loads(Path(r"F:\Trae_projects\runninghub2api\参考\runninghub11.19.devtools").read_text(encoding="utf-8"))
# free family skus from our constants / HAR
target_skus = {
    "2102602472381587457",  # i2v freeCreation
    "2102602278114009090",  # t2v freeCreation
    "2102602670923161601",  # multimodal
}
# also match by name
want_names = ("Minimax H3 RH Enhanced", "Minimax H3-RH")

found = []
for e in har["log"]["entries"]:
    url = e["request"]["url"]
    resp = (e["response"].get("content") or {}).get("text") or ""
    if "fields" not in resp:
        continue
    if not any(s in resp for s in target_skus) and "Minimax H3" not in resp:
        continue
    try:
        data = json.loads(resp)
    except Exception:
        continue

    def walk(o):
        if isinstance(o, dict):
            sku = str(o.get("skuId") or "")
            name = str(o.get("skuNameEn") or o.get("name") or "")
            if sku in target_skus or any(w in name for w in want_names):
                if o.get("fields"):
                    yield o
            for v in o.values():
                yield from walk(v)
        elif isinstance(o, list):
            for it in o:
                yield from walk(it)

    for model in walk(data):
        key = (model.get("skuId"), model.get("bindingId"), model.get("name"))
        if key in {(f.get("skuId"), f.get("bindingId"), f.get("name")) for f in found}:
            continue
        found.append(model)

print(f"found_models={len(found)}")
for model in found:
    print("=" * 72)
    print("name:", model.get("name") or model.get("skuNameEn"))
    print("skuId:", model.get("skuId"))
    print("bindingId:", model.get("bindingId"))
    print("groupName:", model.get("groupName"))
    for f in model.get("fields") or []:
        key = f.get("fieldKey") or f.get("mappedApiParamKey")
        if key not in ("aspectRatio", "ratio", "resolution", "duration", "audioMode", "creationMode", "prompt", "firstFrameUrl"):
            continue
        print(f"\n[{key}] type={f.get('fieldType')} default={f.get('defaultValue')!r}")
        opts = f.get("options") or []
        if opts:
            print("  options:")
            for o in opts:
                print("   -", json.dumps(o, ensure_ascii=False))
        extra = f.get("skuInputExtraJson")
        if extra:
            try:
                ej = json.loads(extra) if isinstance(extra, str) else extra
            except Exception:
                ej = {"raw": str(extra)[:200]}
            # print relevant bits
            keep = {k: ej.get(k) for k in ("paramDesc", "paramDescEn", "min", "max", "step", "maxLength", "fieldName", "nodeName") if k in ej}
            if keep:
                print("  extra:", keep)

print("\n=== actual submit aspectRatio in prepare/commit ===")
for e in har["log"]["entries"]:
    url = e["request"]["url"]
    if not any(k in url for k in ("prepare", "commit", "fee-preview")):
        continue
    post = (e["request"].get("postData") or {}).get("text") or ""
    if not post:
        continue
    try:
        body = json.loads(post)
    except Exception:
        continue
    params = body.get("params") or (body.get("createRequest") or {}).get("params") or {}
    if not isinstance(params, dict):
        continue
    if "aspectRatio" in params or "ratio" in params:
        print(url.split(".ai", 1)[-1][:60], "aspectRatio=", params.get("aspectRatio"), "ratio=", params.get("ratio"), "resolution=", params.get("resolution"), "sku hint in body skuId=", body.get("skuId") or (body.get("createRequest") or {}).get("skuId"))

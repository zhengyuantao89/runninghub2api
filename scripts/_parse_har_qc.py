#!/usr/bin/env python3
import json
from pathlib import Path

p = Path(r"F:\Trae_projects\runninghub2api\参考\runninghub11.19.devtools")
har = json.loads(p.read_text(encoding="utf-8"))
keys = (
    "project/create",
    "page/list",
    "fee-preview",
    "prepare",
    "commit",
    "batchStatus",
)
for e in har["log"]["entries"]:
    url = e["request"]["url"]
    if not any(k in url for k in keys):
        continue
    post = (e["request"].get("postData") or {}).get("text") or ""
    resp = (e["response"].get("content") or {}).get("text") or ""
    print("=" * 72)
    print(e["request"]["method"], url.split("runninghub.ai", 1)[-1])
    print("http", e["response"]["status"], "bodySize", e["request"].get("bodySize"))
    if post:
        body = json.loads(post)
        if "name" in body:
            name = body.get("name") or ""
            print("NAME len=", len(name), "repr=", repr(name))
            print("NAME bytes=", len(name.encode("utf-8")))
        if isinstance(body.get("params"), dict):
            pr = body["params"].get("prompt") or ""
            print("params.prompt_len=", len(pr))
            print("params.keys=", sorted(body["params"].keys()))
            for k in ("aspectRatio", "duration", "resolution", "creationMode", "firstFrameUrl"):
                if k in body["params"]:
                    v = body["params"][k]
                    print(f"  {k}=", str(v)[:120])
        if isinstance(body.get("createRequest"), dict):
            cr = body["createRequest"]
            params = cr.get("params") or {}
            pr = params.get("prompt") or ""
            print("createRequest.prompt_len=", len(pr))
            print("createRequest sku/binding=", cr.get("skuId"), cr.get("bindingId"))
            print("params.keys=", sorted(params.keys()))
            for k in ("aspectRatio", "duration", "resolution", "creationMode", "firstFrameUrl"):
                if k in params:
                    print(f"  {k}=", str(params[k])[:120])
            print("NAME in createRequest?", "name" in cr)
    if resp:
        try:
            rj = json.loads(resp)
        except Exception:
            print("resp non-json", resp[:200])
            continue
        print("resp code/msg=", rj.get("code"), rj.get("msg"))
        data = rj.get("data")
        if isinstance(data, dict):
            bits = {k: data.get(k) for k in ("projectId", "pageId", "prepareToken", "taskId", "name") if k in data}
            if bits:
                print("resp data", bits)
            if isinstance(data.get("feePreview"), dict):
                fp = data["feePreview"]
                print("fee", {k: fp.get(k) for k in ("free", "welfareFree", "passed", "freeQuotaRemaining", "skuId")})

from pathlib import Path
import paramiko

HOST = "82.158.89.24"
lines = Path("服务器.txt").read_text(encoding="utf-8").splitlines()
pw = ""
for i, line in enumerate(lines):
    if HOST in line:
        for j in range(i, min(i + 5, len(lines))):
            if "密码" in lines[j]:
                pw = lines[j].split("：", 1)[-1].split(":", 1)[-1].strip()
                break
c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, 22, "root", pw, timeout=30, allow_agent=False, look_for_keys=False)
checks = [
    "21:9 (Ultrawide)",
    "resolution=1080p",
    "minimax-h3-rh-enhanced-multimodal",
    "audioMode=native",
    "first_frame.png",
]
cmd = "python3 - <<'PY'\nfrom pathlib import Path\ntext=''.join(p.read_text(encoding='utf-8',errors='ignore') for p in Path('/opt/runninghub2api/web/dist/assets').glob('*.js'))\nfor s in %r:\n    print(s, 'YES' if s in text else 'NO')\nprint('bundle', [p.name for p in Path('/opt/runninghub2api/web/dist/assets').glob('*.js')])\nPY" % checks
_, o, e = c.exec_command(cmd, timeout=30)
print(o.read().decode("utf-8", "ignore"))
print(e.read().decode("utf-8", "ignore"))
c.close()

# No process the program can see holds AWS credentials (ADR-0003). Lambda refuses to
# make the runner non-dumpable, so the image entrypoint drops them before the runtime
# starts. run.sh gives the runner containers fake credentials, like Lambda does.
import os

NAMES = {b"AWS_ACCESS_KEY_ID", b"AWS_SECRET_ACCESS_KEY", b"AWS_SESSION_TOKEN"}
found = set()
for pid in filter(str.isdigit, os.listdir("/proc")):
    try:
        with open(f"/proc/{pid}/environ", "rb") as f:
            entries = f.read().split(b"\0")
    except OSError:
        continue
    found |= {name.decode() for name in (e.split(b"=", 1)[0] for e in entries) if name in NAMES}
print("credentials:", " ".join(sorted(found)) or "none")

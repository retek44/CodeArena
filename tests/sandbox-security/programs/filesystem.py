# The root file system is read only; only the working directory and /tmp are writable.
# Writes there carry the marker, so the probe can check that the runner removed them.
import os

MARKER = "codearena-sandbox-marker"


def write(path: str) -> str:
    try:
        with open(path, "a", encoding="utf-8") as f:
            f.write(MARKER)
        return "written"
    except OSError:
        return "denied"


for path in ("/" + MARKER, "/etc/" + MARKER, "/var/task/" + MARKER, "/opt/" + MARKER):
    print(path.rsplit("/", 1)[0] or "/", write(path))
print("cwd", write(os.path.join(os.getcwd(), MARKER)))
print("tmp", write("/tmp/" + MARKER))

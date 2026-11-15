# Runs after every scenario: no process or file with the marker may be left, and the
# runner still judges a normal program (prints the sum of the input).
import os

MARKER = "codearena-sandbox-marker"
processes = 0
for pid in os.listdir("/proc"):
    if not pid.isdigit():
        continue
    try:
        with open(f"/proc/{pid}/cmdline", "rb") as f:
            processes += MARKER.encode() in f.read()
    except OSError:
        pass
files = 0
for root in ("/tmp", "/dev/shm"):
    for _, dirs, names in os.walk(root):
        files += sum(MARKER in n for n in dirs + names)
a, b = map(int, input().split())
print(f"processes: {processes}")
print(f"files: {files}")
print(a + b)

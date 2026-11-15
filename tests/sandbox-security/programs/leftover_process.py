# Leaves a background process behind in a new session and exits at once. The runner
# must remove it before the next run (the probe looks for the marker).
import os
import sys

MARKER = "codearena-sandbox-marker"
if os.fork() == 0:
    os.setsid()
    os.execv(sys.executable, [sys.executable, "-c", "import time; time.sleep(30)", MARKER])
print("started")

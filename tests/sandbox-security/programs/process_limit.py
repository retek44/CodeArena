# Starts many short-lived child processes: the process limit must refuse some of them.
# Each child only sleeps and carries the marker in its command line, so the probe can
# check later that the runner removed every one of them.
import os
import sys

MARKER = "codearena-sandbox-marker"
started = refused = 0
for _ in range(64):
    try:
        pid = os.fork()
    except OSError:
        refused += 1
        continue
    if pid == 0:
        try:
            os.execv(sys.executable, [sys.executable, "-c", "import time; time.sleep(30)", MARKER])
        finally:
            os._exit(0)
    started += 1
print("limited" if refused else f"not limited: {started} children started")

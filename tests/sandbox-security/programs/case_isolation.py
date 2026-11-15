# Two test cases of one submission: the first writes its input (a token) into the
# working directory and /tmp; the second must find no trace of it.
import os
import sys

command, token = sys.stdin.read().split()
if command == "write":
    for path in (os.path.join(os.getcwd(), "token.txt"), "/tmp/token.txt"):
        with open(path, "w", encoding="utf-8") as f:
            f.write(token)
    print("written")
else:
    own_input = os.path.realpath("/proc/self/fd/0")
    found = 0
    for root in ("/tmp", "/dev/shm"):
        for dirpath, _, files in os.walk(root):
            for name in files:
                if os.path.join(dirpath, name) == own_input:
                    continue
                try:
                    with open(os.path.join(dirpath, name), encoding="utf-8", errors="ignore") as f:
                        found += token in f.read()
                except OSError:
                    pass
    print(f"found: {found}")

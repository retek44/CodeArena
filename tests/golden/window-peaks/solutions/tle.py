# Takes the maximum of every window separately: correct, but n·k steps.
import sys

data = sys.stdin.buffer.read().split()
n, k = int(data[0]), int(data[1])
values = list(map(int, data[2 : 2 + n]))
print(" ".join(str(max(values[i : i + k])) for i in range(n - k + 1)))

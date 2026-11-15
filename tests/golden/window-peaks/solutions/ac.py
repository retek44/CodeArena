import sys
from collections import deque

data = sys.stdin.buffer.read().split()
n, k = int(data[0]), int(data[1])
values = list(map(int, data[2 : 2 + n]))
window = deque()
peaks = []
for i, x in enumerate(values):
    while window and values[window[-1]] <= x:
        window.pop()
    window.append(i)
    if window[0] <= i - k:
        window.popleft()
    if i >= k - 1:
        peaks.append(values[window[0]])
print(" ".join(map(str, peaks)))

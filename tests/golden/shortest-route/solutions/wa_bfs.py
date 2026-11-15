# Takes the route with the fewest roads, not the fastest one.
import sys
from collections import deque

data = sys.stdin.buffer.read().split()
n, m = int(data[0]), int(data[1])
roads = [[] for _ in range(n + 1)]
for k in range(m):
    a, b, w = int(data[2 + 3 * k]), int(data[3 + 3 * k]), int(data[4 + 3 * k])
    roads[a].append((b, w))
dist = [None] * (n + 1)
dist[1] = 0
queue = deque([1])
while queue:
    town = queue.popleft()
    for nxt, w in roads[town]:
        if dist[nxt] is None:
            dist[nxt] = dist[town] + w
            queue.append(nxt)
print(dist[n] if dist[n] is not None else -1)

import heapq
import sys

data = sys.stdin.buffer.read().split()
n, m = int(data[0]), int(data[1])
roads = [[] for _ in range(n + 1)]
for k in range(m):
    a, b, w = int(data[2 + 3 * k]), int(data[3 + 3 * k]), int(data[4 + 3 * k])
    roads[a].append((b, w))
INF = float("inf")
dist = [INF] * (n + 1)
dist[1] = 0
heap = [(0, 1)]
while heap:
    d, town = heapq.heappop(heap)
    if d > dist[town]:
        continue
    if town == n:
        break
    for nxt, w in roads[town]:
        if d + w < dist[nxt]:
            dist[nxt] = d + w
            heapq.heappush(heap, (d + w, nxt))
print(dist[n] if dist[n] != INF else -1)

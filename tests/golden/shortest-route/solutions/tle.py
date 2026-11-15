# Bellman-Ford without an early stop: correct, but n·m steps.
import sys

data = sys.stdin.buffer.read().split()
n, m = int(data[0]), int(data[1])
edges = [(int(data[2 + 3 * k]), int(data[3 + 3 * k]), int(data[4 + 3 * k])) for k in range(m)]
INF = float("inf")
dist = [INF] * (n + 1)
dist[1] = 0
for _ in range(n - 1):
    for a, b, w in edges:
        if dist[a] + w < dist[b]:
            dist[b] = dist[a] + w
print(dist[n] if dist[n] != INF else -1)

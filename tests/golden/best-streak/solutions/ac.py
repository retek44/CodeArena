n = int(input())
best = None
current = 0
for x in map(int, input().split()):
    current = x if best is None else max(x, current + x)
    best = current if best is None else max(best, current)
print(best)

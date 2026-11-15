n = int(input())
low = None
best = 0
for p in map(int, input().split()):
    if low is None or p < low:
        low = p
    elif p - low > best:
        best = p - low
print(best)

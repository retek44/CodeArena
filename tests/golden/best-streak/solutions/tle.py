# Tries every run: correct, but quadratic.
n = int(input())
values = list(map(int, input().split()))
best = values[0]
for i in range(n):
    total = 0
    for j in range(i, n):
        total += values[j]
        if total > best:
            best = total
print(best)

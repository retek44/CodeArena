# Tries every pair of days: correct, but quadratic.
n = int(input())
prices = list(map(int, input().split()))
best = 0
for i in range(n):
    for j in range(i + 1, n):
        if prices[j] - prices[i] > best:
            best = prices[j] - prices[i]
print(best)

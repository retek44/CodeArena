k, a = map(int, input().split())
coins = list(map(int, input().split()))
INF = a + 1
best = [0] + [INF] * a
for coin in coins:
    for v in range(coin, a + 1):
        if best[v - coin] + 1 < best[v]:
            best[v] = best[v - coin] + 1
print(best[a] if best[a] <= a else -1)

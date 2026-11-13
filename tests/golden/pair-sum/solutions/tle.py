# Correct but quadratic: too slow for n = 50000.
n, t = map(int, input().split())
values = list(map(int, input().split()))
for i in range(n):
    for j in range(i + 1, n):
        if values[i] + values[j] == t:
            print(i + 1, j + 1)
            raise SystemExit

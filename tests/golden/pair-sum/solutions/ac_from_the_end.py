# Searches from the end: often a different pair than the reference answer.
n, t = map(int, input().split())
values = list(map(int, input().split()))
last = {}
for i in range(n - 1, -1, -1):
    x = values[i]
    if t - x in last:
        print(last[t - x] + 1, i + 1)
        break
    last.setdefault(x, i)

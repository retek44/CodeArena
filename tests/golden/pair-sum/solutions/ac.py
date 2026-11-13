n, t = map(int, input().split())
values = list(map(int, input().split()))
first = {}
for j, x in enumerate(values):
    if t - x in first:
        print(first[t - x] + 1, j + 1)
        break
    first.setdefault(x, j)

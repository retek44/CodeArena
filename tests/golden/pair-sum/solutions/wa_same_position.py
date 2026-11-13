# Bug: allows i == j, so 4 + 4 = 8 is "found" with only one 4 used.
n, t = map(int, input().split())
values = list(map(int, input().split()))
position = {x: i for i, x in enumerate(values)}
for i, x in enumerate(values):
    if t - x in position:
        print(i + 1, position[t - x] + 1)
        break

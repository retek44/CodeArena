# For every wall, scans both sides for the highest walls: correct, but quadratic.
n = int(input())
h = list(map(int, input().split()))
water = 0
for i in range(n):
    water += min(max(h[: i + 1]), max(h[i:])) - h[i]
print(water)

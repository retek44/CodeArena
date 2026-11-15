# Walks every path one by one: correct, but the number of paths grows exponentially.
import sys

MOD = 1_000_000_007
data = sys.stdin.read().split()
r, c = int(data[0]), int(data[1])
grid = data[2:2 + r]
count = 0
stack = [(0, 0)] if grid[0][0] == "." else []
while stack:
    i, j = stack.pop()
    if i == r - 1 and j == c - 1:
        count += 1
        continue
    if i + 1 < r and grid[i + 1][j] == ".":
        stack.append((i + 1, j))
    if j + 1 < c and grid[i][j + 1] == ".":
        stack.append((i, j + 1))
print(count % MOD)

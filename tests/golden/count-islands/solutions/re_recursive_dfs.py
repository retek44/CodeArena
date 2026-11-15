# A correct flood fill, but recursive: a large island exceeds Python's recursion limit.
import sys

data = sys.stdin.read().split()
r, c = int(data[0]), int(data[1])
grid = data[2 : 2 + r]
seen = [[False] * c for _ in range(r)]


def sink(y, x):
    seen[y][x] = True
    for ny, nx in ((y + 1, x), (y - 1, x), (y, x + 1), (y, x - 1)):
        if 0 <= ny < r and 0 <= nx < c and grid[ny][nx] == "#" and not seen[ny][nx]:
            sink(ny, nx)


islands = 0
for i in range(r):
    for j in range(c):
        if grid[i][j] == "#" and not seen[i][j]:
            islands += 1
            sink(i, j)
print(islands)

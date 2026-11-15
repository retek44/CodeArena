# Joins land cells that only touch at a corner.
import sys

data = sys.stdin.read().split()
r, c = int(data[0]), int(data[1])
grid = data[2 : 2 + r]
seen = bytearray(r * c)
islands = 0
STEPS = [(dy, dx) for dy in (-1, 0, 1) for dx in (-1, 0, 1) if dy or dx]
for i in range(r):
    for j in range(c):
        if grid[i][j] != "#" or seen[i * c + j]:
            continue
        islands += 1
        seen[i * c + j] = 1
        stack = [(i, j)]
        while stack:
            y, x = stack.pop()
            for dy, dx in STEPS:
                ny, nx = y + dy, x + dx
                if 0 <= ny < r and 0 <= nx < c and grid[ny][nx] == "#" and not seen[ny * c + nx]:
                    seen[ny * c + nx] = 1
                    stack.append((ny, nx))
print(islands)

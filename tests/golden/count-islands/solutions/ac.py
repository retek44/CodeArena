import sys

data = sys.stdin.read().split()
r, c = int(data[0]), int(data[1])
grid = data[2 : 2 + r]
seen = bytearray(r * c)
islands = 0
for i in range(r):
    row = grid[i]
    for j in range(c):
        if row[j] != "#" or seen[i * c + j]:
            continue
        islands += 1
        seen[i * c + j] = 1
        stack = [(i, j)]
        while stack:
            y, x = stack.pop()
            for ny, nx in ((y + 1, x), (y - 1, x), (y, x + 1), (y, x - 1)):
                if 0 <= ny < r and 0 <= nx < c and grid[ny][nx] == "#" and not seen[ny * c + nx]:
                    seen[ny * c + nx] = 1
                    stack.append((ny, nx))
print(islands)

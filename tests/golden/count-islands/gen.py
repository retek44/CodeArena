"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


def solve(grid):
    r, c = len(grid), len(grid[0])
    seen = [[False] * c for _ in range(r)]
    islands = 0
    for i in range(r):
        for j in range(c):
            if grid[i][j] != "#" or seen[i][j]:
                continue
            islands += 1
            seen[i][j] = True
            stack = [(i, j)]
            while stack:
                y, x = stack.pop()
                for ny, nx in ((y + 1, x), (y - 1, x), (y, x + 1), (y, x - 1)):
                    if 0 <= ny < r and 0 <= nx < c and grid[ny][nx] == "#" and not seen[ny][nx]:
                        seen[ny][nx] = True
                        stack.append((ny, nx))
    return islands


def case(name, grid):
    write(name, f"{len(grid)} {len(grid[0])}\n" + "".join(line + "\n" for line in grid), f"{solve(grid)}\n")


rng = random.Random(20261115)
case("01", ["##..#", "##...", "..#..", "...##"])
case("02", ["."])
case("03", ["#"])
case("04", ["#.", ".#"])
case("05", ["".join("#" if rng.random() < 0.45 else "." for _ in range(400)) for _ in range(400)])
case("06", ["#" * 300] * 300)
case("07", ["".join("#" if (i + j) % 2 == 0 else "." for j in range(200)) for i in range(200)])

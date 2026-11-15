"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


MOD = 1_000_000_007


def solve(grid):
    c = len(grid[0])
    row = [0] * c
    for i, line in enumerate(grid):
        left = 0
        for j in range(c):
            if line[j] == "#":
                left = 0
            elif i == 0 and j == 0:
                left = 1
            else:
                left = (left + row[j]) % MOD
            row[j] = left
    return row[-1]


def case(name, grid):
    write(name, f"{len(grid)} {len(grid[0])}\n" + "".join(line + "\n" for line in grid), f"{solve(grid)}\n")


def park(rng, r, c, fences):
    grid = [["#" if rng.random() < fences else "." for _ in range(c)] for _ in range(r)]
    grid[0][0] = grid[-1][-1] = "."
    return ["".join(line) for line in grid]


rng = random.Random(20261115)
case("01", ["...", ".#.", "..."])
case("02", ["."])
case("03", ["#."])
case("04", ["..", ".#"])
case("05", ["...", "..."])
case("06", park(rng, 700, 700, 0.08))
case("07", ["." * 300] * 300)
case("08", ["."] * 1000)

"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


def solve(h):
    left_max, right_max = [0] * len(h), [0] * len(h)
    best = 0
    for i, x in enumerate(h):
        best = max(best, x)
        left_max[i] = best
    best = 0
    for i in range(len(h) - 1, -1, -1):
        best = max(best, h[i])
        right_max[i] = best
    return sum(min(left_max[i], right_max[i]) - h[i] for i in range(len(h)))


def case(name, h):
    write(name, f"{len(h)}\n{' '.join(map(str, h))}\n", f"{solve(h)}\n")


rng = random.Random(20261115)
case("01", [0, 1, 0, 2, 1, 0, 1, 3, 2, 1, 2, 1])
case("02", [4, 2, 0, 3, 2, 5])
case("03", [7])
case("04", list(range(10)))
case("05", [1_000_000_000, 0, 0, 0, 1_000_000_000])
case("06", [rng.randint(0, 1_000_000_000) for _ in range(50_000)])
valley = [abs(i - 25_000) * 40_000 for i in range(50_001)]
case("07", valley)

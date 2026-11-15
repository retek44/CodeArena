"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


def solve(values):
    best = current = values[0]
    for x in values[1:]:
        current = max(x, current + x)
        best = max(best, current)
    return best


def case(name, values):
    write(name, f"{len(values)}\n{' '.join(map(str, values))}\n", f"{solve(values)}\n")


rng = random.Random(20261115)
case("01", [-2, 3, -1, 4, -6, 5, 1, -2])
case("02", [-7, -3, -9, -4, -8])
case("03", [42])
case("04", [-1_000_000_000])
case("05", [1_000_000_000] * 6)
case("06", [rng.randint(-1_000_000_000, 1_000_000_000) for _ in range(100_000)])
case("07", [rng.randint(-1000, 1010) for _ in range(100_000)])

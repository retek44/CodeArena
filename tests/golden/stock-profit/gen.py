"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


def solve(prices):
    low, best = prices[0], 0
    for p in prices[1:]:
        low = min(low, p)
        best = max(best, p - low)
    return best


def case(name, prices):
    write(name, f"{len(prices)}\n{' '.join(map(str, prices))}\n", f"{solve(prices)}\n")


rng = random.Random(20261115)
case("01", [7, 1, 5, 3, 6, 4])
case("02", [9, 7, 4, 3, 1])
case("03", [5])
case("04", [2, 9, 1, 3])
case("05", [rng.randint(1, 1_000_000_000) for _ in range(50_000)])
case("06", sorted(rng.randint(1, 1_000_000_000) for _ in range(50_000)))

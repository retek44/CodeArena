"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


def solve(values):
    ordered = sorted(values)
    n = len(ordered)
    middle = n // 2
    median = ordered[middle] if n % 2 else (ordered[middle - 1] + ordered[middle]) / 2
    return f"{sum(ordered) / n} {median}"


def case(name, values):
    write(name, f"{len(values)}\n{' '.join(map(str, values))}\n", solve(values) + "\n")


rng = random.Random(20261115)
case("01", [3, 1, 4, 1, 5])
case("02", [1, 2, 3, 10])
case("03", [-7])
case("04", [-5, -1, -3, -2, -4, -6])
case("05", [1_000_000, 999_999])
case("06", [rng.randint(-1_000_000, 1_000_000) for _ in range(50_000)])
case("07", [rng.randint(-1000, 1_000_000) for _ in range(49_999)])

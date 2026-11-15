"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


def solve(a, coins):
    inf = a + 1
    best = [0] + [inf] * a
    for v in range(1, a + 1):
        best[v] = min([best[v - coin] + 1 for coin in coins if coin <= v] + [inf])
    return best[a] if best[a] <= a else -1


def case(name, a, coins):
    write(name, f"{len(coins)} {a}\n{' '.join(map(str, coins))}\n", f"{solve(a, coins)}\n")


rng = random.Random(20261115)
case("01", 11, [1, 2, 5])
case("02", 3, [2])
case("03", 0, [1, 2])
case("04", 6, [1, 3, 4])
case("05", 7, [2, 4])
case("06", 10_000, rng.sample(range(1, 2_000), 100))
case("07", 9_999, rng.sample(range(100, 10_001), 50))
case("08", 10_000, [7, 13, 29])

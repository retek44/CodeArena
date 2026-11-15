"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


from collections import deque


def solve(values, k):
    window, peaks = deque(), []
    for i, x in enumerate(values):
        while window and values[window[-1]] <= x:
            window.pop()
        window.append(i)
        if window[0] <= i - k:
            window.popleft()
        if i >= k - 1:
            peaks.append(values[window[0]])
    return " ".join(map(str, peaks)) + "\n"


def case(name, values, k):
    write(name, f"{len(values)} {k}\n{' '.join(map(str, values))}\n", solve(values, k))


rng = random.Random(20261115)
case("01", [1, 3, -1, -3, 5, 3, 6, 7], 3)
case("02", [4, -2, 9], 1)
case("03", [4, -2, 9, 1], 4)
case("04", list(range(20, 0, -1)), 5)
case("05", [rng.randint(-1_000_000_000, 1_000_000_000) for _ in range(60_000)], 30_000)
case("06", [rng.randint(0, 999) for _ in range(60_000)], 3)

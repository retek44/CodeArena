"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


def solve(bookings):
    merged = []
    for s, e in sorted(bookings):
        if merged and s <= merged[-1][1]:
            merged[-1][1] = max(merged[-1][1], e)
        else:
            merged.append([s, e])
    return "".join(f"{s} {e}\n" for s, e in merged)


def case(name, bookings):
    write(name, f"{len(bookings)}\n" + "".join(f"{s} {e}\n" for s, e in bookings), solve(bookings))


def random_bookings(rng, n, span, longest):
    out = []
    for _ in range(n):
        s = rng.randint(0, span)
        out.append((s, min(1_000_000_000, s + rng.randint(0, longest))))
    return out


rng = random.Random(20261115)
case("01", [(1, 3), (2, 6), (8, 10), (15, 18)])
case("02", [(1, 4), (4, 5)])
case("03", [(1, 10), (2, 3), (4, 5)])
case("04", [(8, 9), (1, 2), (3, 4)])
case("05", [(5, 5)])
case("06", random_bookings(rng, 30_000, 200_000_000, 20_000))
case("07", random_bookings(rng, 30_000, 1_000_000_000, 10))

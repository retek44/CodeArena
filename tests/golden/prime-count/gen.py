"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


LIMIT = 5_000_000


def sieve(limit):
    marks = bytearray([1]) * (limit + 1)
    marks[0] = marks[1] = 0
    for i in range(2, int(limit**0.5) + 1):
        if marks[i]:
            marks[i * i :: i] = bytes(len(range(i * i, limit + 1, i)))
    count, prefix = 0, [0] * (limit + 1)
    for i in range(limit + 1):
        count += marks[i]
        prefix[i] = count
    return prefix


PREFIX = sieve(LIMIT)


def case(name, queries):
    text = f"{len(queries)}\n" + "".join(f"{n}\n" for n in queries)
    write(name, text, "".join(f"{PREFIX[n]}\n" for n in queries))


rng = random.Random(20261115)
case("01", [10, 1, 2, 100])
case("02", [3, 4, 5])
case("03", [LIMIT])
case("04", [rng.randint(1, LIMIT) for _ in range(50_000)])
case("05", [rng.randint(1, 100) for _ in range(100_000)])

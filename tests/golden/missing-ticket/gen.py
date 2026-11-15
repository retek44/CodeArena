"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


def case(name, n, missing, rng):
    numbers = [k for k in range(1, n + 1) if k != missing]
    rng.shuffle(numbers)
    write(name, f"{n}\n{' '.join(map(str, numbers))}\n", f"{missing}\n")


rng = random.Random(20261115)
write("01", "5\n3 1 5 2\n", "4\n")
case("02", 1, 1, rng)
case("03", 2, 2, rng)
case("04", 2, 1, rng)
case("05", 100_000, rng.randint(1, 100_000), rng)
case("06", 99_999, 99_999, rng)

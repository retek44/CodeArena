"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


import string


def solve(line):
    kept = [c.lower() for c in line if c.isalnum()]
    return "YES" if kept == kept[::-1] else "NO"


def case(name, line):
    write(name, line + "\n", solve(line) + "\n")


def noisy_palindrome(rng, half):
    core = [rng.choice(string.ascii_lowercase + string.digits) for _ in range(half)]
    chars = core + core[::-1]
    out = []
    for ch in chars:
        out.append(ch.upper() if rng.random() < 0.3 else ch)
        if rng.random() < 0.2:
            out.append(rng.choice(" ,.;:!?'-"))
    return "".join(out)


rng = random.Random(20261115)
case("01", "Never odd or even")
case("02", "Was it a car, or a cat I saw?")
case("03", "race a car")
case("04", "!!")
case("05", "0P")
case("06", "Ab1 2 1bA")
case("07", noisy_palindrome(rng, 35_000))
broken = list(noisy_palindrome(rng, 35_000))
middle = len(broken) // 3
while not broken[middle].isalnum():
    middle += 1
broken[middle] = "z" if broken[middle].lower() != "z" else "y"
case("08", "".join(broken))

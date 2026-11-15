"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


import string


def solve(s):
    last, left, best = {}, 0, 0
    for i, ch in enumerate(s):
        if last.get(ch, -1) >= left:
            left = last[ch] + 1
        last[ch] = i
        best = max(best, i - left + 1)
    return best


def case(name, s):
    write(name, s + "\n", f"{solve(s)}\n")


ALPHABET = string.ascii_letters + string.digits
rng = random.Random(20261115)
case("01", "abcabcbb")
case("02", "bbbbb")
case("03", "pwwkew")
case("04", "abba")
case("05", "dvdf")
case("06", "x")
case("07", "".join(rng.choice(ALPHABET) for _ in range(100_000)))
case("08", "".join(rng.sample(ALPHABET, 62)) * 1000)

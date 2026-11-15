"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


import string


def solve(a, b):
    prev = list(range(len(b) + 1))
    for i, ca in enumerate(a, 1):
        cur = [i] + [0] * len(b)
        for j, cb in enumerate(b, 1):
            cur[j] = prev[j - 1] if ca == cb else 1 + min(prev[j - 1], prev[j], cur[j - 1])
        prev = cur
    return prev[-1]


def case(name, a, b):
    write(name, f"{a}\n{b}\n", f"{solve(a, b)}\n")


def word(rng, n, letters=string.ascii_lowercase):
    return "".join(rng.choice(letters) for _ in range(n))


def mutate(rng, s, edits):
    chars = list(s)
    for _ in range(edits):
        i = rng.randrange(len(chars))
        kind = rng.randrange(3)
        if kind == 0:
            chars[i] = rng.choice(string.ascii_lowercase)
        elif kind == 1 and len(chars) > 1:
            del chars[i]
        else:
            chars.insert(i, rng.choice(string.ascii_lowercase))
    return "".join(chars)[:1000]


rng = random.Random(20261115)
case("01", "horse", "ros")
case("02", "intention", "execution")
case("03", "a", "a")
case("04", "abc", "yabd")
case("05", word(rng, 1000), word(rng, 1000))
base = word(rng, 990, "abcd")
case("06", base, mutate(rng, base, 120))
case("07", "a", "b" * 10)

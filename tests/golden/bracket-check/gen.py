"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


PAIRS = {")": "(", "]": "[", "}": "{"}


def solve(s):
    stack = []
    for ch in s:
        if ch in PAIRS:
            if not stack or stack.pop() != PAIRS[ch]:
                return "NO"
        else:
            stack.append(ch)
    return "NO" if stack else "YES"


def balanced(rng, length):
    out, stack = [], []
    while len(out) + len(stack) < length:
        if stack and (rng.random() < 0.5 or len(out) + 2 * len(stack) >= length):
            out.append({"(": ")", "[": "]", "{": "}"}[stack.pop()])
        else:
            ch = rng.choice("([{")
            stack.append(ch)
            out.append(ch)
    while stack:
        out.append({"(": ")", "[": "]", "{": "}"}[stack.pop()])
    return "".join(out)


def case(name, s):
    write(name, s + "\n", solve(s) + "\n")


rng = random.Random(20261115)
case("01", "{[()()]}")
case("02", "([)]")
case("03", ")(")
case("04", "((")
case("05", "()[]{}")
case("06", "(" * 50_000 + ")" * 50_000)
case("07", balanced(rng, 100_000))
wrong = balanced(rng, 100_000)
case("08", wrong[:-1] + {")": "]", "]": "}", "}": ")"}[wrong[-1]])
case("09", "]")

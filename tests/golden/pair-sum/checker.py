"""Custom checker for pair-sum: accepts any two different positions whose values add up to t."""

import json
import sys


def verdict(data: dict[str, str]) -> str:
    lines = data["input"].split("\n")
    n, t = map(int, lines[0].split())
    values = list(map(int, lines[1].split()))
    tokens = data["output"].split()
    if len(tokens) != 2:
        return "WA"
    try:
        i, j = int(tokens[0]), int(tokens[1])
    except ValueError:
        return "WA"
    if not (1 <= i <= n and 1 <= j <= n) or i == j:
        return "WA"
    return "AC" if values[i - 1] + values[j - 1] == t else "WA"


print(verdict(json.load(sys.stdin)))

"""Custom checker for task-order: accepts any order of all tasks that obeys every rule."""

import json
import sys


def verdict(data: dict[str, str]) -> str:
    tokens = data["input"].split()
    n, m = int(tokens[0]), int(tokens[1])
    rules = [(int(tokens[2 + 2 * k]), int(tokens[3 + 2 * k])) for k in range(m)]
    output = data["output"].split()
    if data["expected"].split() == ["IMPOSSIBLE"]:
        return "AC" if output == ["IMPOSSIBLE"] else "WA"
    if len(output) != n:
        return "WA"
    position = [0] * (n + 1)
    for place, token in enumerate(output, start=1):
        try:
            task = int(token)
        except ValueError:
            return "WA"
        if not 1 <= task <= n or position[task]:
            return "WA"
        position[task] = place
    return "AC" if all(position[a] < position[b] for a, b in rules) else "WA"


print(verdict(json.load(sys.stdin)))

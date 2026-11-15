"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


import heapq


def solve(n, rules):
    after = [[] for _ in range(n + 1)]
    waiting = [0] * (n + 1)
    for a, b in rules:
        after[a].append(b)
        waiting[b] += 1
    ready = [t for t in range(1, n + 1) if waiting[t] == 0]
    heapq.heapify(ready)
    order = []
    while ready:
        task = heapq.heappop(ready)
        order.append(task)
        for nxt in after[task]:
            waiting[nxt] -= 1
            if waiting[nxt] == 0:
                heapq.heappush(ready, nxt)
    return " ".join(map(str, order)) if len(order) == n else "IMPOSSIBLE"


def case(name, n, rules):
    write(name, f"{n} {len(rules)}\n" + "".join(f"{a} {b}\n" for a, b in rules), solve(n, rules) + "\n")


def project(rng, n, m):
    rank = list(range(1, n + 1))
    rng.shuffle(rank)
    rules = []
    while len(rules) < m:
        i, j = sorted(rng.sample(range(n), 2))
        rules.append((rank[i], rank[j]))
    return rules


rng = random.Random(20261115)
case("01", 4, [(1, 2), (1, 3), (3, 4)])
case("02", 3, [(1, 2), (2, 3), (3, 1)])
case("03", 3, [])
case("04", 2, [(2, 2)])
chain = [(i, i + 1) for i in range(1, 50_000)]
rng.shuffle(chain)
case("05", 50_000, chain)
case("06", 30_000, project(rng, 30_000, 60_000))
cyclic = project(rng, 20_000, 40_000)
a, b = cyclic[0]
cyclic.append((b, a))
rng.shuffle(cyclic)
case("07", 20_000, cyclic)

"""Writes cases/*.in and cases/*.out for this problem (deterministic: run it again after a change)."""

import random
from pathlib import Path

CASES = Path(__file__).parent / "cases"


def write(name, text_in, text_out):
    CASES.mkdir(exist_ok=True)
    (CASES / f"{name}.in").write_text(text_in, newline="\n")
    (CASES / f"{name}.out").write_text(text_out, newline="\n")


import heapq


def solve(n, roads):
    out = [[] for _ in range(n + 1)]
    for a, b, w in roads:
        out[a].append((b, w))
    dist = [None] * (n + 1)
    dist[1] = 0
    heap = [(0, 1)]
    while heap:
        d, town = heapq.heappop(heap)
        if d > dist[town]:
            continue
        for nxt, w in out[town]:
            if dist[nxt] is None or d + w < dist[nxt]:
                dist[nxt] = d + w
                heapq.heappush(heap, (d + w, nxt))
    return -1 if dist[n] is None else dist[n]


def case(name, n, roads):
    text = f"{n} {len(roads)}\n" + "".join(f"{a} {b} {w}\n" for a, b, w in roads)
    write(name, text, f"{solve(n, roads)}\n")


def country(rng, n, m, max_w):
    route = [1] + rng.sample(range(2, n), 150) + [n]
    roads = [(a, b, rng.randint(max_w // 2, max_w)) for a, b in zip(route, route[1:])]
    while len(roads) < m:
        roads.append((rng.randint(1, n), rng.randint(1, n), rng.randint(1, max_w)))
    rng.shuffle(roads)
    return roads


rng = random.Random(20261115)
case("01", 4, [(1, 2, 4), (1, 3, 1), (3, 2, 2), (2, 4, 1), (3, 4, 5)])
case("02", 3, [(1, 2, 5)])
case("03", 4, [(1, 4, 100), (1, 2, 1), (2, 3, 1), (3, 4, 1)])
case("04", 6, [(i, i + 1, 1_000_000_000) for i in range(1, 6)])
case("05", 2, [(2, 1, 7)])
case("06", 50_000, country(rng, 50_000, 100_000, 1000))
case("07", 5_000, country(rng, 5_000, 20_000, 1_000_000_000))

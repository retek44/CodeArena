# A correct memoised recursion, but long words go deeper than Python's recursion limit.
from functools import lru_cache

a = input().strip()
b = input().strip()


@lru_cache(maxsize=None)
def distance(i, j):
    if i == 0 or j == 0:
        return i + j
    if a[i - 1] == b[j - 1]:
        return distance(i - 1, j - 1)
    return 1 + min(distance(i - 1, j - 1), distance(i - 1, j), distance(i, j - 1))


print(distance(len(a), len(b)))

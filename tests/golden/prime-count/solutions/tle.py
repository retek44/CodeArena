# Tests every number up to n by trial division, for every question.
import sys


def is_prime(k):
    if k < 2:
        return False
    d = 2
    while d * d <= k:
        if k % d == 0:
            return False
        d += 1
    return True


data = sys.stdin.buffer.read().split()
q = int(data[0])
for n in map(int, data[1 : 1 + q]):
    print(sum(1 for k in range(2, n + 1) if is_prime(k)))

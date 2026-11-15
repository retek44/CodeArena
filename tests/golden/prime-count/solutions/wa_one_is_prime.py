# Counts 1 as a prime.
import sys

data = sys.stdin.buffer.read().split()
q = int(data[0])
queries = list(map(int, data[1 : 1 + q]))
limit = max(queries)
sieve = bytearray([1]) * (limit + 1)
sieve[0] = 0
for i in range(2, int(limit**0.5) + 1):
    if sieve[i]:
        sieve[i * i :: i] = bytes(len(range(i * i, limit + 1, i)))
answers = [0] * q
count = done = 0
for idx in sorted(range(q), key=queries.__getitem__):
    n = queries[idx]
    count += sieve.count(1, done, n + 1)
    done = n + 1
    answers[idx] = count
sys.stdout.write("\n".join(map(str, answers)) + "\n")

# Counts up one by one: correct, but far too slow for big numbers.
a, b = map(int, input().split())
total = a
step = 1 if b >= 0 else -1
for _ in range(abs(b)):
    total += step
print(total)

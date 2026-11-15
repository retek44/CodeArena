# Orders tasks by depth-first search but never notices a cycle, so it prints an order even
# when none exists.
import sys

data = sys.stdin.buffer.read().split()
n, m = int(data[0]), int(data[1])
after = [[] for _ in range(n + 1)]
for k in range(m):
    after[int(data[2 + 2 * k])].append(int(data[3 + 2 * k]))
seen = [False] * (n + 1)
order = []
for start in range(1, n + 1):
    if seen[start]:
        continue
    seen[start] = True
    stack = [(start, iter(after[start]))]
    while stack:
        task, children = stack[-1]
        child = next(children, None)
        if child is None:
            stack.pop()
            order.append(task)
        elif not seen[child]:
            seen[child] = True
            stack.append((child, iter(after[child])))
print(" ".join(map(str, reversed(order))))

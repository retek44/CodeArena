# A correct depth-first search, but recursive: a long chain of rules exceeds Python's
# recursion limit.
import sys

data = sys.stdin.buffer.read().split()
n, m = int(data[0]), int(data[1])
after = [[] for _ in range(n + 1)]
for k in range(m):
    after[int(data[2 + 2 * k])].append(int(data[3 + 2 * k]))
state = [0] * (n + 1)  # 0 new, 1 on the current path, 2 done
order = []


def visit(task):
    state[task] = 1
    for child in after[task]:
        if state[child] == 1:
            return False
        if state[child] == 0 and not visit(child):
            return False
    state[task] = 2
    order.append(task)
    return True


if all(state[t] == 2 or visit(t) for t in range(1, n + 1)):
    print(" ".join(map(str, reversed(order))))
else:
    print("IMPOSSIBLE")

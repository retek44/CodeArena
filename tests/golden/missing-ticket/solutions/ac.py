import sys

data = sys.stdin.read().split()
n = int(data[0])
print(n * (n + 1) // 2 - sum(map(int, data[1:])))

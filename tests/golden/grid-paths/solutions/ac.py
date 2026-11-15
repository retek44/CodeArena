import sys

MOD = 1_000_000_007
data = sys.stdin.read().split()
r, c = int(data[0]), int(data[1])
row = [0] * c
for i in range(r):
    line = data[2 + i]
    left = 0
    for j in range(c):
        if line[j] == "#":
            left = 0
        elif i == 0 and j == 0:
            left = 1
        else:
            left = (left + row[j]) % MOD
        row[j] = left
print(row[c - 1])

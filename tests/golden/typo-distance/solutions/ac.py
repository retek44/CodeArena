a = input().strip()
b = input().strip()
prev = list(range(len(b) + 1))
for i, ca in enumerate(a, 1):
    cur = [i] + [0] * len(b)
    for j, cb in enumerate(b, 1):
        if ca == cb:
            cur[j] = prev[j - 1]
        else:
            cur[j] = 1 + min(prev[j - 1], prev[j], cur[j - 1])
    prev = cur
print(prev[-1])

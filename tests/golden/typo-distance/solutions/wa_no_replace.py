# Only inserts and deletes (length difference via the longest common subsequence), so a
# replacement costs two edits instead of one.
a = input().strip()
b = input().strip()
prev = [0] * (len(b) + 1)
for ca in a:
    cur = [0] * (len(b) + 1)
    for j, cb in enumerate(b, 1):
        cur[j] = prev[j - 1] + 1 if ca == cb else max(prev[j], cur[j - 1])
    prev = cur
print(len(a) + len(b) - 2 * prev[-1])

# Starts a new window at every repeat and forgets the characters after the first copy
# ("dvdf" gives 2).
s = input().strip()
seen = set()
best = 0
for ch in s:
    if ch in seen:
        seen = set()
    seen.add(ch)
    best = max(best, len(seen))
print(best)

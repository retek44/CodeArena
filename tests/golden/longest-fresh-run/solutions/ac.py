s = input().strip()
last = {}
left = best = 0
for i, ch in enumerate(s):
    if last.get(ch, -1) >= left:
        left = last[ch] + 1
    last[ch] = i
    best = max(best, i - left + 1)
print(best)

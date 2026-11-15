# Jumps the window start to the last repeat even when that is behind it ("abba" gives 3).
s = input().strip()
last = {}
left = best = 0
for i, ch in enumerate(s):
    if ch in last:
        left = last[ch] + 1
    last[ch] = i
    best = max(best, i - left + 1)
print(best)

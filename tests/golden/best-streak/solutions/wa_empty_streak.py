# Lets the run be empty, so a week of losses reports 0 instead of the smallest loss.
n = int(input())
best = current = 0
for x in map(int, input().split()):
    current = max(0, current + x)
    best = max(best, current)
print(best)

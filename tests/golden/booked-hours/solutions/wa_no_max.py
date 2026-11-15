# Replaces the end of the merged booking with the new end, even when the new one ends
# earlier.
import sys

data = sys.stdin.buffer.read().split()
n = int(data[0])
bookings = sorted((int(data[1 + 2 * i]), int(data[2 + 2 * i])) for i in range(n))
merged = []
for s, e in bookings:
    if merged and s <= merged[-1][1]:
        merged[-1][1] = e
    else:
        merged.append([s, e])
sys.stdout.write("".join(f"{s} {e}\n" for s, e in merged))

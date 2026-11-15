# Takes the largest coin that fits, again and again: not always the fewest coins.
k, a = map(int, input().split())
coins = sorted(map(int, input().split()), reverse=True)
count = 0
for coin in coins:
    count += a // coin
    a %= coin
print(count if a == 0 else -1)

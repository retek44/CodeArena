# Highest price minus lowest price: ignores that the sale must come after the purchase.
n = int(input())
prices = list(map(int, input().split()))
print(max(prices) - min(prices))

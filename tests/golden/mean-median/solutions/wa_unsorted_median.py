# Takes the middle value in input order instead of sorted order.
n = int(input())
values = list(map(int, input().split()))
middle = n // 2
median = values[middle] if n % 2 else (values[middle - 1] + values[middle]) / 2
print(sum(values) / n, median)

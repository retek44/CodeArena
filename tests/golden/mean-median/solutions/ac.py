n = int(input())
values = sorted(map(int, input().split()))
mean = sum(values) / n
middle = n // 2
median = values[middle] if n % 2 else (values[middle - 1] + values[middle]) / 2
print(mean, median)

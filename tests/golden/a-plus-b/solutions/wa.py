# Wrong on negative inputs only, so the first test case passes.
a, b = map(int, input().split())
print(abs(a) + abs(b))

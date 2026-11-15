n = int(input())
h = list(map(int, input().split()))
left, right = 0, n - 1
left_max = right_max = water = 0
while left < right:
    if h[left] < h[right]:
        left_max = max(left_max, h[left])
        water += left_max - h[left]
        left += 1
    else:
        right_max = max(right_max, h[right])
        water += right_max - h[right]
        right -= 1
print(water)

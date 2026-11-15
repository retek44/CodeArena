PAIRS = {")": "(", "]": "[", "}": "{"}
stack = []
balanced = True
for ch in input().strip():
    if ch in PAIRS:
        if not stack or stack.pop() != PAIRS[ch]:
            balanced = False
            break
    else:
        stack.append(ch)
print("YES" if balanced and not stack else "NO")

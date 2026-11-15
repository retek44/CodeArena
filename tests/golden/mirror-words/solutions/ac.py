kept = [c.lower() for c in input() if c.isalnum()]
print("YES" if kept == kept[::-1] else "NO")

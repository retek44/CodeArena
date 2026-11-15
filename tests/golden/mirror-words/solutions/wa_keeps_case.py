# Compares letters with their case, so "Never odd or even" is not a palindrome.
kept = [c for c in input() if c.isalnum()]
print("YES" if kept == kept[::-1] else "NO")

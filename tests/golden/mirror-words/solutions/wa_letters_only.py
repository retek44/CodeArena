# Drops digits as well as punctuation, so "0P" looks like a palindrome.
kept = [c.lower() for c in input() if c.isalpha()]
print("YES" if kept == kept[::-1] else "NO")

# Counts each kind of bracket but ignores the order, so "([)]" and ")(" pass as balanced.
s = input().strip()
ok = all(s.count(o) == s.count(c) for o, c in ("()", "[]", "{}"))
print("YES" if ok else "NO")

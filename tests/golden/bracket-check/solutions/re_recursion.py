# A correct recursive parser: deep nesting exceeds Python's recursion limit.
s = input().strip()
pos = 0
PAIRS = {"(": ")", "[": "]", "{": "}"}


def groups():
    global pos
    while pos < len(s) and s[pos] in PAIRS:
        opening = s[pos]
        pos += 1
        groups()
        if pos == len(s) or s[pos] != PAIRS[opening]:
            raise ValueError("unbalanced")
        pos += 1


try:
    groups()
    print("YES" if pos == len(s) else "NO")
except ValueError:
    print("NO")

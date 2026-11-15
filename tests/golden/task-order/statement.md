# Task Order

A project has `n` tasks, numbered 1 to `n`, and `m` rules. The rule `a b` means that task
`a` must be done before task `b`. Print an order of all `n` tasks, separated by spaces, that
obeys every rule. If there are several, print any of them. If no order exists, print
`IMPOSSIBLE`.

The first line holds `n` and `m`. Then `m` lines follow, each with `a b`.

## Constraints

- 1 ≤ n ≤ 5·10^4
- 0 ≤ m ≤ 10^5
- 1 ≤ a, b ≤ n

# Task Order (golden problem, custom checker)

Topological order. Many orders are correct, so `checker.py` checks the order against the
rules; `cases/*.out` holds one reference order (or `IMPOSSIBLE`). `ac_largest_first.py`
prints a different order than the reference and must still be accepted.
`wa_ignores_cycles.py` prints an order even when the rules form a cycle;
`re_deep_recursion.py` is a correct recursive search that exceeds Python's recursion limit
on a long chain.

The checker reads one JSON object (`input`, `expected`, `output`) from stdin and prints
`AC` or `WA` (see `backend/internal/judge`).

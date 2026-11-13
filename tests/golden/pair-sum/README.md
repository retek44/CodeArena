# Pair Sum (golden problem, custom checker)

Several answers are correct, so `checker.py` checks the pair instead of comparing text.
`ac_from_the_end.py` finds a different pair than the reference answers in `cases/*.out`
and must still be accepted. `wa_same_position.py` may print the same position twice.

The checker reads one JSON object (`input`, `expected`, `output`) from stdin and prints
`AC` or `WA` (see `backend/internal/judge`).

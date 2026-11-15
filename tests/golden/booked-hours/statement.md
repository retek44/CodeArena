# Booked Hours

A meeting room has `n` bookings; booking `i` runs from `s_i` to `e_i`. Merge bookings that
overlap or touch (one ends when the next starts) and print the merged bookings ordered by
start, one per line as `start end`.

The first line holds `n`. Then `n` lines follow, each with `s_i e_i`, in any order.

## Constraints

- 1 ≤ n ≤ 10^5
- 0 ≤ s_i ≤ e_i ≤ 10^9

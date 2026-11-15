# Grid Paths

A park is a grid of `r` rows and `c` columns. A cell is `.` when it is open and `#` when a
fence blocks it. You start in the top-left cell and walk to the bottom-right cell, one step
at a time, always to the right or down, and only through open cells.

Print the number of different walks modulo 1 000 000 007. Print 0 when there is none.

The first line holds `r` and `c`. Then `r` lines follow with `c` characters each.

## Constraints

- 1 ≤ r, c ≤ 1000

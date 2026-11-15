// Allocates far more than the heap limit and keeps it.
const kept = [];
for (;;) kept.push(new Array(30_000_000).fill(0.5));

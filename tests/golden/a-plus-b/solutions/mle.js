// Allocates a table far larger than the heap limit (30 million numbers, 240 MB).
const table = new Array(30_000_000).fill(0.5);
const [a, b] = require("fs").readFileSync(0, "utf8").trim().split(/\s+/).map(BigInt);
console.log((a + b).toString(), table.length);

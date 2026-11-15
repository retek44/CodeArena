// Counts up one by one: correct, but far too slow for big numbers.
const [a, b] = require("fs").readFileSync(0, "utf8").trim().split(/\s+/).map(BigInt);
let total = a;
const step = b >= 0n ? 1n : -1n;
const steps = b >= 0n ? b : -b;
for (let i = 0n; i < steps; i++) total += step;
console.log(total.toString());

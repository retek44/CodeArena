// BigInt: |a|, |b| up to 10^18 do not fit into a double exactly.
const [a, b] = require("fs").readFileSync(0, "utf8").trim().split(/\s+/).map(BigInt);
console.log((a + b).toString());

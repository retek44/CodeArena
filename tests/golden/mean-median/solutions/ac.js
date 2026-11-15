const data = require("fs").readFileSync(0, "utf8").trim().split(/\s+/).map(Number);
const n = data[0];
const values = data.slice(1, 1 + n).sort((a, b) => a - b);
const mean = values.reduce((s, x) => s + x, 0) / n;
const middle = n >> 1;
const median = n % 2 ? values[middle] : (values[middle - 1] + values[middle]) / 2;
console.log(`${mean} ${median}`);

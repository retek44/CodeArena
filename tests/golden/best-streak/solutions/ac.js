const data = require("fs").readFileSync(0, "utf8").trim().split(/\s+/).map(Number);
const n = data[0];
let best = -Infinity;
let current = 0;
for (let i = 1; i <= n; i++) {
  const x = data[i];
  current = i === 1 ? x : Math.max(x, current + x);
  best = Math.max(best, current);
}
console.log(String(best));

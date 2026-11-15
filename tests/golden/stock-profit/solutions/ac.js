const data = require("fs").readFileSync(0, "utf8").trim().split(/\s+/).map(Number);
const n = data[0];
let low = Infinity;
let best = 0;
for (let i = 1; i <= n; i++) {
  const p = data[i];
  if (p < low) low = p;
  else if (p - low > best) best = p - low;
}
console.log(String(best));

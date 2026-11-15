const data = require("fs").readFileSync(0, "utf8").trim().split(/\s+/).map(Number);
const [k, a] = data;
const coins = data.slice(2, 2 + k);
const best = new Int32Array(a + 1).fill(a + 1);
best[0] = 0;
for (let v = 1; v <= a; v++) {
  for (const coin of coins) {
    if (coin <= v && best[v - coin] + 1 < best[v]) best[v] = best[v - coin] + 1;
  }
}
console.log(String(best[a] > a ? -1 : best[a]));

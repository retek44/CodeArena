const data = require("fs").readFileSync(0, "utf8").trim().split(/\s+/).map(Number);
const n = data[0];
const m = data[1];
const after = Array.from({ length: n + 1 }, () => []);
const waiting = new Int32Array(n + 1);
for (let k = 0; k < m; k++) {
  const a = data[2 + 2 * k];
  const b = data[3 + 2 * k];
  after[a].push(b);
  waiting[b]++;
}
const order = [];
for (let t = 1; t <= n; t++) if (waiting[t] === 0) order.push(t);
for (let i = 0; i < order.length; i++) {
  for (const next of after[order[i]]) {
    if (--waiting[next] === 0) order.push(next);
  }
}
console.log(order.length === n ? order.join(" ") : "IMPOSSIBLE");

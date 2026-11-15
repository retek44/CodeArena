const data = require("fs").readFileSync(0, "utf8").trim().split(/\s+/).map(Number);
const q = data[0];
const queries = data.slice(1, 1 + q);
let limit = 1;
for (const n of queries) if (n > limit) limit = n;
const composite = new Uint8Array(limit + 1);
const count = new Int32Array(limit + 1);
for (let i = 2; i <= limit; i++) {
  count[i] = count[i - 1];
  if (composite[i]) continue;
  count[i]++;
  for (let j = i * i; j <= limit; j += i) composite[j] = 1;
}
process.stdout.write(queries.map((n) => count[n]).join("\n") + "\n");

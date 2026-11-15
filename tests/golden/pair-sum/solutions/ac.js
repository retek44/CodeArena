const [head, line] = require("fs").readFileSync(0, "utf8").split("\n");
const [n, t] = head.trim().split(/\s+/).map(Number);
const values = line.trim().split(/\s+/).map(Number);
const seen = new Map();
for (let j = 0; j < n; j++) {
  const i = seen.get(t - values[j]);
  if (i !== undefined) {
    console.log(`${i + 1} ${j + 1}`);
    break;
  }
  if (!seen.has(values[j])) seen.set(values[j], j);
}

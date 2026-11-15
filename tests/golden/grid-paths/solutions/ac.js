const lines = require("fs").readFileSync(0, "utf8").split("\n");
const [r, c] = lines[0].trim().split(/\s+/).map(Number);
const MOD = 1000000007;
const row = new Array(c).fill(0);
for (let i = 0; i < r; i++) {
  const line = lines[1 + i].trim();
  let left = 0;
  for (let j = 0; j < c; j++) {
    if (line[j] === "#") left = 0;
    else if (i === 0 && j === 0) left = 1;
    else left = (left + row[j]) % MOD;
    row[j] = left;
  }
}
console.log(String(row[c - 1]));

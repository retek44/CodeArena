const [a, b] = require("fs").readFileSync(0, "utf8").split("\n").map((line) => line.trim());
let prev = Int32Array.from({ length: b.length + 1 }, (_, j) => j);
let cur = new Int32Array(b.length + 1);
for (let i = 1; i <= a.length; i++) {
  cur[0] = i;
  for (let j = 1; j <= b.length; j++) {
    cur[j] = a[i - 1] === b[j - 1] ? prev[j - 1] : 1 + Math.min(prev[j - 1], prev[j], cur[j - 1]);
  }
  [prev, cur] = [cur, prev];
}
console.log(String(prev[b.length]));

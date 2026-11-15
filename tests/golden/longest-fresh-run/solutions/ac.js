const s = require("fs").readFileSync(0, "utf8").trim();
const last = new Int32Array(128).fill(-1);
let left = 0;
let best = 0;
for (let i = 0; i < s.length; i++) {
  const ch = s.charCodeAt(i);
  if (last[ch] >= left) left = last[ch] + 1;
  last[ch] = i;
  best = Math.max(best, i - left + 1);
}
console.log(String(best));

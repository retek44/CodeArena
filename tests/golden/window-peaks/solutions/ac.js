const data = require("fs").readFileSync(0, "utf8").trim().split(/\s+/).map(Number);
const [n, k] = data;
const values = data.slice(2, 2 + n);
const window = new Int32Array(n);
let head = 0;
let tail = 0;
const peaks = [];
for (let i = 0; i < n; i++) {
  while (tail > head && values[window[tail - 1]] <= values[i]) tail--;
  window[tail++] = i;
  if (window[head] <= i - k) head++;
  if (i >= k - 1) peaks.push(values[window[head]]);
}
console.log(peaks.join(" "));

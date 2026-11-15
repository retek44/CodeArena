const data = require("fs").readFileSync(0, "utf8").trim().split(/\s+/).map(Number);
const n = data[0];
const h = data.slice(1, 1 + n);
let left = 0;
let right = n - 1;
let leftMax = 0;
let rightMax = 0;
let water = 0;
while (left < right) {
  if (h[left] < h[right]) {
    leftMax = Math.max(leftMax, h[left]);
    water += leftMax - h[left++];
  } else {
    rightMax = Math.max(rightMax, h[right]);
    water += rightMax - h[right--];
  }
}
console.log(String(water));

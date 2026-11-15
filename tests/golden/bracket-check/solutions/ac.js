const s = require("fs").readFileSync(0, "utf8").trim();
const open = { ")": "(", "]": "[", "}": "{" };
const stack = [];
let balanced = true;
for (const ch of s) {
  if (!(ch in open)) {
    stack.push(ch);
  } else if (stack.pop() !== open[ch]) {
    balanced = false;
    break;
  }
}
console.log(balanced && stack.length === 0 ? "YES" : "NO");

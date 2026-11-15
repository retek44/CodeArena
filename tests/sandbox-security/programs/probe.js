// Runs after every JavaScript scenario: no leftover Node.js process of an earlier run
// may be alive, and the runner still judges a normal program (prints the sum of the input).
const fs = require("fs");

let processes = 0;
for (const pid of fs.readdirSync("/proc")) {
  if (!/^\d+$/.test(pid) || Number(pid) === process.pid) continue;
  try {
    const argv0 = fs.readFileSync(`/proc/${pid}/cmdline`, "utf8").split("\0")[0];
    if (argv0.endsWith("codearena-node")) processes++;
  } catch {
    // The process ended while we looked.
  }
}
const [a, b] = fs.readFileSync(0, "utf8").trim().split(/\s+/).map(BigInt);
console.log(`processes: ${processes}`);
console.log((a + b).toString());

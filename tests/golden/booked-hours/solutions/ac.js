const data = require("fs").readFileSync(0, "utf8").trim().split(/\s+/).map(Number);
const n = data[0];
const bookings = [];
for (let i = 0; i < n; i++) bookings.push([data[1 + 2 * i], data[2 + 2 * i]]);
bookings.sort((x, y) => x[0] - y[0]);
const merged = [];
for (const [s, e] of bookings) {
  const last = merged[merged.length - 1];
  if (last && s <= last[1]) last[1] = Math.max(last[1], e);
  else merged.push([s, e]);
}
process.stdout.write(merged.map(([s, e]) => `${s} ${e}\n`).join(""));

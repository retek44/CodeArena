const data = require("fs").readFileSync(0, "utf8").trim().split(/\s+/).map(Number);
const n = data[0];
let missing = (n * (n + 1)) / 2;
for (let i = 1; i < n; i++) missing -= data[i];
console.log(String(missing));

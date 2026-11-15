// Reads a property of undefined.
const parts = require("fs").readFileSync(0, "utf8").trim().split(/\s+/);
console.log(parts[5].length);

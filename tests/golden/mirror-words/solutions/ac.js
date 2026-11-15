const line = require("fs").readFileSync(0, "utf8").split("\n")[0];
const kept = line.toLowerCase().replace(/[^a-z0-9]/g, "");
console.log(kept === [...kept].reverse().join("") ? "YES" : "NO");

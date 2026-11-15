// Bug: the numbers stay strings, so + concatenates them ("1" + "2" = "12").
const [a, b] = require("fs").readFileSync(0, "utf8").trim().split(/\s+/);
console.log(a + b);

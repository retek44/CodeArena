const r = Number(require("fs").readFileSync(0, "utf8").trim());
console.log((Math.PI * r * r).toFixed(10));

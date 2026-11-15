const lines = require("fs").readFileSync(0, "utf8").split("\n");
const [r, c] = lines[0].trim().split(/\s+/).map(Number);
const grid = lines.slice(1, 1 + r).map((line) => line.trim());
const seen = new Uint8Array(r * c);
const stack = new Int32Array(r * c);
const dy = [1, -1, 0, 0];
const dx = [0, 0, 1, -1];
let islands = 0;
for (let i = 0; i < r; i++) {
  for (let j = 0; j < c; j++) {
    if (grid[i][j] !== "#" || seen[i * c + j]) continue;
    islands++;
    seen[i * c + j] = 1;
    let top = 0;
    stack[top++] = i * c + j;
    while (top > 0) {
      const cell = stack[--top];
      const y = Math.floor(cell / c);
      const x = cell % c;
      for (let d = 0; d < 4; d++) {
        const ny = y + dy[d];
        const nx = x + dx[d];
        if (ny < 0 || ny >= r || nx < 0 || nx >= c) continue;
        if (grid[ny][nx] !== "#" || seen[ny * c + nx]) continue;
        seen[ny * c + nx] = 1;
        stack[top++] = ny * c + nx;
      }
    }
  }
}
console.log(String(islands));

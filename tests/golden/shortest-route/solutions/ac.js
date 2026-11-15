const data = require("fs").readFileSync(0, "utf8").trim().split(/\s+/).map(Number);
const n = data[0];
const m = data[1];
const head = new Int32Array(n + 1).fill(-1);
const next = new Int32Array(m);
const to = new Int32Array(m);
const weight = new Float64Array(m);
for (let k = 0; k < m; k++) {
  const a = data[2 + 3 * k];
  to[k] = data[3 + 3 * k];
  weight[k] = data[4 + 3 * k];
  next[k] = head[a];
  head[a] = k;
}
const dist = new Float64Array(n + 1).fill(Infinity);
dist[1] = 0;
// Binary heap of [distance, town].
const heap = [[0, 1]];
const push = (item) => {
  heap.push(item);
  let i = heap.length - 1;
  while (i > 0) {
    const p = (i - 1) >> 1;
    if (heap[p][0] <= heap[i][0]) break;
    [heap[p], heap[i]] = [heap[i], heap[p]];
    i = p;
  }
};
const pop = () => {
  const top = heap[0];
  const last = heap.pop();
  if (heap.length > 0) {
    heap[0] = last;
    let i = 0;
    for (;;) {
      const l = 2 * i + 1;
      const r = l + 1;
      let s = i;
      if (l < heap.length && heap[l][0] < heap[s][0]) s = l;
      if (r < heap.length && heap[r][0] < heap[s][0]) s = r;
      if (s === i) break;
      [heap[s], heap[i]] = [heap[i], heap[s]];
      i = s;
    }
  }
  return top;
};
while (heap.length > 0) {
  const [d, town] = pop();
  if (d > dist[town]) continue;
  for (let k = head[town]; k !== -1; k = next[k]) {
    const nd = d + weight[k];
    if (nd < dist[to[k]]) {
      dist[to[k]] = nd;
      push([nd, to[k]]);
    }
  }
}
console.log(dist[n] === Infinity ? "-1" : String(dist[n]));

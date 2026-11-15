package main

import (
	"bufio"
	"container/heap"
	"fmt"
	"os"
	"strconv"
)

type road struct {
	to int
	w  int64
}

type item struct {
	d    int64
	town int
}

type queue []item

func (q queue) Len() int           { return len(q) }
func (q queue) Less(i, j int) bool { return q[i].d < q[j].d }
func (q queue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *queue) Push(x any)        { *q = append(*q, x.(item)) }
func (q *queue) Pop() any {
	old := *q
	last := old[len(old)-1]
	*q = old[:len(old)-1]
	return last
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<24)
	in.Split(bufio.ScanWords)
	next := func() int64 {
		in.Scan()
		v, _ := strconv.ParseInt(in.Text(), 10, 64)
		return v
	}
	n, m := int(next()), int(next())
	roads := make([][]road, n+1)
	for k := 0; k < m; k++ {
		a, b, w := int(next()), int(next()), next()
		roads[a] = append(roads[a], road{b, w})
	}
	dist := make([]int64, n+1)
	for i := range dist {
		dist[i] = -1
	}
	dist[1] = 0
	q := &queue{{0, 1}}
	for q.Len() > 0 {
		cur := heap.Pop(q).(item)
		if cur.d > dist[cur.town] {
			continue
		}
		for _, r := range roads[cur.town] {
			if d := cur.d + r.w; dist[r.to] == -1 || d < dist[r.to] {
				dist[r.to] = d
				heap.Push(q, item{d, r.to})
			}
		}
	}
	fmt.Println(dist[n])
}

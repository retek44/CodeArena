package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<24)
	in.Split(bufio.ScanWords)
	next := func() int {
		in.Scan()
		v, _ := strconv.Atoi(in.Text())
		return v
	}
	n, m := next(), next()
	after := make([][]int, n+1)
	waiting := make([]int, n+1)
	for k := 0; k < m; k++ {
		a, b := next(), next()
		after[a] = append(after[a], b)
		waiting[b]++
	}
	order := make([]int, 0, n)
	for t := 1; t <= n; t++ {
		if waiting[t] == 0 {
			order = append(order, t)
		}
	}
	for i := 0; i < len(order); i++ {
		for _, nxt := range after[order[i]] {
			waiting[nxt]--
			if waiting[nxt] == 0 {
				order = append(order, nxt)
			}
		}
	}
	if len(order) != n {
		fmt.Println("IMPOSSIBLE")
		return
	}
	out := bufio.NewWriterSize(os.Stdout, 1<<20)
	defer out.Flush()
	for i, t := range order {
		if i > 0 {
			out.WriteByte(' ')
		}
		out.WriteString(strconv.Itoa(t))
	}
	out.WriteByte('\n')
}

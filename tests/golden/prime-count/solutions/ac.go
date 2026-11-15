package main

import (
	"bufio"
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
	q := next()
	queries := make([]int, q)
	limit := 1
	for i := range queries {
		queries[i] = next()
		limit = max(limit, queries[i])
	}
	composite := make([]bool, limit+1)
	count := make([]int32, limit+1)
	for i := 2; i <= limit; i++ {
		count[i] = count[i-1]
		if composite[i] {
			continue
		}
		count[i]++
		for j := i * i; j <= limit; j += i {
			composite[j] = true
		}
	}
	out := bufio.NewWriterSize(os.Stdout, 1<<20)
	defer out.Flush()
	for _, n := range queries {
		out.WriteString(strconv.Itoa(int(count[n])))
		out.WriteByte('\n')
	}
}

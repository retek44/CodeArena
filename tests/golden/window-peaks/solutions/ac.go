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
	next := func() int64 {
		in.Scan()
		v, _ := strconv.ParseInt(in.Text(), 10, 64)
		return v
	}
	n, k := int(next()), int(next())
	values := make([]int64, n)
	for i := range values {
		values[i] = next()
	}
	window := make([]int, n)
	head, tail := 0, 0
	out := bufio.NewWriterSize(os.Stdout, 1<<20)
	defer out.Flush()
	for i := 0; i < n; i++ {
		for tail > head && values[window[tail-1]] <= values[i] {
			tail--
		}
		window[tail] = i
		tail++
		if window[head] <= i-k {
			head++
		}
		if i >= k-1 {
			if i > k-1 {
				out.WriteByte(' ')
			}
			out.WriteString(strconv.FormatInt(values[window[head]], 10))
		}
	}
	out.WriteByte('\n')
}

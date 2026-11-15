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
	next := func() int64 {
		in.Scan()
		v, _ := strconv.ParseInt(in.Text(), 10, 64)
		return v
	}
	n, t := int(next()), next()
	seen := make(map[int64]int, n)
	for j := 0; j < n; j++ {
		x := next()
		if i, ok := seen[t-x]; ok {
			fmt.Println(i+1, j+1)
			return
		}
		if _, ok := seen[x]; !ok {
			seen[x] = j
		}
	}
}

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
	n := int(next())
	var low, best int64
	for i := 0; i < n; i++ {
		p := next()
		if i == 0 || p < low {
			low = p
		} else if p-low > best {
			best = p - low
		}
	}
	fmt.Println(best)
}

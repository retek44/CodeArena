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
	k, a := int(next()), int(next())
	coins := make([]int, k)
	for i := range coins {
		coins[i] = int(next())
	}
	best := make([]int, a+1)
	for v := 1; v <= a; v++ {
		best[v] = a + 1
		for _, coin := range coins {
			if coin <= v && best[v-coin]+1 < best[v] {
				best[v] = best[v-coin] + 1
			}
		}
	}
	if best[a] > a {
		fmt.Println(-1)
	} else {
		fmt.Println(best[a])
	}
}

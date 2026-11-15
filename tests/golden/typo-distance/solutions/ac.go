package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var a, b string
	fmt.Fscan(in, &a, &b)
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1]
			} else {
				cur[j] = 1 + min(prev[j-1], prev[j], cur[j-1])
			}
		}
		prev, cur = cur, prev
	}
	fmt.Println(prev[len(b)])
}

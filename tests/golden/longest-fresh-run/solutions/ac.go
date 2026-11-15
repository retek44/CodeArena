package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	line, _ := bufio.NewReaderSize(os.Stdin, 1<<20).ReadString('\n')
	s := strings.TrimSpace(line)
	var last [128]int
	for i := range last {
		last[i] = -1
	}
	left, best := 0, 0
	for i := 0; i < len(s); i++ {
		if last[s[i]] >= left {
			left = last[s[i]] + 1
		}
		last[s[i]] = i
		best = max(best, i-left+1)
	}
	fmt.Println(best)
}

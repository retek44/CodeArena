package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	const mod = 1_000_000_007
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	var r, c int
	fmt.Fscan(in, &r, &c)
	row := make([]int64, c)
	var line string
	for i := 0; i < r; i++ {
		fmt.Fscan(in, &line)
		var left int64
		for j := 0; j < c; j++ {
			switch {
			case line[j] == '#':
				left = 0
			case i == 0 && j == 0:
				left = 1
			default:
				left = (left + row[j]) % mod
			}
			row[j] = left
		}
	}
	fmt.Println(row[c-1])
}

package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	var r, c int
	fmt.Fscan(in, &r, &c)
	grid := make([]string, r)
	for i := range grid {
		fmt.Fscan(in, &grid[i])
	}
	seen := make([]bool, r*c)
	stack := make([]int, 0, 1024)
	dy := [4]int{1, -1, 0, 0}
	dx := [4]int{0, 0, 1, -1}
	islands := 0
	for i := 0; i < r; i++ {
		for j := 0; j < c; j++ {
			if grid[i][j] != '#' || seen[i*c+j] {
				continue
			}
			islands++
			seen[i*c+j] = true
			stack = append(stack[:0], i*c+j)
			for len(stack) > 0 {
				cell := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				y, x := cell/c, cell%c
				for d := 0; d < 4; d++ {
					ny, nx := y+dy[d], x+dx[d]
					if ny < 0 || ny >= r || nx < 0 || nx >= c || grid[ny][nx] != '#' || seen[ny*c+nx] {
						continue
					}
					seen[ny*c+nx] = true
					stack = append(stack, ny*c+nx)
				}
			}
		}
	}
	fmt.Println(islands)
}

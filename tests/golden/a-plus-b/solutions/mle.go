// Allocates a table far larger than the memory limit.
package main

import "fmt"

func main() {
	table := make([]int64, 300_000_000)
	var a, b int64
	fmt.Scan(&a, &b)
	table[12345] = 1
	fmt.Println(a + b + table[12345] - 1)
}

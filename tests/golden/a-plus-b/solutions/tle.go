// Counts up one by one: correct, but far too slow for big numbers.
package main

import "fmt"

func main() {
	var a, b int64
	fmt.Scan(&a, &b)
	total, step, steps := a, int64(1), b
	if b < 0 {
		step, steps = -1, -b
	}
	for i := int64(0); i < steps; i++ {
		total += step
	}
	fmt.Println(total)
}

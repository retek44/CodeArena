// Reads past the end of an empty slice.
package main

import "fmt"

func main() {
	var values []int64
	var a, b int64
	fmt.Scan(&a, &b)
	i := 5
	fmt.Println(values[i] + a + b)
}

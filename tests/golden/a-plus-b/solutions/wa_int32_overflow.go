// Bug: int32 overflows for |a|, |b| up to 10^18.
package main

import "fmt"

func main() {
	var a, b int64
	fmt.Scan(&a, &b)
	fmt.Println(int32(a + b))
}

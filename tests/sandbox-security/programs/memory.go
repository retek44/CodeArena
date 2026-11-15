// Allocates far more than the memory limit and keeps it.
package main

var kept [][]byte

func main() {
	for {
		block := make([]byte, 64<<20)
		for i := range block {
			block[i] = 1
		}
		kept = append(kept, block)
	}
}

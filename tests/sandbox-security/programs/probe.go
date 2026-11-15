// Runs after every Go scenario: no leftover process of an earlier run may be alive, and
// the runner still judges a normal program (prints the sum of the input).
package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
)

func main() {
	processes := 0
	entries, _ := os.ReadDir("/proc")
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		cmdline, err := os.ReadFile("/proc/" + entry.Name() + "/cmdline")
		if err == nil && string(bytes.SplitN(cmdline, []byte{0}, 2)[0]) == "./main" {
			processes++
		}
	}
	var a, b int64
	fmt.Scan(&a, &b)
	fmt.Println("processes:", processes)
	fmt.Println(a + b)
}

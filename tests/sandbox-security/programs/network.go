// The runner has no route out: a TCP connection must fail, and fail fast.
package main

import (
	"fmt"
	"net"
	"time"
)

func tcp(address string) string {
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return "blocked"
	}
	conn.Close()
	return "open"
}

func main() {
	fmt.Println("tcp:", tcp("1.1.1.1:53"))
	fmt.Println("tcp:", tcp("8.8.8.8:443"))
}

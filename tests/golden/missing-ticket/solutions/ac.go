package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<24)
	in.Split(bufio.ScanWords)
	next := func() int64 {
		in.Scan()
		v, _ := strconv.ParseInt(in.Text(), 10, 64)
		return v
	}
	n := next()
	missing := n * (n + 1) / 2
	for i := int64(1); i < n; i++ {
		missing -= next()
	}
	fmt.Println(missing)
}

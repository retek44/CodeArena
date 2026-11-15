package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
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
	n := int(next())
	values := make([]int64, n)
	var sum int64
	for i := range values {
		values[i] = next()
		sum += values[i]
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	median := float64(values[n/2])
	if n%2 == 0 {
		median = float64(values[n/2-1]+values[n/2]) / 2
	}
	fmt.Printf("%.9f %.9f\n", float64(sum)/float64(n), median)
}

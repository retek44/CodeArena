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
	n := int(next())
	h := make([]int64, n)
	for i := range h {
		h[i] = next()
	}
	left, right := 0, n-1
	var leftMax, rightMax, water int64
	for left < right {
		if h[left] < h[right] {
			leftMax = max(leftMax, h[left])
			water += leftMax - h[left]
			left++
		} else {
			rightMax = max(rightMax, h[right])
			water += rightMax - h[right]
			right--
		}
	}
	fmt.Println(water)
}

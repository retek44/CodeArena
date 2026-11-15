package main

import (
	"bufio"
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
	bookings := make([][2]int64, n)
	for i := range bookings {
		bookings[i] = [2]int64{next(), next()}
	}
	sort.Slice(bookings, func(i, j int) bool {
		if bookings[i][0] != bookings[j][0] {
			return bookings[i][0] < bookings[j][0]
		}
		return bookings[i][1] < bookings[j][1]
	})
	merged := make([][2]int64, 0, n)
	for _, b := range bookings {
		if last := len(merged) - 1; last >= 0 && b[0] <= merged[last][1] {
			merged[last][1] = max(merged[last][1], b[1])
		} else {
			merged = append(merged, b)
		}
	}
	out := bufio.NewWriterSize(os.Stdout, 1<<20)
	defer out.Flush()
	for _, m := range merged {
		out.WriteString(strconv.FormatInt(m[0], 10) + " " + strconv.FormatInt(m[1], 10) + "\n")
	}
}

package main

import (
	"fmt"
	"math"
)

func main() {
	var r float64
	fmt.Scan(&r)
	fmt.Printf("%.10f\n", math.Pi*r*r)
}

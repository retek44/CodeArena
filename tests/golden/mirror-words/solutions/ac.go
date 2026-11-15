package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	line, _ := bufio.NewReaderSize(os.Stdin, 1<<20).ReadString('\n')
	kept := make([]byte, 0, len(line))
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9':
			kept = append(kept, ch)
		case ch >= 'A' && ch <= 'Z':
			kept = append(kept, ch+'a'-'A')
		}
	}
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		if kept[i] != kept[j] {
			fmt.Println("NO")
			return
		}
	}
	fmt.Println("YES")
}

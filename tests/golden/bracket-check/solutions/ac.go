package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	reader := bufio.NewReaderSize(os.Stdin, 1<<20)
	line, _ := reader.ReadString('\n')
	s := strings.TrimSpace(line)
	open := map[byte]byte{')': '(', ']': '[', '}': '{'}
	stack := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		want, closing := open[ch]
		if !closing {
			stack = append(stack, ch)
			continue
		}
		if len(stack) == 0 || stack[len(stack)-1] != want {
			fmt.Println("NO")
			return
		}
		stack = stack[:len(stack)-1]
	}
	if len(stack) == 0 {
		fmt.Println("YES")
	} else {
		fmt.Println("NO")
	}
}

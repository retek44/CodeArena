// Burns CPU forever: the CPU time limit must stop it.
package main

var counter int64

func main() {
	for {
		counter++
	}
}

// Package checker compares program output with the expected output.
package checker

import "strings"

// Exact accepts output that equals the expected output after normalization:
// CRLF becomes LF, trailing spaces and tabs on each line are ignored, and
// trailing empty lines are ignored. Everything else must match byte for byte.
func Exact(got, want string) bool {
	return normalize(got) == normalize(want)
}

func normalize(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

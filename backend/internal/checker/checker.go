// Package checker compares program output with the expected output.
//
// Exact and Float run in the worker. Custom checkers (problem-specific programs, for
// problems with more than one correct answer) run in the sandbox: see package judge.
package checker

import (
	"math"
	"strconv"
	"strings"
)

// DefaultTolerance is used by Float when the problem sets none.
const DefaultTolerance = 1e-6

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

// Float accepts output whose whitespace-separated tokens match the expected tokens:
// numbers may differ by tol absolutely or relatively (|got-want| <= tol*max(1,|want|)),
// every other token must be equal. Line breaks do not matter. NaN and infinities
// never match a finite number.
func Float(got, want string, tol float64) bool {
	if tol <= 0 {
		tol = DefaultTolerance
	}
	g, w := strings.Fields(got), strings.Fields(want)
	if len(g) != len(w) {
		return false
	}
	for i := range w {
		if g[i] == w[i] {
			continue
		}
		wv, werr := strconv.ParseFloat(w[i], 64)
		gv, gerr := strconv.ParseFloat(g[i], 64)
		if werr != nil || gerr != nil {
			return false
		}
		if math.IsNaN(wv) || math.IsNaN(gv) || math.IsInf(wv, 0) || math.IsInf(gv, 0) {
			return false
		}
		if math.Abs(gv-wv) > tol*math.Max(1, math.Abs(wv)) {
			return false
		}
	}
	return true
}

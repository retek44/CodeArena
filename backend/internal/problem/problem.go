// Package problem loads problems from a directory (local CLI and golden tests).
//
// Layout:
//
//	problem.json      {"slug": "...", "time_ms": 1000, "memory_mb": 64, "checker": "exact"}
//	cases/NAME.in     input
//	cases/NAME.out    expected output
//
// In AWS, problems come from PostgreSQL and S3 instead (phase 2).
package problem

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Problem is a problem with its test cases.
type Problem struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	TimeMS   int    `json:"time_ms"`
	MemoryMB int    `json:"memory_mb"`
	Checker  string `json:"checker"`
	Cases    []Case `json:"-"`
}

// Case is one test case.
type Case struct {
	Name     string
	Input    string
	Expected string
}

// Load reads a problem directory.
func Load(dir string) (*Problem, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "problem.json")) //nolint:gosec // local tool: the user chooses the directory
	if err != nil {
		return nil, fmt.Errorf("read problem.json: %w", err)
	}
	p := &Problem{}
	if err := json.Unmarshal(raw, p); err != nil {
		return nil, fmt.Errorf("parse problem.json: %w", err)
	}
	if p.Checker == "" {
		p.Checker = "exact"
	}
	if p.Checker != "exact" {
		return nil, fmt.Errorf("checker %q is not supported yet", p.Checker)
	}
	if p.TimeMS <= 0 || p.MemoryMB <= 0 {
		return nil, errors.New("problem.json: time_ms and memory_mb must be positive")
	}

	inputs, err := filepath.Glob(filepath.Join(dir, "cases", "*.in"))
	if err != nil {
		return nil, fmt.Errorf("list cases: %w", err)
	}
	if len(inputs) == 0 {
		return nil, errors.New("no test cases in cases/")
	}
	sort.Strings(inputs)

	for _, in := range inputs {
		name := strings.TrimSuffix(filepath.Base(in), ".in")
		input, err := os.ReadFile(in) //nolint:gosec // path comes from Glob inside the problem directory
		if err != nil {
			return nil, fmt.Errorf("read case %s: %w", name, err)
		}
		expected, err := os.ReadFile(strings.TrimSuffix(in, ".in") + ".out") //nolint:gosec // same directory as the input
		if err != nil {
			return nil, fmt.Errorf("case %s has no .out file: %w", name, err)
		}
		p.Cases = append(p.Cases, Case{Name: name, Input: string(input), Expected: string(expected)})
	}
	return p, nil
}

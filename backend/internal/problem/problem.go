// Package problem loads problems from a directory (local CLI and golden tests).
//
// Layout:
//
//	problem.json      {"slug": "...", "time_ms": 1000, "memory_mb": 64, "checker": "exact"}
//	                  checker: exact (default), float (with "tolerance") or custom
//	statement.md      problem statement shown to users (optional)
//	checker.py        custom checker (checker "custom" only), see package judge
//	cases/NAME.in     input
//	cases/NAME.out    expected output (for a custom checker: a reference answer)
//
// The judge reads problems from PostgreSQL; cmd/seed loads directories in this layout.
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

// Checkers.
const (
	CheckerExact  = "exact"
	CheckerFloat  = "float"
	CheckerCustom = "custom"
)

// Problem is a problem with its test cases.
type Problem struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	TimeMS   int    `json:"time_ms"`
	MemoryMB int    `json:"memory_mb"`
	Checker  string `json:"checker"`
	// Difficulty: easy (default), medium or hard.
	Difficulty string `json:"difficulty,omitempty"`
	// Tolerance of the float checker; 0 means checker.DefaultTolerance.
	Tolerance float64 `json:"tolerance,omitempty"`
	// CheckerSource is the Python source of a custom checker (checker.py).
	CheckerSource string `json:"-"`
	// Statement is the Markdown from statement.md; empty without the file.
	Statement string `json:"-"`
	Cases     []Case `json:"-"`
}

// Validate checks the difficulty and the checker settings.
func (p *Problem) Validate() error {
	switch p.Difficulty {
	case "", "easy", "medium", "hard":
	default:
		return fmt.Errorf("unknown difficulty %q (easy, medium or hard)", p.Difficulty)
	}
	switch p.Checker {
	case CheckerExact:
	case CheckerFloat:
		if p.Tolerance < 0 {
			return errors.New("tolerance must not be negative")
		}
	case CheckerCustom:
		if strings.TrimSpace(p.CheckerSource) == "" {
			return errors.New("checker custom needs checker.py")
		}
	default:
		return fmt.Errorf("unknown checker %q (exact, float or custom)", p.Checker)
	}
	return nil
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
		p.Checker = CheckerExact
	}
	if p.Checker == CheckerCustom {
		source, err := os.ReadFile(filepath.Join(dir, "checker.py")) //nolint:gosec // same directory as problem.json
		if err != nil {
			return nil, fmt.Errorf("read checker.py: %w", err)
		}
		p.CheckerSource = string(source)
	}
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("problem.json: %w", err)
	}
	if p.TimeMS <= 0 || p.MemoryMB <= 0 {
		return nil, errors.New("problem.json: time_ms and memory_mb must be positive")
	}

	statement, err := os.ReadFile(filepath.Join(dir, "statement.md")) //nolint:gosec // same directory as problem.json
	switch {
	case err == nil:
		p.Statement = string(statement)
	case !errors.Is(err, os.ErrNotExist):
		return nil, fmt.Errorf("read statement.md: %w", err)
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

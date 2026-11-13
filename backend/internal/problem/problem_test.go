package problem_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/retek44/CodeArena/backend/internal/problem"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSortsCases(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "problem.json"), `{"slug":"p","time_ms":1000,"memory_mb":64}`)
	write(t, filepath.Join(dir, "cases", "02.in"), "b")
	write(t, filepath.Join(dir, "cases", "02.out"), "B")
	write(t, filepath.Join(dir, "cases", "01.in"), "a")
	write(t, filepath.Join(dir, "cases", "01.out"), "A")

	p, err := problem.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Checker != "exact" {
		t.Errorf("checker = %q, want default exact", p.Checker)
	}
	if len(p.Cases) != 2 || p.Cases[0].Name != "01" || p.Cases[1].Expected != "B" {
		t.Errorf("cases = %+v", p.Cases)
	}
}

func TestLoadErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]map[string]string{
		"no problem.json":     {"cases/01.in": "", "cases/01.out": ""},
		"bad json":            {"problem.json": "{", "cases/01.in": "", "cases/01.out": ""},
		"zero limits":         {"problem.json": `{"slug":"p"}`, "cases/01.in": "", "cases/01.out": ""},
		"unsupported checker": {"problem.json": `{"slug":"p","time_ms":1,"memory_mb":1,"checker":"custom"}`, "cases/01.in": "", "cases/01.out": ""},
		"no cases":            {"problem.json": `{"slug":"p","time_ms":1,"memory_mb":1}`},
		"missing .out":        {"problem.json": `{"slug":"p","time_ms":1,"memory_mb":1}`, "cases/01.in": ""},
	}
	for name, files := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for rel, content := range files {
				write(t, filepath.Join(dir, rel), content)
			}
			if _, err := problem.Load(dir); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestLoadGoldenProblem(t *testing.T) {
	t.Parallel()

	p, err := problem.Load(filepath.Join("..", "..", "..", "tests", "golden", "a-plus-b"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Slug != "a-plus-b" || len(p.Cases) == 0 {
		t.Errorf("unexpected problem: %+v", p)
	}
}

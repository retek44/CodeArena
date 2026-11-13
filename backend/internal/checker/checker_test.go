package checker_test

import (
	"testing"

	"github.com/retek44/CodeArena/backend/internal/checker"
)

func TestExact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		got, want string
		accept    bool
	}{
		{"identical", "3\n", "3\n", true},
		{"missing final newline", "3", "3\n", true},
		{"trailing spaces and tabs", "1 2 \t\n3  \n", "1 2\n3\n", true},
		{"trailing empty lines", "3\n\n\n", "3\n", true},
		{"crlf", "1\r\n2\r\n", "1\n2\n", true},
		{"empty output for empty answer", "", "", true},
		{"different value", "4\n", "3\n", false},
		{"leading space matters", " 3\n", "3\n", false},
		{"inner space matters", "1  2\n", "1 2\n", false},
		{"missing line", "1\n", "1\n2\n", false},
		{"extra line", "1\n2\n", "1\n", false},
		{"empty line in the middle matters", "1\n\n2\n", "1\n2\n", false},
		{"case matters", "yes\n", "YES\n", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := checker.Exact(tc.got, tc.want); got != tc.accept {
				t.Errorf("Exact(%q, %q) = %v, want %v", tc.got, tc.want, got, tc.accept)
			}
		})
	}
}

func TestFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		got, want string
		tol       float64
		accept    bool
	}{
		{"identical", "3.141593\n", "3.141593\n", 1e-6, true},
		{"within absolute tolerance", "0.0000004\n", "0\n", 1e-6, true},
		{"within relative tolerance", "1000000.5\n", "1000000\n", 1e-6, true},
		{"outside tolerance", "3.14\n", "3.141593\n", 1e-6, false},
		{"more digits are fine", "3.14159265358979\n", "3.141593\n", 1e-6, true},
		{"line breaks do not matter", "1.0 2.0\n", "1\n2\n", 1e-6, true},
		{"scientific notation", "1e-3\n", "0.001\n", 1e-9, true},
		{"words must match", "YES 1.0\n", "yes 1.0\n", 1e-6, false},
		{"word for number", "abc\n", "1\n", 1e-6, false},
		{"missing token", "1.0\n", "1.0 2.0\n", 1e-6, false},
		{"extra token", "1.0 2.0 3.0\n", "1.0 2.0\n", 1e-6, false},
		{"nan never matches", "NaN\n", "1\n", 1e-6, false},
		{"infinity never matches", "Inf\n", "1e308\n", 1, false},
		{"zero tolerance uses the default", "1.0000001\n", "1\n", 0, true},
		{"empty output for empty answer", "", "\n", 1e-6, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := checker.Float(tc.got, tc.want, tc.tol); got != tc.accept {
				t.Errorf("Float(%q, %q, %g) = %v, want %v", tc.got, tc.want, tc.tol, got, tc.accept)
			}
		})
	}
}

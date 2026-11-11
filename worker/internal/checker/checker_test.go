package checker_test

import (
	"testing"

	"github.com/retek44/CodeArena/worker/internal/checker"
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

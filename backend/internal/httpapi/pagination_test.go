package httpapi

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestParseLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw     string
		want    int
		wantErr bool
	}{
		{"", defaultLimit, false},
		{"1", 1, false},
		{"100", 100, false},
		{"0", 0, true},
		{"101", 0, true},
		{"-1", 0, true},
		{"ten", 0, true},
	}
	for _, tc := range tests {
		t.Run("limit="+tc.raw, func(t *testing.T) {
			t.Parallel()

			got, err := parseLimit(url.Values{"limit": {tc.raw}})
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Errorf("parseLimit(%q) = %d, %v; want %d, error=%v", tc.raw, got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestProblemCursorRoundTrip(t *testing.T) {
	t.Parallel()

	got, err := decodeProblemCursor(encodeProblemCursor(42))
	if err != nil || got != 42 {
		t.Fatalf("round trip = %d, %v; want 42", got, err)
	}
}

func TestSubmissionCursorRoundTrip(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 11, 11, 12, 0, 0, 123456789, time.UTC)
	id := uuid.New()

	gotTime, gotID, err := decodeSubmissionCursor(encodeSubmissionCursor(created, id))
	if err != nil || !gotTime.Equal(created) || gotID != id {
		t.Fatalf("round trip = %v, %v, %v; want %v, %v", gotTime, gotID, err, created, id)
	}
}

func TestDecodeCursorRejectsTampering(t *testing.T) {
	t.Parallel()

	enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	problemCursors := map[string]string{
		"not base64":         "%%%",
		"submission cursor":  encodeSubmissionCursor(time.Now(), uuid.New()),
		"zero id":            enc("p:0"),
		"negative id":        enc("p:-5"),
		"not a number":       enc("p:abc"),
		"too long":           strings.Repeat("A", maxCursorLen+1),
		"sql in the payload": enc("p:1 OR 1=1"),
	}
	for name, c := range problemCursors {
		t.Run("problem/"+name, func(t *testing.T) {
			t.Parallel()
			if _, err := decodeProblemCursor(c); err == nil {
				t.Errorf("decodeProblemCursor(%q) accepted a bad cursor", c)
			}
		})
	}

	submissionCursors := map[string]string{
		"problem cursor": encodeProblemCursor(1),
		"missing id":     enc("s:123"),
		"bad time":       enc("s:abc:" + uuid.NewString()),
		"bad uuid":       enc("s:123:not-a-uuid"),
	}
	for name, c := range submissionCursors {
		t.Run("submission/"+name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := decodeSubmissionCursor(c); err == nil {
				t.Errorf("decodeSubmissionCursor(%q) accepted a bad cursor", c)
			}
		})
	}
}

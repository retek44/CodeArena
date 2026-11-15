package httpapi_test

import (
	"testing"
	"time"

	"github.com/retek44/CodeArena/backend/internal/httpapi"
)

func TestParseRateLimit(t *testing.T) {
	got, err := httpapi.ParseRateLimit("300/1h")
	if err != nil || got != (httpapi.RateLimit{Max: 300, Window: time.Hour}) {
		t.Errorf("ParseRateLimit(300/1h) = %+v, %v", got, err)
	}
	for _, raw := range []string{"", "300", "0/1h", "-1/1h", "x/1h", "300/", "300/0s", "300/-1m", "300/hour"} {
		if _, err := httpapi.ParseRateLimit(raw); err == nil {
			t.Errorf("ParseRateLimit(%q): want an error", raw)
		}
	}
}

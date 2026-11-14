package telemetry

import (
	"context"
	"strings"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
)

// NewQueryTracer returns a pgx tracer that records a span for each query, but only
// inside a trace. Background loops (outbox relay polling, sweeper, contest jobs) query
// every few seconds; as root spans they would only add noise and X-Ray cost.
func NewQueryTracer() pgx.QueryTracer {
	return childOnly{otelpgx.NewTracer(otelpgx.WithDisableAcquireTracer(), otelpgx.WithSpanNameFunc(querySpanName))}
}

// querySpanName names a span after the sqlc query ("-- name: LockPendingOutbox :many"
// gives "LockPendingOutbox"); other statements (Ent) get their first keyword ("SELECT").
func querySpanName(stmt string) string {
	stmt = strings.TrimSpace(stmt)
	if rest, ok := strings.CutPrefix(stmt, "-- name: "); ok {
		if name, _, found := strings.Cut(rest, " "); found {
			return name
		}
	}
	if keyword, _, _ := strings.Cut(stmt, " "); keyword != "" {
		return strings.ToUpper(keyword)
	}
	return "query"
}

type childOnly struct {
	inner pgx.QueryTracer
}

type untracedKey struct{}

func (t childOnly) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return context.WithValue(ctx, untracedKey{}, true)
	}
	return t.inner.TraceQueryStart(ctx, conn, data)
}

func (t childOnly) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if ctx.Value(untracedKey{}) != nil {
		return
	}
	t.inner.TraceQueryEnd(ctx, conn, data)
}

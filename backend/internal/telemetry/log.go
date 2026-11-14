package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// NewLogHandler wraps h: a record logged with a context that carries a span gets the
// fields trace_id (W3C, hex) and xray_trace_id (1-xxxxxxxx-yyyy..., the form the X-Ray
// and CloudWatch consoles search for). Log with the *Context methods to use it.
func NewLogHandler(h slog.Handler) slog.Handler {
	return logHandler{h}
}

type logHandler struct {
	slog.Handler
}

func (h logHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		id := sc.TraceID().String()
		r.AddAttrs(
			slog.String("trace_id", id),
			slog.String("xray_trace_id", "1-"+id[:8]+"-"+id[8:]),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h logHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return logHandler{h.Handler.WithAttrs(attrs)}
}

func (h logHandler) WithGroup(name string) slog.Handler {
	return logHandler{h.Handler.WithGroup(name)}
}

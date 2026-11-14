package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func spanContext(t *testing.T) trace.SpanContext {
	t.Helper()
	tid, _ := trace.TraceIDFromHex("6a1f2c3d4e5f60718293a4b5c6d7e8f9")
	sid, _ := trace.SpanIDFromHex("0102030405060708")
	return trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled})
}

func TestLogHandlerAddsTraceIDs(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewLogHandler(slog.NewJSONHandler(&buf, nil))).With("component", "test")
	ctx := trace.ContextWithSpanContext(context.Background(), spanContext(t))

	logger.InfoContext(ctx, "judged")
	var line map[string]string
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"trace_id":      "6a1f2c3d4e5f60718293a4b5c6d7e8f9",
		"xray_trace_id": "1-6a1f2c3d-4e5f60718293a4b5c6d7e8f9",
		"span_id":       "0102030405060708",
		"component":     "test",
	}
	for k, v := range want {
		if line[k] != v {
			t.Errorf("%s = %q, want %q", k, line[k], v)
		}
	}

	buf.Reset()
	logger.Info("no span")
	if bytes.Contains(buf.Bytes(), []byte("trace_id")) {
		t.Errorf("record without a span has trace fields: %s", buf.String())
	}
}

func TestSetupWithoutEndpointOnlyPropagates(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	shutdown, err := Setup(context.Background(), "test", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(trace.ContextWithSpanContext(context.Background(), spanContext(t)), carrier)
	if carrier.Get("traceparent") != "00-6a1f2c3d4e5f60718293a4b5c6d7e8f9-0102030405060708-01" {
		t.Errorf("traceparent = %q", carrier.Get("traceparent"))
	}
	if carrier.Get("X-Amzn-Trace-Id") != "Root=1-6a1f2c3d-4e5f60718293a4b5c6d7e8f9;Parent=0102030405060708;Sampled=1" {
		t.Errorf("X-Amzn-Trace-Id = %q", carrier.Get("X-Amzn-Trace-Id"))
	}
}

type countingTracer struct{ starts, ends int }

func (c *countingTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.starts++
	return ctx
}

func (c *countingTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) { c.ends++ }

func TestQueryTracerOnlyInsideATrace(t *testing.T) {
	inner := &countingTracer{}
	tracer := childOnly{inner}

	ctx := tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	if inner.starts != 0 || inner.ends != 0 {
		t.Errorf("query without a span traced: %d starts, %d ends", inner.starts, inner.ends)
	}

	ctx = tracer.TraceQueryStart(trace.ContextWithSpanContext(context.Background(), spanContext(t)), nil, pgx.TraceQueryStartData{})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	if inner.starts != 1 || inner.ends != 1 {
		t.Errorf("query inside a trace: %d starts, %d ends; want 1, 1", inner.starts, inner.ends)
	}
}

func TestQuerySpanName(t *testing.T) {
	for stmt, want := range map[string]string{
		"-- name: LockPendingOutbox :many\nSELECT id FROM outbox_messages": "LockPendingOutbox",
		`SELECT "problems"."id" FROM "problems"`:                           "SELECT",
		"  insert into t values (1)":                                       "INSERT",
		"":                                                                 "query",
	} {
		if got := querySpanName(stmt); got != want {
			t.Errorf("querySpanName(%q) = %q, want %q", stmt, got, want)
		}
	}
}

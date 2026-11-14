// Package telemetry sets up OpenTelemetry tracing (ADR-0018).
//
// A submission is one trace: the API request, the outbox relay, the SQS message, the
// worker, the runner Lambda and the EventBridge event. Spans go over OTLP/HTTP to the
// ADOT collector sidecar in ECS (which exports them to AWS X-Ray), or to Jaeger
// locally. Trace IDs use the X-Ray format, so X-Ray accepts them unchanged and the
// Lambda functions (active tracing) join the same trace from the X-Amzn-Trace-Id header.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"

	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// Name is the instrumentation scope of the spans this module starts itself.
const Name = "github.com/retek44/CodeArena/backend"

// Tracer returns the tracer for spans started by CodeArena code.
func Tracer() trace.Tracer {
	return otel.Tracer(Name)
}

// Propagator carries the trace context across process boundaries: W3C trace context
// (HTTP, SQS message attributes, the outbox) and the X-Ray header (AWS services).
func Propagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, xray.Propagator{})
}

// Enabled reports whether an OTLP endpoint is configured.
func Enabled() bool {
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" || os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != ""
}

// Setup installs the global tracer provider and propagator. Without an OTLP endpoint
// (OTEL_EXPORTER_OTLP_ENDPOINT) no spans are recorded, but the trace context still
// propagates. The standard OTEL_* variables apply, e.g. OTEL_TRACES_SAMPLER and
// OTEL_TRACES_SAMPLER_ARG for the sampling rate. Call the returned function before
// the process exits: it flushes the spans still in the batch.
func Setup(ctx context.Context, service, version string) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(Propagator())
	if !Enabled() {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create OTLP exporter: %w", err)
	}
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(semconv.ServiceName(service), semconv.ServiceVersion(version)),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithIDGenerator(xray.NewIDGenerator()),
	)
	otel.SetTracerProvider(provider)
	return func(ctx context.Context) error {
		return errors.Join(provider.ForceFlush(ctx), provider.Shutdown(ctx))
	}, nil
}

// TagSubmission marks the context's span with the submission ID. The X-Ray exporter
// indexes this attribute, so a submission's trace can be searched by its ID.
func TagSubmission(ctx context.Context, id string) {
	trace.SpanFromContext(ctx).SetAttributes(attribute.String("codearena.submission_id", id))
}

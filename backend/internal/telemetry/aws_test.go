package telemetry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestAWSCallsGetASpanAndTheXRayHeader(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() { otel.SetTracerProvider(noop.NewTracerProvider()) })

	var header string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get("X-Amzn-Trace-Id")
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		w.Header().Set("X-Amzn-Requestid", "req-1")
		_, _ = w.Write([]byte(`{"MessageId":"m-1"}`))
	}))
	defer server.Close()

	cfg := aws.Config{
		Region:           "eu-central-1",
		Credentials:      credentials.NewStaticCredentialsProvider("test", "test", ""),
		RetryMaxAttempts: 1,
	}
	InstrumentAWS(&cfg)
	client := sqs.NewFromConfig(cfg, func(o *sqs.Options) { o.BaseEndpoint = aws.String(server.URL) })

	ctx, parent := provider.Tracer("test").Start(t.Context(), "parent")
	if _, err := client.SendMessage(ctx, &sqs.SendMessageInput{QueueUrl: aws.String(server.URL + "/q"), MessageBody: aws.String("x")}); err != nil {
		t.Fatal(err)
	}
	parent.End()

	var call sdktrace.ReadOnlySpan
	for _, s := range spans.Ended() {
		if s.Name() == "SQS.SendMessage" {
			call = s
		}
	}
	if call == nil {
		t.Fatal("no SQS.SendMessage span")
	}
	if call.Parent().SpanID() != parent.SpanContext().SpanID() || call.SpanKind() != trace.SpanKindClient {
		t.Errorf("span parent %s kind %s, want client child of the caller", call.Parent().SpanID(), call.SpanKind())
	}
	want := "Root=1-" + parent.SpanContext().TraceID().String()[:8] + "-"
	if !strings.HasPrefix(header, want) || !strings.Contains(header, "Parent="+call.SpanContext().SpanID().String()) {
		t.Errorf("X-Amzn-Trace-Id = %q, want trace %s and parent %s", header, want, call.SpanContext().SpanID())
	}
}

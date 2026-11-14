# ADR-0018: OpenTelemetry traces through ADOT to X-Ray

- Status: accepted
- Date: 2026-11-14

## Context

A submission crosses five processes: the API request, the outbox relay (a goroutine in
the API, seconds later), SQS, the worker and the runner Lambda; then EventBridge and the
WebSocket Lambda. Logs show each step, but not which lines belong to one submission or
where its time went. PLAN.md asks for OpenTelemetry with an ADOT sidecar and X-Ray.

## Decision

- **SDK:** `backend/internal/telemetry` sets up the OpenTelemetry Go SDK. Spans go over
  OTLP/HTTP to `OTEL_EXPORTER_OTLP_ENDPOINT`; without that variable nothing is recorded
  (tests, plain `make local-up`). The standard `OTEL_*` variables apply (sampler, resource).
  Trace IDs come from the X-Ray ID generator (time prefix), so X-Ray accepts them unchanged.
- **Propagation:** W3C trace context plus the X-Ray header.
  - HTTP: `otelhttp` names the server span after the matched route. The API is public:
    an incoming `traceparent` only becomes a link, so clients cannot choose trace IDs or
    force sampling. `/healthz` is not traced.
  - Outbox: the request's `traceparent` is stored in `outbox_messages.trace_parent`; the
    relay starts its producer span (`submission.created send`) as a child of it. Without
    this column the trace would end at the commit.
  - SQS: the `traceparent` goes in message attributes; the consumer span
    (`<queue> process`) is a child of the producer span.
  - AWS SDK: a small middleware adds a client span per call (`SQS.SendMessage`) with the
    attributes the X-Ray exporter turns into AWS subsegments, and sends `X-Amzn-Trace-Id`.
    The runner and hint Lambdas already use active tracing and join the trace from that
    header. (The contrib `otelaws` package is deprecated, and the SDK's own tracing hook
    is per client and does not send the header.)
  - EventBridge: each `PutEvents` entry carries the X-Ray header in `TraceHeader`, so the
    WebSocket push Lambda appears in the same trace.
  - PostgreSQL: `otelpgx` spans named after the sqlc query (`ClaimSubmission`), only inside
    a trace: the relay and sweeper poll every few seconds, and root spans for those polls
    would be noise and X-Ray cost.
- **Logs:** a slog handler adds `trace_id`, `xray_trace_id` and `span_id` to records
  logged with a context, so CloudWatch log lines and the trace find each other.
- **Search:** spans carry `codearena.submission_id`; the X-Ray exporter indexes it as an
  annotation (`annotation.codearena_submission_id = "..."`).
- **AWS:** the API and worker tasks run the ADOT collector as a non-essential sidecar
  (config in `AOT_CONFIG_CONTENT`: OTLP receiver on 127.0.0.1, batch, `awsxray` exporter)
  with `AWSXRayDaemonWriteAccess` on their task roles. `tracing = { enabled, sample_ratio }`
  in `infra/modules/app` switches it off or samples (parent-based ratio).
- **Locally:** `make local-traces` starts Jaeger (profile `tracing`) and points the
  services at it; traces show on http://localhost:16686.

## Consequences

- One trace per submission, from `POST /api/submissions` to `SQS.DeleteMessage`, with the
  runner Lambda as an X-Ray segment of the same trace.
- The sidecar takes a share of the 0.5 GB task memory (`memoryReservation` 64 MB) and adds
  image pull time on task start. A crashing collector loses traces, never requests.
- X-Ray's free tier is 100 000 traces recorded per month; with the demo traffic a sample
  ratio of 1 stays inside it. Raise the traffic, lower `sample_ratio`.
- The Python Lambdas show as X-Ray segments (active tracing), without spans of their own
  code; the runner's work is one segment, which is what the judge latency needs.

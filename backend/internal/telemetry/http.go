package telemetry

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// HTTPHandler starts a server span per request, named after the matched route
// ("GET /api/submissions/{id}"). The API is public, so a client's traceparent only
// becomes a link: callers cannot choose the trace ID or force sampling. Health checks
// are not traced: the load balancer calls them every few seconds.
func HTTPHandler(h http.Handler) http.Handler {
	return otelhttp.NewHandler(h, "http",
		otelhttp.WithPublicEndpointFn(func(*http.Request) bool { return true }),
		otelhttp.WithFilter(func(r *http.Request) bool { return r.URL.Path != "/healthz" }),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			if r.Pattern != "" {
				return r.Pattern
			}
			return r.Method
		}),
	)
}

// HTTPTransport adds a client span and the trace headers to outgoing requests.
func HTTPTransport(base http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(base)
}

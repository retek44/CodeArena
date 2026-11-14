package telemetry

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// InstrumentAWS adds a client span to every AWS SDK call made with cfg ("SQS.SendMessage")
// and sends the X-Amzn-Trace-Id header, so the runner and hint Lambdas (active tracing)
// join the trace. Only the X-Ray header is sent: AWS services do not read W3C headers.
// The span attributes are the ones the X-Ray exporter turns into AWS subsegments.
func InstrumentAWS(cfg *aws.Config) {
	cfg.APIOptions = append(cfg.APIOptions, addAWSMiddleware)
}

func addAWSMiddleware(stack *middleware.Stack) error {
	// After the service metadata middleware, which sets the service ID and operation.
	if err := stack.Initialize.Add(middleware.InitializeMiddlewareFunc("CodeArenaTraceSpan", awsSpan), middleware.After); err != nil {
		return err
	}
	// After signing: the header is not part of the signature.
	return stack.Finalize.Add(middleware.FinalizeMiddlewareFunc("CodeArenaTraceHeader", traceHeader), middleware.After)
}

func awsSpan(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
	service, operation := awsmiddleware.GetServiceID(ctx), awsmiddleware.GetOperationName(ctx)
	ctx, span := Tracer().Start(ctx, service+"."+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("rpc.system", "aws-api"),
			attribute.String("rpc.service", service),
			attribute.String("rpc.method", operation),
			attribute.String("cloud.region", awsmiddleware.GetRegion(ctx)),
		))
	defer span.End()

	out, metadata, err := next.HandleInitialize(ctx, in)
	if id, ok := awsmiddleware.GetRequestIDMetadata(metadata); ok {
		span.SetAttributes(attribute.String("aws.request_id", id))
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "AWS call failed")
	}
	return out, metadata, err
}

func traceHeader(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
	if req, ok := in.Request.(*smithyhttp.Request); ok {
		xray.Propagator{}.Inject(ctx, propagation.HeaderCarrier(req.Header))
	}
	return next.HandleFinalize(ctx, in)
}

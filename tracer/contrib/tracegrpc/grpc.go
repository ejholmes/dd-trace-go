package tracegrpc

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/DataDog/dd-trace-go/tracer"

	context "golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// pass trace ids with these headers
const (
	traceIDKey  = "x-datadog-trace-id"
	parentIDKey = "x-datadog-parent-id"
)

// UnaryServerInterceptor will trace requests to the given grpc server.
func UnaryServerInterceptor(t *tracer.Tracer) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !t.Enabled() {
			return handler(ctx, req)
		}

		span := serverSpan(t, ctx, info.FullMethod)
		resp, err := handler(tracer.ContextWithSpan(ctx, span), req)
		span.FinishWithErr(err)
		return resp, err
	}
}

// UnaryClientInterceptor will add tracing to a gprc client.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		var child *tracer.Span
		span, ok := tracer.SpanFromContext(ctx)

		// only trace the request if this is already part of a trace.
		// does this make sense?
		if ok && span.Tracer() != nil {
			t := span.Tracer()
			child = t.NewChildSpan("grpc.client", span)
			child.SetMeta("grpc.method", method)
			ctx = setIDs(child, ctx)
			ctx = tracer.ContextWithSpan(ctx, child)
			// FIXME[matt] add the host / port information here
			// https://github.com/grpc/grpc-go/issues/951
		}

		err := invoker(ctx, method, req, reply, cc, opts...)
		if child != nil {
			child.SetMeta("grpc.code", grpc.Code(err).String())
			child.FinishWithErr(err)

		}
		return err
	}
}

func serverSpan(t *tracer.Tracer, ctx context.Context, method string) *tracer.Span {
	service, resource := parseMethod(method)

	span := t.NewRootSpan("grpc.server", service, resource)
	span.SetMeta("gprc.method", method)
	span.Type = "go"

	traceID, parentID := getIDs(ctx)
	if traceID != 0 && parentID != 0 {
		span.TraceID = traceID
		span.ParentID = parentID
	}

	return span
}

func parseMethod(method string) (service, resource string) {

	start := 0
	if len(method) > 0 && method[0] == '/' {
		start = 1
	}

	if idx := strings.LastIndexByte(method, '/'); idx > 0 {
		service = method[start:idx]
		method = method[idx+1:]
		return service, method
	}

	return "", ""
}

// setIDs will set the trace ids on the context{
func setIDs(span *tracer.Span, ctx context.Context) context.Context {
	if span == nil || span.TraceID == 0 {
		return ctx
	}

	md := metadata.New(map[string]string{
		traceIDKey:  fmt.Sprint(span.TraceID),
		parentIDKey: fmt.Sprint(span.ParentID),
	})
	if existing, ok := metadata.FromContext(ctx); ok {
		md = metadata.Join(existing, md)
	}
	return metadata.NewContext(ctx, md)
}

// getIDs will return ids embededd an ahe context.
func getIDs(ctx context.Context) (traceID, parentID uint64) {
	if md, ok := metadata.FromContext(ctx); ok {
		if id := getID(md, traceIDKey); id > 0 {
			traceID = id
		}
		if id := getID(md, parentIDKey); id > 0 {
			parentID = id
		}
	}
	return traceID, parentID
}

// getID parses an id from the metadata.
func getID(md metadata.MD, name string) uint64 {
	for _, str := range md[name] {
		id, err := strconv.Atoi(str)
		if err == nil {
			return uint64(id)
		}
	}
	return 0
}

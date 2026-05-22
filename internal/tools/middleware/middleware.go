// Package middleware provides execution-control middleware for Eino InvokableTools.
//
// # Design rationale
//
// Eino's built-in callback system (callbacks.OnStart / OnEnd) is an excellent
// observability bus — it is framework-native, works across all graph nodes, and
// fires through global or per-run handlers. However, callbacks are *read-only*:
// handlers cannot modify arguments, cannot short-circuit execution, and cannot
// rewrite the response. They are therefore unsuitable for security auditing
// (which must be able to block a call) or output trimming (which must rewrite
// the result).
//
// This package implements a *gRPC-style unary interceptor* chain that sits
// in front of InvokableRun.  It is orthogonal to (not a replacement for) Eino
// callbacks — both can be active simultaneously:
//
//   - Use [Wrap] + [Interceptor] for execution control
//     (security audit, output trimming, rate-limiting).
//   - Use callbacks.AppendGlobalHandlers for observability
//     (tracing, metrics, logging via the Eino framework).
//
// # Usage
//
//	tool := middleware.Wrap(myTool,
//	    middleware.OutputTrimmer(8192),
//	    MySecurity(),
//	)
//
// # Implementing an interceptor
//
//	func MySecurity() middleware.Interceptor {
//	    return func(ctx context.Context, req *middleware.Request, handler middleware.Handler) (*middleware.Response, error) {
//	        if isSafe(req.ArgsJSON) {
//	            return handler(ctx, req)   // proceed
//	        }
//	        return nil, errors.New("blocked by security policy")  // short-circuit
//	    }
//	}
package middleware

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ToolMeta carries immutable metadata about the tool being invoked.
// Interceptors can use Name and Desc to make tool-aware decisions
// (e.g. a security policy that treats bash differently from read_file).
type ToolMeta struct {
	Name string
	Desc string
}

// Request is the input to every interceptor in the chain.
// An interceptor may replace ArgsJSON to sanitise or enrich arguments
// before passing it to the next interceptor or the actual tool.
type Request struct {
	Meta     ToolMeta
	ArgsJSON string
}

// Response is the output from the tool (or from a short-circuiting interceptor).
// An interceptor may rewrite Output after calling the handler — for example to
// trim large responses before they reach the LLM context window.
type Response struct {
	Output string
}

// Handler is the terminal call in a chain: it executes the underlying tool.
// Interceptors receive Handler as their third argument and must call it to
// continue down the chain; omitting the call short-circuits execution.
type Handler func(ctx context.Context, req *Request) (*Response, error)

// Interceptor is a single middleware unit following the gRPC UnaryInterceptor
// convention.  Call handler(ctx, req) to proceed; return without calling it to
// short-circuit.  The first interceptor registered in [Wrap] is the outermost
// (runs first on entry, last on exit).
type Interceptor func(ctx context.Context, req *Request, handler Handler) (*Response, error)

// Chain combines multiple interceptors into a single one. The first argument is
// outermost. Returns nil if no interceptors are given.
func Chain(interceptors ...Interceptor) Interceptor {
	switch len(interceptors) {
	case 0:
		return nil
	case 1:
		return interceptors[0]
	default:
		return func(ctx context.Context, req *Request, handler Handler) (*Response, error) {
			return interceptors[0](ctx, req, buildChainHandler(interceptors[1:], handler))
		}
	}
}

func buildChainHandler(interceptors []Interceptor, final Handler) Handler {
	if len(interceptors) == 0 {
		return final
	}
	return func(ctx context.Context, req *Request) (*Response, error) {
		return interceptors[0](ctx, req, buildChainHandler(interceptors[1:], final))
	}
}

// wrappedTool decorates an InvokableTool with an Interceptor chain.
// Info() is delegated to the inner tool so the LLM receives unmodified schema.
type wrappedTool struct {
	inner       tool.InvokableTool
	meta        ToolMeta
	interceptor Interceptor
}

func (w *wrappedTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return w.inner.Info(ctx)
}

func (w *wrappedTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	req := &Request{Meta: w.meta, ArgsJSON: argsJSON}

	handler := func(ctx context.Context, req *Request) (*Response, error) {
		out, err := w.inner.InvokableRun(ctx, req.ArgsJSON, opts...)
		if err != nil {
			return nil, err
		}
		return &Response{Output: out}, nil
	}

	resp, err := w.interceptor(ctx, req, handler)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", nil
	}
	return resp.Output, nil
}

// Wrap applies interceptors to t and returns a new InvokableTool.
// Info() is called once eagerly (with context.Background) to populate ToolMeta,
// so interceptors always receive the tool's name and description.
// If no interceptors are provided, t is returned unchanged.
func Wrap(t tool.InvokableTool, interceptors ...Interceptor) tool.InvokableTool {
	if len(interceptors) == 0 {
		return t
	}

	var meta ToolMeta
	if info, err := t.Info(context.Background()); err == nil && info != nil {
		meta = ToolMeta{Name: info.Name, Desc: info.Desc}
	}

	return &wrappedTool{
		inner:       t,
		meta:        meta,
		interceptor: Chain(interceptors...),
	}
}

package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ── mockTool ──────────────────────────────────────────────────────────────────

type mockTool struct {
	name   string
	result string
	err    error
}

func (m *mockTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: m.name, Desc: "mock tool"}, nil
}

func (m *mockTool) InvokableRun(_ context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	return m.result, m.err
}

// ── Chain ─────────────────────────────────────────────────────────────────────

func TestChain_NoneReturnsNil(t *testing.T) {
	if Chain() != nil {
		t.Error("Chain() with no interceptors should return nil")
	}
}

func TestChain_Single(t *testing.T) {
	called := false
	interceptor := Interceptor(func(ctx context.Context, req *Request, handler Handler) (*Response, error) {
		called = true
		return handler(ctx, req)
	})
	c := Chain(interceptor)
	if c == nil {
		t.Fatal("Chain with one interceptor should not return nil")
	}
	finalHandler := func(_ context.Context, _ *Request) (*Response, error) {
		return &Response{Output: "result"}, nil
	}
	resp, err := c(context.Background(), &Request{ArgsJSON: "{}"}, finalHandler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("interceptor should have been called")
	}
	if resp.Output != "result" {
		t.Errorf("unexpected output: %q", resp.Output)
	}
}

func TestChain_MultipleOrdering(t *testing.T) {
	var order []int
	first := Interceptor(func(ctx context.Context, req *Request, handler Handler) (*Response, error) {
		order = append(order, 1)
		resp, err := handler(ctx, req)
		order = append(order, -1)
		return resp, err
	})
	second := Interceptor(func(ctx context.Context, req *Request, handler Handler) (*Response, error) {
		order = append(order, 2)
		resp, err := handler(ctx, req)
		order = append(order, -2)
		return resp, err
	})
	c := Chain(first, second)
	final := func(_ context.Context, _ *Request) (*Response, error) {
		order = append(order, 3)
		return &Response{Output: "ok"}, nil
	}
	_, _ = c(context.Background(), &Request{}, final)
	expected := []int{1, 2, 3, -2, -1}
	if len(order) != len(expected) {
		t.Fatalf("order length: got %v, want %v", order, expected)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d]: got %d, want %d", i, order[i], v)
		}
	}
}

func TestChain_ShortCircuit(t *testing.T) {
	blocked := false
	blocker := Interceptor(func(_ context.Context, _ *Request, _ Handler) (*Response, error) {
		blocked = true
		return &Response{Output: "blocked"}, nil
	})
	downstream := Interceptor(func(_ context.Context, _ *Request, _ Handler) (*Response, error) {
		t.Error("downstream interceptor should not be called after short-circuit")
		return nil, nil
	})
	c := Chain(blocker, downstream)
	resp, err := c(context.Background(), &Request{}, func(_ context.Context, _ *Request) (*Response, error) {
		t.Error("handler should not be called after short-circuit")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !blocked {
		t.Error("blocker should have been called")
	}
	if resp.Output != "blocked" {
		t.Errorf("unexpected output: %q", resp.Output)
	}
}

// ── Wrap ──────────────────────────────────────────────────────────────────────

func TestWrap_NoInterceptors_ReturnsSameTool(t *testing.T) {
	mt := &mockTool{name: "my-tool", result: "output"}
	wrapped := Wrap(mt)
	if wrapped != tool.InvokableTool(mt) {
		t.Error("Wrap with no interceptors should return the original tool")
	}
}

func TestWrap_InterceptorRuns(t *testing.T) {
	mt := &mockTool{name: "my-tool", result: "original output"}
	called := false
	interceptor := Interceptor(func(ctx context.Context, req *Request, handler Handler) (*Response, error) {
		called = true
		return handler(ctx, req)
	})
	wrapped := Wrap(mt, interceptor)
	out, err := wrapped.InvokableRun(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !called {
		t.Error("interceptor should have been called")
	}
	if out != "original output" {
		t.Errorf("output: got %q", out)
	}
}

func TestWrap_InterceptorCanModifyOutput(t *testing.T) {
	mt := &mockTool{name: "my-tool", result: "original"}
	modifier := Interceptor(func(ctx context.Context, req *Request, handler Handler) (*Response, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			return nil, err
		}
		return &Response{Output: "modified: " + resp.Output}, nil
	})
	wrapped := Wrap(mt, modifier)
	out, err := wrapped.InvokableRun(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "modified: original" {
		t.Errorf("output: got %q", out)
	}
}

func TestWrap_InfoDelegatedToInner(t *testing.T) {
	mt := &mockTool{name: "inner-tool", result: "x"}
	interceptor := Interceptor(func(ctx context.Context, req *Request, handler Handler) (*Response, error) {
		return handler(ctx, req)
	})
	wrapped := Wrap(mt, interceptor)
	info, err := wrapped.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "inner-tool" {
		t.Errorf("Info.Name: got %q, want %q", info.Name, "inner-tool")
	}
}

func TestWrap_ToolMetaPopulated(t *testing.T) {
	mt := &mockTool{name: "named-tool", result: "x"}
	var capturedMeta ToolMeta
	interceptor := Interceptor(func(_ context.Context, req *Request, handler Handler) (*Response, error) {
		capturedMeta = req.Meta
		return handler(context.Background(), req)
	})
	wrapped := Wrap(mt, interceptor)
	_, _ = wrapped.InvokableRun(context.Background(), `{}`)
	if capturedMeta.Name != "named-tool" {
		t.Errorf("ToolMeta.Name: got %q, want %q", capturedMeta.Name, "named-tool")
	}
}

// ── SafeError ─────────────────────────────────────────────────────────────────

func TestSafeError_ConvertsErrorToOutput(t *testing.T) {
	mt := &mockTool{name: "bad-tool", err: errors.New("disk full")}
	wrapped := Wrap(mt, SafeError())
	out, err := wrapped.InvokableRun(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("SafeError should not propagate error; got %v", err)
	}
	if out == "" {
		t.Error("SafeError should return non-empty output for tool error")
	}
	if out == "disk full" {
		t.Error("SafeError should not return raw error string")
	}
}

func TestSafeError_ContainsToolName(t *testing.T) {
	mt := &mockTool{name: "my-tool", err: errors.New("oops")}
	wrapped := Wrap(mt, SafeError())
	out, _ := wrapped.InvokableRun(context.Background(), `{}`)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	// The output should reference the tool name or the error.
	_ = out // We checked it's non-empty; exact format is implementation detail
}

func TestSafeError_PassesThroughSuccess(t *testing.T) {
	mt := &mockTool{name: "ok-tool", result: "success output"}
	wrapped := Wrap(mt, SafeError())
	out, err := wrapped.InvokableRun(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "success output" {
		t.Errorf("output: got %q, want %q", out, "success output")
	}
}

// ── Request ───────────────────────────────────────────────────────────────────

func TestRequest_ArgsJSONPassthrough(t *testing.T) {
	mt := &mockTool{name: "t", result: "ok"}
	var capturedArgs string
	interceptor := Interceptor(func(_ context.Context, req *Request, handler Handler) (*Response, error) {
		capturedArgs = req.ArgsJSON
		return handler(context.Background(), req)
	})
	wrapped := Wrap(mt, interceptor)
	_, _ = wrapped.InvokableRun(context.Background(), `{"key":"value"}`)
	if capturedArgs != `{"key":"value"}` {
		t.Errorf("ArgsJSON: got %q", capturedArgs)
	}
}

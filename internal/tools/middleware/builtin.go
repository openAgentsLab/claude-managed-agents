package middleware

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
)

// SafeError returns an Interceptor that converts tool execution errors into
// plain-text responses instead of propagating them as Go errors.
//
// Without this interceptor a tool error bubbles up through the ReAct loop and
// terminates the entire agent run. With it the model receives a human-readable
// error description and can decide how to recover — retry with different args,
// try another tool, or report the problem to the user.
//
// InterruptRerunError is never swallowed — it must propagate for the ADK
// interrupt/resume cycle to work correctly.
//
// Place SafeError innermost in the chain (last argument to [Wrap]) so it sees
// the raw error from the actual tool before any other interceptor.
//
//	tool := middleware.Wrap(myTool, middleware.SafeError())
func SafeError() Interceptor {
	return func(ctx context.Context, req *Request, handler Handler) (*Response, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			if _, ok := compose.IsInterruptRerunError(err); ok {
				return nil, err
			}
			return &Response{Output: fmt.Sprintf("[tool error] %s: %v", req.Meta.Name, err)}, nil
		}
		return resp, nil
	}
}

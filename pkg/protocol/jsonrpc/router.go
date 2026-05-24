package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

type HandlerFunc func(context.Context, Request) (any, *Error)

type Router struct {
	mu       sync.RWMutex
	handlers map[string]HandlerFunc
}

func NewRouter() *Router {
	return &Router{handlers: make(map[string]HandlerFunc)}
}

func (r *Router) Register(method string, handler HandlerFunc) error {
	method = strings.TrimSpace(method)
	if method == "" {
		return fmt.Errorf("jsonrpc method is required")
	}
	if handler == nil {
		return fmt.Errorf("jsonrpc handler is required for %s", method)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[method] = handler
	return nil
}

func (r *Router) Handle(ctx context.Context, req Request) Response {
	if err := req.Validate(); err != nil {
		resp, _ := NewErrorResponse(req.ID, CodeInvalidRequest, err.Error(), nil)
		return resp
	}
	r.mu.RLock()
	handler := r.handlers[req.Method]
	r.mu.RUnlock()
	if handler == nil {
		resp, _ := NewErrorResponse(req.ID, CodeMethodNotFound, "method not found", map[string]any{"method": req.Method})
		return resp
	}
	result, rpcErr := handler(ctx, req)
	if rpcErr != nil {
		return Response{ID: req.ID, Error: rpcErr}
	}
	resp, err := NewResultResponse(req.ID, result)
	if err != nil {
		fallback, _ := NewErrorResponse(req.ID, CodeInternalError, err.Error(), nil)
		return fallback
	}
	return resp
}

type Requester interface {
	Do(context.Context, Request) (Response, error)
}

type InProcessClient struct {
	requester Requester
	nextID    atomic.Int64
}

func NewInProcessClient(requester Requester) *InProcessClient {
	return &InProcessClient{requester: requester}
}

func (c *InProcessClient) Call(ctx context.Context, method string, params any, out any) error {
	if c == nil || c.requester == nil {
		return fmt.Errorf("jsonrpc requester is nil")
	}
	id := NumberID(c.nextID.Add(1))
	req, err := NewRequest(id, method, params)
	if err != nil {
		return err
	}
	resp, err := c.requester.Do(ctx, req)
	if err != nil {
		return err
	}
	if err := resp.Validate(); err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("jsonrpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	if out == nil || len(resp.Result) == 0 {
		return nil
	}
	return json.Unmarshal(resp.Result, out)
}

type InProcessServer struct {
	Router *Router
}

func (s InProcessServer) Do(ctx context.Context, req Request) (Response, error) {
	if s.Router == nil {
		resp, _ := NewErrorResponse(req.ID, CodeInternalError, "jsonrpc router is nil", nil)
		return resp, nil
	}
	return s.Router.Handle(ctx, req), nil
}

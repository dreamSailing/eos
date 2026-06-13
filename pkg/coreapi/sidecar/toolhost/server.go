package toolhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
)

const (
	MethodToolExecute = "tool/execute"
	MethodToolCatalog = "tool/catalog"
)

type Server struct {
	host ToolHost
}

func NewServer(host ToolHost) *Server {
	return &Server{host: host}
}

func (s *Server) Serve(ctx context.Context, reader io.Reader, writer io.Writer) error {
	stream := jsonrpc.NewStream(reader, writer)
	router := jsonrpc.NewRouter()
	if err := router.Register(MethodToolExecute, s.handleToolExecute); err != nil {
		return fmt.Errorf("toolhost: register tool/execute: %w", err)
	}
	if err := router.Register(MethodToolCatalog, s.handleToolCatalog); err != nil {
		return fmt.Errorf("toolhost: register tool/catalog: %w", err)
	}
	return jsonrpc.ServeStream(ctx, router, stream)
}

func (s *Server) handleToolCatalog(ctx context.Context, req jsonrpc.Request) (any, *jsonrpc.Error) {
	var catalogReq CatalogRequest
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &catalogReq); err != nil {
			return nil, &jsonrpc.Error{
				Code:    jsonrpc.CodeInvalidParams,
				Message: fmt.Sprintf("toolhost: invalid catalog params: %v", err),
			}
		}
	}

	host, ok := s.host.(ToolCatalogHost)
	if !ok {
		return nil, &jsonrpc.Error{
			Code:    jsonrpc.CodeInternalError,
			Message: "toolhost: catalog is not available",
		}
	}

	defs, err := host.ListCatalog(ctx, catalogReq)
	if err != nil {
		return nil, &jsonrpc.Error{
			Code:    jsonrpc.CodeInternalError,
			Message: fmt.Sprintf("toolhost: catalog: %v", err),
		}
	}
	return defs, nil
}

func (s *Server) handleToolExecute(ctx context.Context, req jsonrpc.Request) (any, *jsonrpc.Error) {
	var toolReq ExecuteRequest
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &toolReq); err != nil {
			return nil, &jsonrpc.Error{
				Code:    jsonrpc.CodeInvalidParams,
				Message: fmt.Sprintf("toolhost: invalid params: %v", err),
			}
		}
	}

	if toolReq.Name == "" {
		return nil, &jsonrpc.Error{
			Code:    jsonrpc.CodeInvalidParams,
			Message: "toolhost: tool name is required",
		}
	}

	result, err := s.host.Execute(ctx, toolReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return ExecuteResponse{
				Name:      toolReq.Name,
				RequestID: toolReq.RequestID,
				Status:    "error",
				Display:   fmt.Sprintf("tool execution cancelled: %v", err),
				Error:     fmt.Sprintf("cancelled: %v", err),
			}, nil
		}
		return nil, &jsonrpc.Error{
			Code:    jsonrpc.CodeInternalError,
			Message: fmt.Sprintf("toolhost: execute %s: %v", toolReq.Name, err),
		}
	}

	return result, nil
}

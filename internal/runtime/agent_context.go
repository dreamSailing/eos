package runtime

import (
	"context"
	"strings"

	"github.com/dreamSailing/eos/internal/tools"
)

type agentContextKey struct{}

type agentContextInfo struct {
	ID   string
	Name string
}

func withCurrentAgentContext(ctx context.Context, id string, name string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	info := agentContextInfo{
		ID:   strings.TrimSpace(id),
		Name: strings.TrimSpace(name),
	}
	if info.ID == "" && info.Name == "" {
		return ctx
	}
	return context.WithValue(ctx, agentContextKey{}, info)
}

func currentAgentContext(ctx context.Context) agentContextInfo {
	if ctx == nil {
		return agentContextInfo{}
	}
	if info, ok := ctx.Value(agentContextKey{}).(agentContextInfo); ok {
		info.ID = strings.TrimSpace(info.ID)
		info.Name = strings.TrimSpace(info.Name)
		return info
	}
	return agentContextInfo{}
}

func sourceAgentContext(ctx context.Context) agentContextInfo {
	info := currentAgentContext(ctx)
	if info.Name != "" {
		return info
	}
	role := strings.TrimSpace(tools.RoleFromContext(ctx))
	if role != "" {
		info.Name = role
	}
	if info.Name == "" {
		info.Name = "assistant"
	}
	return info
}

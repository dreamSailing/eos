package tools

import (
	"context"
	"strings"
)

type ctxKey string

const (
	ctxKeyRole         ctxKey = "vb.role"
	ctxKeyAllowedTools ctxKey = "vb.allowed_tools"
	ctxKeyLanguage     ctxKey = "vb.language"
	ctxKeyTraceID      ctxKey = "vb.trace_id"
	ctxKeyWorkspaceRoot ctxKey = "vb.workspace_root"
)

var OnToolCall func(traceID string, toolName string)
var OnToolResult func(traceID string, toolName string, success bool)

// LanguageFromContext extracts the language code from context, defaulting to "zh"
func LanguageFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyLanguage).(string); ok && v != "" {
		return v
	}
	return "zh"
}

// WithLanguage returns a context with the given language code
func WithLanguage(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, ctxKeyLanguage, lang)
}

// WithRole returns a context with the given role
func WithRole(ctx context.Context, role string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKeyRole, strings.TrimSpace(role))
}

// RoleFromContext extracts the role from context
func RoleFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ctxKeyRole).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// WithAllowedTools returns a context with the given allowed tools map
func WithAllowedTools(ctx context.Context, allowed map[string]bool) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if allowed == nil {
		return context.WithValue(ctx, ctxKeyAllowedTools, nil)
	}
	cp := make(map[string]bool, len(allowed))
	for k, v := range allowed {
		cp[strings.ToLower(strings.TrimSpace(k))] = v
	}
	return context.WithValue(ctx, ctxKeyAllowedTools, cp)
}

// AllowedToolsFromContext extracts the allowed tools map from context
func AllowedToolsFromContext(ctx context.Context) map[string]bool {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(ctxKeyAllowedTools).(map[string]bool); ok {
		return v
	}
	return nil
}

func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ctxKeyTraceID).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKeyTraceID, strings.TrimSpace(traceID))
}

func WithWorkspaceRoot(ctx context.Context, root string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKeyWorkspaceRoot, strings.TrimSpace(root))
}

func WorkspaceRootFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ctxKeyWorkspaceRoot).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func NotifyToolCall(ctx context.Context, toolName string) {
	traceID := TraceIDFromContext(ctx)
	if traceID == "" {
		return
	}
	toolName = strings.ToLower(strings.TrimSpace(toolName))
	if toolName == "" {
		return
	}
	if OnToolCall != nil {
		OnToolCall(traceID, toolName)
	}
}

func NotifyToolResult(ctx context.Context, toolName string, success bool) {
	traceID := TraceIDFromContext(ctx)
	if traceID == "" {
		return
	}
	toolName = strings.ToLower(strings.TrimSpace(toolName))
	if toolName == "" {
		return
	}
	if OnToolResult != nil {
		OnToolResult(traceID, toolName, success)
	}
}

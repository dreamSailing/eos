package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"strings"
)

type ctxKey string

const (
	ctxKeyRole          ctxKey = "eos.role"
	ctxKeyAllowedTools  ctxKey = "eos.allowed_tools"
	ctxKeyLanguage      ctxKey = "eos.language"
	ctxKeyTraceID       ctxKey = "eos.trace_id"
	ctxKeyWorkspaceRoot ctxKey = "eos.workspace_root"
	ctxKeyAccessMode    ctxKey = "eos.access_mode"
	ctxKeyApprovalMode  ctxKey = "eos.approval_mode"
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

func WithAccessMode(ctx context.Context, mode string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKeyAccessMode, strings.TrimSpace(mode))
}

func AccessModeFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ctxKeyAccessMode).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func WithApprovalMode(ctx context.Context, mode string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKeyApprovalMode, strings.TrimSpace(mode))
}

func ApprovalModeFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ctxKeyApprovalMode).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func WorkspaceRootFromContext(ctx context.Context) string {
	if ctx != nil {
		if traceID := TraceIDFromContext(ctx); traceID != "" {
			if remote, ok := GetRemoteRepoContext(traceID); ok && strings.TrimSpace(remote.LocalPath) != "" {
				return strings.TrimSpace(remote.LocalPath)
			}
		}
		if v, ok := ctx.Value(ctxKeyWorkspaceRoot).(string); ok {
			return strings.TrimSpace(v)
		}
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

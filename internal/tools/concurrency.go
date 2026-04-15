package tools

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// generateToolID creates a short unique tool call ID
func generateToolID() string {
	return uuid.New().String()[:8]
}

// IsConcurrencySafeByDefinition checks if a tool is marked safe for concurrent execution.
// Low-risk tools default to concurrency-safe unless explicitly marked otherwise.
func IsConcurrencySafeByDefinition(toolName string) bool {
	if def, ok := GetToolDefinition(toolName); ok {
		if def.ConcurrencySafe {
			return true
		}
		// Default: all low-risk tools are safe for concurrent execution
		return def.RiskLevel == RiskLevelLow
	}
	return false
}

// BatchExecutor executes tool calls with safe parallelism
type BatchExecutor struct {
	mgr *Manager
}

// NewBatchExecutor creates a new BatchExecutor
func NewBatchExecutor(mgr *Manager) *BatchExecutor {
	return &BatchExecutor{mgr: mgr}
}

type indexedCall struct {
	Index int
	Call  ToolCall
}

// ExecuteConcurrent executes a batch of tool calls with safe parallelism.
// Safe tools run concurrently, unsafe tools run serially.
// Results are returned in the original call order.
func (be *BatchExecutor) ExecuteConcurrent(ctx context.Context, toolCalls []ToolCall) []ToolResult {
	results := make([]ToolResult, len(toolCalls))
	now := time.Now().Unix()

	// Assign UUIDs
	for idx := range toolCalls {
		if toolCalls[idx].ID == "" {
			toolCalls[idx].ID = generateToolID()
		}
	}

	// Partition calls into safe (parallel) and unsafe (serial)
	var safeCalls []indexedCall
	var unsafeCalls []indexedCall
	for i, call := range toolCalls {
		ic := indexedCall{Index: i, Call: call}
		if IsConcurrencySafeByDefinition(call.Tool) {
			safeCalls = append(safeCalls, ic)
		} else {
			unsafeCalls = append(unsafeCalls, ic)
		}
	}

	// Execute safe calls in parallel
	if len(safeCalls) > 0 {
		var wg sync.WaitGroup
		sem := make(chan struct{}, maxParallel)

		for _, ic := range safeCalls {
			wg.Add(1)
			sem <- struct{}{}

			go func(idx int, c ToolCall) {
				defer wg.Done()
				defer func() { <-sem }()

				select {
				case <-ctx.Done():
					results[idx] = ToolResult{
						ID: c.ID, Type: "tool_result", Tool: c.Tool,
						Status: "error", Error: "Canceled", Ts: now,
					}
					return
				default:
				}

				results[idx] = be.mgr.executeSingleWithCache(ctx, c, now)
			}(ic.Index, ic.Call)
		}

		wg.Wait()
	}

	// Execute unsafe calls serially
	for _, ic := range unsafeCalls {
		select {
		case <-ctx.Done():
			results[ic.Index] = ToolResult{
				ID: ic.Call.ID, Type: "tool_result", Tool: ic.Call.Tool,
				Status: "error", Error: "Canceled", Ts: now,
			}
			continue
		default:
		}
		results[ic.Index] = be.mgr.executeSingleWithCache(ctx, ic.Call, now)
	}

	return results
}

// ExecuteBatchPartitioned executes a batch of tool calls using the partitioned strategy.
func (m *Manager) ExecuteBatchPartitioned(ctx context.Context, toolCalls []ToolCall) []ToolResult {
	be := NewBatchExecutor(m)
	return be.ExecuteConcurrent(ctx, toolCalls)
}

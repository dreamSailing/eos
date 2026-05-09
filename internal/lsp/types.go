//go:build !without_lsp

package lsp

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import "encoding/json"

// LSP 相关类型定义

// JSONRPCVersion JSON-RPC 协议版本
const JSONRPCVersion = "2.0"

// Message JSON-RPC 基础消息
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error JSON-RPC 错误
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ErrorCodes LSP 定义的错误码
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603

	ServerNotInitialized = -32002
	UnknownErrorCode     = -32001
	RequestFailed        = -32803
	ServerCancelled      = -32802
	ContentModified      = -32801
	RequestCancelled     = -32800
)

// InitializeParams 初始化参数
type InitializeParams struct {
	ProcessID    int                `json:"processId,omitempty"`
	RootPath     string             `json:"rootPath,omitempty"`
	RootURI      string             `json:"rootUri,omitempty"`
	Capabilities ClientCapabilities `json:"capabilities"`
}

// ClientCapabilities 客户端能力
type ClientCapabilities struct {
	TextDocument TextDocumentClientCapabilities `json:"textDocument,omitempty"`
}

// TextDocumentClientCapabilities 文本文档能力
type TextDocumentClientCapabilities struct {
	Diagnostic DiagnosticCapabilities `json:"diagnostic,omitempty"`
}

// DiagnosticCapabilities 诊断能力
type DiagnosticCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// InitializeResult 初始化结果
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

// ServerCapabilities 服务器能力
type ServerCapabilities struct {
	TextDocumentSyncOptions any `json:"textDocumentSync,omitempty"`
	DiagnosticProvider      any `json:"diagnosticProvider,omitempty"`
	CompletionProvider      any `json:"completionProvider,omitempty"`
	DefinitionProvider      any `json:"definitionProvider,omitempty"`
	ReferencesProvider      any `json:"referencesProvider,omitempty"`
}

// DidOpenTextDocumentParams 打开文档通知参数
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// TextDocumentItem 文本文档项
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// DidChangeTextDocumentParams 修改文档通知参数
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

// VersionedTextDocumentIdentifier 版本化文档标识符
type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// TextDocumentContentChangeEvent 文档内容变更事件
type TextDocumentContentChangeEvent struct {
	Range       *Range `json:"range,omitempty"`
	RangeLength int    `json:"rangeLength,omitempty"`
	Text        string `json:"text"`
}

// DocumentURI 文档 URI
type DocumentURI string

// Position 位置
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range 范围
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location 位置
type Location struct {
	URI   DocumentURI `json:"uri"`
	Range Range       `json:"range"`
}

// Diagnostic 诊断信息
type Diagnostic struct {
	Range              Range                `json:"range"`
	Severity           DiagnosticSeverity   `json:"severity,omitempty"`
	Code               any                  `json:"code,omitempty"`
	Source             string               `json:"source,omitempty"`
	Message            string               `json:"message"`
	Tags               []DiagnosticTag      `json:"tags,omitempty"`
	RelatedInformation []RelatedInformation `json:"relatedInformation,omitempty"`
}

// DiagnosticSeverity 诊断严重程度
type DiagnosticSeverity int

const (
	SeverityError       DiagnosticSeverity = 1
	SeverityWarning     DiagnosticSeverity = 2
	SeverityInformation DiagnosticSeverity = 3
	SeverityHint        DiagnosticSeverity = 4
)

// DiagnosticTag 诊断标签
type DiagnosticTag int

const (
	TagUnnecessary DiagnosticTag = 1
	TagDeprecated  DiagnosticTag = 2
)

// RelatedInformation 相关信息
type RelatedInformation struct {
	Location Location `json:"location"`
	Message  string   `json:"message"`
}

// PublishDiagnosticsParams 发布诊断参数
type PublishDiagnosticsParams struct {
	URI         DocumentURI  `json:"uri"`
	Version     int          `json:"version"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// CompletionParams 补全参数
type CompletionParams struct {
	TextDocumentPositionParams `json:"textDocument"`
}

// TextDocumentPositionParams 文档位置参数
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// TextDocumentIdentifier 文档标识符
type TextDocumentIdentifier struct {
	URI DocumentURI `json:"uri"`
}

// CompletionItem 补全项
type CompletionItem struct {
	Label            string             `json:"label"`
	Kind             CompletionItemKind `json:"kind,omitempty"`
	Detail           string             `json:"detail,omitempty"`
	Documentation    any                `json:"documentation,omitempty"`
	SortText         string             `json:"sortText,omitempty"`
	FilterText       string             `json:"filterText,omitempty"`
	InsertText       string             `json:"insertText,omitempty"`
	InsertTextFormat InsertTextFormat   `json:"insertTextFormat,omitempty"`
}

// CompletionItemKind 补全项类型
type CompletionItemKind int

const (
	KindText          CompletionItemKind = 1
	KindMethod        CompletionItemKind = 2
	KindFunction      CompletionItemKind = 3
	KindConstructor   CompletionItemKind = 4
	KindField         CompletionItemKind = 5
	KindVariable      CompletionItemKind = 6
	KindClass         CompletionItemKind = 7
	KindInterface     CompletionItemKind = 8
	KindModule        CompletionItemKind = 9
	KindProperty      CompletionItemKind = 10
	KindUnit          CompletionItemKind = 11
	KindValue         CompletionItemKind = 12
	KindEnum          CompletionItemKind = 13
	KindKeyword       CompletionItemKind = 14
	KindSnippet       CompletionItemKind = 15
	KindColor         CompletionItemKind = 16
	KindFile          CompletionItemKind = 17
	KindReference     CompletionItemKind = 18
	KindFolder        CompletionItemKind = 19
	KindEnumMember    CompletionItemKind = 20
	KindConstant      CompletionItemKind = 21
	KindStruct        CompletionItemKind = 22
	KindEvent         CompletionItemKind = 23
	KindOperator      CompletionItemKind = 24
	KindTypeParameter CompletionItemKind = 25
)

// InsertTextFormat 插入文本格式
type InsertTextFormat int

const (
	PlainText InsertTextFormat = 1
	Snippet   InsertTextFormat = 2
)

// CompletionList 补全列表
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// DefinitionParams 定义参数
type DefinitionParams struct {
	TextDocumentPositionParams `json:"textDocument"`
}

// ReferenceParams 引用参数
type ReferenceParams struct {
	TextDocumentPositionParams `json:"textDocument"`
	Context                    ReferenceContext `json:"context"`
}

// ReferenceContext 引用上下文
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

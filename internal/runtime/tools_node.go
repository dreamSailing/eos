package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


// Tools node split into multiple files by functionality:
// - tools_node_desc.go: Tool description utilities (GetAvailableToolsDescription, GetToolNamesForDisplay)
// - tools_node_dispatch.go: Dispatch tool implementation (DispatchToolImpl)
// - tools_node_impl.go: Tool implementation (ToolImpl, InvokableRun)
// - tools_node_builder.go: Builder functions (BuildRuntimeTools, BuildRuntimeReadOnlyTools, BuildDispatchTools)
// - tools_node_exec.go: ToolsNode execution method (ToolsNode)

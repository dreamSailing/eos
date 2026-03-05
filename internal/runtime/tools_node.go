package runtime

// Tools node split into multiple files by functionality:
// - tools_node_desc.go: Tool description utilities (GetAvailableToolsDescription, GetToolNamesForDisplay)
// - tools_node_dispatch.go: Dispatch tool implementation (DispatchToolImpl)
// - tools_node_impl.go: Tool implementation (ToolImpl, InvokableRun)
// - tools_node_builder.go: Builder functions (BuildRuntimeTools, BuildRuntimeReadOnlyTools, BuildDispatchTools)
// - tools_node_exec.go: ToolsNode execution method (ToolsNode)

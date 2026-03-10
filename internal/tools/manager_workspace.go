package tools

func (m *Manager) SetWorkspaceRoot(root string) {
	if m == nil || m.fileOps == nil {
		return
	}
	m.fileOps.SetRoot(root)
}


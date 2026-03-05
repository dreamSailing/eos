package tools

import "testing"

func TestClassifyToolDanger_WriteIsDangerous(t *testing.T) {
	_, _, _, dangerous := ClassifyToolDanger(ToolCall{
		Tool: "fs",
		Parameters: map[string]interface{}{
			"mode": "write",
			"path": "a.txt",
		},
	})
	if !dangerous {
		t.Fatalf("expected dangerous=true")
	}
}

func TestClassifyToolDanger_EditIsDangerous(t *testing.T) {
	_, _, _, dangerous := ClassifyToolDanger(ToolCall{
		Tool: ToolEdit,
		Parameters: map[string]interface{}{
			"mode": "single",
		},
	})
	if !dangerous {
		t.Fatalf("expected dangerous=true")
	}
}


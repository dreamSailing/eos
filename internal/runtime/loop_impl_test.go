package runtime

import (
	"errors"
	"testing"
)

func TestSlidingWindowLoopDetector_SameReadArgsForceBreak(t *testing.T) {
	d := NewSlidingWindowLoopDetector()
	args := map[string]interface{}{"path": "internal/ui/panels_context.go"}
	var err error
	for i := 0; i < 4; i++ {
		err = d.CheckLoop("read", args)
	}
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrLoopForceBreak) {
		t.Fatalf("expected ErrLoopForceBreak, got %v", err)
	}
}


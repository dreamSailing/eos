package filedialog

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ErrUnavailable = errors.New("file dialog unavailable")
	ErrCanceled    = errors.New("file dialog canceled")
)

func IsUnavailable(err error) bool {
	return errors.Is(err, ErrUnavailable)
}

func IsCanceled(err error) bool {
	return errors.Is(err, ErrCanceled)
}

func normalizeDirectory(raw string) (string, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\r", ""))
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\n", ""))
	if raw == "" {
		return "", ErrCanceled
	}
	path, err := filepath.Abs(filepath.Clean(raw))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return path, nil
}

func lookPathAny(names ...string) (string, bool) {
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			continue
		}
		path, err := exec.LookPath(name)
		if err == nil && strings.TrimSpace(path) != "" {
			return path, true
		}
	}
	return "", false
}

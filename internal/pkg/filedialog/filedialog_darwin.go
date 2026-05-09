//go:build darwin

package filedialog

import (
	"fmt"
	"os/exec"
	"strings"
)

func ChooseDirectory(title string) (string, error) {
	bin, ok := lookPathAny("osascript")
	if !ok {
		return "", ErrUnavailable
	}
	script := fmt.Sprintf(`POSIX path of (choose folder with prompt %q)`, title)
	cmd := exec.Command(bin, "-e", script)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		lower := strings.ToLower(text)
		if strings.Contains(lower, "-128") || strings.Contains(lower, "user canceled") {
			return "", ErrCanceled
		}
		return "", fmt.Errorf("%w: %s", ErrUnavailable, text)
	}
	return normalizeDirectory(text)
}

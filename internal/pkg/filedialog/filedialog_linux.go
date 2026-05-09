//go:build linux

package filedialog

import (
	"fmt"
	"os/exec"
	"strings"
)

func ChooseDirectory(title string) (string, error) {
	type chooser struct {
		names []string
		args  []string
	}
	choices := []chooser{
		{names: []string{"zenity"}, args: []string{"--file-selection", "--directory", "--title", title}},
		{names: []string{"kdialog"}, args: []string{"--getexistingdirectory", ".", "--title", title}},
		{names: []string{"qarma"}, args: []string{"--file-selection", "--directory", "--title", title}},
	}
	available := false
	for _, choice := range choices {
		bin, ok := lookPathAny(choice.names...)
		if !ok {
			continue
		}
		available = true
		cmd := exec.Command(bin, choice.args...)
		out, err := cmd.CombinedOutput()
		text := strings.TrimSpace(string(out))
		if err == nil {
			return normalizeDirectory(text)
		}
		lower := strings.ToLower(text)
		switch {
		case strings.Contains(lower, "cannot open display"),
			strings.Contains(lower, "gtk-warning"),
			strings.Contains(lower, "no protocol specified"),
			strings.Contains(lower, "qt.qpa"),
			strings.Contains(lower, "display"):
			return "", fmt.Errorf("%w: %s", ErrUnavailable, text)
		case text == "":
			continue
		default:
			return "", fmt.Errorf("%w: %s", ErrCanceled, text)
		}
	}
	if !available {
		return "", ErrUnavailable
	}
	return "", ErrCanceled
}

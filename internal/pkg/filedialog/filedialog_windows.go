//go:build windows

package filedialog

import (
	"fmt"
	"os/exec"
	"strings"
)

func ChooseDirectory(title string) (string, error) {
	bin, ok := lookPathAny("powershell", "powershell.exe")
	if !ok {
		return "", ErrUnavailable
	}
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$dialog = New-Object System.Windows.Forms.FolderBrowserDialog
$dialog.Description = %q
$dialog.ShowNewFolderButton = $true
if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
  [Console]::Out.Write($dialog.SelectedPath)
}
`, title)
	cmd := exec.Command(bin, "-NoProfile", "-NonInteractive", "-STA", "-Command", script)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if strings.TrimSpace(text) == "" {
			return "", ErrCanceled
		}
		return "", fmt.Errorf("%w: %s", ErrUnavailable, text)
	}
	return normalizeDirectory(text)
}

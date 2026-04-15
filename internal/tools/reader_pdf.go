package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ReadPDF reads a PDF file and extracts text content.
// pages parameter format: "1-5", "10-20", or empty for all pages.
func ReadPDF(path string, pages string) (string, error) {
	// Check file exists
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("PDF file not found: %s", err)
	}

	// Check file size
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Size() > 50*1024*1024 { // 50MB limit
		return "", fmt.Errorf("PDF file too large (>%dMB)", 50)
	}

	// Try pdftotext first (poppler-utils)
	if text, err := extractWithPdftotext(path, pages); err == nil && text != "" {
		return text, nil
	}

	// Fallback: try python3 with PyPDF2/pdfplumber
	if text, err := extractWithPython(path, pages); err == nil && text != "" {
		return text, nil
	}

	// Last resort: return metadata
	return extractPDFMetadata(path)
}

func extractWithPdftotext(path, pages string) (string, error) {
	args := []string{}
	if pages != "" {
		first, last := parsePageRange(pages)
		if first > 0 {
			args = append(args, "-f", strconv.Itoa(first))
		}
		if last > 0 {
			args = append(args, "-l", strconv.Itoa(last))
		}
	}
	args = append(args, "-layout", path, "-")

	cmd := exec.Command("pdftotext", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	text := string(out)
	if len(text) > 100000 {
		text = text[:100000] + "\n... (truncated)"
	}
	return text, nil
}

func extractWithPython(path, pages string) (string, error) {
	first, last := parsePageRange(pages)

	script := `
import sys
try:
    import pdfplumber
    with pdfplumber.open(sys.argv[1]) as pdf:
        start = %d - 1 if %d > 0 else 0
        end = %d if %d > 0 else len(pdf.pages)
        for page in pdf.pages[start:end]:
            text = page.extract_text()
            if text:
                print(text)
except ImportError:
    try:
        from PyPDF2 import PdfReader
        reader = PdfReader(sys.argv[1])
        start = %d - 1 if %d > 0 else 0
        end = %d if %d > 0 else len(reader.pages)
        for page in reader.pages[start:end]:
            text = page.extract_text()
            if text:
                print(text)
    except ImportError:
        sys.exit(1)
`
	script = fmt.Sprintf(script, first, first, last, last, first, first, last, last)

	cmd := exec.Command("python3", "-c", script, path)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	text := string(out)
	if len(text) > 100000 {
		text = text[:100000] + "\n... (truncated)"
	}
	return text, nil
}

func extractPDFMetadata(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("[PDF File: %s, Size: %d bytes]\nNote: PDF text extraction requires pdftotext (poppler-utils) or python3 with pdfplumber/PyPDF2 installed.",
		filepath.Base(path), info.Size()), nil
}

func parsePageRange(pages string) (first, last int) {
	if pages == "" {
		return 0, 0
	}
	parts := strings.SplitN(pages, "-", 2)
	if len(parts) == 1 {
		n, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		return n, n
	}
	first, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
	last, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
	return
}

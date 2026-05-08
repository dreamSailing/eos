package document

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dreamSailing/eos/internal/pkg/utils"
)

func Convert(sourcePath string, opts ConversionOptions) (ConversionResult, error) {
	srcFormat := NormalizeFormat(sourcePath)
	targetFormat := NormalizeFormat(opts.TargetFormat)
	if srcFormat == "" {
		return ConversionResult{}, fmt.Errorf("unsupported source format: %s", sourcePath)
	}
	if targetFormat == "" {
		return ConversionResult{}, fmt.Errorf("unsupported target format: %s", opts.TargetFormat)
	}
	destination := strings.TrimSpace(opts.DestinationPath)
	if destination == "" {
		base := strings.TrimSuffix(sourcePath, filepath.Ext(sourcePath))
		destination = base + "." + targetFormat
	}
	fidelity := strings.ToLower(strings.TrimSpace(opts.Fidelity))
	if fidelity == "" {
		fidelity = "high"
	}

	result := ConversionResult{
		SourcePath:      sourcePath,
		DestinationPath: destination,
		SourceFormat:    srcFormat,
		TargetFormat:    targetFormat,
	}

	if srcFormat == targetFormat {
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return result, err
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return result, err
		}
		if err := os.WriteFile(destination, data, 0o644); err != nil {
			return result, err
		}
		result.UsedEngine = "copy"
		return result, nil
	}

	if fidelity == "high" {
		if ok, warn, err := convertWithSoffice(sourcePath, destination, targetFormat); err == nil && ok {
			result.UsedEngine = "soffice"
			if warn != "" {
				result.Warnings = append(result.Warnings, warn)
			}
			return result, nil
		} else if err == nil && !ok {
			if warn != "" {
				result.Warnings = append(result.Warnings, warn)
			}
		} else if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("high-fidelity conversion unavailable: %v", err))
		}
	}

	if err := fallbackConvert(sourcePath, destination, srcFormat, targetFormat); err != nil {
		return result, err
	}
	result.Degraded = fidelity == "high"
	result.UsedEngine = "content"
	if result.Degraded {
		result.Warnings = append(result.Warnings, "converted via content-level fallback; layout fidelity may be reduced")
	}
	return result, nil
}

func convertWithSoffice(sourcePath, destinationPath, targetFormat string) (bool, string, error) {
	sofficePath, err := exec.LookPath("soffice")
	if err != nil {
		return false, "soffice not found, falling back to content-level conversion", nil
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return false, "", err
	}
	outDir := filepath.Dir(destinationPath)
	cmd := utils.Command(sofficePath, "--headless", "--convert-to", targetFormat, "--outdir", outDir, sourcePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, "", fmt.Errorf("soffice convert failed: %s", strings.TrimSpace(string(output)))
	}
	produced := filepath.Join(outDir, strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))+"."+targetFormat)
	if produced != destinationPath {
		data, err := os.ReadFile(produced)
		if err != nil {
			return false, "", err
		}
		if err := os.WriteFile(destinationPath, data, 0o644); err != nil {
			return false, "", err
		}
		if produced != destinationPath {
			_ = os.Remove(produced)
		}
	}
	return true, strings.TrimSpace(string(output)), nil
}

func fallbackConvert(sourcePath, destinationPath, srcFormat, targetFormat string) error {
	switch srcFormat {
	case "docx":
		doc, err := ReadDOCX(sourcePath)
		if err != nil {
			return err
		}
		switch targetFormat {
		case "pdf":
			return WritePDF(destinationPath, doc)
		case "xlsx":
			return WriteXLSX(destinationPath, ToWorkbookModelFromDocument(doc))
		}
	case "pdf":
		doc, err := ReadPDF(sourcePath)
		if err != nil {
			return err
		}
		switch targetFormat {
		case "docx":
			return WriteDOCX(destinationPath, doc)
		case "xlsx":
			return WriteXLSX(destinationPath, ToWorkbookModelFromDocument(doc))
		}
	case "xlsx":
		book, err := ReadXLSX(sourcePath)
		if err != nil {
			return err
		}
		switch targetFormat {
		case "pdf":
			return WritePDF(destinationPath, ToDocumentModelFromWorkbook(book))
		case "docx":
			return WriteDOCX(destinationPath, ToDocumentModelFromWorkbook(book))
		}
	}
	return fmt.Errorf("unsupported conversion: %s -> %s", srcFormat, targetFormat)
}

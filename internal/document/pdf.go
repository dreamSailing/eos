package document

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-pdf/fpdf"
	pdfreader "github.com/ledongthuc/pdf"
)

func ReadPDF(path string) (DocumentModel, error) {
	f, r, err := pdfreader.Open(path)
	if err != nil {
		return DocumentModel{}, fmt.Errorf("failed to open pdf: %w", err)
	}
	defer f.Close()

	textReader, err := r.GetPlainText()
	if err != nil {
		return DocumentModel{}, fmt.Errorf("failed to extract pdf text: %w", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(textReader); err != nil {
		return DocumentModel{}, fmt.Errorf("failed to read pdf text: %w", err)
	}
	model := DocumentFromText("", buf.String())
	if model.Metadata == nil {
		model.Metadata = map[string]string{}
	}
	model.Metadata["source_format"] = "pdf"
	return model, nil
}

func WritePDF(path string, model DocumentModel) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()
	pdf.SetFont("Arial", "", 12)

	if strings.TrimSpace(model.Title) != "" {
		pdf.SetFont("Arial", "B", 18)
		pdf.MultiCell(0, 10, model.Title, "", "L", false)
		pdf.Ln(2)
		pdf.SetFont("Arial", "", 12)
	}

	for _, block := range model.Blocks {
		switch block.Type {
		case BlockHeading:
			size := 16.0
			if block.Level >= 2 {
				size = 14
			}
			if block.Level >= 3 {
				size = 13
			}
			pdf.SetFont("Arial", "B", size)
			pdf.MultiCell(0, 8, block.Text, "", "L", false)
			pdf.Ln(1)
			pdf.SetFont("Arial", "", 12)
		case BlockParagraph:
			pdf.MultiCell(0, 6, block.Text, "", "L", false)
			pdf.Ln(2)
		case BlockTable:
			writePDFTable(pdf, block.Rows)
			pdf.Ln(3)
		}
	}
	return pdf.OutputFileAndClose(path)
}

func writePDFTable(pdf *fpdf.Fpdf, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	maxCols := 1
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	pageWidth, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	usable := pageWidth - left - right
	colWidth := usable / float64(maxCols)
	for _, row := range rows {
		startX := pdf.GetX()
		startY := pdf.GetY()
		maxHeight := 8.0
		heights := make([]float64, maxCols)
		for i := 0; i < maxCols; i++ {
			text := ""
			if i < len(row) {
				text = row[i]
			}
			lines := pdf.SplitLines([]byte(text), colWidth-2)
			heights[i] = float64(len(lines))*5 + 3
			if heights[i] > maxHeight {
				maxHeight = heights[i]
			}
		}
		x := startX
		for i := 0; i < maxCols; i++ {
			text := ""
			if i < len(row) {
				text = row[i]
			}
			pdf.Rect(x, startY, colWidth, maxHeight, "")
			pdf.SetXY(x+1, startY+1)
			pdf.MultiCell(colWidth-2, 5, text, "", "L", false)
			x += colWidth
			pdf.SetXY(x, startY)
		}
		pdf.SetXY(startX, startY+maxHeight)
	}
}

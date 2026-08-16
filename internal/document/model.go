package document

import (
	"fmt"
	"path/filepath"
	"strings"
)

type BlockType string

const (
	BlockParagraph BlockType = "paragraph"
	BlockHeading   BlockType = "heading"
	BlockTable     BlockType = "table"
)

type DocumentBlock struct {
	Type  BlockType  `json:"type"`
	Text  string     `json:"text,omitempty"`
	Level int        `json:"level,omitempty"`
	Rows  [][]string `json:"rows,omitempty"`
}

type DocumentModel struct {
	Title    string            `json:"title,omitempty"`
	Blocks   []DocumentBlock   `json:"blocks,omitempty"`
	Warnings []string          `json:"warnings,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type WorkbookSheet struct {
	Name string     `json:"name"`
	Rows [][]string `json:"rows,omitempty"`
}

type WorkbookModel struct {
	Title    string            `json:"title,omitempty"`
	Sheets   []WorkbookSheet   `json:"sheets,omitempty"`
	Warnings []string          `json:"warnings,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type ConversionOptions struct {
	DestinationPath string
	TargetFormat    string
	Fidelity        string
}

type ConversionResult struct {
	SourcePath      string   `json:"source_path,omitempty"`
	DestinationPath string   `json:"destination_path,omitempty"`
	SourceFormat    string   `json:"source_format,omitempty"`
	TargetFormat    string   `json:"target_format,omitempty"`
	UsedEngine      string   `json:"used_engine,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
}

func (m DocumentModel) PlainText() string {
	var parts []string
	if strings.TrimSpace(m.Title) != "" {
		parts = append(parts, strings.TrimSpace(m.Title))
	}
	for _, block := range m.Blocks {
		switch block.Type {
		case BlockHeading, BlockParagraph:
			if strings.TrimSpace(block.Text) != "" {
				parts = append(parts, strings.TrimSpace(block.Text))
			}
		case BlockTable:
			for _, row := range block.Rows {
				parts = append(parts, strings.Join(row, " | "))
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func (m WorkbookModel) PlainText() string {
	var parts []string
	if strings.TrimSpace(m.Title) != "" {
		parts = append(parts, strings.TrimSpace(m.Title))
	}
	for i, sheet := range m.Sheets {
		// 默认名用真实表序号（i+1），而不是已输出的行数。
		parts = append(parts, fmt.Sprintf("# Sheet: %s", sheetNameOrDefault(sheet.Name, i+1)))
		for _, row := range sheet.Rows {
			parts = append(parts, strings.Join(row, "\t"))
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func (m WorkbookModel) FirstSheetName() string {
	if len(m.Sheets) == 0 {
		return "Sheet1"
	}
	return sheetNameOrDefault(m.Sheets[0].Name, 1)
}

func NormalizeFormat(pathOrFormat string) string {
	s := strings.ToLower(strings.TrimSpace(pathOrFormat))
	if s == "" {
		return ""
	}
	switch s {
	case "docx", ".docx":
		return "docx"
	case "xlsx", ".xlsx":
		return "xlsx"
	case "pdf", ".pdf":
		return "pdf"
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(s)), ".")
	switch ext {
	case "docx", "xlsx", "pdf":
		return ext
	default:
		return ""
	}
}

func sheetNameOrDefault(name string, index int) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	if index <= 0 {
		index = 1
	}
	return fmt.Sprintf("Sheet%d", index)
}

func DocumentFromText(title, content string) DocumentModel {
	model := DocumentModel{Title: strings.TrimSpace(title)}
	for _, chunk := range splitParagraphs(content) {
		model.Blocks = append(model.Blocks, DocumentBlock{Type: BlockParagraph, Text: chunk})
	}
	if len(model.Blocks) == 0 && strings.TrimSpace(model.Title) != "" {
		model.Blocks = append(model.Blocks, DocumentBlock{Type: BlockParagraph, Text: model.Title})
	}
	return model
}

func WorkbookFromText(title, content string) WorkbookModel {
	model := WorkbookModel{Title: strings.TrimSpace(title)}
	sheet := WorkbookSheet{Name: "Sheet1"}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sheet.Rows = append(sheet.Rows, []string{line})
	}
	if len(sheet.Rows) == 0 {
		sheet.Rows = append(sheet.Rows, []string{""})
	}
	model.Sheets = []WorkbookSheet{sheet}
	return model
}

func splitParagraphs(content string) []string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	chunks := strings.Split(normalized, "\n\n")
	out := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		out = append(out, chunk)
	}
	if len(out) == 0 && strings.TrimSpace(content) != "" {
		out = append(out, strings.TrimSpace(content))
	}
	return out
}

func ToDocumentModelFromWorkbook(book WorkbookModel) DocumentModel {
	doc := DocumentModel{Title: book.Title, Warnings: append([]string{}, book.Warnings...), Metadata: book.Metadata}
	for i, sheet := range book.Sheets {
		doc.Blocks = append(doc.Blocks, DocumentBlock{Type: BlockHeading, Level: 1, Text: sheetNameOrDefault(sheet.Name, i+1)})
		doc.Blocks = append(doc.Blocks, DocumentBlock{Type: BlockTable, Rows: sheet.Rows})
	}
	return doc
}

func ToWorkbookModelFromDocument(doc DocumentModel) WorkbookModel {
	book := WorkbookModel{Title: doc.Title, Warnings: append([]string{}, doc.Warnings...), Metadata: doc.Metadata}
	current := WorkbookSheet{Name: "Sheet1"}
	flush := func() {
		if len(current.Rows) == 0 && strings.TrimSpace(current.Name) == "" {
			return
		}
		if len(current.Rows) == 0 {
			current.Rows = append(current.Rows, []string{""})
		}
		book.Sheets = append(book.Sheets, current)
	}
	hasContent := false
	for _, block := range doc.Blocks {
		switch block.Type {
		case BlockHeading:
			if hasContent {
				flush()
				current = WorkbookSheet{}
			}
			current.Name = block.Text
		case BlockTable:
			hasContent = true
			if len(block.Rows) == 0 {
				current.Rows = append(current.Rows, []string{""})
				continue
			}
			current.Rows = append(current.Rows, block.Rows...)
		case BlockParagraph:
			hasContent = true
			for _, line := range strings.Split(block.Text, "\n") {
				current.Rows = append(current.Rows, []string{strings.TrimSpace(line)})
			}
		}
	}
	flush()
	if len(book.Sheets) == 0 {
		book.Sheets = []WorkbookSheet{{Name: "Sheet1", Rows: [][]string{{""}}}}
	}
	for i := range book.Sheets {
		book.Sheets[i].Name = sheetNameOrDefault(book.Sheets[i].Name, i+1)
	}
	return book
}

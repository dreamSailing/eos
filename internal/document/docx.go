package document

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ReadDOCX(path string) (DocumentModel, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return DocumentModel{}, fmt.Errorf("failed to open docx: %w", err)
	}
	defer zr.Close()

	var data []byte
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return DocumentModel{}, fmt.Errorf("failed to open word/document.xml: %w", err)
			}
			data, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return DocumentModel{}, fmt.Errorf("failed to read word/document.xml: %w", err)
			}
			break
		}
	}
	if len(data) == 0 {
		return DocumentModel{}, fmt.Errorf("docx missing word/document.xml")
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	model := DocumentModel{Metadata: map[string]string{"source_format": "docx"}}
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return model, fmt.Errorf("failed to parse docx xml: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "p":
			block, err := parseDocxParagraph(decoder, start)
			if err != nil {
				return model, err
			}
			if block.Type != "" && (strings.TrimSpace(block.Text) != "" || len(block.Rows) > 0) {
				model.Blocks = append(model.Blocks, block)
			}
		case "tbl":
			rows, err := parseDocxTable(decoder, start)
			if err != nil {
				return model, err
			}
			if len(rows) > 0 {
				model.Blocks = append(model.Blocks, DocumentBlock{Type: BlockTable, Rows: rows})
			}
		}
	}
	return model, nil
}

func WriteDOCX(path string, model DocumentModel) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	parts := map[string]string{
		"[Content_Types].xml":          contentTypesXML(),
		"_rels/.rels":                  rootRelsXML(),
		"word/_rels/document.xml.rels": documentRelsXML(),
		"word/styles.xml":              stylesXML(),
		"word/document.xml":            buildDocumentXML(model),
	}
	for name, content := range parts {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, strings.NewReader(content)); err != nil {
			return err
		}
	}
	return zw.Close()
}

func parseDocxParagraph(decoder *xml.Decoder, start xml.StartElement) (DocumentBlock, error) {
	block := DocumentBlock{Type: BlockParagraph}
	var text strings.Builder
	level := 0
	for {
		tok, err := decoder.Token()
		if err != nil {
			return DocumentBlock{}, fmt.Errorf("failed to parse paragraph: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "pStyle":
				for _, attr := range t.Attr {
					if attr.Name.Local == "val" && strings.HasPrefix(strings.ToLower(attr.Value), "heading") {
						if n, convErr := strconv.Atoi(strings.TrimPrefix(strings.ToLower(attr.Value), "heading")); convErr == nil {
							level = n
						} else {
							level = 1
						}
					}
				}
			case "t":
				var value string
				if err := decoder.DecodeElement(&value, &t); err != nil {
					return DocumentBlock{}, fmt.Errorf("failed to decode paragraph text: %w", err)
				}
				text.WriteString(value)
			case "tab":
				text.WriteString("\t")
			case "br":
				text.WriteString("\n")
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				block.Text = strings.TrimSpace(text.String())
				if level > 0 {
					block.Type = BlockHeading
					block.Level = level
				}
				return block, nil
			}
		}
	}
}

func parseDocxTable(decoder *xml.Decoder, start xml.StartElement) ([][]string, error) {
	var rows [][]string
	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("failed to parse table: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "tr" {
				row, err := parseDocxTableRow(decoder, t)
				if err != nil {
					return nil, err
				}
				rows = append(rows, row)
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return rows, nil
			}
		}
	}
}

func parseDocxTableRow(decoder *xml.Decoder, start xml.StartElement) ([]string, error) {
	var row []string
	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("failed to parse table row: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "tc" {
				cell, err := parseDocxTableCell(decoder, t)
				if err != nil {
					return nil, err
				}
				row = append(row, cell)
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return row, nil
			}
		}
	}
}

func parseDocxTableCell(decoder *xml.Decoder, start xml.StartElement) (string, error) {
	var lines []string
	for {
		tok, err := decoder.Token()
		if err != nil {
			return "", fmt.Errorf("failed to parse table cell: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "p" {
				block, err := parseDocxParagraph(decoder, t)
				if err != nil {
					return "", err
				}
				if strings.TrimSpace(block.Text) != "" {
					lines = append(lines, block.Text)
				}
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return strings.Join(lines, "\n"), nil
			}
		}
	}
}

func buildDocumentXML(model DocumentModel) string {
	var body strings.Builder
	if strings.TrimSpace(model.Title) != "" {
		body.WriteString(buildParagraphXML(model.Title, 1, true))
	}
	for _, block := range model.Blocks {
		switch block.Type {
		case BlockHeading:
			level := block.Level
			if level <= 0 {
				level = 1
			}
			body.WriteString(buildParagraphXML(block.Text, level, true))
		case BlockParagraph:
			for _, part := range splitParagraphs(block.Text) {
				body.WriteString(buildParagraphXML(part, 0, false))
			}
		case BlockTable:
			body.WriteString(buildTableXML(block.Rows))
		}
	}
	if body.Len() == 0 {
		body.WriteString(buildParagraphXML("", 0, false))
	}
	body.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="708" w:footer="708" w:gutter="0"/></w:sectPr>`)
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:wpc="http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" xmlns:o="urn:schemas-microsoft-com:office:office" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:wp14="http://schemas.microsoft.com/office/word/2010/wordprocessingDrawing" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:w10="urn:schemas-microsoft-com:office:word" xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml" xmlns:wpg="http://schemas.microsoft.com/office/word/2010/wordprocessingGroup" xmlns:wpi="http://schemas.microsoft.com/office/word/2010/wordprocessingInk" xmlns:wne="http://schemas.microsoft.com/office/word/2006/wordml" xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" mc:Ignorable="w14 wp14"><w:body>` + body.String() + `</w:body></w:document>`
}

func buildParagraphXML(text string, level int, heading bool) string {
	var style string
	if heading {
		if level <= 0 {
			level = 1
		}
		style = fmt.Sprintf(`<w:pPr><w:pStyle w:val="Heading%d"/></w:pPr>`, level)
	}
	escaped := escapeXMLText(text)
	escaped = strings.ReplaceAll(escaped, "\n", `</w:t><w:br/><w:t>`)
	return `<w:p>` + style + `<w:r><w:t xml:space="preserve">` + escaped + `</w:t></w:r></w:p>`
}

func buildTableXML(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/><w:tblBorders><w:top w:val="single" w:sz="4" w:space="0" w:color="auto"/><w:left w:val="single" w:sz="4" w:space="0" w:color="auto"/><w:bottom w:val="single" w:sz="4" w:space="0" w:color="auto"/><w:right w:val="single" w:sz="4" w:space="0" w:color="auto"/><w:insideH w:val="single" w:sz="4" w:space="0" w:color="auto"/><w:insideV w:val="single" w:sz="4" w:space="0" w:color="auto"/></w:tblBorders></w:tblPr>`)
	for _, row := range rows {
		sb.WriteString(`<w:tr>`)
		for _, cell := range row {
			sb.WriteString(`<w:tc><w:p><w:r><w:t xml:space="preserve">` + escapeXMLText(cell) + `</w:t></w:r></w:p></w:tc>`)
		}
		sb.WriteString(`</w:tr>`)
	}
	sb.WriteString(`</w:tbl>`)
	return sb.String()
}

func contentTypesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`
}

func rootRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`
}

func documentRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`
}

func stylesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>
  <w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:qFormat/><w:rPr><w:b/><w:sz w:val="32"/></w:rPr></w:style>
  <w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:qFormat/><w:rPr><w:b/><w:sz w:val="28"/></w:rPr></w:style>
  <w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:qFormat/><w:rPr><w:b/><w:sz w:val="24"/></w:rPr></w:style>
</w:styles>`
}

func escapeXMLText(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

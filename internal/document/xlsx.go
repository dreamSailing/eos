package document

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

func ReadXLSX(path string) (WorkbookModel, error) {
	file, err := excelize.OpenFile(path)
	if err != nil {
		return WorkbookModel{}, fmt.Errorf("failed to open xlsx: %w", err)
	}
	defer file.Close()

	model := WorkbookModel{Metadata: map[string]string{"source_format": "xlsx"}}
	for _, sheetName := range file.GetSheetList() {
		rows, err := file.GetRows(sheetName)
		if err != nil {
			return WorkbookModel{}, fmt.Errorf("failed to read sheet %s: %w", sheetName, err)
		}
		sheet := WorkbookSheet{Name: sheetName}
		for _, row := range rows {
			copied := make([]string, len(row))
			copy(copied, row)
			sheet.Rows = append(sheet.Rows, copied)
		}
		model.Sheets = append(model.Sheets, sheet)
	}
	if len(model.Sheets) == 0 {
		model.Sheets = append(model.Sheets, WorkbookSheet{Name: "Sheet1", Rows: [][]string{{""}}})
	}
	return model, nil
}

func WriteXLSX(path string, model WorkbookModel) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f := excelize.NewFile()
	defer f.Close()

	defaultSheet := f.GetSheetName(0)
	if defaultSheet == "" {
		defaultSheet = "Sheet1"
	}

	sheets := model.Sheets
	if len(sheets) == 0 {
		sheets = []WorkbookSheet{{Name: "Sheet1", Rows: [][]string{{""}}}}
	}

	for i, sheet := range sheets {
		name := sheetNameOrDefault(sheet.Name, i+1)
		actualName := name
		if i == 0 {
			if err := f.SetSheetName(defaultSheet, name); err != nil {
				actualName = defaultSheet
			}
		} else {
			idx, err := f.NewSheet(name)
			if err != nil {
				return err
			}
			f.SetActiveSheet(idx)
		}
		targetSheet := actualName
		if i > 0 {
			targetSheet = name
		}
		if len(sheet.Rows) == 0 {
			sheet.Rows = [][]string{{""}}
		}
		for r, row := range sheet.Rows {
			for c, cell := range row {
				axis, err := excelize.CoordinatesToCellName(c+1, r+1)
				if err != nil {
					return err
				}
				if err := f.SetCellValue(targetSheet, axis, cell); err != nil {
					return err
				}
			}
		}
	}
	return f.SaveAs(path)
}

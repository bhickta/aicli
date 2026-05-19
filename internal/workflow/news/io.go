package news

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

func loadItems(path string) ([]Item, error) {
	if strings.EqualFold(filepath.Ext(path), ".xlsx") {
		return parseXLSX(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseItems(data)
}

func parseItems(data []byte) ([]Item, error) {
	var items []Item
	if err := json.Unmarshal(data, &items); err == nil {
		return items, nil
	}
	var wrapped struct {
		Items []Item `json:"items"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, err
	}
	return wrapped.Items, nil
}

func parseXLSX(path string) ([]Item, error) {
	file, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return []Item{}, nil
	}
	rows, err := file.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []Item{}, nil
	}
	header := map[string]int{}
	for i, cell := range rows[0] {
		header[normalize(cell)] = i
	}
	items := []Item{}
	for _, row := range rows[1:] {
		items = append(items, Item{
			Title:   cell(row, header["title"]),
			Content: cell(row, header["content"]),
			Source:  cell(row, header["source"]),
			Date:    cell(row, header["date"]),
		})
	}
	return items, nil
}

func exportXLSX(path string, items []Item) error {
	file := excelize.NewFile()
	sheet := file.GetSheetName(0)
	headers := []string{"title", "content", "source", "date"}
	for i, header := range headers {
		cellName, _ := excelize.CoordinatesToCellName(i+1, 1)
		file.SetCellValue(sheet, cellName, header)
	}
	for rowIndex, item := range items {
		values := []string{item.Title, item.Content, item.Source, item.Date}
		for colIndex, value := range values {
			cellName, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex+2)
			file.SetCellValue(sheet, cellName, value)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return file.SaveAs(path)
}

func cell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return row[index]
}

package pdf

import (
	"math"
	"regexp"
	"strings"
)

type tableRow struct {
	cellByCol map[int]string
}

func newRow() *tableRow {
	return &tableRow{
		cellByCol: make(map[int]string),
	}
}
func (r *tableRow) cellCount() int {
	return len(r.cellByCol)
}
func (r *tableRow) addCell(cell string, col int) {
	r.cellByCol[col] = cell
}
func (r *tableRow) isEmpty() bool {
	for _, cell := range r.cellByCol {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}
func (r *tableRow) cells() []string {
	var cells []string
	for i := 0; i < len(r.cellByCol); i++ {
		cells = append(cells, r.cellByCol[i])
	}
	return cells
}

// TableDef represents a table in the PDF file
type TableDef struct {
	StartRegex *regexp.Regexp
	EndRegex   *regexp.Regexp
	Columns    []*ColumnDef
	startPoint Point
	endPoint   Point
}

// ColumnDef represents a column in a table
type ColumnDef struct {
	Header *regexp.Regexp
	x      float64
}

func NewColumnDef(header *regexp.Regexp) *ColumnDef {
	return &ColumnDef{
		Header: header,
	}
}

func (table *TableDef) getRelevantPages(reader *Reader) ([]Page, error) {
	var pages []Page
	count := reader.NumPage()
	for i := 1; i <= count; i++ {
		page := reader.Page(i)
		pageText, err := page.GetPlainText(nil)
		if err != nil {
			return nil, err
		}

		if !table.StartRegex.MatchString(pageText) {
			continue
		}
		pages = append(pages, page)

		if table.EndRegex.MatchString(pageText) {
			break
		}
	}

	return pages, nil
}
func (table *TableDef) isColumnHeader(text string) bool {
	for i := 0; i < len(table.Columns); i++ {
		column := table.Columns[i]
		if column.Header.MatchString(strings.TrimSpace(text)) {
			return true
		}
	}
	return false
}
func (table *TableDef) getColumn(header string) *ColumnDef {
	var column *ColumnDef
	for _, c := range table.Columns {
		if c.Header.MatchString(header) {
			column = c
			break
		}
	}
	return column
}

// parse finds and parses the table in the PDF
func (table *TableDef) parse(reader *Reader, action func(cells []string)) error {
	pages, err := table.getRelevantPages(reader)
	if err != nil {
		return err
	}

	count := len(pages)
	for i := range pages {
		page := pages[i]
		texts := page.GetTextByRect(func(point Point, accumText string) {
			if table.endPoint.Y == 0 &&
				table.EndRegex.MatchString(accumText) {
				table.endPoint = point
			} else if table.startPoint.Y == 0 &&
				table.StartRegex.MatchString(accumText) {
				table.startPoint = point
			}
		})

		var pageTexts []*RectText
		for _, text := range texts {
			if text.S == "" ||
				(i == 0 && text.Rect.Min.Y > table.startPoint.Y) || // If first page, check whether the rect is before the table
				(i == (count-1) && (text.Rect.Max.Y < table.endPoint.Y)) { // If last page, check whether the rect is after the table
				continue
			}
			if table.isColumnHeader(text.S) {
				column := table.getColumn(strings.TrimSpace(text.S))
				column.x = text.Rect.Min.X
				continue
			}

			pageTexts = append(pageTexts, text)
		}

		count := len(table.Columns)
		var rows []*tableRow
		currentRow := newRow()
		for _, text := range pageTexts {
			for i := range table.Columns {
				column := table.Columns[i]
				diff := math.Abs(text.Rect.Min.X - column.x)
				if text.Rect.Min.X == column.x || diff <= 1 {
					if currentRow.cellCount() == count {
						if !currentRow.isEmpty() {
							rows = append(rows, currentRow)
						}
						currentRow = newRow()
					}
					currentRow.addCell(strings.TrimSpace(text.S), i)
					break
				}
			}
		}
		if !currentRow.isEmpty() {
			rows = append(rows, currentRow)
		}

		for i := range rows {
			row := rows[i]
			action(row.cells())
		}
	}

	return nil
}

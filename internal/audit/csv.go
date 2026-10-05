package audit

import (
	"encoding/csv"
	"io"
	"strings"
)

type csvWriter struct{ w *csv.Writer }

func newCSV(w io.Writer) *csvWriter { return &csvWriter{csv.NewWriter(w)} }

// row writes one CSV row. Cells that a spreadsheet would run as a formula
// (starting =, +, -, @) get a leading apostrophe.
func (c *csvWriter) row(cells ...string) {
	for i, cell := range cells {
		if cell != "" && strings.ContainsRune("=+-@", rune(cell[0])) {
			cells[i] = "'" + cell
		}
	}
	_ = c.w.Write(cells)
}

func (c *csvWriter) flush() error {
	c.w.Flush()
	return c.w.Error()
}

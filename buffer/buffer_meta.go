package buffer

import (
	"github.com/ge-editor/editorview/file"
	"github.com/ge-editor/editorview/mark"
)

func newMeta() *Meta {
	return &Meta{
		Cursor: file.Cursor{
			RowIndex: 0,
			ColIndex: 0,
		},
		Cx:                  0,
		Cy:                  0,
		PrevCx:              0, // Horizontal position of the cursor when vertically moving the cursor
		PrevDrawnY:          0, // Up to which line number was drawn
		PrevRowIndex:        0, // When the logical number of lines increases
		PrevNumberOfLogical: 0, // When the logical number of lines increases
		PrevLogicalCY:       0, // When the logical number of lines increases
		ModelineCx:          0, // Number of columns to display
		Mark:                nil,
	}
}

type Meta struct {
	file.Cursor
	Cx                  int
	Cy                  int
	PrevCx              int // Horizontal position of the cursor when vertically moving the cursor
	PrevDrawnY          int // Up to which line number was drawn
	PrevRowIndex        int // When the logical number of lines increases
	PrevNumberOfLogical int // When the logical number of lines increases
	PrevLogicalCY       int // When the logical number of lines increases
	ModelineCx          int // Number of columns to display in the modeline
	Mark                *mark.Mark

	StartDrawRowIndex     int
	StartDrawLogicalIndex int
	EndDrawRowIndex       int
	// EndDrawLogicalIndex   int
}

package file

// Create new Cursor struct and return
func NewCursor(row, col int) Cursor {
	return Cursor{
		RowIndex: row,
		ColIndex: col,
	}
}

type Cursor struct {
	RowIndex int
	ColIndex int
}

func (c Cursor) Equals(other Cursor) bool {
	return c.RowIndex == other.RowIndex && c.ColIndex == other.ColIndex
}

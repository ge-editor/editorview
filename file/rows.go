package file

import (
	"unicode/utf8"
)

type row []byte

func (l *row) IsColIndexAtRowEnd(colIndex int) bool {
	_, size := utf8.DecodeRune((*l)[colIndex:])
	return len(*l) == colIndex+size
}

type rows [][]byte

// New creates a new buffer with an initial capacity of 64
func (r *rows) New() {
	*r = make(rows, 0, 64)
}

// AddRow adds a new []byte to the lines **slices**
func (r *rows) AddRow(data []byte) {
	*r = append(*r, data)
}

// InsertRow inserts a new line at the specified index
func (r *rows) InsertRow(rowIndex int, newRow []byte) bool {
	if rowIndex < 0 || rowIndex > len(*r) {
		return false
	}
	// *l = slices.Insert(*l, index, newLine)
	*r = append(*r, nil)                     // Expand the slice by one element
	copy((*r)[rowIndex+1:], (*r)[rowIndex:]) // Shift elements to the right
	(*r)[rowIndex] = newRow                  // Set the new line
	return true
}

// SetRow sets the content of a specific line by index
func (r *rows) SetRow(rowIndex int, row []byte) bool {
	if rowIndex < 0 || rowIndex >= len(*r) {
		return false
	}
	(*r)[rowIndex] = row
	return true
}

// GetRow retrieves a []byte from the lines by index
func (r *rows) GetRow(index int) (*row, bool) {
	if index < 0 || index >= len(*r) {
		return nil, false
	}
	return (*row)(&(*r)[index]), true
}

// RemoveLine removes a []byte from the lines by index
func (r *rows) RemoveRow(rowIndex int) bool {
	if rowIndex < 0 || rowIndex >= len(*r) {
		return false
	}
	*r = append((*r)[:rowIndex], (*r)[rowIndex+1:]...)
	return true
}

func (r *rows) IsRowIndexLastRow(rowIndex int) bool {
	return len(*r)-1 == rowIndex
}

// RowLength returns the number of lines
func (r *rows) RowLength() int {
	return len(*r)
}

// Clear removes all lines
func (r *rows) Clear() {
	*r = (*r)[:0]
}

// InsertToCol inserts a byte slice into a specific line at the specified position
func (r *rows) InsertToCol(rowIndex int, insertIndex int, data []byte) bool {
	if rowIndex < 0 || rowIndex >= len(*r) {
		return false
	}
	if insertIndex < 0 || insertIndex > len((*r)[rowIndex]) {
		return false
	}
	row := (*r)[rowIndex]
	(*r)[rowIndex] = append(row[:insertIndex], append(data, row[insertIndex:]...)...)
	return true
}

// AddToRow adds a byte to a specific []byte by index
func (r *rows) AddToRow(index int, b []byte) bool {
	if index < 0 || index >= len(*r) {
		return false
	}
	(*r)[index] = append((*r)[index], b...)
	return true
}

// GetColLength returns the length of a specific []byte by index
func (r *rows) GetColLength(rowIndex int) (int, bool) {
	if rowIndex < 0 || rowIndex >= len(*r) {
		return 0, false
	}
	return len((*r)[rowIndex]), true
}

func (r *rows) String(rowIndex int) (string, bool) {
	if rowIndex < 0 || rowIndex >= len(*r) {
		return "", false
	}
	return string((*r)[rowIndex]), true
}

// DecodeRune decodes a rune from the specified line and position
func (r *rows) DecodeRune(rowIndex int, colIndex int) (ch rune, size int, ok bool) {
	if rowIndex < 0 || rowIndex >= len(*r) {
		return 0, 0, false
	}
	if colIndex < 0 || colIndex >= len((*r)[rowIndex]) {
		return 0, 0, false
	}
	ch, size = utf8.DecodeRune((*r)[rowIndex][colIndex:])
	if ch == utf8.RuneError && size == 1 { // encoding is invalid
		return 0, 0, false
	}
	return ch, size, true
}

func (r *rows) DecodeEndRune(rowIndex int) (ch rune, size, colIndex int, ok bool) {
	return r.DecodePrevRune(rowIndex, len((*r)[rowIndex])) // +1
}

// DecodePrevRune decodes the previous rune from the specified line and position
// i: colIndex
func (r *rows) DecodePrevRune(rowIndex int, colIndex int) (ch rune, size, i int, ok bool) {
	if rowIndex < 0 || rowIndex >= len(*r) {
		return 0, 0, 0, false
	}
	if colIndex <= 0 || colIndex > len((*r)[rowIndex]) {
		return 0, 0, 0, false
	}
	// Move back to find the start of the previous rune
	i = colIndex - 1
	for i >= 0 && !utf8.RuneStart((*r)[rowIndex][i]) {
		i--
	}
	if i < 0 {
		return 0, 0, 0, false
	}
	ch, size = utf8.DecodeRune((*r)[rowIndex][i:])
	if ch == utf8.RuneError && size == 1 {
		return 0, 0, 0, false
	}
	if colIndex-i != size {
		return 0, 0, 0, false
	}
	return ch, size, i, true
}

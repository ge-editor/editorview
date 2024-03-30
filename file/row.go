package file

import (
	"unicode/utf8"
)

type Row []rune

func NewRow() *Row {
	r := make(Row, 0, 64) // cap is...
	return &r
}

func (m *Row) Ch(index int) rune {
	return (*m)[index]
}

// Return the indented string at the beginning of the line
func (m *Row) BigginingSpaces() (runes []rune) {
	for _, ch := range *m {
		if ch == ' ' || ch == '\t' {
			runes = append(runes, ch)
			continue
		}
		return
	}
	return
}

// end of line determination
func (m *Row) IsEndOfRow(colIndex int) bool {
	return colIndex == m.LenCh()-1
}

// Return length of the Row
func (m *Row) LenCh() int {
	return len(*m)
}

// Convert []byte to []rune and append to row.
func (m *Row) bytes(b []byte) {
	for i := 0; i < len(b); {
		c, size := utf8.DecodeRune(b[i:])
		i += size
		m.append(c)
	}
}

func (m *Row) String() string {
	s := ""
	for _, c := range *m {
		s += string(c)
	}
	return s
}

func (m *Row) append(c rune) {
	// Will not be nil as long as NewRows is used
	/*
		if m == nil {
			r := NewRow()
			m = r
		}
	*/
	*m = append(*m, c)
}

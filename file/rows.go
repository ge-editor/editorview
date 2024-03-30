package file

func NewRows() *rows {
	r := make(rows, 0, 64) // cap is...
	return &r
}

type rows []*Row

func (m *rows) IsLastRow(rowIndex int) bool {
	return rowIndex == len(*m)-1
}

func (m *rows) LenRows() int {
	return len(*m)
}

func (m *rows) Row(index int) *Row {
	return (*m)[index]
}

func (m *rows) append(rw *Row) {
	// Will not be nil as long as function NewRows is called
	/*
		if m == nil {
			r := NewRows()
			m = r
		} else {
			*m = append(*m, rw)
		}
	*/
	*m = append(*m, rw)
}

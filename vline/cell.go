package vline

import "github.com/ge-editor/gecore/screen"

type cell struct {
	width int8
	class screen.CharClass
}

func (m *cell) IsEmpty() bool {
	return m.class == 0
}

func (m cell) GetCellWidth() int {
	return int(m.width)
}

func (m *cell) SetCellWidth(w int) {
	m.width = int8(w)
}

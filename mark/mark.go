package mark

import (
	"github.com/ge-editor/utils"

	"github.com/ge-editor/te/file"
)

func NewMarks() *marks {
	return &marks{}
}

func NewMark(filePath string, current file.Cursor, content string) *Mark {
	return &Mark{
		FilePath: filePath,
		Cursor:   current,
		Content:  content,
	}
}

type Mark struct {
	FilePath string
	file.Cursor
	Content string
}

type marks []*Mark

// SetMark mark if exists unset and append
func (m *marks) SetMark(a *Mark) {
	m.UnsetMark(a)
	*m = append(*m, a)
}

func (m *marks) UnsetMark(d *Mark) bool {
	i := m.index(d)
	if i < 0 {
		return false
	}

	*m = append((*m)[:i], (*m)[i+1:]...)
	return true
}

// Find the last matching mark using the path member in the Marks struct Array
// return nil if not found
func (m *marks) FindLastByPath(filePath string) *Mark {
	for i := len(*m) - 1; i >= 0; i-- {
		if utils.SameFile((*m)[i].FilePath, filePath) {
			return (*m)[i]
		}
	}
	return nil
}

func (m *marks) Prev(d *Mark) *Mark {
	i := m.index(d)
	if i <= 0 {
		return nil
	}
	return (*m)[i-1]
}

func (m *marks) Next(d *Mark) *Mark {
	i := m.index(d)
	if i < 0 || i >= len(*m)-1 {
		return nil
	}
	return (*m)[i+1]
}

// Find *Mark from []*Mark then return index
// return -1 if not found
func (m *marks) index(d *Mark) int {
	for i := len(*m) - 1; i >= 0; i-- { // reverse
		if d == (*m)[i] || m.equal(d, (*m)[i]) {
			return i
		}

	}
	return -1
}

func (m *marks) equal(a, b *Mark) bool {
	if !utils.SameFile(a.FilePath, b.FilePath) {
		return false
	}
	if a.RowIndex == b.RowIndex && a.ColIndex == b.ColIndex {
		return true
	}
	return false
}

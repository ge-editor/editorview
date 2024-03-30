package vline

import (
	"fmt"
	"runtime"

	"github.com/ge-editor/gecore/screen"
	"github.com/ge-editor/gecore/verb"
)

type cells []cell

func (m *cells) make(size int) {
	l := len(*m)
	if size < l {
		*m = (*m)[:size]
	} else if size > l {
		*m = append(*m, make(cells, size-l)...)
	}
}

func (m *cells) GetCell(index int) *cell {
	if index > len(*m)-1 {
		// Error
		s := screen.Get()
		pc, file, line, _ := runtime.Caller(1)
		funcName := runtime.FuncForPC(pc).Name()
		str := fmt.Sprintf("Failed: GetCell: index: %d, len: %d, file: %s, line: %d, function: %s", index, len(*m), file, line, funcName)
		verb.PP(str)
		s.Echo(str)
		return &(*m)[0]
	}
	return &(*m)[index]
}

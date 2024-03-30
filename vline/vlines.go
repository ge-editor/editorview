package vline

import (
	"slices"

	"github.com/ge-editor/te/file"

	"github.com/ge-editor/utils"
)

func NewVlines(file *file.File) *Vlines {
	return &Vlines{
		file: file,
		// width:       width,
		// height:      height,
		// tabWidth:    tabWidth,
		// vlines:      make([]*Vline, height, cap),
		// offsetIndex: 0,
	}
}

type Vlines struct {
	file        *file.File
	width       int
	height      int
	tabWidth    int
	vlines      []*Vline
	offsetIndex int
}

// Re-allocate vlines when resized or tab width is changed
// tcell calls Resize first, so need to call this at that time
func (vs *Vlines) Resize(width, height, tabWidth int) {
	currentLen := len(vs.vlines)
	newLen := height * 4

	if newLen < currentLen {
		vs.vlines = vs.vlines[:newLen]
	} else if newLen > currentLen {
		vs.vlines = append(vs.vlines, make([]*Vline, newLen-currentLen)...)
	}

	vs.width = width
	vs.height = height
	vs.tabWidth = tabWidth
}

// Allocate vlines based on rowIndex
// Set offsetIndex
// It should be modified so that unnecessary allocation is not performed.
func (vs *Vlines) AllocateVlines(rowIndex int) {
	newStart := rowIndex - vs.height*2
	if newStart < 0 {
		newStart = 0
	}
	newEnd := newStart + vs.height*4

	vs.offsetIndex = newStart
	vs.vlines = make([]*Vline, newEnd-newStart+1)
}

func (vs *Vlines) SetFile(file *file.File) {
	vs.file = file
	vs.ReleaseAll()
}

func (vs *Vlines) SetTabWidth(tabWidth int) {
	vs.tabWidth = tabWidth
	vs.ReleaseAll()
}

func (vs *Vlines) GetVline(rowIndex int) *Vline {
	row := vs.file.Row(rowIndex)

	rowIndex -= vs.offsetIndex
	if vs.vlines[rowIndex] == nil {
		vs.vlines[rowIndex] = &Vline{}
		vs.vlines[rowIndex].calc(row, vs.width, vs.tabWidth) // calc
	}
	// vs.vlines[rowIndex].calc(row, vs.width, vs.tabWidth) // should avoid this call
	return vs.vlines[rowIndex]
}

// Overwrite the specified rowIndex range of the vlines slice with nil
// Release(a) index a only
// Release(a, b) regeon a:b
// Release(a, -1) regeon a:
func (vs *Vlines) Release(startIndex, endIndex int) {
	startIndex, endIndex = utils.FindOverlap(0, len(vs.vlines)-1, startIndex, endIndex)
	if startIndex == -1 {
		return
	}

	copy(vs.vlines[startIndex:endIndex+1], make([]*Vline, endIndex-startIndex+1))
}

func (vs *Vlines) ReleaseAll() {
	l := len(vs.vlines)
	copy(vs.vlines[0:l], make([]*Vline, l))
}

// Insert n rows of vline at rowIndex position
// Outside the index range is ignored
func (vs *Vlines) InsertN(rowIndex, n int) {
	index := rowIndex - vs.offsetIndex
	if index < 0 {
		n += index
		index = 0
	}
	if index > len(vs.vlines) {
		return
	}
	// Create a slice for insertion
	result := append(vs.vlines[:index], make([]*Vline, n)...)
	// Add the slice after the specified index
	result = append(result, vs.vlines[index:]...)
	vs.vlines = result
}

// insert slice at position startIndex to endIndex
// Outside the index range is ignored
func (vs *Vlines) Insert(startIndex, endIndex int) {
	startIndex -= vs.offsetIndex
	endIndex -= vs.offsetIndex
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex > len(vs.vlines) {
		return
	}
	n := endIndex - startIndex + 1
	// Create a slice for insertion
	result := append(vs.vlines[:startIndex], make([]*Vline, n)...)
	// Add the slice after the specified index
	result = append(result, vs.vlines[startIndex:]...)
	vs.vlines = result
}

// Delete startIndex to endIndex
// Outside the index range is ignored
func (vs *Vlines) Delete(startIndex, endIndex int) {
	startIndex, endIndex = utils.FindOverlap(0, len(vs.vlines)-1, startIndex-vs.offsetIndex, endIndex-vs.offsetIndex)
	if startIndex == -1 {
		return
	}
	vs.vlines = slices.Delete(vs.vlines, startIndex, endIndex+1)
}

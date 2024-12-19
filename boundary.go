package te

import (
	"fmt"
	"slices"

	"github.com/ge-editor/gecore/screen"

	"github.com/ge-editor/theme"
)

type Boundary struct {
	StartIndex int
	StopIndex  int
	Width      int
	TotalWidth int
}

func (b *Boundary) Clear() {
	*b = Boundary{}
}

func (b *Boundary) IsEmpty() bool {
	return b.StopIndex == 0 && b.Width == 0
}

// -------------------------------

// Don't want to separate it into a separate package.
type BoundariesArray struct {
	editor           *Editor
	boundariesArray_ [][]Boundary // Should not call this member value directly.
	// By using an interface, can completely hide this member value access from same package. But there is overhead.
}

func NewBoundariesArray(editor *Editor) BoundariesArray {
	return BoundariesArray{
		editor:           editor,
		boundariesArray_: make([][]Boundary, 0, 64),
	}
}

// Return number of logical row
func (b *BoundariesArray) BoundariesLen(rowIndex int) int {
	b.beAvailable(rowIndex)
	return len(b.boundariesArray_[rowIndex])
}

func (b *BoundariesArray) LastBoundary(rowIndex int) Boundary {
	b.beAvailable(rowIndex)
	return b.boundariesArray_[rowIndex][b.BoundariesLen(rowIndex)-1]
}

// Return the logical row boundary information
func (b *BoundariesArray) Boundary(rowIndex, logicalRowIndex int) Boundary {
	b.beAvailable(rowIndex)
	return b.boundariesArray_[rowIndex][logicalRowIndex]
}

// Return BoundariesArray length
func (b *BoundariesArray) Len() int {
	return len(b.boundariesArray_)
}

// Insert count number of elements into BoundariesArray at rowIndex position.
func (b *BoundariesArray) Insert(rowIndex, count int) {
	if rowIndex < 0 || count < 0 {
		screen.Get().Echo(fmt.Sprintf("Error: rowIndex and count must be non-negative, rowIndex: %d, count: %d", rowIndex, count))
		return
	}

	appendCount := rowIndex - (b.Len() - 1)
	if appendCount > 0 {
		b.boundariesArray_ = append(b.boundariesArray_, make([][]Boundary, appendCount)...)
	}

	b.boundariesArray_ = slices.Insert(b.boundariesArray_, rowIndex, make([][]Boundary, count)...)
}

func (b *BoundariesArray) Delete(rowIndex, count int) error {
	if rowIndex < 0 || count < 0 {
		return fmt.Errorf("rowIndex and count must be non-negative")
	}
	if rowIndex >= len(b.boundariesArray_) {
		return fmt.Errorf("rowIndex out of range")
	}
	if rowIndex+count > len(b.boundariesArray_) {
		count = len(b.boundariesArray_) - rowIndex
	}

	b.boundariesArray_ = slices.Delete(b.boundariesArray_, rowIndex, rowIndex+count)
	return nil
}

func (b *BoundariesArray) ClearAll() {
	b.boundariesArray_ = nil
}

// Set a Boundary-Array (logical row information) at a specified row index, extending the slice if necessary
func (b *BoundariesArray) Set(rowIndex int, bs []Boundary) {
	i := rowIndex - len(b.boundariesArray_) + 1
	if i > 0 {
		b.boundariesArray_ = append(b.boundariesArray_, make([][]Boundary, i)...)
	}
	b.boundariesArray_[rowIndex] = bs
}

// Check if a specified row index is dirty (i.e., out of bounds or nil)
func (b *BoundariesArray) isDirty(rowIndex int) bool {
	// return true
	return rowIndex >= len(b.boundariesArray_) || b.boundariesArray_[rowIndex] == nil
}

// Be available the Boundaries for row index.
// Need call this function before reference the Editor.BoundariesArray.
func (b *BoundariesArray) beAvailable(rowIndex int) {
	if b.isDirty(rowIndex) {
		foundPositionIndex := -1 // Don't use this, dummy value for call drawLine function.
		b.editor.drawLine(0, rowIndex, -1, false, &foundPositionIndex, nil, 0, theme.ColorDefault)
	}
}

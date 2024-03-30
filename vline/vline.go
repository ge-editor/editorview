package vline

import (
	"github.com/ge-editor/gecore/define"
	"github.com/ge-editor/gecore/screen"

	"github.com/ge-editor/theme"

	"github.com/ge-editor/te/file"
)

type Vline struct {
	viewWidth int
	cells
	boundaries []boundary
}

// Determine if it is the last logical row.
func (v *Vline) IsEndOfLogicalRow(colIndex int) bool {
	for _, b := range *v.Boundaries() {
		if colIndex == b.Index() {
			return true
			// return i
		}
	}
	return false
	// return -1
}

// Return the row and col index when it matches the head of a logical row.
func (v *Vline) GetHeadOfLogicalRowIndex(colIndex int) int {
	for i, b := range *v.Boundaries() {
		if colIndex == b.Index()+1 {
			return i + 1 //, colIndex // b.Index() + 1
		}
	}
	return -1 //, -1
}

// Return value is the last logical row or not.
func (v *Vline) IsLastLogicalRow(colIndex int) bool {
	bs := v.Boundaries()
	l := len(*bs)
	if l == 1 {
		return true
	}
	if colIndex > (*bs)[l-2].Index() {
		return true
	}
	return false
}

// Return value is min and max column index of the logical row.
func (v *Vline) GetMinAndMaxIndexOfLogicalRow(logicalRowIndex int) (int, int) {
	bs := v.Boundaries()
	l := len(*bs)
	if logicalRowIndex == -1 {
		logicalRowIndex = l - 1 // last logical line
	}
	if logicalRowIndex > l-1 {
		return -1, -1
	}
	if logicalRowIndex == 0 {
		return 0, (*bs)[0].Index()
	}
	return (*bs)[logicalRowIndex-1].Index() + 1, (*bs)[logicalRowIndex].Index()
}

// What index of the logical row?
func (v *Vline) GetIndexOfLogicalRow(colIndex int) int {
	for i, b := range *v.Boundaries() {
		if colIndex <= b.Index() {
			return i
		}
	}
	return -1
}

// Return logical row boundaries information.
func (v *Vline) Boundaries() *[]boundary {
	return &v.boundaries
}

func (v *Vline) GetBoundary(rowIndex int) boundary {
	return v.boundaries[rowIndex]
}

// Return logical row length
func (v *Vline) LenLogicalRow() int {
	return len(v.boundaries)
}

// Returns the screen position of the cursor corresponding to the specified column index in logical rows.
// colIndex: column index
func (v Vline) CursorPositionOnScreenLogicalLine(colIndex int) (cx, cy int) {
	for cy = 0; cy < len(v.boundaries); cy++ {
		if colIndex <= v.boundaries[cy].index {
			start := 0
			if cy > 0 {
				start = v.boundaries[cy-1].index + 1
			}
			for i := start; i < colIndex; i++ {
				cx += v.cells.GetCell(i).GetCellWidth()
			}
			return cx, cy
		}
	}
	return -1, -1 // overflow
}

// Split the line into logical lines
// - row : line buffer
// - screenWidth: Width of the screen at the time this function is executed
// - tabWidth: tab width
func (v *Vline) calc(row *file.Row, screenWidth int, tabWidth int) {

	v.viewWidth = screenWidth
	v.cells.make(row.LenCh())
	// verb.PP("%q, row.LenCh() %d, cells %v", row.String(), row.LenCh(), v.cells)
	v.boundaries = v.boundaries[:0]

	var logical, breakpoint boundary
	var p2, p1 cell // before prev p1, prev c, current rune.

	totalWidth := 0 // for tabstop
	y := 0

	// Calculate by section to 1: cursor position, 2: logical row information
	for index := 0; index < row.LenCh(); index++ {
		isLastRune := index == row.LenCh()-1
		c := &v.cells[index]
		ch := row.Ch(index)
		updateCell(c, ch, totalWidth, tabWidth)
		// For tab, set the on-screen width of tab to c.Width
		/*
			if ch == '\t' {
				updateTabWidth(c, screenWidth, isLastRune, logical.widths)
			}
		*/

		// Check if it is valid as a newline position
		// If valid, put in to breakpoint variable
		breakpoint.validNewlinePosition(screenWidth-8, index, &p2, &p1, c, &logical)

		// When calculating how many characters fit on a line,
		// if the line fills the screen to the right,
		// the rightmost character will be '-', '\n' or EOF.
		// Set the width to 0 so that these characters are not added to
		// the line width so they are not pushed to the next line.
		lfWidth := theme.LF_WIDTH
		if ch == '\n' || ch == define.EOF {
			lfWidth = 0
		}

		// verb.PP("********* %s logical.width(%d)+c.width(%d)+LF_WIDTH(%d) > v.Width(%d)", string(c.r), logical.width, c.width, LF_WIDTH, v.Width)
		if logical.widths+c.GetCellWidth()+lfWidth > screenWidth { // Exceeds the width of the screen
			if breakpoint.isEmpty() {
				if c.class&screen.PROHIBITED > 0 {
					// Wrap the previous character p1 and subsequent characters to the next logical line
					// verb.PP("********* 1 / last_rune %v, breakpoint is empty, prohibited", last_rune)
					// logical, without c
					v.boundaries = append(v.boundaries, boundary{index: index - 2, widths: logical.widths - p1.GetCellWidth()}) // before p1
					if isLastRune {
						v.boundaries = append(v.boundaries, boundary{index: index, widths: p1.GetCellWidth() + c.GetCellWidth()}) // p1 and later
						break
					}
					y++
					logical.widths = p1.GetCellWidth()
				} else { // c.class != PROHIBITED
					// Wrap current character c to next line
					// logical, without c
					v.boundaries = append(v.boundaries, logical)
					if isLastRune {
						v.boundaries = append(v.boundaries, boundary{index: index, widths: c.GetCellWidth()})
						break
					}
					y++
					logical.widths = 0
				}
			} else { // breakpoint is not empty
				// Wrap breakpoint and subsequent characters to next line
				// verb.PP("********* 3 / last_rune %v, breakpoint is not empty", isLastRune)
				// logical, without c
				breakpoint.index -= 1 // Wrap at breakpoint, set the index before breakpoint.
				v.boundaries = append(v.boundaries, breakpoint)
				if isLastRune {
					v.boundaries = append(v.boundaries, boundary{index: index, widths: logical.widths + c.GetCellWidth() - breakpoint.widths})
					break
				}
				y++
				logical.widths = logical.widths - breakpoint.widths
				breakpoint.clear()
			}
		} else { // Fits on one line, no need to wrap
			// verb.PP("********* 4 / last_rune: %v %s", last_rune, string(c.r))
			// logical
			if isLastRune {
				v.boundaries = append(v.boundaries, boundary{index: index, widths: logical.widths + c.GetCellWidth()})
				break
			}
		}

		// At the end of the loop
		// updateCellAttribute(c, ch) // updateCell
		// logical, add c
		logical.index, logical.widths = index, logical.widths+c.GetCellWidth()
		//
		totalWidth += c.GetCellWidth() // for tabstop
		p2 = p1
		p1 = *c
	} // for index

	// verb.PP("%#v", *v)
}

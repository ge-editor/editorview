package te

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/atotto/clipboard"

	"github.com/ge-editor/gecore/define"
	"github.com/ge-editor/gecore/kill_buffer"
	"github.com/ge-editor/gecore/screen"
	"github.com/ge-editor/gecore/verb"

	"github.com/ge-editor/utils"

	"github.com/ge-editor/theme"

	"github.com/ge-editor/te/file"
	"github.com/ge-editor/te/mark"
)

// ------------------------------------------------------------------
// File
// ------------------------------------------------------------------

// If the file has already been read, use that buffer
func (e *Editor) OpenFile(path string) error {
	ff, meta, err := BufferSets.GetFileAndMeta(path)
	e.File = ff
	e.Meta = meta

	// e.vlines.SetFile__(ff)
	// e.vlines.AllocateVlines__(meta.RowIndex)

	return err // return message
}

// If the file does not exist, a backup error will occur
func (e *Editor) SaveFile() {
	backupMessage := ""
	if err := e.Backup(); err != nil {
		backupMessage = " (" + err.Error() + ")"
	}

	err := e.Save()
	if err != nil {
		e.screen.Echo(err.Error() + backupMessage)
	} else {
		e.screen.Echo("Wrote " + e.GetPath() + backupMessage)
	}

	e.UndoAction.MoveTo(e.RedoAction)
}

// If an existing file is specified, it will be overwritten
// Backup works so no data is lost, but...
func (e *Editor) ChangeFilePath(path string) {
	e.SetPath(path)
}

// ------------------------------------------------------------------
// Move cursor
// ------------------------------------------------------------------

// Move cursor to next word.
func (e *Editor) MoveCursorNextWord() {
	verb.PP("MoveCursorNextWord")
	x, y := e.Cx, e.Cy

	lines := e.Rows()
	line, _ := lines.GetRow(e.RowIndex)
	if line.IsColIndexAtRowEnd(e.ColIndex) {
		if lines.IsRowIndexLastRow(e.RowIndex) {
			e.screen.Echo("End of buffer")
			return
		}
		y++
		e.RowIndex++
		x = 0
		e.ColIndex = 0
		return
	}

	var prevCc, cc screen.CharClass
	notUppercaseBit := ^screen.UPPERCASE
	for {
		ch, size, ok := lines.DecodeRune(e.RowIndex, e.ColIndex)
		if !ok {
			verb.PP("error")
			panic("err")
		}
		w, ok := e.runeWidth(ch, e.RowIndex, e.ColIndex)
		if !ok {
			verb.PP("error")
		}
		prevCc = cc
		cc = screen.GetCharClass(ch)
		if prevCc != 0 {
			if prevCc&screen.UPPERCASE == 0 && cc&screen.UPPERCASE > 0 {
				break
			}
			prevCc &= notUppercaseBit
			cc &= notUppercaseBit
			if prevCc != cc && cc&screen.TAB == 0 && cc&screen.SPACE == 0 && cc&screen.SYMBOL == 0 {
				break
			}
		}
		//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
		if e.isEndOfLogicalRow(e.RowIndex, e.ColIndex) {
			y++
			x = 0
		} else {
			x += w
		}
		e.ColIndex += size
	}

	e.PrevCx = x
	e.moveCursor(x, y)
}

func (e *Editor) MoveCursorPreviousWord() {
	x, y := e.Cx, e.Cy

	if e.ColIndex == 0 {
		if e.RowIndex == 0 {
			e.screen.Echo("Beginning of buffer")
			return
		}
		y--
		e.RowIndex-- // previous line
		//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
		//bs := e.bsay.Boundaries(e.RowIndex)
		//lastBs := bs.LastBoundary()
		lastBs := e.bsArray.LastBoundary(e.RowIndex)
		// lastBs := e.bsay[e.RowIndex].boundaries[e.bsay[e.RowIndex].Len()-1]
		ch, _, colIndex, _ := e.Rows().DecodeEndRune(e.RowIndex)
		w, _ := e.runeWidth(ch, e.RowIndex, colIndex)
		x = lastBs.Width - w  // on linefeed
		e.ColIndex = colIndex // lastBs.stopIndex - size
	} else {
		var prevCc, cc screen.CharClass
		notUppercaseBit := ^screen.UPPERCASE
		for {
			//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
			before, ok := e.getIndexOfLogicalRow(e.RowIndex, e.ColIndex)
			if !ok {
				panic("1")
			}
			ch, _, colIndex, ok := e.Rows().DecodePrevRune(e.RowIndex, e.ColIndex)
			if !ok {
				// panic("2")
				break
			}
			w, ok := e.runeWidth(ch, e.RowIndex, e.ColIndex)
			if !ok {
				verb.PP("error")
			}
			prevCc = cc
			cc = screen.GetCharClass(ch)
			if prevCc != 0 {
				savePrevCC, saveCC := prevCc, cc
				prevCc &= notUppercaseBit
				cc &= notUppercaseBit
				if prevCc != cc && (cc&screen.TAB > 0 || cc&screen.SPACE > 0 || cc&screen.SYMBOL > 0) {
					break
				}
				prevCc, cc = savePrevCC, saveCC
			}
			e.ColIndex = colIndex
			after, ok := e.getIndexOfLogicalRow(e.RowIndex, e.ColIndex)
			if !ok {
				panic("3")
			}
			if after < before {
				y--
				//bs := e.bsay.Boundaries(e.RowIndex)
				//x = bs[after].Width - w
				x = e.bsArray.Boundary(e.RowIndex, after).Width
				// x = e.bsay[e.RowIndex].boundaries[after].Width - w
			} else {
				x -= w
			}
			if prevCc != 0 && prevCc&screen.UPPERCASE == 0 && cc&screen.UPPERCASE > 0 {
				break
			}
		}
	}

	e.PrevCx = x
	e.moveCursor(x, y)
}

// Move cursor one character forward.
func (e *Editor) MoveCursorForward() {
	// verb.PP("MoveCursorForward")
	x, y := e.Cx, e.Cy

	lines := e.Rows()
	line, _ := lines.GetRow(e.RowIndex)
	if line.IsColIndexAtRowEnd(e.ColIndex) {
		if lines.IsRowIndexLastRow(e.RowIndex) {
			e.screen.Echo("End of buffer")
			return
		}
		y++
		e.RowIndex++
		x = 0
		e.ColIndex = 0
		// verb.PP("MoveCursorForward 1")
	} else {
		// verb.PP("MoveCursorForward 2")
		// bo := e.boundariesArray.GetBoundaries(e.RowIndex)
		ch, size, ok := lines.DecodeRune(e.RowIndex, e.ColIndex)
		if !ok {
			verb.PP("error")
			panic("err")
		}
		w, ok := e.runeWidth(ch, e.RowIndex, e.ColIndex)
		if !ok {
			verb.PP("error")
		}
		// verb.PP("MoveCursorForward %q, %d %d %v", ch, size, w, ok)
		//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
		if e.isEndOfLogicalRow(e.RowIndex, e.ColIndex) {
			// verb.PP("MoveCursorForward 3 %q %d,%d", ch, e.RowIndex, e.ColIndex)
			y++
			x = 0
		} else {
			// verb.PP("MoveCursorForward 4")
			x += w
		}
		e.ColIndex += size
	}

	// verb.PP("MoveCursorForward x,y %d,%d col %d", x, y, e.ColIndex)
	e.PrevCx = x
	e.moveCursor(x, y)
}

// Move cursor one character backward.
func (e *Editor) MoveCursorBackward() {
	x, y := e.Cx, e.Cy

	if e.ColIndex == 0 {
		if e.RowIndex == 0 {
			e.screen.Echo("Beginning of buffer")
			return
		}
		y--
		e.RowIndex-- // previous line
		//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
		//bs := e.bsay.Boundaries(e.RowIndex)
		//lastBs := bs.LastBoundary()
		lastBs := e.bsArray.LastBoundary(e.RowIndex)
		// lastBs := e.bsay[e.RowIndex].boundaries[e.bsay[e.RowIndex].Len()-1]
		ch, _, colIndex, _ := e.Rows().DecodeEndRune(e.RowIndex)
		w, _ := e.runeWidth(ch, e.RowIndex, colIndex)
		x = lastBs.Width - w  // on linefeed
		e.ColIndex = colIndex // lastBs.stopIndex - size
	} else {
		//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
		before, ok := e.getIndexOfLogicalRow(e.RowIndex, e.ColIndex)
		if !ok {
			panic("1")
		}
		ch, _, colIndex, ok := e.Rows().DecodePrevRune(e.RowIndex, e.ColIndex)
		if !ok {
			panic("2")
		}
		w, _ := e.runeWidth(ch, e.RowIndex, e.ColIndex)
		e.ColIndex = colIndex
		after, ok := e.getIndexOfLogicalRow(e.RowIndex, e.ColIndex)
		if !ok {
			panic("3")
		}
		if after < before {
			y--
			//bs := e.bsay.Boundaries(e.RowIndex)
			//x = bs[after].Width - w
			x = e.bsArray.Boundary(e.RowIndex, after).Width - w
			// x = e.bsay[e.RowIndex].boundaries[after].Width - w
		} else {
			// x -= vl.GetCell__(e.ColIndex).Width
			x -= w
		}
	}

	e.PrevCx = x
	e.moveCursor(x, y)
}

// Move cursor to the next line.
func (e *Editor) MoveCursorNextLine() {
	var bo Boundary

	//e.makeAvailableBoundariesArray(e.RowIndex)        // -------- !
	if e.inEndOfLogicalRow(e.RowIndex, e.ColIndex) { // last logical line
		if e.Rows().IsRowIndexLastRow(e.RowIndex) {
			e.screen.Echo("End of buffer")
			return
		}
		// move to next line
		e.RowIndex++
		//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
		// bo = e.bsay.Boundaries(e.RowIndex)[0]
		bo = e.bsArray.Boundary(e.RowIndex, 0)
		// bo = e.bsay[e.RowIndex].boundaries[0]      // first logical line
	} else {
		// move to next logical line
		//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
		// What index number in logic line?
		i, _ := e.getIndexOfLogicalRow(e.RowIndex, e.ColIndex)
		i++ // next logical line
		// bo = e.bsay.Boundaries(e.RowIndex)[i]
		bo = e.bsArray.Boundary(e.RowIndex, i)
		// bo = e.bsay[e.RowIndex].boundaries[i]
	}

	x := 0
	i := bo.StartIndex
	for {
		ch, size, ok := e.Rows().DecodeRune(e.RowIndex, i)
		if !ok {
			panic("MoveCursorNextLine")
		}
		w, _ := e.runeWidth(ch, e.RowIndex, i)
		if x+w >= e.PrevCx || i+size >= bo.StopIndex {
			break
		}
		x += w
		i += size
	}
	e.ColIndex = i

	e.Cx = x
	e.Cy++
}

// Move cursor to the previous line.
func (e *Editor) MoveCursorPrevLine() {
	// What index number in logic row?
	//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
	indexOfLogicalRow, _ := e.getIndexOfLogicalRow(e.RowIndex, e.ColIndex)

	var bo Boundary
	if indexOfLogicalRow == 0 { // first logical line
		if e.RowIndex == 0 {
			e.screen.Echo("Beginning of buffer")
			return
		}
		// move to prev row
		e.RowIndex--
		//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
		//bs := e.bsay.Boundaries(e.RowIndex)
		//bo = bs.LastBoundary() // last logical row
		bo = e.bsArray.LastBoundary(e.RowIndex) // last logical row
		// bo = e.bsay[e.RowIndex].boundaries[e.bsay[e.RowIndex].Len()-1] // last logical row
	} else {
		//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
		//bs := e.bsay.Boundaries(e.RowIndex)
		//bo = bs[indexOfLogicalRow-1]                          //
		bo = e.bsArray.Boundary(e.RowIndex, indexOfLogicalRow-1) //
		// bo = e.bsay[e.RowIndex].boundaries[indexOfLogicalRow-1] //
	}

	x := 0
	i := bo.StartIndex
	for {
		ch, size, _ := e.Rows().DecodeRune(e.RowIndex, i)
		w, _ := e.runeWidth(ch, e.RowIndex, i)
		if x+w > e.PrevCx || i+size >= bo.StopIndex {
			break
		}
		x += w
		i += size
	}
	e.ColIndex = i

	e.moveCursor(x, e.Cy-1)
}

// Move cursor to the end of the line.
func (e *Editor) MoveCursorEndOfLine() {
	//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
	// What index number in logic row?
	indexOfLogicalRow, _ := e.getIndexOfLogicalRow(e.RowIndex, e.ColIndex)

	colLength, _ := e.Rows().GetColLength(e.RowIndex)
	e.ColIndex = colLength - 1

	// cursor display position
	// e.Cy += len(e.boundariesArray[e.RowIndex]) - 1 - indexOfLogicalRow
	// bs := e.bsay.Boundaries(e.RowIndex)
	// e.Cy += bs.Len() - 1 - indexOfLogicalRow
	e.Cy += e.bsArray.BoundariesLen(e.RowIndex) - 1 - indexOfLogicalRow
	// e.Cy += e.bsay[e.RowIndex].Len() - 1 - indexOfLogicalRow
}

// Move cursor to the end of logical the line.
// At the end of a logical line, the cursor should at the beginning of the next logical line. so I see...
func (e *Editor) MoveCursorEndOfLogicalLine() {
	//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
	// What index number in logic row?
	indexOfLogicalRow, _ := e.getIndexOfLogicalRow(e.RowIndex, e.ColIndex)
	//bs := e.bsay.Boundaries(e.RowIndex)
	// bs := &e.bsay[e.RowIndex].boundaries
	// bo := bs[indexOfLogicalRow]
	bo := e.bsArray.Boundary(e.RowIndex, indexOfLogicalRow)
	// if indexOfLogicalRow == len(*bs)-1 {
	// if indexOfLogicalRow == bs.Len()-1 {
	if indexOfLogicalRow == e.bsArray.BoundariesLen(e.RowIndex)-1 {
		e.ColIndex = bo.StopIndex - 1
		e.Cx = bo.Width - 1
	} else {
		// Move to the first character of the next logical row
		e.ColIndex = bo.StopIndex
		e.Cy++
		e.Cx = 0
	}

	e.PrevCx = e.Cx
}

// Move cursor to the beginning of the line.
func (e *Editor) MoveCursorBeginningOfLine() {
	if e.RowIndex == 0 && e.ColIndex == 0 {
		e.screen.Echo("Beginning of buffer")
		return
	}

	//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
	nowIndexOfLogicalRow, _ := e.getIndexOfLogicalRow(e.RowIndex, e.ColIndex)
	lines := e.Rows()
	indentedIndex := 0
	indentedWidth := 0
	for indentedIndex < len((*lines)[e.RowIndex]) {
		ch, size, _ := lines.DecodeRune(e.RowIndex, indentedIndex)
		if ch != ' ' && ch != '\t' || indentedIndex >= e.ColIndex {
			break
		}
		w, _ := e.runeWidth(ch, e.RowIndex, indentedIndex)
		indentedWidth += w
		indentedIndex += size
	}
	newIndexOfLogicalRow, _ := e.getIndexOfLogicalRow(e.RowIndex, indentedIndex)

	if indentedIndex < e.ColIndex {
		e.ColIndex = indentedIndex
		e.Cx = indentedWidth
	} else {
		e.ColIndex = 0
		e.Cx = 0
	}

	if newIndexOfLogicalRow < nowIndexOfLogicalRow {
		e.Cy -= nowIndexOfLogicalRow - newIndexOfLogicalRow
	}
}

// Move cursor to the beginning of the logical line.
func (e *Editor) MoveCursorBeginningOfLogicalLine() {
	if e.RowIndex == 0 && e.ColIndex == 0 {
		e.screen.Echo("Beginning of buffer")
		return
	}

	//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
	nowIndexOfLogicalRow, _ := e.getIndexOfLogicalRow(e.RowIndex, e.ColIndex)
	if nowIndexOfLogicalRow == 0 {
		e.MoveCursorBeginningOfLine()
		return
	}
	// bo := e.boundariesArray[e.RowIndex][nowIndexOfLogicalRow]
	//bs := e.bsay.Boundaries(e.RowIndex)
	//bo := bs[nowIndexOfLogicalRow]
	bo := e.bsArray.Boundary(e.RowIndex, nowIndexOfLogicalRow)
	// bo := bse.bsay[e.RowIndex].boundaries[nowIndexOfLogicalRow]
	if e.ColIndex == bo.StartIndex {
		return
	}
	e.ColIndex = bo.StartIndex
	e.Cx = 0
}

func (e *Editor) MoveCursorBeginningOfFile() {
	e.RowIndex = 0
	e.ColIndex = 0
	e.Cy = 0
	e.Cx = 0
	// e.vlines.AllocateVlines__(e.RowIndex)
}

func (e *Editor) MoveCursorEndOfFile() {
	e.RowIndex = e.Rows().RowLength() - 1
	// e.drawLine(0, e.RowIndex, 0, false)
	// bo := e.boundariesArray[e.RowIndex][len(e.boundariesArray[e.RowIndex])-1]
	//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
	//bs := e.bsay.Boundaries(e.RowIndex)
	//lastBs := bs.LastBoundary()
	lastBs := e.bsArray.LastBoundary(e.RowIndex)
	// bo := e.bsay[e.RowIndex].boundaries[e.bsay[e.RowIndex].Len()-1]
	e.ColIndex = lastBs.StopIndex - 1 // left of the LF or EOF
	e.Cx = lastBs.Width - 1           // left of the LF or EOF
	e.Cy = e.editArea.Height - e.verticalThreshold
}

// Move view 'n' lines forward or backward only if it's possible.
func (e *Editor) MoveViewHalfForward() {
	n := e.Height / 2
	e.screen.Echo("")

	// What index is e.ColIndex in the logical line?
	indexOfLogicalRow, _ := e.getIndexOfLogicalRow(e.RowIndex, e.ColIndex)

	// Compute total, logicalRowLength, rowIndex and vl
	total := -indexOfLogicalRow
	logicalRowLength := 0
	rowIndex := e.RowIndex
	for ; ; rowIndex++ {
		// e.drawLine(0, rowIndex, 0, false)
		//e.makeAvailableBoundariesArray(rowIndex) // -------- !
		// logicalRowLength = len(e.boundariesArray[rowIndex])
		// logicalRowLength = e.bsay.Boundaries(rowIndex).Len()
		logicalRowLength = e.bsArray.BoundariesLen(rowIndex)
		// Add the number of logical row. first subtract the number of logical row before the cursor
		total += logicalRowLength
		// indexOfLogicalRow = 0 // No need to subtract after the second time, so 0
		if total >= n || rowIndex == e.Rows().RowLength()-1 {
			break
		}
	}
	e.RowIndex = rowIndex

	// Compute logicalRowIndex
	indexOfLogicalRow = logicalRowLength - 1
	// If the number of lines to scroll is exceeded,
	// subtract the number of logical lines exceeded
	if total > n {
		indexOfLogicalRow -= total - n
		/*
			if logicalRowIndex < 0 {
				panic("logicalRowIndex")
			}
		*/
	}

	e.ColIndex, _ = e.getColumnIndexClosestToCursorXPosition(e.RowIndex, indexOfLogicalRow, e.PrevCx)
}

// Move view 'n' lines forward or backward.
func (e *Editor) MoveViewHalfBackward( /* n int */ ) {
	n := e.Height / 2
	e.screen.Echo("")

	// What index number in logic row?
	//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
	indexOfLogicalRow, _ := e.getIndexOfLogicalRow(e.RowIndex, e.ColIndex)

	total := n
	logicalRowLength := indexOfLogicalRow
	rowIndex := e.RowIndex
	for {
		total -= logicalRowLength // Subtract the number of logical rows
		if total <= 0 || rowIndex == 0 {
			break
		}
		rowIndex--
		//e.makeAvailableBoundariesArray(rowIndex) // -------- !
		// logicalRowLength = vl.LenLogicalRow__()
		// logicalRowLength = len(e.boundariesArray[rowIndex])
		// logicalRowLength = e.bsay.Boundaries(rowIndex).Len()
		logicalRowLength = e.bsArray.BoundariesLen(rowIndex)
	}
	e.RowIndex = rowIndex

	// Compute logicalRowIndex
	indexOfLogicalRow = 0 // logicalRowLength - 1
	// If the number of lines to scroll is exceeded,
	// subtract the number of logical lines exceeded
	if total > n {
		indexOfLogicalRow -= n
	}

	// e.drawLine(0, e.RowIndex, e.PrevCx, false)
	// e.makeAvailableBoundaries(e.RowIndex) // -------- !
	e.ColIndex, _ = e.getColumnIndexClosestToCursorXPosition(e.RowIndex, indexOfLogicalRow, e.PrevCx)
}

func (e *Editor) MoveCursorToLine(lineNumber int) {
	if lineNumber < 1 || lineNumber > e.Rows().RowLength() {
		return
	}
	e.RowIndex = lineNumber - 1
	e.ColIndex = 0
	e.Cy = (e.Height - 1) / 2
}

func (e *Editor) InsertTab() {
	if e.IsSoftTab() {
		w := utils.TabWidth(e.Cx, e.GetTabWidth())
		for i := 0; i < w; i++ {
			// e.InsertRune__(' ')
			e.insertBytes([]byte{' '}, true)
		}
	} else {
		// e.InsertRune__('\t')
		e.insertBytes([]byte{'\t'}, true)
	}
}

// ------------------------------------------------------------------
// Edit
// ------------------------------------------------------------------

// Wrapper is insertBytes
func (e *Editor) InsertString(s string) {
	e.insertBytes([]byte(s), true)
}

// Wrapper is insertBytes
func (e *Editor) InsertRune(ch rune) {
	e.insertBytes(utils.RuneToBytes(ch), true)
}

func (e *Editor) DeleteRuneBackward() {
	start := e.Cursor
	stop := e.Cursor
	_, _, colIndex, _ := e.Rows().DecodePrevRune(e.RowIndex, e.ColIndex)

	if e.ColIndex == 0 {
		if e.RowIndex == 0 {
			e.screen.Echo("Beginning of buffer")
			return
		}
		// join to prev row
		lines := e.Rows()
		start.RowIndex--
		start.ColIndex = len((*lines)[start.RowIndex]) - 1
	} else {
		start.ColIndex = colIndex
	}
	removed := e.RemoveRegion(start, stop)
	if removed == nil {
		return
	}
	e.Cursor = start

	// e.screen.Echo(fmt.Sprintf("DeleteRuneBackward %d:%d", start.RowIndex+1, stop.RowIndex+1))
	//e.makeAvailableBoundariesArray(start.RowIndex) // -------- !
	if count := stop.RowIndex - start.RowIndex; count > 0 {
		e.bsArray.Delete(start.RowIndex+1, count)
	}

	e.UndoAction.Push(file.Action{Class: file.DELETE_BACKWARD, Before: stop, After: start, Data: *removed})
	e.syncCursorAndBufferForEdit(DELETE, start, stop)

	e.PrevCx = -1
}

// If at the EOL, move contents of the next line to the end of the current line,
// erasing the next line after that. Otherwise, delete one character under the
// cursor.
func (e *Editor) DeleteRune() {
	start := e.Cursor
	stop := e.Cursor

	beRemovedCh, size, _ := e.Rows().DecodeRune(e.RowIndex, e.ColIndex)
	if beRemovedCh == '\n' {
		stop.RowIndex++
		stop.ColIndex = 0
	} else {
		stop.ColIndex += size
	}
	removed := e.RemoveRegion(start, stop)
	if removed == nil {
		return
	}

	// e.screen.Echo(fmt.Sprintf("DeleteRune %d:%d", start.RowIndex+1, stop.RowIndex+1))
	//e.makeAvailableBoundariesArray(start.RowIndex) // -------- !
	if count := stop.RowIndex - start.RowIndex; count > 0 {
		e.bsArray.Delete(start.RowIndex+1, count)
	}

	e.UndoAction.Push(file.Action{Class: file.DELETE, Before: start, After: start, Data: *removed})
	e.syncCursorAndBufferForEdit(DELETE, stop, start)

	e.PrevCx = -1
}

func (e *Editor) Autoindent() {
	lines := e.Rows()
	line := (*lines)[e.RowIndex]
	indent := make([]byte, 0, len(line))
	indent = append(indent, '\n')
	for i := 0; i < len(line); {
		ch, size := utf8.DecodeRune(line[i:])
		if ch == ' ' || ch == '\t' {
			indent = append(indent, utils.RuneToBytes(ch)...)
			i += size
			continue
		}
		break
	}
	e.insertBytes(indent, true)
}

// ------------------------------------------------------------------
// Mark
// ------------------------------------------------------------------

func (e *Editor) SetMark() {
	content := e.getContentWidthoutSpecialCharactor(e.Cursor, 20)

	newMark := mark.NewMark(e.GetPath(), e.Cursor, content)

	if Marks.UnsetMark(newMark) {
		e.screen.Echo("Unset mark")
		return
	}
	Marks.SetMark(newMark)
	e.screen.Echo("Set mark")
}

func (e *Editor) SwapCursorAndMark() {
	m := Marks.FindLastByPath(e.GetPath())
	if m == nil {
		e.screen.Echo("The mark is not set now, so there is no region")
		return
	}

	Marks.UnsetMark(m)
	Marks.SetMark(mark.NewMark(e.GetPath(), e.Cursor, e.getContentWidthoutSpecialCharactor(e.Cursor, 20)))
	e.Cursor = m.Cursor
}

// ------------------------------------------------------------------
// Region
// ------------------------------------------------------------------

// Copy cursor region to Kill Buffer and Clipboard
func (e *Editor) CopyRegion() {
	mark := Marks.FindLastByPath(e.GetPath())
	if mark == nil {
		e.screen.Echo("The mark is not set now, so there is no region")
		return
	}
	if mark.RowIndex == e.RowIndex && mark.ColIndex == e.ColIndex {
		e.screen.Echo("Mark and cursor position are the same, so there is no region")
		return
	}

	var err error
	if mark.RowIndex == e.RowIndex {
		if mark.ColIndex > e.ColIndex {
			err = e.copyRegion(e.Cursor, mark.Cursor)
		} else {
			err = e.copyRegion(mark.Cursor, e.Cursor)
		}
	} else if mark.RowIndex > e.RowIndex {
		err = e.copyRegion(e.Cursor, mark.Cursor)
	} else {
		err = e.copyRegion(mark.Cursor, e.Cursor)
	}
	if err != nil {
		e.screen.Echo("Copied, " + err.Error())
	} else {
		e.screen.Echo("Copied")
	}
}

// Delete start to stop bytes and push the bytes to undo-stack and kill-buffer
// 開始から終了までのバイトを削除し、そのバイトを undo スタックと kill バッファにプッシュする
func (e *Editor) killRegion(start, stop file.Cursor) {
	e.Cursor = start
	removed := e.RemoveRegion(start, stop)
	if removed == nil {
		return
	}

	//e.makeAvailableBoundariesArray(start.RowIndex) // -------- !
	if count := stop.RowIndex - start.RowIndex; count > 0 {
		e.bsArray.Delete(start.RowIndex+1, count)
	}

	e.syncCursorAndBufferForEdit(DELETE, start, stop)
	e.UndoAction.Push(file.Action{Class: file.DELETE_BACKWARD, Before: start, After: start, Data: *removed})
	if err := kill_buffer.KillBuffer.PushKillBuffer([]byte(string(*removed))); err != nil {
		e.screen.Echo(err.Error())
	}
}

// Kill region between last mark to cursor
// and push undo and kill buffers
func (e *Editor) KillRegion() {
	mark := Marks.FindLastByPath(e.GetPath())
	if mark == nil {
		e.screen.Echo("The mark is not set now, so there is no region")
		return
	}
	if mark.RowIndex == e.RowIndex && mark.ColIndex == e.ColIndex {
		e.screen.Echo("Mark and cursor position are the same, so there is no region")
		return
	}

	if mark.RowIndex == e.RowIndex {
		if mark.ColIndex > e.ColIndex {
			e.killRegion(e.Cursor, mark.Cursor)
		} else {
			e.killRegion(mark.Cursor, e.Cursor)
		}
	} else if mark.RowIndex > e.RowIndex {
		e.killRegion(e.Cursor, mark.Cursor)
	} else {
		e.killRegion(mark.Cursor, e.Cursor)
	}
	e.screen.Echo("Copied")
}

// Kill line:
// If not at the EOL, remove contents of the current line from the cursor to the end.
// Otherwise behave like 'delete'.
// 行を削除します:
// EOL でない場合は、カーソルから末尾までの現在の行の内容を削除します。
// それ以外の場合は、「delete」のように動作します。
func (e *Editor) KillLine() {
	var removed *[]byte
	stop := e.Cursor

	lines := e.Rows()
	line, _ := lines.GetRow(e.RowIndex)
	//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
	if line.IsColIndexAtRowEnd(e.ColIndex) {
		if lines.IsRowIndexLastRow(e.RowIndex) {
			e.screen.Echo("End of buffer")
			return
		}
		// delete linefeed
		e.DeleteRune()
		return
	}

	l, _ := lines.GetColLength(e.RowIndex)
	stop.ColIndex = l - 1
	removed = e.RemoveRegion(e.Cursor, stop)
	if removed == nil {
		return
	}

	// e.screen.Echo(fmt.Sprintf("KillLine %d:%d", start.RowIndex+1, stop.RowIndex+1))
	//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
	if count := stop.RowIndex - e.RowIndex; count > 0 {
		e.bsArray.Delete(e.RowIndex+1, count)
	}
	// e.bsay.bsayDelete(e.RowIndex+1, stop.RowIndex+1)

	e.syncCursorAndBufferForEdit(DELETE, e.Cursor, stop)
	e.UndoAction.Push(file.Action{Class: file.DELETE, Before: e.Cursor, After: e.Cursor, Data: *removed})

	e.PrevCx = -1
}

// ------------------------------------------------------------------
// Yank
// ------------------------------------------------------------------

func (e *Editor) YankFromClipboard() {
	s, err := clipboard.ReadAll()
	if err != nil {
		e.screen.Echo(err.Error())
		return
	}
	e.insertBytes([]byte(s), true)
}

func (e *Editor) Yank() {
	r := kill_buffer.KillBuffer.GetLast()
	if r == nil {
		return
	}
	e.insertBytes(r, true)
}

// ------------------------------------------------------------------
// Undo / Redo
// ------------------------------------------------------------------

func (e *Editor) Undo() {
	if e.UndoAction.IsEmpty() {
		e.screen.Echo("No further undo information")
		return
	}

	a, _ := e.UndoAction.Pop()
	// verb.PP("e.UndoAction.Pop() %v %v '%s'", a.Before, a.After, string(a.Data))
	if a.Class == file.INSERT {
		e.RemoveRegion(a.Before, a.After)

		// e.screen.Echo(fmt.Sprintf("Undo %d:%d", start.RowIndex+1, stop.RowIndex+1))
		//e.makeAvailableBoundariesArray(a.Before.RowIndex) // -------- !
		// e.bsay.bsayDelete(a.Before.RowIndex+1, a.After.RowIndex+1)
		if count := a.After.RowIndex - a.Before.RowIndex; count > 0 {
			e.bsArray.Delete(a.Before.RowIndex+1, count)
		}

		e.syncCursorAndBufferForEdit(DELETE, a.Before, a.After)
		e.Cursor = a.Before
	} else if a.Class == file.DELETE {
		e.Cursor = a.Before
		e.insertBytes(a.Data, false)
		e.syncCursorAndBufferForEdit(INSERT, a.Before, a.After)
		e.Cursor = a.Before
	} else if a.Class == file.DELETE_BACKWARD {
		e.Cursor = a.After
		e.insertBytes(a.Data, false)
		e.syncCursorAndBufferForEdit(INSERT, a.After, a.Before)
	} else {
		return
	}

	e.screen.Echo("Undo!")
	e.RedoAction.Push(a)
}

func (e *Editor) Redo() {
	if e.RedoAction.IsEmpty() {
		e.screen.Echo("No further redo information")
		return
	}

	a, _ := e.RedoAction.Pop()
	if a.Class == file.INSERT {
		e.Cursor = a.Before
		e.insertBytes(a.Data, false)
	} else if a.Class == file.DELETE_BACKWARD {
		e.RemoveRegion(a.After, a.Before)

		// e.screen.Echo(fmt.Sprintf("Redo DELETE_BACKWARD %d:%d", a.After.RowIndex+1, a.Before.RowIndex+1))
		//e.makeAvailableBoundariesArray(a.After.RowIndex) // -------- !
		// e.bsay.bsayDelete(a.After.RowIndex+1, a.Before.RowIndex+1)
		if count := a.Before.RowIndex - a.After.RowIndex; count > 0 {
			e.bsArray.Delete(a.After.RowIndex+1, count)
		}

		e.syncCursorAndBufferForEdit(DELETE, a.After, a.Before)
	} else if a.Class == file.DELETE {
		lines := file.SplitByLF(a.Data)
		last := lines[len(lines)-1]
		cursor := a.After
		cursor.RowIndex += len(lines) - 1
		cursor.ColIndex += len(last)
		if last[len(last)-1] == '\n' {
			cursor.RowIndex++
			cursor.ColIndex = 0
		}
		e.RemoveRegion(a.Before, cursor)

		// e.screen.Echo(fmt.Sprintf("Redo DELETE %d:%d", a.Before.RowIndex+1, cursor.RowIndex+1))
		//e.makeAvailableBoundariesArray(a.Before.RowIndex) // -------- !
		// e.bsay.bsayDelete(a.Before.RowIndex+1, cursor.RowIndex+1)
		if count := cursor.RowIndex - a.Before.RowIndex; count > 0 {
			e.bsArray.Delete(a.Before.RowIndex+1, count)
		}

		e.syncCursorAndBufferForEdit(DELETE, a.Before, cursor)
	} else {
		return
	}
	e.Cursor = a.After

	e.screen.Echo("Redo!")
	e.UndoAction.Push(a)
}

// ------------------------------------------------------------------
// Search and replace
// ------------------------------------------------------------------

func (e *Editor) GetFindIndexes() []foundPosition {
	return e.foundIndexes
}

func (e *Editor) MoveNextFoundWord() {
	if len(e.foundIndexes) == 0 {
		return
	}

	if e.currentSearchIndex == -1 {
		for i := 0; i < len(e.foundIndexes); i++ {
			if e.foundIndexes[i].start.RowIndex >= e.RowIndex {
				e.currentSearchIndex = i
				break
			}
		}
	} else if e.currentSearchIndex == len(e.foundIndexes)-1 {
		e.currentSearchIndex = 0
	} else {
		e.currentSearchIndex++
	}

	if e.currentSearchIndex < 0 {
		e.currentSearchIndex = 0
	} else if e.currentSearchIndex >= len(e.foundIndexes) {
		e.currentSearchIndex = len(e.foundIndexes) - 1
	}

	f := e.foundIndexes[e.currentSearchIndex]
	e.RowIndex = f.start.RowIndex
	e.ColIndex = f.start.ColIndex
	e.Draw()
	e.drawModeline()
}

func (e *Editor) MovePrevFoundWord() {
	if len(e.foundIndexes) == 0 {
		return
	}

	if e.currentSearchIndex == -1 {
		for i := len(e.foundIndexes) - 1; i >= 0; i-- {
			if e.foundIndexes[i].start.RowIndex <= e.RowIndex {
				e.currentSearchIndex = i
				break
			}
		}
	} else if e.currentSearchIndex == 0 {
		e.currentSearchIndex = len(e.foundIndexes) - 1
	} else {
		e.currentSearchIndex--
	}

	if e.currentSearchIndex < 0 {
		e.currentSearchIndex = 0
	} else if e.currentSearchIndex >= len(e.foundIndexes) {
		e.currentSearchIndex = len(e.foundIndexes) - 1
	}

	e.RowIndex = e.foundIndexes[e.currentSearchIndex].start.RowIndex
	e.ColIndex = e.foundIndexes[e.currentSearchIndex].start.ColIndex
	e.Draw()
	e.drawModeline()
}

// When not using regular expressions
func (e *Editor) SearchText(text string, caseSensitive, isRegexp bool, ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	e.currentSearchIndex = -1
	e.foundIndexes = []foundPosition{}

	textLen := len(text)
	if textLen == 0 {
		return
	}

	if isRegexp {
		e.SearchRegexp(text, caseSensitive, ctx)
	} else {
		e.searchText(text, caseSensitive, ctx)
	}
}

func (e *Editor) SearchRegexp(searchTerm string, caseSensitive bool, ctx context.Context) {
	// rows := e.Rows__()
	rows := e.Rows()
	re, err := regexp.Compile(searchTerm)
	if err != nil {
		return
	}
	for i := 0; i < rows.RowLength(); i++ {
		//s := rows.Row__(i).String__()
		matches := re.FindAllSubmatchIndex((*rows)[i], -1)
		// s := string((*rows)[i])
		// matches := re.FindAllStringSubmatchIndex(s, -1)
		if matches == nil {
			continue
		}
		for _, match := range matches {
			select {
			case <-ctx.Done():
				return
			default:
				//startIndex := utf8.RuneCountInString(s[:match[0]])
				//endIndex := utf8.RuneCountInString(s[:match[1]])
				//e.searchIndexes = append(e.searchIndexes, foundPosition{startRowIndex: i, startColIndex: startIndex, endRowIndex: i, endColIndex: endIndex})
				e.foundIndexes = append(e.foundIndexes, newFoundPosition(i, match[0], i, match[1]))
			}
		}
	}
}

func (e *Editor) searchText(text string, caseSensitive bool, ctx context.Context) {
	if !caseSensitive {
		text = strings.ToLower(text)
	}
	textBytes := []byte(text)
	textBytesLen := len(textBytes)

	lines := e.Rows()
	for i := 0; i < lines.RowLength(); i++ {
		line := (*lines)[i]
		index := 0
	loop:
		for limit := 0; ; limit++ {
			select {
			case <-ctx.Done():
				return
			default:
				substring := line[index:]
				if !caseSensitive {
					substring = bytes.ToLower(substring)
				}
				findIndex := bytes.Index(substring, textBytes)
				if findIndex == -1 {
					break loop
				}
				startIndex := len(line[:index+findIndex])
				stopIndex := startIndex + textBytesLen
				e.foundIndexes = append(e.foundIndexes, newFoundPosition(i, startIndex, i, stopIndex))
				index += findIndex + textBytesLen
			}

			if limit > 100_000 {
				e.screen.Echo("Search text over 100,000")
				return
			}
		}
	}
}

func (e *Editor) ReplaceCurrentSearchString(str string) {
	if e.currentSearchIndex == -1 {
		return
	}
	foundPosition := e.foundIndexes[e.currentSearchIndex]
	e.killRegion(file.Cursor{RowIndex: foundPosition.start.RowIndex, ColIndex: foundPosition.start.ColIndex},
		file.Cursor{RowIndex: foundPosition.start.RowIndex, ColIndex: foundPosition.stop.ColIndex})
	e.InsertString(str)

	// Correct the changed indexes within the same line where replacement is made
	// How many rune characters change due to replacement?
	// l := utf8.RuneCountInString(str) - (foundPosition.endColIndex - foundPosition.startColIndex)
	l := len([]byte(str)) - (foundPosition.stop.ColIndex - foundPosition.start.ColIndex)
	// RowIndex where replacement is made
	rowIndex := e.foundIndexes[e.currentSearchIndex].start.RowIndex
	// Correct the changed indexes due to replacement within the same line
	for i := e.currentSearchIndex + 1; i < len(e.foundIndexes) && e.foundIndexes[i].start.RowIndex == rowIndex; i++ {
		e.foundIndexes[i].start.ColIndex += l
		e.foundIndexes[i].stop.ColIndex += l
	}
	// Exclude the replaced search result
	// What if the replacement still matches the search after replacement? No consideration for now
	e.foundIndexes = slices.Delete(e.foundIndexes, e.currentSearchIndex, e.currentSearchIndex+1)
}

// ------------------------------------------------------------------
// Other
// ------------------------------------------------------------------

func (e *Editor) CharInfo() {
	ch, _ := utf8.DecodeRune((*e.Rows())[e.RowIndex][e.ColIndex:])
	str := e.runeToDisplayStringForModeline(ch)
	s := fmt.Sprintf("Char: '%s' (dec: %d, oct: %s, hex: %02X, %s), Cursor index: %d,%d", str, ch, strconv.FormatInt(int64(ch), 8), ch, utils.WidthKindString(ch), e.RowIndex, e.ColIndex)
	e.screen.Echo(s)
}

// ------------------------------------------------------------------
// Functions
// ------------------------------------------------------------------

// insertBytes は、バイトスライスを現在のカーソル位置に挿入し、カーソルを前進させます。
func (e *Editor) insertBytes(bytes []byte, enableUndo bool) {
	beforeCursor := e.Cursor
	bytesArray := file.SplitByLF(bytes)
	lines := e.Rows()
	// verb.PP("InsertBytes %s", string(bytes))

	for _, b := range bytesArray {
		bLen := len(b)
		if b[bLen-1] == '\n' {
			// verb.PP("1 b '%s' %d+%d", string(b), e.ColIndex, bLen)
			// 新しい行を追加
			lines.InsertRow(e.RowIndex+1, (*lines)[e.RowIndex][e.ColIndex:])
			// 現在の行にバイトスライスを追加
			lines.SetRow(e.RowIndex,
				append(
					append(
						make([]byte, 0, e.ColIndex+bLen),
						(*lines)[e.RowIndex][:e.ColIndex]...),
					b...))

			//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
			e.RowIndex++
			e.bsArray.Insert(e.RowIndex, 1)
			//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
			e.ColIndex = 0
		} else {
			//verb.PP("b '%s' %d+%d", string(b), e.ColIndex, bLen)
			lines.SetRow(e.RowIndex, slices.Insert((*lines)[e.RowIndex], e.ColIndex, b...))

			//verb.PP("'%s'", string((*lines)[e.RowIndex]))
			e.ColIndex += bLen
			//verb.PP("'%s'", string((*lines)[e.RowIndex][e.ColIndex:]))

			//e.makeAvailableBoundariesArray(e.RowIndex) // -------- !
		}
	}

	if enableUndo {
		e.UndoAction.Push(file.Action{Class: file.INSERT, Before: beforeCursor, After: e.Cursor, Data: bytes})
	}
	e.syncCursorAndBufferForEdit(INSERT, beforeCursor, e.Cursor)

	// コメントアウトされた部分は、将来的に必要な場合に対応
	// e.PrevCx = -1
	// e.dirtyFlag = true
}

// The provided Go function getColumnIndexClosestToCursorXPosition calculates the column index (colIndex) and the cursor's horizontal position (cx) in the logical row at a specific horizontal cursor position (cursorXPos).
// It does so by decoding the UTF-8 runes in the row and accumulating their widths until it reaches or surpasses cursorXPos. Here's an explanation of the code
// この関数 getColumnIndexClosestToCursorXPosition は、特定の水平カーソル位置 (cursorXPos) に最も近い論理行のカラムインデックス (colIndex) とカーソル位置 (cx) を計算します。
// この処理は、行内の UTF-8 ルーンをデコードし、それらの幅を累積して cursorXPos に到達または超えるまで続けます。
func (e *Editor) getColumnIndexClosestToCursorXPosition(rowIndex, indexOfLogicalRow, cursorXPos int) (colIndex, cx int) {
	// Get the boundaries of the logical row within the physical row.
	// bo := e.boundariesArray[rowIndex][indexOfLogicalRow]
	//bs := e.bsay.Boundaries(rowIndex)
	//bo := bs[indexOfLogicalRow]
	bo := e.bsArray.Boundary(rowIndex, indexOfLogicalRow)
	// bo := e.bsay[rowIndex].boundaries[indexOfLogicalRow]
	// Get the line content for the specified rowIndex.
	row := &(*e.Rows())[rowIndex]
	// Initialize colIndex to the start index of the logical row.
	for colIndex = bo.StartIndex; ; {
		// Decode the next rune starting from colIndex.
		ch, size := utf8.DecodeRune((*row)[colIndex:])
		// Get the display width of the rune.
		w, _ := e.runeWidth(ch, rowIndex, colIndex)
		// Check if adding the width of the rune exceeds cursorXPos or if colIndex reaches the end of the logical row.
		if cx+w > cursorXPos || colIndex+size >= bo.StopIndex {
			return colIndex, cx
		}
		// Update the horizontal cursor position.
		cx += w
		// Advance colIndex by the size of the decoded rune.
		colIndex += size
	}
}

// Return content widthout special charactor
func (e *Editor) getContentWidthoutSpecialCharactor(current file.Cursor, maxContentWidth int) (content string) {
	isSpecialChar := func(ch rune) bool {
		return ch < 32 || ch == define.DEL || ch == '　' || ch == define.NO_BREAK_SPACE
	}

	width := 0
	skip := false
	startCol := current.ColIndex
	for y := current.RowIndex; y < e.Rows().RowLength(); y++ {
		row, _ := e.Rows().GetRow(y)
		for x := startCol; x < len(*row); {
			ch, size := utf8.DecodeRune((*row)[x:])
			w, _ := e.runeWidth(ch, y, x)
			x += size // Don't use x below
			s := string(ch)
			if isSpecialChar(ch) {
				if skip {
					continue
				}
				skip = true
				if ch == '\n' {
					s = string(theme.MarkLinefeed)
				} else if ch == '\t' {
					s = string(' ')
				} else {
					s = string(theme.MarkContinue)
				}
				w = 1
			} else {
				skip = false
			}
			if width+w > maxContentWidth {
				return content
			}
			content += s
			width += w
		}
		startCol = 0
	}
	return content
}

func (e *Editor) centerViewOnCursor() {
	e.Cy = int(e.editArea.Height / 2)
}

func (e *Editor) moveCursor(x, y int) {
	e.Cx = x
	e.Cy = y
	e.screen.Echo("")
	e.showCursor(x, y)
}

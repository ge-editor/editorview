package te

import (
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

	"github.com/ge-editor/utils"

	"github.com/ge-editor/theme"

	"github.com/ge-editor/te/file"
	"github.com/ge-editor/te/mark"
)

func (e *Editor) InsertString(s string) (file.Cursor, bool) {
	chs := []rune(s)
	cursor, ok := e.File.Insert(e.Cursor, chs)
	if ok {
		e.UndoAction.Push(file.Action{Class: file.Insert, Before: e.Cursor, After: cursor, Data: chs})
		e.SyncEdits(insert, e.File, e.Cursor, cursor)
		e.Cursor = cursor
	}
	return cursor, ok
}

// Move cursor one character forward.
func (e *Editor) MoveCursorForward() {
	x, y := e.Cx, e.Cy

	row := e.Row(e.RowIndex)
	if row.IsEndOfRow(e.ColIndex) {
		if e.IsLastRow(e.RowIndex) {
			e.screen.Echo("End of buffer")
			return
		}
		y++
		e.RowIndex++
		x = 0
		e.ColIndex = 0
	} else {
		vl := e.vlines.GetVline(e.RowIndex)
		if vl.IsEndOfLogicalRow(e.ColIndex) {
			y++
			x = 0
		} else {
			x += vl.GetCell(e.ColIndex).GetCellWidth()
		}
		e.ColIndex++
	}

	e.PrevCx = x
	e.moveCursor(x, y)
}

// If the file has already been read, use that buffer
func (e *Editor) OpenFile(path string) error {
	ff, meta, err := BufferSets.GetFileAndMeta(path)
	e.File = ff
	e.Meta = meta

	e.vlines.SetFile(ff)
	e.vlines.AllocateVlines(meta.RowIndex)

	return err // return message
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
		vl := e.vlines.GetVline(e.RowIndex)
		bs := vl.Boundaries()
		lastBs := (*bs)[len(*bs)-1]
		x = lastBs.Widths() - 1 // on linefeed
		e.ColIndex = lastBs.Index()
	} else {
		vl := e.vlines.GetVline(e.RowIndex)
		before := vl.GetIndexOfLogicalRow(e.ColIndex)
		e.ColIndex--
		vl = e.vlines.GetVline(e.RowIndex)
		after := vl.GetIndexOfLogicalRow(e.ColIndex)
		if after < before {
			// if vl.GetIndexOfLogicalRow(e.ColIndex) < logicalRowIndex {
			y--
			b := (*vl.Boundaries())[after]
			x = b.Widths() - vl.GetCell(e.ColIndex).GetCellWidth() // Move to the end of the previous row
		} else {
			x -= vl.GetCell(e.ColIndex).GetCellWidth()
		}
	}

	e.PrevCx = x
	e.moveCursor(x, y)
}

// Move cursor to the next line.
func (e *Editor) MoveCursorNextLine() {
	min, max := 0, 0

	vl := e.vlines.GetVline(e.RowIndex)
	if vl.IsLastLogicalRow(e.ColIndex) { // last logical line
		if e.IsLastRow(e.RowIndex) {
			e.screen.Echo("End of buffer")
			return
		}
		// move to next row
		e.RowIndex++
		vl = e.vlines.GetVline(e.RowIndex)
		min, max = vl.GetMinAndMaxIndexOfLogicalRow(0) // first logical line
	} else {
		// move to next logical line
		// What index number in logic row?
		logicalRowIndex := vl.GetIndexOfLogicalRow(e.ColIndex)
		min, max = vl.GetMinAndMaxIndexOfLogicalRow(logicalRowIndex + 1) // next logical line
	}

	x := 0
	for i := min; ; i++ {
		e.ColIndex = i
		w := vl.GetCell(i).GetCellWidth()
		if x+w > e.PrevCx || i == max {
			break
		}
		x += w
	}

	e.moveCursor(x, e.Cy+1)
}

// Move cursor to the previous line.
func (e *Editor) MoveCursorPrevLine() {
	// What index number in logic row?
	vl := e.vlines.GetVline(e.RowIndex)
	rowIndex := vl.GetIndexOfLogicalRow(e.ColIndex)

	min, max := 0, 0
	if rowIndex == 0 { // first logical line
		if e.RowIndex == 0 {
			e.screen.Echo("Beginning of buffer")
			return
		}
		// move to prev row
		e.RowIndex-- // previous row
		vl := e.vlines.GetVline(e.RowIndex)
		min, max = vl.GetMinAndMaxIndexOfLogicalRow(-1) // last logical line
	} else {
		min, max = vl.GetMinAndMaxIndexOfLogicalRow(rowIndex - 1) // last logical line
	}

	vl = e.vlines.GetVline(e.RowIndex)
	x := 0
	for i := min; ; i++ {
		e.ColIndex = i
		cell := vl.GetCell(i)
		w := cell.GetCellWidth()
		if x+w > e.PrevCx || i == max {
			break
		}
		x += w
	}

	e.moveCursor(x, e.Cy-1)
}

// Move cursor to the end of the line.
func (e *Editor) MoveCursorEndOfLine() {
	vl := e.vlines.GetVline(e.RowIndex)
	// What index number in logic row?
	rowIndex := vl.GetIndexOfLogicalRow(e.ColIndex)

	row := e.Row(e.RowIndex)
	e.ColIndex = row.LenCh() - 1

	// cursor display position
	bs := vl.Boundaries()
	e.Cy += len(*bs) - 1 - rowIndex
}

// Move cursor to the end of logical the line.
// At the end of a logical line, the cursor should at the beginning of the next logical line. so I see...
func (e *Editor) MoveCursorEndOfLogicalLine() {
	vl := e.vlines.GetVline(e.RowIndex)
	// What index number in logic row?
	rowIndex := vl.GetIndexOfLogicalRow(e.ColIndex)
	bs := vl.Boundaries()

	for i := rowIndex; i < len(*bs); i++ {
		if i == len(*bs)-1 {
			e.ColIndex = (*bs)[i].Index()
			break
		} else {
			if e.ColIndex > (*bs)[i].Index() {
				continue
			}
			e.ColIndex = (*bs)[i].Index() + 1
			break
		}
	}

	// What index number in logic row?
	e.Cy += vl.GetIndexOfLogicalRow(e.ColIndex) - rowIndex
	e.PrevCx = -1
}

// Move cursor to the beginning of the line.
func (e *Editor) MoveCursorBeginningOfLine() {
	if e.RowIndex == 0 && e.ColIndex == 0 {
		e.screen.Echo("Beginning of buffer")
		return
	}

	start := 0
	row := e.Row(e.RowIndex)
	end := row.LenCh() - 1

	vl := e.vlines.GetVline(e.RowIndex)
	// What index number in logic row?
	rowIndex := vl.GetIndexOfLogicalRow(e.ColIndex)

	// find indent
	indentIndex := start
	for indentIndex = start; indentIndex <= end; indentIndex++ {
		if row.Ch(indentIndex) == ' ' || row.Ch(indentIndex) == '\t' {
			continue
		}
		break
	}

	if indentIndex == e.ColIndex || indentIndex == end {
		indentIndex = 0
	}
	e.ColIndex = indentIndex

	e.Cy -= rowIndex
}

// Move cursor to the beginning of the logical line.
func (e *Editor) MoveCursorBeginningOfLogicalLine() {
	if e.RowIndex == 0 && e.ColIndex == 0 {
		e.screen.Echo("Beginning of buffer")
		return
	}

	vl := e.vlines.GetVline(e.RowIndex)
	// What index number in logic row?
	rowIndex := vl.GetIndexOfLogicalRow(e.ColIndex)

	bs := vl.Boundaries()
	start := 0
	end := (*bs)[0].Index()
	if rowIndex > 0 {
		if e.ColIndex == (*bs)[rowIndex-1].Index()+1 { // logical beginning of line
			if rowIndex > 1 {
				start = (*bs)[rowIndex-2].Index() + 1
			}
			end = (*bs)[rowIndex-1].Index()
		} else {
			start = (*bs)[rowIndex-1].Index() + 1
			end = (*bs)[rowIndex].Index()
		}
	}

	// find indent
	row := e.Row(e.RowIndex)
	indentIndex := start
	for indentIndex = start; indentIndex <= end; indentIndex++ {
		if row.Ch(indentIndex) == ' ' || row.Ch(indentIndex) == '\t' {
			continue
		}
		break
	}

	if e.ColIndex < indentIndex && indentIndex > 0 {
		e.ColIndex = indentIndex
	} else {
		if indentIndex > start && indentIndex < e.ColIndex {
			e.ColIndex = indentIndex
		} else {
			e.ColIndex = start
		}
	}

	e.Cy -= rowIndex - vl.GetIndexOfLogicalRow(e.ColIndex)
	e.PrevCx = -1
}

func (e *Editor) MoveCursorBeginningOfFile() {
	e.RowIndex = 0
	e.ColIndex = 0
	e.Cy = 0
	e.Cx = 0
	e.vlines.AllocateVlines(e.RowIndex)
}

func (e *Editor) MoveCursorEndOfFile() {
	e.RowIndex = e.Rows().LenRows() - 1
	row := e.Row(e.RowIndex)
	e.ColIndex = row.LenCh() - 1
	e.Cy = e.editArea.Height - e.verticalThreshold
	// e.Cx = 0
	e.vlines.AllocateVlines(e.RowIndex)
	// e.vlines.ReleaseAll()
}

// Insert a rune 'ch' at the current cursor position, advance cursor one character forward.
func (e *Editor) InsertRune(ch rune) {
	vl := e.vlines.GetVline(e.RowIndex)
	logicalRowIndex := vl.GetIndexOfLogicalRow(e.ColIndex)
	// rowIndex := e.RowIndex

	cursor, ok := e.Insert(e.Cursor, []rune{ch})
	if !ok {
		e.screen.Echo("Error insert_rune")
		return
	}
	e.UndoAction.Push(file.Action{Class: file.Insert, Before: e.Cursor, After: cursor, Data: []rune{ch}})

	e.SyncEdits(insert, e.File, e.Cursor, cursor)
	e.Cursor = cursor
	if ch == '\n' {
		// e.vlines.InsertN(rowIndex+1, 1)
		e.Cy++
	} else {
		vl = e.vlines.GetVline(e.RowIndex)
		if vl.GetIndexOfLogicalRow(e.ColIndex) > logicalRowIndex {
			e.Cy++
		}
	}

	/*
		e.PrevCx = -1
		e.dirtyFlag = true
	*/
}

func (e *Editor) DeleteRuneBackward() {
	before := e.Cursor

	if e.ColIndex == 0 {
		if e.RowIndex == 0 {
			e.screen.Echo("Beginning of buffer")
			return
		}
		// join to prev row
		e.RowIndex--
		vl := e.vlines.GetVline(e.RowIndex)
		bs := vl.Boundaries()
		e.ColIndex = (*bs)[len(*bs)-1].Index()
	} else {
		e.ColIndex--
	}

	removed := e.RemoveRegion(e.Cursor, before)
	e.vlines.Release(e.RowIndex, -1) //
	e.UndoAction.Push(file.Action{Class: file.DeleteBackward, Before: before, After: e.Cursor, Data: *removed})
	e.SyncEdits(delete, e.File, e.Cursor, before)

	e.PrevCx = -1
}

// If at the EOL, move contents of the next line to the end of the current line,
// erasing the next line after that. Otherwise, delete one character under the
// cursor.
func (e *Editor) DeleteRune() {
	e.killLineOrDeleteRune(false)
}

// Move view 'n' lines forward or backward only if it's possible.
func (e *Editor) MoveViewHalfForward() {
	n := e.Height / 2
	e.screen.Echo("")

	vl := e.vlines.GetVline(e.RowIndex)
	// What index is e.ColIndex in the logical line?
	logicalRowIndex := vl.GetIndexOfLogicalRow(e.ColIndex)

	// Compute total, logicalRowLength, rowIndex and vl
	total := 0
	logicalRowLength := 0
	rowIndex := e.RowIndex
	for ; ; rowIndex++ {
		vl = e.vlines.GetVline(rowIndex)
		logicalRowLength = vl.LenLogicalRow()
		// Add the number of logical row. first subtract the number of logical row before the cursor
		total += logicalRowLength - logicalRowIndex
		logicalRowIndex = 0 // No need to subtract after the second time, so 0
		if total >= n || rowIndex == e.LenRows()-1 {
			break
		}
	}
	e.RowIndex = rowIndex

	// Compute logicalRowIndex
	logicalRowIndex = logicalRowLength - 1
	// If the number of lines to scroll is exceeded,
	// subtract the number of logical lines exceeded
	if total > n {
		logicalRowIndex -= total - n
		/*
			if logicalRowIndex < 0 {
				panic("logicalRowIndex")
			}
		*/
	}

	// Compute e.ColIndex from logicalRowIndex
	e.ColIndex = (func() int {
		min, max := vl.GetMinAndMaxIndexOfLogicalRow(logicalRowIndex)
		w := 0
		for colIndex := min; colIndex <= max; colIndex++ {
			if w+vl.GetCell(colIndex).GetCellWidth() > e.PrevCx {
				return colIndex
			}
			w += vl.GetCell(colIndex).GetCellWidth()
		}
		return max
	})()
}

// Move view 'n' lines forward or backward.
func (e *Editor) MoveViewHalfBackward( /* n int */ ) {
	n := e.Height / 2
	e.screen.Echo("")

	vl := e.vlines.GetVline(e.RowIndex)
	// What index number in logic row?
	logicalRowIndex := vl.GetIndexOfLogicalRow(e.ColIndex)

	total := n
	logicalRowLength := logicalRowIndex
	rowIndex := e.RowIndex
	// for i := logicalRowIndex; i >= 0; i-- {
	for {
		total -= logicalRowLength // Subtract the number of logical rows
		if total <= 0 || rowIndex == 0 {
			break
		}
		rowIndex--
		vl = e.vlines.GetVline(rowIndex)
		logicalRowLength = vl.LenLogicalRow()
	}
	e.RowIndex = rowIndex

	// Compute logicalRowIndex
	logicalRowIndex = 0 // logicalRowLength - 1
	// If the number of lines to scroll is exceeded,
	// subtract the number of logical lines exceeded
	if total > n {
		// logicalRowIndex += total - n
		logicalRowIndex -= n
		/*
			if logicalRowIndex < 0 {
				panic("logicalRowIndex")
			}
		*/
	}

	// Compute e.ColIndex from logicalRowIndex
	e.ColIndex = (func() int {
		min, max := vl.GetMinAndMaxIndexOfLogicalRow(logicalRowIndex)
		w := 0
		for colIndex := min; colIndex <= max; colIndex++ {
			if w+vl.GetCell(colIndex).GetCellWidth() > e.PrevCx {
				return colIndex
			}
			w += vl.GetCell(colIndex).GetCellWidth()
		}
		return max
	})()
}

func (e *Editor) ReplaceCurrentSearchString(str string) {
	if e.currentSearchIndex == -1 {
		return
	}
	foundPosition := e.searchIndexes[e.currentSearchIndex]
	e.killRegion(file.Cursor{RowIndex: foundPosition.row, ColIndex: foundPosition.start},
		file.Cursor{RowIndex: foundPosition.row, ColIndex: foundPosition.end})
	e.InsertString(str)

	// Correct the changed indexes within the same line where replacement is made
	// How many rune characters change due to replacement?
	l := utf8.RuneCountInString(str) - (foundPosition.end - foundPosition.start)
	// RowIndex where replacement is made
	rowIndex := e.searchIndexes[e.currentSearchIndex].row
	// Correct the changed indexes due to replacement within the same line
	for i := e.currentSearchIndex + 1; i < len(e.searchIndexes) && e.searchIndexes[i].row == rowIndex; i++ {
		e.searchIndexes[i].start += l
		e.searchIndexes[i].end += l
	}
	// Exclude the replaced search result
	// What if the replacement still matches the search after replacement? No consideration for now
	e.searchIndexes = slices.Delete(e.searchIndexes, e.currentSearchIndex, e.currentSearchIndex+1)
}

// Kill region and push undo and kill buffers
func (e *Editor) killRegion(a, b file.Cursor) {
	e.Cursor = a
	removed := e.RemoveRegion(a, b)
	if removed == nil {
		return
	}
	e.SyncEdits(delete, e.File, a, b)
	e.UndoAction.Push(file.Action{Class: file.DeleteBackward, Before: a, After: a, Data: *removed})
	if err := kill_buffer.KillBuffer.PushKillBuffer(*removed); err != nil {
		e.screen.Echo(err.Error())
	}
}

func (e *Editor) Kill_region() {
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
func (e *Editor) killLineOrDeleteRune(isKillLine bool) {
	// x, y := e.Cx, e.Cy
	var removed *[]rune
	var b file.Cursor

	row := e.Row(e.RowIndex)
	if row.IsEndOfRow(e.ColIndex) {
		if e.IsLastRow(e.RowIndex) {
			e.screen.Echo("End of buffer")
			return
		}
		// delete linefeed
		b = file.NewCursor(e.RowIndex+1, 0)
		removed = e.RemoveRegion(e.Cursor, b)
		// verb.PP("removed '%s'", string(*removed))
	} else {
		if isKillLine {
			b = file.NewCursor(e.RowIndex, row.LenCh()-1)
			removed = e.RemoveRegion(e.Cursor, b)
		} else {
			// delete rune
			b = file.NewCursor(e.RowIndex, e.ColIndex+1)
			removed = e.RemoveRegion(e.Cursor, b)
			// verb.PP("delete rune %v %v", e.Cursor, b)
		}
	}
	e.SyncEdits(delete, e.File, e.Cursor, b)
	e.UndoAction.Push(file.Action{Class: file.Delete, Before: e.Cursor, After: e.Cursor, Data: *removed})

	e.PrevCx = -1
	// e.SetDirtyFlag(true)
	// row.RedrawFlag = true
}

// If not at the EOL, remove contents of the current line from the cursor to the end.
// Otherwise behave like 'delete'.
func (e *Editor) KillLine() {
	e.killLineOrDeleteRune(true)
}

func (e *Editor) YankFromClipboard() {
	s, err := clipboard.ReadAll()
	if err != nil {
		e.screen.Echo(err.Error())
		return
	}
	r := []rune(s)
	cursor, ok := e.Insert(e.Cursor, r)
	if !ok {
		e.screen.Echo("Error yank")
		return
	}
	e.SyncEdits(insert, e.File, e.Cursor, cursor)
	e.UndoAction.Push(file.Action{Class: file.Insert, Before: e.Cursor, After: cursor, Data: r})
	e.Cursor = cursor
}

func (e *Editor) Yank() {
	r := kill_buffer.KillBuffer.GetLast()
	if r == nil {
		return
	}
	cursor, ok := e.Insert(e.Cursor, r)
	if !ok {
		e.screen.Echo("Error yank")
		return
	}
	e.SyncEdits(insert, e.File, e.Cursor, cursor)
	e.UndoAction.Push(file.Action{Class: file.Insert, Before: e.Cursor, After: cursor, Data: r})
	e.Cursor = cursor
}

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

func (e *Editor) Undo() {
	if e.UndoAction.IsEmpty() {
		e.screen.Echo("No further undo information")
		return
	}

	a, _ := e.UndoAction.Pop()
	if a.Class == file.Insert {
		// verb.PP("vc_undo %v %v", a.Before, a.After)
		e.RemoveRegion(a.Before, a.After)
		e.SyncEdits(delete, e.File, a.Before, a.After)
		e.Cursor = a.Before
	} else { // a.Class == Delete or DeleteBackward
		var ok bool
		if a.Class == file.Delete {
			_, ok = e.Insert(a.Before, a.Data)
			e.SyncEdits(insert, e.File, a.Before, a.After)
		} else {
			_, ok = e.Insert(a.After, a.Data)
			e.SyncEdits(insert, e.File, a.After, a.Before)
			e.Cursor = a.Before
		}
		if !ok {
			e.screen.Echo("Error undo")
			return
		}
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
	if a.Class == file.Insert {
		e.Insert(a.Before, a.Data)
		e.SyncEdits(insert, e.File, a.Before, a.After)
	} else if a.Class == file.DeleteBackward {
		e.RemoveRegion(a.After, a.Before)
		e.SyncEdits(delete, e.File, a.After, a.Before)
	} else { // a.Class == Delete
		lines := file.Split(a.Data, '\n')
		last := lines[len(lines)-1]
		cursor := a.After
		cursor.RowIndex += len(lines) - 1
		cursor.ColIndex += len(last)
		if last[len(last)-1] == '\n' {
			cursor.RowIndex++
			cursor.ColIndex = 0
		}
		e.RemoveRegion(a.Before, cursor)
		e.SyncEdits(delete, e.File, a.Before, cursor)
	}
	e.Cursor = a.After

	e.screen.Echo("Redo!")
	e.UndoAction.Push(a)
}

func (e *Editor) moveCursor(x, y int) {
	e.Cx = x
	e.Cy = y
	e.screen.Echo("")
	e.showCursor(x, y)
}

func (e *Editor) CharInfo() {
	s := ""
	row := e.Row(e.RowIndex)
	ch := row.Ch(e.ColIndex)
	str := e.runeToDisplayString(ch)
	s += fmt.Sprintf("Char: '%s' (dec: %d, oct: %s, hex: %02X, %s), Cursor index: %d,%d", str, ch, strconv.FormatInt(int64(ch), 8), ch, utils.WidthKindString(ch), e.RowIndex, e.ColIndex)
	e.screen.Echo(s)
}

// "lemp" stands for "line edit mode params"
func (e *Editor) MoveCursorToLine(lineNumber int) {
	if lineNumber < 1 || lineNumber > e.LenRows() {
		return
	}
	e.RowIndex = lineNumber - 1
	e.ColIndex = 0
	e.Cy = (e.Height - 1) / 2

	e.vlines.AllocateVlines(e.RowIndex)
}

// ============================
// Search text
// ============================

func (e *Editor) GetFindIndexes() []foundPosition {
	return e.searchIndexes
}

func (e *Editor) MoveNextFoundWord() {
	if len(e.searchIndexes) == 0 {
		return
	}

	if e.currentSearchIndex == -1 {
		for i := 0; i < len(e.searchIndexes); i++ {
			if e.searchIndexes[i].row >= e.RowIndex {
				e.currentSearchIndex = i
				break
			}
		}
	} else if e.currentSearchIndex == len(e.searchIndexes)-1 {
		e.currentSearchIndex = 0
	} else {
		e.currentSearchIndex++
	}

	if e.currentSearchIndex < 0 {
		e.currentSearchIndex = 0
	} else if e.currentSearchIndex >= len(e.searchIndexes) {
		e.currentSearchIndex = len(e.searchIndexes) - 1
	}

	f := e.searchIndexes[e.currentSearchIndex]
	e.RowIndex = f.row
	e.ColIndex = f.start
	e.Draw()
	e.drawModeline()
}

func (e *Editor) MovePrevFoundWord() {
	if len(e.searchIndexes) == 0 {
		return
	}

	if e.currentSearchIndex == -1 {
		for i := len(e.searchIndexes) - 1; i >= 0; i-- {
			if e.searchIndexes[i].row <= e.RowIndex {
				e.currentSearchIndex = i
				break
			}
		}
	} else if e.currentSearchIndex == 0 {
		e.currentSearchIndex = len(e.searchIndexes) - 1
	} else {
		e.currentSearchIndex--
	}

	if e.currentSearchIndex < 0 {
		e.currentSearchIndex = 0
	} else if e.currentSearchIndex >= len(e.searchIndexes) {
		e.currentSearchIndex = len(e.searchIndexes) - 1
	}

	e.RowIndex = e.searchIndexes[e.currentSearchIndex].row
	e.ColIndex = e.searchIndexes[e.currentSearchIndex].start
	e.Draw()
	e.drawModeline()
}

// When not using regular expressions
func (e *Editor) SearchText(text string, caseSensitive, isRegexp bool, ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	e.currentSearchIndex = -1
	e.searchIndexes = []foundPosition{}

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
	rows := e.Rows()
	re, err := regexp.Compile(searchTerm)
	if err != nil {
		return
	}
	for i := 0; i < rows.LenRows(); i++ {
		s := rows.Row(i).String()
		matches := re.FindAllStringSubmatchIndex(s, -1)
		if matches == nil {
			continue
		}
		for _, match := range matches {
			select {
			case <-ctx.Done():
				return
			default:
				startIndex := utf8.RuneCountInString(s[:match[0]])
				endIndex := utf8.RuneCountInString(s[:match[1]])
				e.searchIndexes = append(e.searchIndexes, foundPosition{row: i, start: startIndex, end: endIndex})
			}
		}
	}
}

func (e *Editor) searchText(text string, caseSensitive bool, ctx context.Context) {
	l := utf8.RuneCountInString(text)
	rows := e.Rows()
	for i := 0; i < rows.LenRows(); i++ {
		s := rows.Row(i).String()
		index := 0
	loop:
		for limit := 0; ; limit++ {
			select {
			case <-ctx.Done():
				return
			default:
				s2 := s[index:]
				text2 := text
				if !caseSensitive {
					s2 = strings.ToLower(s2)
					text2 = strings.ToLower(text2)
				}
				findIndex := strings.Index(s2, text2)
				if findIndex == -1 {
					break loop
				}
				startIndex := utf8.RuneCountInString(s[:index+findIndex])
				endIndex := startIndex + l
				e.searchIndexes = append(e.searchIndexes, foundPosition{row: i, start: startIndex, end: endIndex})
				index += findIndex + len(text)
			}

			if limit > 100_000 {
				e.screen.Echo("Search text over 100,000")
				return
			}
		}
	}
}

func (e *Editor) Autoindent() {
	row := e.Row(e.RowIndex)
	spaces := row.BigginingSpaces()
	e.InsertRune('\n')
	for _, c := range spaces {
		e.InsertRune(c)
	}
}

func (e *Editor) InsertTab() {
	if e.IsSoftTab() {
		w := utils.TabWidth(e.Cx, e.GetTabWidth())
		for i := 0; i < w; i++ {
			e.InsertRune(' ')
		}
	} else {
		e.InsertRune('\t')
	}
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

// Return content widthout special charactor
func (e *Editor) getContentWidthoutSpecialCharactor(current file.Cursor, maxContentWidth int) string {
	isSpecialChar := func(ch rune) bool {
		return ch < 32 || ch == define.DEL || ch == '　' || ch == define.NO_BREAK_SPACE
	}

	content := ""
	width := 0
	skip := false
	start := current.ColIndex
	for y := current.RowIndex; y < e.LenRows(); y++ {
		row := e.Row(y)
		vl := e.vlines.GetVline(y)
		for x := start; x < row.LenCh(); x++ {
			ch := row.Ch(x)
			s := string(ch)
			w := vl.GetCell(x).GetCellWidth()
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
		start = 0
	}
	return content
}

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

func (e *Editor) centerViewOnCursor() {
	e.Cy = int(e.editArea.Height / 2)
}

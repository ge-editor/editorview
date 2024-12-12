// Editor Struct implements the gecore.tree.Leaf interface

package te

import (
	"fmt"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"

	"github.com/ge-editor/gecore"
	"github.com/ge-editor/gecore/define"
	"github.com/ge-editor/gecore/kill_buffer"
	"github.com/ge-editor/gecore/screen"
	"github.com/ge-editor/gecore/tree"
	"github.com/ge-editor/gecore/verb"

	"github.com/ge-editor/utils"

	"github.com/ge-editor/theme"

	"github.com/ge-editor/te/buffer"
	"github.com/ge-editor/te/file"
	"github.com/ge-editor/te/mark"
)

const (
	verticalThreshold = 5
)

var (
	// Initialization has been moved to the newEditor function
	// BufferSets, _ = buffer.NewBufferSets(gecore.Files)
	BufferSets *buffer.BufferSets
	Marks      = mark.NewMarks()
)

func newEditor() *Editor {
	if BufferSets == nil {
		var err error
		BufferSets, err = buffer.NewBufferSets(gecore.Files)
		if err != nil {
			fmt.Println(err.Error())
			verb.PP("%s", err.Error())
		}
	}
	e := &Editor{
		File: (*BufferSets)[0].File,
		Meta: (*BufferSets)[0].PopMeta(),
	}
	e.bsArray = NewBoundariesArray(e)
	return e
}

// Record the character width at the character's position on the screen in the map array.
func (e *Editor) setSpecialCharWidths(rowIndex, colIndex, width int) {
	// Initialize map array
	if e.specialCharWidths == nil {
		e.specialCharWidths = make([]map[int]int, len(*e.Rows()))
	}

	// Add the array size up to the row index
	for len(e.specialCharWidths) <= rowIndex {
		e.specialCharWidths = append(e.specialCharWidths, nil)
	}

	// Initialize index item of array to map
	if e.specialCharWidths[rowIndex] == nil {
		e.specialCharWidths[rowIndex] = make(map[int]int)
	}

	// Sets the character width for a row and column position
	e.specialCharWidths[rowIndex][colIndex] = width
}

func newFoundPosition(startRowIndex, startColIndex, stopRowIndex, stopColIndex int) foundPosition {
	return foundPosition{
		start: file.Cursor{
			RowIndex: startRowIndex,
			ColIndex: startColIndex,
		},
		stop: file.Cursor{
			RowIndex: stopRowIndex,
			ColIndex: stopColIndex,
		},
	}
}

// Position found in search results
type foundPosition struct {
	start file.Cursor
	stop  file.Cursor
}

// Returns the first index of the found search position that matches the row index.
// Return -1, not found match position.
func (e *Editor) getFoundPosition(rowIndex int) int {
	for i := 0; i < len(e.foundIndexes); i++ {
		if e.foundIndexes[i].start.RowIndex >= rowIndex {
			return i
		}
	}
	return -1
}

// ------------------------------------------------------------------
// cell
// ------------------------------------------------------------------

type cell struct {
	size  int
	width int
	class screen.CharClass
}

func (m *cell) isEmpty() bool {
	return m.class == 0
}

func (c *cell) clear() {
	*c = cell{}
}

// ------------------------------------------------------------------
// Editor implement gecore Leaf interface
// ------------------------------------------------------------------

// Editor Struct implements the gecore.tree.Leaf interface
type Editor struct {
	parentView tree.View
	screen     *screen.Screen
	active     bool

	utils.Rect            // overall position on screen
	viewArea   utils.Rect // include mode line
	editArea   utils.Rect

	verticalThreshold int // Changes depending on screen size

	*file.File
	*buffer.Meta

	bsArray BoundariesArray // boundaries array of logical row

	specialCharWidths []map[int]int // Tab width

	currentSearchIndex int
	foundIndexes       []foundPosition
}

// ------------------------------------------------------------------
// Methods of gecore Leaf interface
// ------------------------------------------------------------------

func (e *Editor) View() *tree.View {
	return &e.parentView
}

func (e *Editor) Resize(width, height int, rect utils.Rect) {
	e.Width, e.Height = width, height

	e.viewArea = rect
	e.editArea = rect
	if !e.rightmost() {
		e.editArea.Width -= 1 // right bar
	}
	e.editArea.Height -= 1 // status
	//e.drawRightBar()

	e.verticalThreshold = utils.Threshold(verticalThreshold, e.editArea.Height)

	e.bsArray.ClearAll()
}

func (e *Editor) Draw() {
	e.drawView()
	e.drawRightBar()
}

func (e *Editor) Redraw() {
	e.centerViewOnCursor()
	e.Draw()
}

// This function is called once for each tree.Leaf on the screen.
func (e *Editor) Kill(leaf *tree.Leaf, isActive bool) *tree.Leaf {
	bufferSetsIndex := -1

	if !isActive {
		bufferSetsIndex = BufferSets.GetIndexByBufferFile(e.File)
		if bufferSetsIndex >= 0 {
			return leaf
			//var tv tree.Leaf = e
			//return &tv
		}
	}

	leafEditor := (*leaf).(*Editor)
	if isActive {
		bufferSetsIndex = BufferSets.RemoveByBufferFile(leafEditor.File) // 該当するバッファを取り除く
	}
	l := len(*BufferSets)
	if l == 0 {
		// If the number of buffers is 0, create a new buffer
		ff, meta, err := BufferSets.GetFileAndMeta("unnamed")
		e.File = ff
		e.Meta = meta
		e.screen.Echo(err.Error())
	} else {
		// Set the held buffer to the editor
		if bufferSetsIndex < 0 {
			bufferSetsIndex = 0
		} else if bufferSetsIndex > l-1 {
			bufferSetsIndex = l - 1
		}
		bs := (*BufferSets)[bufferSetsIndex]
		e.File = bs.File
		e.Meta = bs.PopMeta()
	}

	var tv tree.Leaf = e
	return &tv
}

func (e *Editor) ViewActive(a bool) {
	e.active = a
}

// If event requires special handling
// For EventResize, use the Resize method of interface
func (e *Editor) Event(tev *tcell.Event) *tcell.Event {
	return tev
}

func (e *Editor) Resume() {
}

func (e *Editor) Init() {
}

func (e *Editor) WillClose() {
}

// Return the index of the logical line that contains the specified column.
func (e *Editor) getIndexOfLogicalRow(rowIndex, colIndex int) (int, bool) {
	l := e.bsArray.BoundariesLen(rowIndex)
	for i := 0; i < l; i++ {
		bo := e.bsArray.Boundary(rowIndex, i)
		if colIndex >= bo.StartIndex && colIndex < bo.StopIndex {
			return i, true
		}
	}
	return 0, false
}

// Check if the column index is within the last boundary of the specified row
// Return false: out of index or not initialized.
func (e *Editor) inEndOfLogicalRow(rowIndex, colIndex int) bool {
	lastBoundary := e.bsArray.LastBoundary(rowIndex)
	return colIndex >= lastBoundary.StartIndex && colIndex < lastBoundary.StopIndex
}

// ------------------------------------------------------------------
// SyncEditor
// ------------------------------------------------------------------

type syncType int

const (
	INSERT syncType = iota
	DELETE
)

// syncEdits adjusts cursor positions and buffer boundaries based on the type of edit (insert or delete).
func (e *Editor) syncCursorAndBufferForEdit(sync syncType, start, end file.Cursor) {
	// Ensure start is before end; swap if necessary.
	if start.RowIndex > end.RowIndex || (start.RowIndex == end.RowIndex && start.ColIndex > end.ColIndex) {
		start, end = end, start
	}

	// Synchronize cursor positions in the buffer sets associated with the edited file.
	for _, buffSet := range *BufferSets {
		// Skip if this buffer set is not linked to the file being edited.
		if buffSet.File != e.File {
			continue
		}

		for _, meta := range buffSet.GetMetas() {
			// Adjust cursor based on the type of edit.
			switch sync {
			case INSERT:
				meta.Cursor.AdjustForInsertion(start, end)
			case DELETE:
				meta.Cursor.AdjustForDeletion(start, end)
			}
		}
		break
	}

	// Synchronize cursor positions and buffer boundaries in other editors linked to the same file.
	leaves := tree.GetLeavesByViewName("te")
	for _, leaf := range leaves {
		editor := (*leaf).(*Editor)
		// Skip if the editor is linked to a different file or is the current editor.
		if editor.File != e.File {
			continue
		}
		// Adjust foundIndex that is results of search and replace
		for i := 0; i < len(editor.foundIndexes); i++ {
			verb.PP("fc1 %v", editor.foundIndexes[i])
			switch sync {
			case INSERT:
				editor.foundIndexes[i].start.AdjustForInsertion(start, end)
				editor.foundIndexes[i].stop.AdjustForInsertion(start, end)
			case DELETE:
				editor.foundIndexes[i].start.AdjustForDeletion(start, end)
				editor.foundIndexes[i].stop.AdjustForDeletion(start, end)
			}
			verb.PP("fc2 %v", editor.foundIndexes[i])
		}
		if editor == e {
			continue
		}

		switch sync {
		case INSERT:
			editor.Cursor.AdjustForInsertion(start, end)

			// Update buffer boundary array if rows were inserted.
			if end.RowIndex-start.RowIndex > 0 {
				editor.bsArray.Insert(start.RowIndex+1, end.RowIndex-(start.RowIndex+1))
			}
			// Optional: Update virtual lines or boundaries if needed.
			// editor.vlines.Release__(start.RowIndex, start.RowIndex+1)
			// editor.vlines.Insert__(start.RowIndex+1, end.RowIndex)

		case DELETE:
			editor.Cursor.AdjustForDeletion(start, end)

			// Update buffer boundary array if rows were deleted.
			if count := end.RowIndex - start.RowIndex; count > 0 {
				editor.bsArray.Delete(start.RowIndex+1, count)
			}
			// Optional: Update virtual lines or boundaries if needed.
			// editor.vlines.Release__(start.RowIndex, start.RowIndex+1)
			// editor.vlines.Delete__(start.RowIndex+1, end.RowIndex)
		}
	}
}

// ------------------------------------------------------------------
//
// ------------------------------------------------------------------

func (e *Editor) GetBuffers() *buffer.BufferSets {
	return BufferSets
}

// Should use OpenFile instead of?
func (e *Editor) SetFile(ff *file.File) {
	e.File = ff
	// e.vlines.SetFile__(ff)
	// e.vlines.Release()
}

// Convert rune to displaying string on mode line
// Conversion target:
//   - control code: ^X
//   - linefeed code
//
// Line feed code is depending the editing buffer linefeed
func (e *Editor) runeToDisplayStringForModeline(ch rune) string {
	str := ""
	if ch == define.DEL { // DEL
		str = `^?`
	} else if ch == '\t' {
		str = `\t`
	} else if ch == define.LF {
		linefeed := e.GetLinefeed()
		if linefeed == "LF" {
			str = `\n`
		} else if linefeed == "CRLF" {
			str = `\r\n`
		} else {
			str = `\r`
		}
	} else if ch < 32 {
		str = fmt.Sprintf("^%c", ch+64)
	} else {
		str = string(ch)
	}
	return str
}

func (e *Editor) drawModeline() {
	// cursor position
	readonly := "-"
	if e.IsReadonly() {
		readonly = "R"
	}
	modified := "-"
	if e.IsDirtyFlag() {
		modified = "*"
	}

	s := fmt.Sprintf("-%s%s- %s (%d,%d) ", readonly, modified, e.GetDispPath(), e.RowIndex+1, e.ModelineCx)
	// s += fmt.Sprintf("%s %s %s", e.GetEncoding(), e.GetLinefeed(), e.GetClass())
	s += fmt.Sprintf(`%s %s "%s"`, e.GetEncoding(), e.GetLinefeed(), (*e.LangMode).Name())

	// char code
	ch, _, _ := e.Rows().DecodeRune(e.RowIndex, e.ColIndex)
	str := e.runeToDisplayStringForModeline(ch)
	s += fmt.Sprintf(" ('%s', %d, 0x%02X)", str, ch, ch)

	a := theme.ColorModelineInactive
	if e.active {
		a = theme.ColorModeLineActive
	}
	e.screen.DrawString(e.editArea.X, e.editArea.Y+e.editArea.Height, e.editArea.Width, s, a)
}

// Use Editor.editArea as relative coordinates
func (e *Editor) showCursor(x, y int) {
	if e.active {
		e.screen.ShowCursor(e.editArea.X+x, e.editArea.Y+y)
	}
}

// Editor.editArea as relative coordinates
func (e *Editor) setCell(x, y int, style tcell.Style, ch rune, chWidth int) {
	if y < 0 || x < 0 {
		return
	}
	e.screen.SetContent(e.editArea.X+x, e.editArea.Y+y, ch, nil, style)
	for i := 1; i < chWidth; i++ {
		e.screen.SetContent(e.editArea.X+x+i, e.editArea.Y+y, 0, nil, style)
	}
}

// Editor.editArea as relative coordinates
func (e *Editor) fill(rect utils.Rect, cell screen.Cell) {
	rect.X += e.editArea.X
	rect.Y += e.editArea.Y
	e.screen.Fill(rect, cell)
}

// Returns bool whether it is the rightmost view
func (e *Editor) rightmost() bool {
	return e.viewArea.X+e.viewArea.Width >= e.Width
}

func (e *Editor) drawRightBar() {
	if e.rightmost() {
		return
	}

	x := e.viewArea.X + e.viewArea.Width - 1
	for y := e.viewArea.Y; y < e.viewArea.Y+e.viewArea.Height; y++ {
		// e.screen.Set(x, y, screen.Cell{Ch: '|', Style: theme.ColorRightbar})
		// e.screen.Set(x, y, screen.Cell{Ch: ' ', Style: theme.ColorRightbar})
		e.screen.SetContent(x, y, ' ', nil, theme.ColorRightbar)
	}
}

// Copy region to kill buffer
func (e *Editor) copyRegion(a, b file.Cursor) error {
	s := e.GetRegion(a, b)
	if s == nil {
		return nil
	}
	err := kill_buffer.KillBuffer.PushKillBuffer([]byte(string(*s)))
	return err
}

// Draw the screen based on Editor.currentRowIndex, logical row position logicalCY, and cursor position Editor.Cy
func (e *Editor) drawView() {
	/*
		lines := e.Lines()
		for i := 0; i < lines.Length(); i++ {
			s, _ := lines.String(i)
			verb.PP("%d %s", i, s)
		}
	*/

	// Modify the range of rowIndex and colIndex
	/*
		if e.RowIndex >= e.LenRows() {
			e.RowIndex = e.LenRows() - 1
		}
		currentRow := e.Row(e.RowIndex) // 2
		if e.ColIndex >= currentRow.LenCh() {
			e.ColIndex = currentRow.LenCh() - 1
		}
	*/
	// e.vlines.AllocateVlines__(e.RowIndex)
	foundPositionIndex := -1

	width, height := e.editArea.Width, e.editArea.Height
	logicalCX, logicalCY := 0, 0

	// Whether everything can fit into TreeLeaf
	e.StartDrawRowIndex = 0
	isAll := false
	totalRowAboveCursor := -1 // Total of row above the cursor
	var lcx, lcy int
	if e.RowIndex <= height || e.Rows().RowLength() <= height {
		isAll = true
		sumLines := 0
		for i := 0; i < e.Rows().RowLength(); i++ {
			// verb.PP("loop 1")
			e.drawLine(0, i, -1, false, &foundPositionIndex) // compute boundary
			// verb.PP("bo1 %#v", e.boundariesArray[i])
			if i == e.RowIndex {
				lcx, lcy = e.cursorPositionOnScreenLogicalRow(i, e.ColIndex)
				totalRowAboveCursor = sumLines
				// e.screen.Echo(fmt.Sprintf("lcx:%d,lcy:%d", lcx, lcy))
			}
			sumLines += e.bsArray.BoundariesLen(i) //.Boundaries(i).Len()
			if sumLines > height {
				isAll = false
				break
			}
		}
	}

	if isAll {
		e.Cy = lcy + totalRowAboveCursor
		e.Cx = lcx
		// e.StartDrawLogicalIndex = e.Cy
		// e.StartDrawLogicalIndex = 0
	} else {
		// cursor is below verticalThreshold
		if e.Cy >= height-e.verticalThreshold {
			e.Cy = height - e.verticalThreshold - 1
		}

		// cursor position
		e.drawLine(0, e.RowIndex, 0, false, &foundPositionIndex) // compute boundary
		logicalCX, logicalCY = e.cursorPositionOnScreenLogicalRow(e.RowIndex, e.ColIndex)

		// cursor is above verticalThreshold
		if e.Cy < e.verticalThreshold {
			e.Cy = e.verticalThreshold
			if totalRowAboveCursor >= 0 && e.Cy > totalRowAboveCursor {
				e.Cx, e.Cy = lcx, lcy
				e.Cy += totalRowAboveCursor
				e.StartDrawLogicalIndex = e.Cy
			}
		}

		// From the cursor position to up
		y := e.Cy - logicalCY
		for i := e.RowIndex - 1; i >= 0; i-- {
			e.drawLine(0, i, 0, false, &foundPositionIndex)
			// verb.PP("loop 2")
			// vl := e.vlines.GetVline__(i)
			// y -= vl.LenLogicalRow__()
			// y -= len(e.boundariesArray[i])
			// y -= e.bsay.Boundaries(i).Len()
			y -= e.bsArray.BoundariesLen(i) //.Boundaries(i).Len()
			if y <= 0 {
				e.StartDrawRowIndex = i
				e.StartDrawLogicalIndex = -y
				break
			}
		}
		if y > 0 {
			// Correct the cursor position so that there is no gap above the first line
			e.Cy -= y
		}
		e.Cx = logicalCX
	}

	// Draw screen
	y := -e.StartDrawLogicalIndex
	/* if isAll {
		y = 0
	} */
	// Set to 0 to find search results from the beginning
	// e.nextDrawFoundIndex =0
	foundPositionIndex = e.getFoundPosition(e.StartDrawRowIndex)
	// verb.PP("*foundPositionIndex %d", foundPositionIndex)
	var i int
	for i = e.StartDrawRowIndex; i < e.Rows().RowLength(); i++ {
		//verb.PP("loop 3")
		if y >= height || y > 10000 {
			break
		}
		e.drawLine(y, i, logicalCY, true, &foundPositionIndex)
		y += e.bsArray.BoundariesLen(i) // .Boundaries(i).Len()
	}
	e.EndDrawRowIndex = i
	e.showCursor(e.Cx, e.Cy)

	e.PrevDrawnY = y

	// clear remaining area
	e.fill(utils.Rect{X: 0, Y: y, Width: width, Height: height - y}, screen.Cell{Style: theme.ColorDefault})

	// Calculate the number of cursor digits to display on the mode line
	e.ModelineCx = logicalCX + 1
	for i := 0; i < logicalCY; i++ {
		e.ModelineCx += e.bsArray.Boundary(e.RowIndex, i).Width
	}
	e.drawModeline()
	// e.screen.Echo(fmt.Sprintf("line: %d:%d-%d", e.StartDrawRowIndex, e.StartDrawLogicalIndex, e.EndDrawRowIndex))
}

func is(class screen.CharClass, flag screen.CharClass) bool {
	return class&flag != 0
}

func isBreakpoint(p2, p1, c cell) bool {

	if p2.isEmpty() || p1.isEmpty() || c.isEmpty() {
		return false
	}

	return ((is(p2.class, screen.PROHIBITED) && is(p1.class, screen.PROHIBITED) && is(c.class, screen.PROHIBITED)) ||
		(is(p1.class, screen.PROHIBITED) && !is(c.class, screen.PROHIBITED) && !((is(p2.class, screen.NUMBER) && !is(p2.class, screen.WIDECHAR)) && is(p1.class, screen.DECIMAL_SEPARATOR) && (is(c.class, screen.NUMBER) && !is(c.class, screen.WIDECHAR))))) ||
		(!is(p1.class, screen.PROHIBITED) && !is(c.class, screen.PROHIBITED)) && (!is(p1.class, c.class) || p1.width != c.width)
}

type Number interface {
	int | int32 | int64 | float32 | float64
}

// isCursorInRange2 checks if the cursor position (row, col) is within the range
// defined by the top-left (row1, col1) and bottom-right (row2, col2) corners.
// It returns:
//
//	-1 if the cursor is before the range,
//	 1 if the cursor is after the range,
//	 0 if the cursor is within the range.
func isCursorInRange[T Number](row, col, row1, col1, row2, col2 T) T {
	// Handle cases where the range is reversed (either vertically or horizontally)
	if row1 > row2 {
		row1, row2 = row2, row1
		col1, col2 = col2, col1
	} else if row1 == row2 && col1 > col2 {
		col1, col2 = col2, col1
	}

	// If the row is outside the range, return false
	if row < row1 {
		return -1
	}
	if row > row2 {
		return 1
	}

	// If the range is within a single row (row1 == row2)
	if row1 == row2 {
		if col < col1 {
			return -1
		}
		if col >= col2 {
			return 1
		}
		return 0
	}

	// If the cursor is on the starting row (row == row1), check the column range
	if row == row1 {
		if col < col1 {
			return -1
		}
		return 0
	}

	// If the cursor is on the ending row (row == row2), check the column range
	if row == row2 {
		if col >= col2 {
			return 1
		}
		return 0
	}

	// If the cursor is on a row between the starting and ending rows,
	// it is always within the range
	return 0
}

// compute boundary of rowIndex
// draw one row
//   - n: y position within the Leaf to draw the row
//   - cursorLogicalCY: Logical row number where the cursor is located,
//     If the row to draw is not the cursor row, set -1 and call
func (e *Editor) drawLine(n, rowIndex, cursorLogicalCY int, draw bool, foundPositionIndex *int) int {
	// verb.PP("drawLine %d", rowIndex)
	y, x := n, 0
	var p2, p1, c cell
	var breakpoint Boundary
	lines := e.Rows()
	isEndOfRow := rowIndex == lines.RowLength()-1
	lineLength, ok := lines.GetColLength(rowIndex)
	totalWidth := 0 // for compute tab stop
	if !ok {
		panic("GetColLength")
	}

	bo := []Boundary{}
	startIndex := 0

	for i := 0; i < lineLength; {
		isLastCh := i == lineLength-1
		var ch, ch2 rune
		ch2 = 0
		ch, c.size, ok = lines.DecodeRune(rowIndex, i)
		if !ok {
			panic(fmt.Sprintf("%d '%s'", rowIndex, string((*lines)[rowIndex])))
		}
		c.width = utils.RuneWidth(ch)
		c.class = screen.GetCharClass(ch)

		isUnderline := func() bool {
			return rowIndex == e.RowIndex && cursorLogicalCY == y-n
		}

		style := theme.ColorDefault

		// Special char width
		if ch == define.EOF && isLastCh && isEndOfRow {
			ch = theme.MarkEOF
			c.width = 1 // End of file
			style = theme.ColorMarkEOF
		} else if ch == '\t' {
			ch = theme.MarkTab
			c.width = utils.TabWidth(totalWidth, e.GetTabWidth())
			style = theme.ColorTab
			e.setSpecialCharWidths(rowIndex, i, c.width)
		} else if ch == define.LF {
			ch = theme.MarkLinefeed
			style = theme.ColorMarkLinefeed
		} else if is(c.class, screen.CONTROLCODE) {
			ch2 = ch + 64
			ch = '^'
			c.width = 2 // ^X
			style = theme.ColorControlCode
		}

		// Is index in the found word
		if *foundPositionIndex >= 0 && *foundPositionIndex < len(e.foundIndexes) {
			u := isCursorInRange(rowIndex, i,
				e.foundIndexes[*foundPositionIndex].start.RowIndex, e.foundIndexes[*foundPositionIndex].start.ColIndex,
				e.foundIndexes[*foundPositionIndex].stop.RowIndex, e.foundIndexes[*foundPositionIndex].stop.ColIndex)

			if u == 0 {
				if isCursorInRange(e.RowIndex, e.ColIndex,
					e.foundIndexes[*foundPositionIndex].start.RowIndex, e.foundIndexes[*foundPositionIndex].start.ColIndex,
					e.foundIndexes[*foundPositionIndex].stop.RowIndex, e.foundIndexes[*foundPositionIndex].stop.ColIndex) == 0 {
					style = theme.ColorSearchFoundOnCursor
				} else {
					style = theme.ColorFind
				}
			} else if u == 1 {
				*foundPositionIndex++
			}
		}
		style = style.Underline(isUnderline())

		if x+c.width >= e.editArea.Width-8 && isBreakpoint(p2, p1, c) {
			breakpoint = Boundary{StartIndex: startIndex, StopIndex: i /* + c.size */, Width: x /* + c.width */, TotalWidth: totalWidth /* + c.width */}
		}

		if x+c.width >= e.editArea.Width {
			if isLastCh {
				// ch is LF or EOF
				bo = append(bo, Boundary{StartIndex: startIndex, StopIndex: i + c.size, Width: x + c.width, TotalWidth: totalWidth + c.width})
				if draw {
					e.setCell(x, y, style, ch, c.width)
					e.fill(utils.Rect{X: x + c.width, Y: y, Width: e.editArea.Width - (x + c.width), Height: 1}, screen.Cell{Style: theme.ColorDefault.Underline(isUnderline())})
				}
			} else if breakpoint.IsEmpty() {
				if ch == '\t' && e.editArea.Width-x > 1 {
					bo = append(bo, Boundary{StartIndex: startIndex, StopIndex: i, Width: x, TotalWidth: totalWidth})
					startIndex = i
					if draw {
						tmpWidth := e.editArea.Width - x - 1
						e.setCell(x, y, style, ch, tmpWidth)
						e.setCell(x+tmpWidth, y, theme.ColorMarkContinue.Underline(isUnderline()), theme.MarkContinue, 1)
					}
					y++
					x = 0
				} else {
					bo = append(bo, Boundary{StartIndex: startIndex, StopIndex: i, Width: x, TotalWidth: totalWidth})
					startIndex = i
					if draw {
						e.setCell(x, y, theme.ColorMarkContinue.Underline(isUnderline()), theme.MarkContinue, 1)
						e.fill(utils.Rect{X: x + 1, Y: y, Width: e.editArea.Width - (x + 1), Height: 1}, screen.Cell{Style: theme.ColorDefault.Underline(isUnderline())})
					}
					y++
					x = 0
					if draw {
						style = style.Underline(isUnderline())
						if is(c.class, screen.CONTROLCODE) {
							e.setCell(x, y, style, ch, 1)
							e.setCell(x+1, y, style, ch2, 1)
						} else {
							e.setCell(x, y, style, ch, c.width)
						}
					}
				}
			} else {
				bo = append(bo, breakpoint)
				startIndex = breakpoint.StopIndex

				i = breakpoint.StopIndex
				x = breakpoint.Width
				totalWidth = breakpoint.TotalWidth
				if draw {
					e.setCell(x, y, theme.ColorMarkContinue.Underline(isUnderline()), theme.MarkContinue, 1)
					e.fill(utils.Rect{X: x + 1, Y: y, Width: e.editArea.Width - (x + 1), Height: 1}, screen.Cell{Style: theme.ColorDefault.Underline(isUnderline())})
				}
				y++
				x = 0
				//
				breakpoint.Clear()
				p2.clear()
				p1.clear()
				c.clear()
				continue // ! --------------------
			}
		} else {
			if draw {
				e.setCell(x, y, style, ch, c.width)
			}
			if isLastCh {
				bo = append(bo, Boundary{StartIndex: startIndex, StopIndex: i + c.size, Width: x + c.width, TotalWidth: totalWidth + c.width})
				if draw {
					e.fill(utils.Rect{X: x + c.width, Y: y, Width: e.editArea.Width - (x + c.width), Height: 1}, screen.Cell{Style: theme.ColorDefault.Underline(isUnderline())})
				}
				y++
			}
		}

		// -- tail of loop --
		p2 = p1
		p1 = c
		x += c.width
		totalWidth += c.width
		i += c.size
	}

	//
	e.bsArray.Set(rowIndex, bo)
	return y - n
}

// Returns the screen position of the cursor corresponding to the specified column index in logical rows.
func (e *Editor) cursorPositionOnScreenLogicalRow(rowIndex, colIndex int) (lx, ly int) {
	if rowIndex >= e.bsArray.Len() {
		// verb.PP("lx,ly %d,%d", -1, -1)
		return -1, -1
	}
	for ly = 0; ly < e.bsArray.BoundariesLen(rowIndex); ly++ {
		if colIndex >= e.bsArray.Boundary(rowIndex, ly).StartIndex && colIndex < e.bsArray.Boundary(rowIndex, ly).StopIndex {
			for i := e.bsArray.Boundary(rowIndex, ly).StartIndex; i < colIndex; {
				ch, size, ok := e.Rows().DecodeRune(rowIndex, i)
				if !ok {
					break
				}
				w, _ := e.runeWidth(ch, rowIndex, i)
				lx += w
				i += size
			}
			//verb.PP("cx,cy %d,%d", cx, cy)
			return lx, ly
		}
	}
	// verb.PP("lx,ly %d,%d", -1, -1)
	return -1, -1 // overflow
}

func (e *Editor) runeWidth(ch rune, rowIndex, colIndex int) (w int, ok bool) {
	if ch == '\t' {
		w, ok = e.specialCharWidths[rowIndex][colIndex]
		if !ok {
			e.screen.Echo(fmt.Sprintf("Not found tab width row:%d, col:%d", rowIndex, colIndex))
			return 0, false
		}
		// e.screen.Echo(fmt.Sprintf("tab width %d:%d %d", rowIndex, colIndex, w))
	} else {
		w = utils.RuneWidth(ch)
	}
	return w, true
}

func (e *Editor) isEndOfLogicalRow(rowIndex, colIndex int) bool {
	_, size := utf8.DecodeRune((*e.Rows())[rowIndex][colIndex:])
	for i := 0; i < e.bsArray.BoundariesLen(rowIndex); i++ {
		if colIndex+size == e.bsArray.Boundary(rowIndex, i).StopIndex {
			return true
		}
	}
	return false
}

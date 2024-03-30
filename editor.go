// Editor Struct implements the gecore.tree.Leaf interface

package te

import (
	"fmt"

	"github.com/gdamore/tcell/v2"

	"github.com/ge-editor/gecore"
	"github.com/ge-editor/gecore/define"
	"github.com/ge-editor/gecore/kill_buffer"
	"github.com/ge-editor/gecore/screen"
	"github.com/ge-editor/gecore/tree"

	"github.com/ge-editor/utils"

	"github.com/ge-editor/theme"

	"github.com/ge-editor/te/buffer"
	"github.com/ge-editor/te/file"
	"github.com/ge-editor/te/mark"
	"github.com/ge-editor/te/vline"
)

const (
	verticalThreshold = 5
)

var (
	BufferSets, _ = buffer.NewBufferSets(gecore.Files)
	Marks         = mark.NewMarks()
)

func newEditor() *Editor {
	return &Editor{
		File:   (*BufferSets)[0].File,
		Meta:   (*BufferSets)[0].PopMeta(),
		vlines: vline.NewVlines((*BufferSets)[0].File), //  make([]*vline.Vline, 0, 1),
	}
}

// Position found in search results
type foundPosition struct {
	row   int
	start int
	end   int
}

// This function is used to color the characters in the search results when drawing the screen.
//
// Find search results whose row value is greater than or equal to rowIndex from the text search results array e.searchIndexes.
// Search in e.searchIndexes from the position where index is e.drawSearchIndex.
// If found, update e.drawSearchIndex to the next search start index.
// When searching from the first index, e.drawSearchIndex must be set to 0.
// Return -1 if not found
func (e *Editor) getSearchResultIndex(rowIndex int) int {
	if e.drawSearchIndex == -1 {
		return -1
	}
	for i := e.drawSearchIndex; i < len(e.searchIndexes); i++ {
		if e.searchIndexes[i].row == rowIndex {
			// Update to next search start index
			e.drawSearchIndex = i + 1
			return i
		} else if e.searchIndexes[i].row > rowIndex {
			// Update to next search start index
			e.drawSearchIndex = i
			return i
		}
	}
	e.drawSearchIndex = -1
	return -1
}

// Editor Struct implements the gecore.tree.Leaf interface
type Editor struct {
	parentView tree.View
	screen     *screen.Screen
	active     bool

	utils.Rect            // over all // position on screen
	viewArea   utils.Rect // include mode line
	editArea   utils.Rect

	verticalThreshold int // Changes depending on screen size

	*file.File
	*buffer.Meta
	vlines *vline.Vlines // []*vline.Vline

	currentSearchIndex int
	searchIndexes      []foundPosition
	drawSearchIndex    int
}

type syncType int

const (
	insert syncType = iota // The first line is e.vlines.Release, the rest are e.vlines.Insert
	delete                 // The first line is e.vlines.Release, the rest are e.vlines.Delete
	modify                 // All is e.vlines.Release
)

// Synchronize edits to other buffers
//
// Synchronize cursor position and vline:
//   - BufferSets.metas[*].Cursor
//   - Editor.Cursor
//   - Editor.vlines
func (e *Editor) SyncEdits(sync syncType, ff *file.File, start, end file.Cursor) {
	// Swap
	if start.RowIndex > end.RowIndex || (start.RowIndex == end.RowIndex && start.ColIndex > end.ColIndex) {
		tmp := start
		start = end
		end = tmp
	}

	// verb.PP("SyncEdited start: %v, end: %v", start, end)
	// Synchronize cursor position
	for _, buffSet := range *BufferSets {
		if buffSet.File != ff {
			continue
		}
		for _, meta := range buffSet.GetMetas() {
			switch sync {
			case insert:
				meta.Cursor = syncCursorInsert(meta.Cursor, start, end)
			case delete:
				meta.Cursor = syncCursorDelete(meta.Cursor, start, end)
			}
		}
		break
	}

	// Synchronize other buffers
	leaves := tree.GetLeavesByViewName("te")
	// verb.PP("SyncEdited leaves: %v", leaves)
	for _, leaf := range leaves {
		editor := (*leaf).(*Editor)
		if editor.File != ff /* || editor == e */ {
			// Not a terget File // or Active editor
			continue
		}

		switch sync {
		case insert:
			editor.Cursor = syncCursorInsert(editor.Cursor, start, end)
			e.vlines.Release(start.RowIndex, start.RowIndex+1) // first line
			e.vlines.Insert(start.RowIndex+1, end.RowIndex)
		case delete:
			editor.Cursor = syncCursorDelete(editor.Cursor, start, end)
			e.vlines.Release(start.RowIndex, start.RowIndex+1) // first line
			e.vlines.Delete(start.RowIndex+1, end.RowIndex)
		case modify:
			e.vlines.Release(start.RowIndex, end.RowIndex)
		}
		// verb.PP("SyncEdited c: %v, start: %v, end: %v", editor.Cursor, start, end)
		editor.vlines.Release(start.RowIndex, -1)
	}
}

func syncCursorDelete(current, start, end file.Cursor) file.Cursor {
	// No need to update since the change is after the cursor row
	if current.RowIndex < start.RowIndex {
		return current
	}

	// Same row as cursor
	if current.RowIndex == start.RowIndex {
		// Since the change is after the cursor, leave it as is
		if current.ColIndex < start.ColIndex {
			return current
		}
		// Change only within the cursor line
		current.ColIndex = start.ColIndex
		return current
	}

	if current.RowIndex > end.RowIndex {
		current.RowIndex -= end.RowIndex - start.RowIndex
		return current
	}

	// if current.RowIndex <= end.RowIndex {
	current.RowIndex = start.RowIndex
	current.ColIndex = start.ColIndex - 1
	return current
	// }
}

func syncCursorInsert(current, start, end file.Cursor) file.Cursor {
	// No need to update since the change is after the cursor row
	if current.RowIndex < start.RowIndex {
		return current
	}

	rowLen := end.RowIndex - start.RowIndex

	// Same row as cursor
	if current.RowIndex == start.RowIndex {
		// No need to change as it is added after the cursor
		if current.ColIndex < start.ColIndex {
			return current
		}
		// Change only within the cursor line
		if rowLen == 0 {
			current.ColIndex += end.ColIndex - start.ColIndex
			return current
		}
		current.ColIndex = current.ColIndex - start.ColIndex + end.ColIndex
		current.RowIndex += rowLen
		return current
	}

	// if current.RowIndex > start.RowIndex {
	current.RowIndex += rowLen
	return current
	// }
}

func (e *Editor) GetBuffers() *buffer.BufferSets {
	return BufferSets
}

// Should use OpenFile instead of?
func (e *Editor) SetFile(ff *file.File) {
	e.File = ff
	e.vlines.SetFile(ff)
	// e.vlines.Release()
}

func (e *Editor) View() *tree.View {
	return &e.parentView
}

func (e *Editor) WillClose() {
}

func (e *Editor) Resume() {
}

func (e *Editor) Init() {
}

func (e *Editor) ViewActive(a bool) {
	e.active = a
}

func (e *Editor) Resize(width, height int, rect utils.Rect) {
	e.Width, e.Height = width, height

	e.viewArea = rect
	e.editArea = rect
	if !e.rightmost() {
		e.editArea.Width -= 1 // right bar
	}
	e.editArea.Height -= 1 // status
	e.drawRightBar()

	e.verticalThreshold = utils.Threshold(verticalThreshold, e.editArea.Height)

	e.vlines.Resize(e.editArea.Width, e.editArea.Height, e.GetTabWidth())
}

// Convert rune to displaying string on mode line
// Conversion target:
//   - control code: ^X
//   - linefeed code
//
// Line feed code is depending the editing buffer linefeed
func (e *Editor) runeToDisplayString(ch rune) string {
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
	s += fmt.Sprintf("%s %s %s", e.GetEncoding(), e.GetLinefeed(), e.GetClass())

	// char code
	row := e.Row(e.RowIndex)
	ch := row.Ch(e.ColIndex)
	str := e.runeToDisplayString(ch)
	s += fmt.Sprintf(" ('%s', %d, 0x%02X)", str, ch, ch)

	a := theme.ColorModelineInactive
	// if e.stat == tree.Active {
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

// This Kill function will be called twice in a row
// First call to active tree.Leaf
// The second time is a circular call to tree
//
// Process active tree.Leaf in first call
// Remove from buffer BufferSets matching leafEditor.File
//
// Handle non-active windows in second call
// Check if Editor.File exists in BufferSets
// If it doesn't exist, replace it with another buffer
func (e *Editor) Kill(leaf *tree.Leaf, isActive bool) *tree.Leaf {
	// Second call
	if !isActive {
		bufferSetsIndex := BufferSets.GetIndexByBufferFile(e.File)
		if bufferSetsIndex >= 0 {
			var tv tree.Leaf = e
			return &tv
		}
	}

	leafEditor := (*leaf).(*Editor)
	bufferSetsIndex := BufferSets.RemoveByBufferFile(leafEditor.File) // 該当するバッファを取り除く
	l := len(*BufferSets)
	if l == 0 {
		// If the number of buffers is 0, create a new buffer
		ff, meta, err := BufferSets.GetFileAndMeta("unnamed")
		e.File = ff
		e.Meta = meta
		e.vlines.SetFile(ff)
		// e.vlines.Release()
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
		e.vlines.SetFile(bs.File)
		// e.vlines.Release()
	}

	var tv tree.Leaf = e
	return &tv
}

func (e *Editor) Event(tev *tcell.Event) *tcell.Event {
	switch ev := (*tev).(type) {
	case *tcell.EventInterrupt:
		// verb.PP("EventInterrupt")
		// return tev
	case *tcell.EventResize:
		e.Width, e.Height = ev.Size()
		e.screen.Clear()
		e.Draw()
		// return tev
	case *tcell.EventKey:
	}
	return tev
}

// Copy region to kill buffer
func (e *Editor) copyRegion(a, b file.Cursor) error {
	s := e.GetRegion(a, b)
	if s == nil {
		return nil
	}
	err := kill_buffer.KillBuffer.PushKillBuffer(*s)
	return err
}

func (e *Editor) Draw() {
	e.drawView()
	e.drawRightBar()
}

func (e *Editor) Redraw() {
	e.centerViewOnCursor()
	e.Draw()
}

// Draw the screen based on Editor.currentRowIndex, logical row position logicalCY, and cursor position Editor.Cy
func (e *Editor) drawView() {
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
	e.vlines.AllocateVlines(e.RowIndex)

	width, height := e.editArea.Width, e.editArea.Height
	logicalCX, logicalCY := 0, 0

	// Whether everything can fit into TreeLeaf
	e.StartDrawRowIndex = 0
	isAll := false
	if e.LenRows() <= height {
		isAll = true
		sumLines := 0
		for i := 0; i < e.LenRows(); i++ {
			vl := e.vlines.GetVline(i)
			if i == e.RowIndex {
				logicalCX, logicalCY = vl.CursorPositionOnScreenLogicalLine(e.ColIndex)
				e.Cx, e.Cy = logicalCX, logicalCY
				e.Cy += sumLines
				e.StartDrawLogicalIndex = e.Cy
			}
			sumLines += vl.LenLogicalRow()
			if sumLines > height {
				isAll = false
				break
			}
		}
	}

	if !isAll {
		// cursor is below verticalThreshold
		if e.Cy >= height-e.verticalThreshold {
			e.Cy = height - e.verticalThreshold - 1
		}
		// cursor is above verticalThreshold
		if e.Cy < e.verticalThreshold {
			e.Cy = e.verticalThreshold
		}

		// cursor position
		vl := e.vlines.GetVline(e.RowIndex)
		logicalCX, logicalCY = vl.CursorPositionOnScreenLogicalLine(e.ColIndex)

		// From the cursor position to up
		y := e.Cy - logicalCY
		for i := e.RowIndex - 1; i >= 0; i-- {
			vl := e.vlines.GetVline(i)
			y -= vl.LenLogicalRow()
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
	} // if !isAll

	// Draw screen
	y := -e.StartDrawLogicalIndex
	if isAll {
		y = 0
	}
	// Set to 0 to find search results from the beginning
	e.drawSearchIndex = 0
	for i := e.StartDrawRowIndex; i < e.LenRows(); i++ {
		if y >= height {
			break
		}
		vl := e.vlines.GetVline(i)
		e.DrawLine(y, i, logicalCY)
		y += vl.LenLogicalRow()
		// e.EndDrawRowIndex = i
	}
	e.showCursor(e.Cx, e.Cy)

	e.PrevDrawnY = y

	// clear remaining area
	e.fill(utils.Rect{X: 0, Y: y, Width: width, Height: height - y}, screen.Cell{Style: theme.ColorDefault})

	// Calculate the number of cursor digits to display on the mode line
	vl := e.vlines.GetVline(e.RowIndex)
	e.ModelineCx = logicalCX + 1
	bs := vl.Boundaries()
	for i := 0; i < logicalCY; i++ {
		e.ModelineCx += (*bs)[i].Widths()
	}

	// e.screen.Echo(fmt.Sprintf("line: %d:%d-%d", e.StartDrawRowIndex, e.StartDrawLogicalIndex, e.EndDrawRowIndex))

	e.drawModeline()
}

// draw one row
//   - n: y position within the Leaf to draw the row
//   - cursorLogicalCY: Logical row number where the cursor is located,
//     If the row to draw is not the cursor row, set -1 and call
func (e *Editor) DrawLine(n, rowIndex, cursorLogicalCY int) {
	logicalLineIndex := 0
	x := 0
	vl := e.vlines.GetVline(rowIndex)
	bs := vl.Boundaries()
	startIndex := 0
	if n < 0 {
		logicalLineIndex = -n
		if logicalLineIndex > 0 {
			if logicalLineIndex >= len(*bs) {
				return
			}
			startIndex = (*bs)[logicalLineIndex-1].Index() + 1
		}
	}
	row := e.Row(rowIndex)

	// Set search found style
	drawSearchIndex := e.getSearchResultIndex(rowIndex)
	for i := startIndex; i < row.LenCh(); i++ {
		var ch2 rune = 0
		ch := row.Ch(i)
		// verb.PP("rowIndex %d, startIndex %d, i %d, len %d, ch %c, %#v", rowIndex, startIndex, i, row.LenCh(), ch, *row)
		chWidth := vl.GetCell(i).GetCellWidth()
		// verb.PP("rowIndex %d, startIndex %d, i %d, len %d, ch %c, %#v", rowIndex, startIndex, i, row.LenCh(), ch, *row)
		style := theme.ColorDefault
		underline := false

		switch ch {
		case define.DEL:
			ch2 = '?'
			ch = '^'
			style = theme.ColorControlCode
		case '\n':
			ch = theme.MarkLinefeed
			style = theme.ColorMarkLinefeed
		case '\t':
			ch = theme.MarkTab
			style = theme.ColorTab
		case define.EOF:
			if e.IsLastRow(rowIndex) && i == row.LenCh()-1 {
				ch = theme.MarkEOF
				style = theme.ColorMarkEOF
				chWidth = theme.MarkEOF_WIDTH
			} else {
				ch2 = 'Z'
				ch = '^'
				style = theme.ColorControlCode
			}
		default:
			if ch < 32 {
				ch2 = ch + 64
				ch = '^'
				style = theme.ColorControlCode
			} else if ch == '　' || ch == ' ' {
				style = theme.ColorSpace
			}
		}

		// Set search found style
		if drawSearchIndex >= 0 {
			foundPosition := e.searchIndexes[drawSearchIndex]
			if foundPosition.row == rowIndex && foundPosition.start <= i && foundPosition.end > i {
				if foundPosition.row == e.RowIndex && foundPosition.start <= e.ColIndex && foundPosition.end > e.ColIndex {
					style = theme.ColorSearchFoundOnCursor
				} else {
					style = theme.ColorSearchFound
				}
				if foundPosition.end-1 == i {
					drawSearchIndex = e.getSearchResultIndex(rowIndex)
				}
			}
		}

		// verb.PP("logicalLineIndex %d == cursorLogicalCY %d", logicalLineIndex, cursorLogicalCY)
		if rowIndex == e.RowIndex && logicalLineIndex == cursorLogicalCY {
			underline = true
		}
		e.setCell(x, logicalLineIndex+n, style.Underline(underline), ch, chWidth)
		if ch2 > 0 {
			e.setCell(x+1, logicalLineIndex+n, style.Underline(underline), ch2, 1)
		}

		// verb.PP("bs %#v, %d", bs, logicalLineIndex)
		if len(*bs)-1 < logicalLineIndex {
			logicalLineIndex = 0
		}
		if i == (*bs)[logicalLineIndex].Index() {
			e.fill(utils.Rect{X: x + chWidth, Y: logicalLineIndex + n, Width: e.editArea.Width - (x + chWidth), Height: 1}, screen.Cell{Style: theme.ColorDefault.Underline(underline)})
			if logicalLineIndex < len(*bs)-1 {
				e.setCell(x+chWidth, logicalLineIndex+n, theme.ColorMarkContinue.Underline(underline), theme.MarkContinue, 1)
			}

			// Next logical line
			logicalLineIndex++
			if n+logicalLineIndex >= e.editArea.Height {
				break
			}
			x = 0
		} else {
			x += chWidth
		}
	}
}

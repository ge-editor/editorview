package file

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ge-editor/gecore/define"
	"github.com/ge-editor/gecore/verb"

	"github.com/ge-editor/utils"
)

type flags int8
type linefeed int8

const (
	readonly flags = 1 << iota
	softTab

	lf linefeed = 1 << iota
	crlf
	cr
)

type File struct {
	rawPath  string
	path     string
	base     string
	dispPath string

	size    int64
	mode    os.FileMode
	modTime time.Time

	*rows
	encoding string
	linefeed
	tabWidth int
	flags    // readonly, softTab

	UndoAction *ActionGroup
	RedoAction *ActionGroup

	class string // file type string
}

// Call New() or Load() after invoking this function
func NewFile(rawPath string) *File {
	ff := &File{
		rawPath:  rawPath,
		path:     "",
		base:     "",
		dispPath: "",

		size:    0,
		mode:    fs.ModePerm,
		modTime: time.Now(),

		rows:     nil,
		encoding: "UTF-8",
		linefeed: lf,
		tabWidth: 4,
		flags:    0,

		UndoAction: &ActionGroup{},
		RedoAction: &ActionGroup{},

		class: filepath.Ext(rawPath),
	}
	ff.init()
	return ff
}

// Initialize File with File.rawPath
func (ff *File) init() {
	if ff.rawPath == "" {
		ff.rawPath = "unnamed"
	}

	ff.size = 0
	ff.mode = fs.ModePerm
	ff.modTime = time.Now()
	info, err := os.Stat(ff.rawPath)
	if err == nil {
		if info.IsDir() {
			ff.rawPath = "unnamed"
		}
		ff.size = info.Size()
		ff.mode = info.Mode()
		ff.modTime = info.ModTime()
	}
	ff.path, err = filepath.Abs(ff.rawPath)
	if err != nil {
		ff.path = ""
	}

	dir := ""
	dir, ff.base = filepath.Split(ff.path)
	ff.dispPath = ff.base
	dir = utils.LastPartOfPath(dir)
	wd, err := os.Getwd()
	if err == nil {
		if utils.SameFile(wd, dir) {
			ff.dispPath = filepath.Join(dir, ff.dispPath)
		}
	}

	ff.class = filepath.Ext(ff.path)
}

func (ff *File) ChangePath(path string) {
	ff.rawPath = path
	ff.init()
}

// No undo/redo functionality
func (ff *File) RemoveRegion(cursor1, cursor2 Cursor) *[]rune {
	return ff.removeRegion(cursor1, cursor2, true)
}

// No undo/redo functionality
func (ff *File) GetRegion(cursor1, cursor2 Cursor) *[]rune {
	return ff.removeRegion(cursor1, cursor2, false)
}

// No undo/redo functionality
func (ff *File) RemoveRow(cursor1 Cursor) *[]rune {
	row1, _ := cursor1.RowIndex, cursor1.ColIndex

	rw1 := (*ff.rows)[row1]
	if (*rw1)[rw1.LenCh()-1] == '\n' {
		removed := make([]rune, rw1.LenCh())
		copy(removed, *rw1)
		*ff.rows = slices.Delete((*ff.rows), row1, row1+1)
		return &removed
	} else { // EOF
		removed := make([]rune, rw1.LenCh()-1)
		copy(removed, (*rw1)[:rw1.LenCh()-1])
		*rw1 = (*rw1)[rw1.LenCh()-1:] // EOF only
		return &removed
	}
}

// No undo/redo functionality
func (ff *File) removeRegion(cursor1, cursor2 Cursor, remove bool) *[]rune {
	row1, col1 := cursor1.RowIndex, cursor1.ColIndex
	row2, col2 := cursor2.RowIndex, cursor2.ColIndex

	// Checked row index. The start position of the region is after the end position, or the end position is beyond the last line
	if row1 > row2 || row2 > ff.LenRows()-1 {
		return nil
	}

	rw1 := (*ff.rows)[row1]
	// Checked col index. The start position of the region is the right of linefeed or EOF
	if col1 > rw1.LenCh()-1 {
		return nil
	}

	rw2 := (*ff.rows)[row2]
	// Checked col index. The end position of the region is the right of linefeed or EOF
	if col2 > rw2.LenCh()-1 {
		return nil
	}

	if row1 == row2 {
		if col1 == col2 {
			// The start and end positions of the region are the same.
			return nil
		}
		removed := make([]rune, col2-col1)
		copy(removed, (*rw1)[col1:col2])
		if remove {
			*rw1 = slices.Delete(*rw1, col1, col2)
		}
		return &removed
	}

	// Compute cap
	removed := make([]rune, 0, func() int {
		total := len((*rw1)[col1:]) // first row
		for i := row1 + 1; i < row2; i++ {
			total += (*ff.rows)[i].LenCh() // middle row
		}
		total += len((*rw2)[:col2]) // last row
		return total
	}())
	// first row
	removed = append(removed, (*rw1)[col1:]...)
	if remove {
		*rw1 = (*rw1)[:col1]
	}
	// middle rows
	for i := row1 + 1; i < row2; i++ {
		removed = append(removed, *(*ff.rows)[i]...)
	}
	// last row
	removed = append(removed, (*rw2)[:col2]...)
	// Remove middle and last rows
	if remove {
		*rw1 = append(*rw1, (*rw2)[col2:]...)
		// if (row2+1)-(row1+1) > 0 {
		if row2-row1 > 0 {
			*ff.rows = slices.Delete((*ff.rows), row1+1, row2+1)
		}
	}
	return &removed
}

// Split s []rune by sep rune
// sep is not deleted
func Split(s []rune, sep rune) (r [][]rune) {
	for {
		if len(s) == 0 {
			break
		}

		i := slices.Index(s, sep)
		if i == -1 {
			r = append(r, s)
			break
		}

		i += 1 // including separator
		r = append(r, s[:i])
		s = s[i:]
	}
	return
}

// Return row,col,
// No undo/redo functionality
func (ff *File) Insert(cursor Cursor, s []rune) (Cursor, bool) {
	rowsLen := ff.LenRows()
	if cursor.RowIndex > rowsLen-1 {
		verb.PP("row index is beyond the last line")
		return cursor, false
	}

	lines := Split(s, '\n')
	for _, line := range lines {
		if line[len(line)-1] == '\n' {
			//verb.PP("1")
			rw := (*ff.rows).Row(cursor.RowIndex)
			carryOverLen := rw.LenCh() - cursor.ColIndex
			if !ff.insertRunes(cursor, line) {
				return cursor, false
			}
			if carryOverLen > 0 {
				pushLine := make(Row, carryOverLen)
				copy(pushLine, (*rw)[cursor.ColIndex+len(line):])
				ff.insertRows(cursor.RowIndex+1, rows{&pushLine})
			}
			*rw = (*rw)[:cursor.ColIndex+len(line)]

			cursor.RowIndex++
			cursor.ColIndex = 0
		} else {
			if !ff.insertRunes(cursor, line) {
				return cursor, false
			}
			cursor.ColIndex += len(line)
		}

	}
	return cursor, true
}

func (ff *File) insertRows(rowIndex int, rw rows) {
	/*
		if rowIndex > m.LenRows()-1 {
			// row index is beyond the last line
			return false
		}
	*/

	*ff.rows = slices.Insert(*ff.rows, rowIndex, rw...)
}

func (ff *File) insertRunes(cursor Cursor, s []rune) bool {
	rowsLen := ff.LenRows()
	if cursor.RowIndex > rowsLen-1 {
		// row index is beyond the last line
		return false
	}

	rw := (*ff.rows)[cursor.RowIndex]
	rowLen := rw.LenCh()
	if cursor.ColIndex > rowLen-1 {
		// col index is the right of linefeed or EOF
		return false
	}

	*rw = slices.Insert(*rw, cursor.ColIndex, s...)
	return true
}

// New file
func (ff *File) New() error {
	ff.rows = NewRows()
	row := NewRow()
	// Add EOF
	row.append(define.EOF)
	ff.rows.append(row)
	// Set linefeed type
	ff.linefeed = lf

	// m.rows.Dump()
	return nil
}

// Load file
func (ff *File) Load() error {
	/* encoding, err := (*encorder).GuessCharset(ff.path, 128)
	if err != nil {
		return err
	}
	ff.encoding = encoding */

	fp, err := os.Open(ff.path)
	if err != nil {
		return err
	}
	defer fp.Close()
	scanLines := newScanLines(ff.encoding)
	scanner := bufio.NewScanner(fp)
	scanner.Split(scanLines.scanLines)
	ff.rows = NewRows()
	for scanner.Scan() {
		row := NewRow()
		line := scanner.Bytes() // Not reallocate
		row.bytes(line)
		ff.rows.append(row)
	}
	err = scanner.Err()
	if err != nil && err != io.EOF {
		return err
	}

	// Add EOF
	row := ff.Row(ff.LenRows() - 1)
	if row.LenCh() > 0 && row.Ch(row.LenCh()-1) == '\n' {
		row := NewRow()
		row.append(define.EOF)
		ff.rows.append(row)
	} else {
		row.append(define.EOF)
	}

	// Set linefeed type
	ff.linefeed = []linefeed{lf, crlf, cr}[utils.MaxValueIndex([]int{scanLines.countLF, scanLines.countCRLF, scanLines.countCR})]

	// m.rows.Dump()
	return nil
}

func (ff *File) Save() error {
	sb := strings.Builder{}

	var linefeed []rune
	if ff.linefeed&lf > 0 {
		linefeed = []rune{'\n'}
	} else if ff.linefeed&crlf > 0 {
		linefeed = []rune{'\r', '\n'}
	} else { // cr
		linefeed = []rune{'\r'}
	}
	restoreLineFeed := func() {
		for _, ch := range linefeed {
			sb.WriteRune(ch)
		}
	}

	lastIndex := ff.LenRows() - 1
	for i, row := range *ff.rows {
		if row == nil {
			return fmt.Errorf("row is nothing")
		}
		lineBufferLen := row.LenCh()
		for j := 0; j < lineBufferLen; j++ {
			ch := row.Ch(j)
			if j == lineBufferLen-1 { // end of line
				if ch == define.LF {
					restoreLineFeed()
					continue
				} else if ch == define.EOF && i == lastIndex {
					// Skip EOF mark
					break
				}
			}
			sb.WriteRune(ch)
		}
	}

	err := os.WriteFile(ff.path, []byte(sb.String()), 0644)
	/*
		if err == nil {
			ff.flags ^= dirty
		}
	*/
	return err
}

// would like to consider other formats such as dates.
func (ff *File) Backup() error {
	for i := 1; i < 1_000_000; i++ {
		backup := fmt.Sprintf("%s.~%d~", ff.path, i)
		if !utils.ExistsFile(backup) {
			return utils.CopyFile(ff.path, backup)
		}
	}
	return fmt.Errorf("too many backups")
}

// Setter/Getter

func (ff *File) SetPath(path string) {
	ff.path = path
	ff.base = filepath.Base(path)
	ff.class = filepath.Ext(path)
	ff.dispPath = ff.base
}

func (ff *File) GetPath() string {
	return ff.path
}

func (ff *File) GetBase() string {
	return ff.base
}

func (ff *File) GetDispPath() string {
	return ff.dispPath
}

func (ff *File) GetClass() string {
	return ff.class
}

func (ff *File) Rows() *rows {
	return ff.rows
}

func (ff *File) Row(rowIndex int) *Row {
	return (*ff.rows)[rowIndex]
}

func (ff *File) GetEncoding() string {
	return ff.encoding
}

func (ff *File) GetLinefeed() string {
	if ff.linefeed&lf > 0 {
		return "LF"
	}
	if ff.linefeed&crlf > 0 {
		return "CRLF"
	}
	return "CR"
}

func (ff *File) GetTabWidth() int {
	return ff.tabWidth
}

// Flags

func (ff *File) SetReadonly(b bool) {
	if b {
		ff.flags |= readonly
	} else {
		ff.flags &= ^readonly
	}
}

func (ff *File) IsReadonly() bool {
	return ff.flags&readonly > 0
}

/*
	 func (ff *File) SetDirtyFlag(b bool) {
		if b {
			ff.flags |= dirty
		} else {
			ff.flags &= ^dirty
		}
	}
*/

func (ff *File) IsDirtyFlag() bool {
	return !ff.UndoAction.IsEmpty()
}

func (ff *File) SetSoftTab(b bool) {
	if b {
		ff.flags |= softTab
	} else {
		ff.flags &= ^softTab
	}
}

func (ff *File) IsSoftTab() bool {
	return ff.flags&softTab > 0
}

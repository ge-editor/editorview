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

	"github.com/ge-editor/utils"
)

type flags int8
type linefeed int8

const (
	READONLY flags = 1 << iota
	SOFT_TAB

	LF linefeed = 1 << iota
	CRLF
	CR
)

type File struct {
	rawPath  string
	path     string
	base     string
	dispPath string

	size    int64
	mode    os.FileMode
	modTime time.Time

	rows
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
		linefeed: LF,
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

func (ff *File) Rows() *rows {
	return &ff.rows
}

func (ff *File) ChangePath(path string) {
	ff.rawPath = path
	ff.init()
}

// No undo/redo functionality
func (ff *File) RemoveRegion(cursor1, cursor2 Cursor) *[]byte {
	return ff.removeRegion(cursor1, cursor2, true)
}

// No undo/redo functionality
func (ff *File) GetRegion(cursor1, cursor2 Cursor) *[]byte {
	return ff.removeRegion(cursor1, cursor2, false)
}

// No undo/redo functionality
func (ff *File) removeRegion(cursor1, cursor2 Cursor, doRemove bool) *[]byte {
	row1, col1 := cursor1.RowIndex, cursor1.ColIndex
	row2, col2 := cursor2.RowIndex, cursor2.ColIndex

	// Checked row index. The start position of the region is after the end position, or the end position is beyond the last line
	if row1 > row2 || row2 > ff.rows.RowLength()-1 {
		return nil
	}
	if row1 == row2 && col1 >= col2 {
		return nil
	}

	rw1 := &ff.rows[row1]
	// Checked col index. The start position of the region is the right of linefeed or EOF
	if col1 > len(*rw1)-1 {
		return nil
	}

	rw2 := &ff.rows[row2]
	// Checked col index. The end position of the region is the right of linefeed or EOF
	if col2 > len(*rw2)-1 {
		return nil
	}

	if row1 == row2 {
		removed := make([]byte, col2-col1)
		copy(removed, (*rw1)[col1:col2])
		if doRemove {
			*rw1 = slices.Delete(*rw1, col1, col2)
		}
		return &removed
	}

	// Compute cap and allocate
	removed := make([]byte, 0, func() int {
		total := len((*rw1)[col1:]) // first row
		for i := row1 + 1; i < row2; i++ {
			total += len(ff.rows[i]) // middle row
		}
		total += len((*rw2)[:col2]) // last row
		return total
	}())
	// first row
	removed = append(removed, (*rw1)[col1:]...)
	if doRemove {
		*rw1 = (*rw1)[:col1]
	}
	// middle rows
	for i := row1 + 1; i < row2; i++ {
		removed = append(removed, ff.rows[i]...)
	}
	// last row
	removed = append(removed, (*rw2)[:col2]...)
	// Remove middle and last rows
	if doRemove {
		*rw1 = append(*rw1, (*rw2)[col2:]...)
		// if (row2+1)-(row1+1) > 0 {
		if row2-row1 > 0 {
			ff.rows = slices.Delete(ff.rows, row1+1, row2+1)
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

// SplitByLF split s []byte by lf
// lf is not deleted
func SplitByLF(s []byte) (results [][]byte) {
	for {
		if len(s) == 0 {
			break
		}

		i := slices.Index(s, '\n')
		if i == -1 {
			results = append(results, s)
			break
		}

		i += 1 // including separator
		results = append(results, s[:i])
		s = s[i:]
	}
	return
}

// New file
func (ff *File) New() error {
	ff.rows.New()
	ff.rows.AddRow([]byte{define.EOF})

	// Set linefeed type
	ff.linefeed = LF
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
	// ff.rows__ = NewRows()
	//
	ff.rows.New()
	for scanner.Scan() {
		line := scanner.Bytes() // Not reallocate

		//row := NewRow()
		//row.bytes(line)
		//ff.rows__.append(row)

		// allocate to buffer
		b := make([]byte, 0, len(line))
		b = append(b, line...)
		ff.rows.AddRow(b)
	}
	err = scanner.Err()
	if err != nil && err != io.EOF {
		return err
	}

	// var row *Row__
	// if the file size is zero
	// if ff.LenRows__() == 0 {
	if ff.rows.RowLength() == 0 {
		//row = NewRow()
		//ff.rows__.append(row)

		ff.rows.AddRow([]byte{define.EOF})
	} else {
		// row = ff.Row__(ff.Rows__().LenRows__() - 1)
		//
		linesIndex := ff.rows.RowLength() - 1
		lineIndex, _ := ff.rows.GetColLength(linesIndex)
		if ch, _, _ := ff.rows.DecodeRune(linesIndex, lineIndex-1); ch == '\n' {
			ff.rows.AddRow([]byte{define.EOF})
		} else {
			ff.rows.AddToRow(linesIndex, []byte{define.EOF})
		}
	}
	/*
		if row.LenCh__() > 0 && row.Ch__(row.LenCh__()-1) == '\n' {
			// Add EOF
			// if row.LenCh() > 0 && row.Ch(row.LenCh()-1) == '\n' {
			row = NewRow()
			ff.rows__.append(row)
		}
		row.append(define.EOF)
	*/
	// verb.PP("%d,%d", linesIndex, lineIndex-1)

	// Set linefeed type
	ff.linefeed = []linefeed{LF, CRLF, CR}[utils.MaxValueIndex([]int{scanLines.countLF, scanLines.countCRLF, scanLines.countCR})]

	// dump
	/*
		lines := ff.bows
		for i := 0; i < lines.Length(); i++ {
			s, _ := lines.String(i)
			//panic(s)
			verb.PP("%d %s", i, s)
		}
	*/
	// m.rows.Dump()
	return nil
}
func (ff *File) Save() error {
	var sb strings.Builder // Consider using strings.Builder for potential performance gains

	linefeed := []byte{'\n'} // Default to LF
	if ff.linefeed&CRLF > 0 {
		linefeed = []byte{'\r', '\n'}
	} else if ff.linefeed&CR > 0 {
		linefeed = []byte{'\r'}
	}

	lastRowIndex := ff.rows.RowLength() - 1
	for i, row := range *ff.Rows() {
		if row == nil {
			return fmt.Errorf("row is nothing")
		}
		lineBufferLen, _ := ff.rows.GetColLength(i)
		if i == lastRowIndex && row[lineBufferLen-1] == define.EOF {
			// skip EOF mark
			sb.Write(row[:lineBufferLen-1])
			break
		} else if row[lineBufferLen-1] == define.LF {
			sb.Write(row[:lineBufferLen-1]) // skip linefeed and
			sb.Write(linefeed)              // append
		} else {
			sb.Write(row[:lineBufferLen])
		}
	}

	return os.WriteFile(ff.path, []byte(sb.String()), 0644)
}

/*
func (ff *File) Save2() error {
	// sb := strings.Builder{}
	sb := bytes.Buffer{}

	var linefeed []byte
	if ff.linefeed&lf > 0 {
		linefeed = []byte{'\n'}
	} else if ff.linefeed&crlf > 0 {
		linefeed = []byte{'\r', '\n'}
	} else { // cr
		linefeed = []byte{'\r'}
	}

	// 3. メモリ上の占有領域 (概算)
	// fmt.Println("メモリ上の占有領域 (概算):", runtime.Sizeof(data))
	// sb.Grow(int(unsafe.Sizeof(ff.bows)))
	lastRowIndex := ff.lines.RowLength() - 1
	for i, row := range *ff.Lines() {
		if row == nil {
			return fmt.Errorf("row is nothing")
		}
		lineBufferLen, _ := ff.lines.GetColLength(i)
		index := lineBufferLen
		if i == lastRowIndex && row[index] == define.EOF {
			index-- // skip EOF mark
		}
		if row[index-1] == define.LF {
			sb.Write(row[:index-1])
			sb.Write(linefeed)
		} else {
			sb.Write(row[:index])
		}
	}

	// err := os.WriteFile(ff.path, []byte(sb.String()), 0644)
	err := os.WriteFile(ff.path, sb.Bytes(), 0644)
	/
		if err == nil {
			ff.flags ^= dirty
		}
	/
	return err
}
*/

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

func (ff *File) GetEncoding() string {
	return ff.encoding
}

func (ff *File) GetLinefeed() string {
	if ff.linefeed&LF > 0 {
		return "LF"
	}
	if ff.linefeed&CRLF > 0 {
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
		ff.flags |= READONLY
	} else {
		ff.flags &= ^READONLY
	}
}

func (ff *File) IsReadonly() bool {
	return ff.flags&READONLY > 0
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
		ff.flags |= SOFT_TAB
	} else {
		ff.flags &= ^SOFT_TAB
	}
}

func (ff *File) IsSoftTab() bool {
	return ff.flags&SOFT_TAB > 0
}

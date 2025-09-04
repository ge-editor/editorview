package file

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ge-editor/gecore"
	"github.com/ge-editor/gecore/define"
	"github.com/ge-editor/gecore/lang"

	"github.com/ge-editor/editorview/file/rows"
	"github.com/ge-editor/utils"
)

type flags int8
type linefeed int8

const (
	READONLY flags = 1 << iota

	LF linefeed = 1 << iota
	CRLF
	CR
)

type File struct {
	rawPath  string
	path     string
	base     string
	ext      string
	dispPath string

	size    int64
	mode    os.FileMode
	modTime time.Time

	langMode *lang.Mode

	*rows.RowsStruct
	encoding string
	linefeed

	flags // readonly

	UndoAction *ActionGroup
	RedoAction *ActionGroup
}

// Call New() or Load() after invoking this function
func NewFile(rawPath string) *File {
	langMode := lang.Modes.GetMode(rawPath)

	ff := &File{
		rawPath:  rawPath,
		path:     "",
		base:     "",
		ext:      filepath.Ext(rawPath),
		dispPath: "",

		size:    0,
		mode:    fs.ModePerm,
		modTime: time.Now(),

		langMode: langMode,

		RowsStruct: nil,
		encoding:   "UTF-8",
		linefeed:   LF,

		flags: 0,

		UndoAction: &ActionGroup{},
		RedoAction: &ActionGroup{},
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

	ff.ext = filepath.Ext(ff.path)
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

// Not within undo/redo functionality
func (ff *File) removeRegion(start, end Cursor, doRemove bool) *[]byte {
	// Checked row index. The start position of the region is after the end position, or the end position is beyond the last line
	if start.RowIndex > end.RowIndex || end.RowIndex >= ff.RowsLength() {
		return nil
	}
	if start.RowIndex == end.RowIndex && start.ColIndex >= end.ColIndex {
		return nil
	}

	topRow := ff.Rows().Row(start.RowIndex)
	// Checked col index. The start position of the region is the right of linefeed or EOF
	if start.ColIndex >= topRow.Length() {
		return nil
	}

	bottomRow := ff.Rows().Row(end.RowIndex)
	// Checked col index. The end position of the region is the right of linefeed or EOF
	if end.ColIndex >= bottomRow.Length() {
		return nil
	}

	if start.RowIndex == end.RowIndex {
		removed := topRow.SubBytes(start.ColIndex, end.ColIndex)
		if doRemove {
			*topRow = topRow.Delete(start.ColIndex, end.ColIndex)
		}
		return &removed
	}

	// Compute cap and allocate
	removed := make([]byte, 0, func() int {
		cap := topRow.Length() - start.ColIndex // top row
		for i := start.RowIndex + 1; i < end.RowIndex; i++ {
			cap += ff.Rows().Row(i).Length() // middle row
		}
		cap += end.ColIndex // bottom row byte size
		return cap
	}())

	// top row
	removed = append(removed, (*topRow)[start.ColIndex:]...)
	if doRemove {
		*topRow = (*topRow)[:start.ColIndex]
	}
	// middle rows
	for i := start.RowIndex + 1; i < end.RowIndex; i++ {
		removed = append(removed, ff.Rows().Row(i).Bytes()...)
	}
	// bottom row
	removed = append(removed, (*bottomRow)[:end.ColIndex]...)
	// Remove middle and bottom rows
	if doRemove {
		*topRow = append(*topRow, (*bottomRow)[end.ColIndex:]...)
		if end.RowIndex-start.RowIndex > 0 {
			ff.Rows().Delete(start.RowIndex+1, end.RowIndex+1)
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
	ff.RowsStruct = rows.New()
	ff.Rows().Add([]byte{define.EOF})

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
	// ff.RowsStruct.New()
	ff.RowsStruct = rows.New()
	for scanner.Scan() {
		line := scanner.Bytes() // Not reallocate

		//row := NewRow()
		//row.bytes(line)
		//ff.rows__.append(row)

		// allocate to buffer
		b := make([]byte, 0, len(line))
		b = append(b, line...)
		ff.Rows().Add(b)
	}
	err = scanner.Err()
	if err != nil && err != io.EOF {
		return err
	}

	// var row *Row__
	// if the file size is zero
	// if ff.LenRows__() == 0 {
	if ff.RowsLength() == 0 {
		//row = NewRow()
		//ff.rows__.append(row)

		ff.Rows().Add([]byte{define.EOF})
	} else {
		// row = ff.Row__(ff.Rows__().LenRows__() - 1)
		//
		linesIndex := ff.RowsLength() - 1
		// lineIndex, _ := ff.rows.GetColLength(linesIndex)
		lineIndex := ff.Rows().Row(linesIndex).Length()
		if ch, _, _ := ff.Rows().Row(linesIndex).DecodeRune(lineIndex - 1); ch == '\n' {
			ff.Rows().Add([]byte{define.EOF})
		} else {
			ff.Rows().Row(linesIndex).Add([]byte{define.EOF})
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

func (ff *File) SetLangMode(langMode *lang.Mode) {
	ff.langMode = langMode
}

func (ff *File) GetLangMode() *lang.Mode {
	return ff.langMode
}

const (
	ErrFormatted gecore.ErrorCode = iota + 1
	// ErrNotFormatting
	// ErrFormattingFailed
	ErrSaved
	// ErrSavingFailed
	// ErrPermissionDenied
)

// Return error is joined errors
func (ff *File) Save() (results error) {
	if (*ff.langMode).IsFormattingBeforeSave() {
		// Combine [][]byte into a single byte slice
		sourceBytes, _, err := utils.JoinBytes(ff.BytesArray())
		if err != nil {
			return err
		}
		// Remove EOF Mark
		sourceBytes = sourceBytes[:len(sourceBytes)-1]
		formatted, err := (*ff.langMode).Formatting(sourceBytes)
		if err == nil {
			// Add EOF Mark and split formatted source
			formattedRows := bytes.SplitAfter(append(formatted, define.EOF), []byte("\n"))
			ff.SetRows(formattedRows)
			results = errors.Join(results, gecore.NewGeError(ErrFormatted, "formatted"))
		}
	}

	var sb strings.Builder // Consider using strings.Builder for potential performance gains

	linefeed := []byte{'\n'} // Default to LF
	if ff.linefeed&CRLF > 0 {
		linefeed = []byte{'\r', '\n'}
	} else if ff.linefeed&CR > 0 {
		linefeed = []byte{'\r'}
	}

	lastRowIndex := ff.RowsLength() - 1
	for i, row := range *ff.Rows() {
		if row == nil {
			return errors.Join(results, fmt.Errorf("row is nothing"))
		}
		lineBufferLen := ff.Rows().Row(i).Length()
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

	err := os.WriteFile(ff.path, []byte(sb.String()), 0644)
	if err == nil {
		err = gecore.NewGeError(ErrSaved, "saved")
	}
	return errors.Join(results, err)
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
	ff.ext = filepath.Ext(path)
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
	return ff.ext
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
	return (*ff.langMode).GetTabWidth()
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

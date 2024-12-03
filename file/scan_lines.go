// copied from bufio/scan.go
// 2023-05-09

package file

import (
	"bytes"
)

func newScanLines(encoding string) *scanLines_ {
	return &scanLines_{encoding: encoding}
}

type scanLines_ struct {
	countLF, countCRLF, countCR int
	encoding                    string
}

// scanLines is a split function for a Scanner that returns each line of
// text, stripped of any trailing end-of-line marker. The returned line may
// be empty. The end-of-line marker is one optional carriage return followed
// by one mandatory newline. In regular expression notation, it is `\r?\n`.
// The last non-empty line of input will be returned even if it has no
// newline.
//
// Convert the newline code to LF and leave it at the end of the line
// Count the types of newline codes
// Line feed codes correspond to LF and CRLF as in the original source
func (sl *scanLines_) scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	/* if err = (*encorder).Encoder(sl.encoding, &data); err != nil {
		return 0, nil, err
	} */

	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full newline-terminated line.
		if i > 0 && data[i-1] == '\r' {
			sl.countCRLF++
			data = append(data[0:i-1], '\n')
		} else {
			sl.countLF++
			data = data[0 : i+1]
		}
		return i + 1, data, nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		if len(data) > 0 && data[len(data)-1] == '\r' {
			sl.countCR++
			data = append(data[0:len(data)-1], '\n')
		}
		return len(data), data, nil
		// const EOF = 0x1a
		// return len(data), append(data, 0x1a), nil
	}
	// Request more data.
	return 0, nil, nil
}

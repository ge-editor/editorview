package pkg_error

import (
	"errors"
)

// Custom error
var (
	// Buffer messages
	ErrorNewFile    = errors.New("(New file)")
	ErrorLoadedFile = errors.New("(Loaded)")

	// Encoded messages
	ErrMac      = errors.New("Normalized from UTF-8-mac to UTF-8")
	ErrShiftJis = errors.New("Encoded from ShiftJIS to UTF-8")
	ErrEucJp    = errors.New("Encoded from EUC-JP to UTF-8")
)

package vline

import "github.com/ge-editor/gecore/screen"

type boundary struct {
	index  int
	widths int
}

func (bo boundary) Index() int {
	return bo.index
}

func (bo boundary) Widths() int {
	return bo.widths
}

func (bo *boundary) isEmpty() bool {
	return bo.widths == 0
}

func (bo *boundary) clear() {
	bo.index, bo.widths = 0, 0
}

// Handling of prohibited characters
// If enabled, set it in the breakpoint variable
// Check if it is a valid logical line break position and set it to bo *boundary if it is valid
//
//   - wordWrapThreshold: Set the calculation range to how many cells from the right edge of the screen
//   - Index:
//   - p2: two cells before c
//   - p1: cell before c
//   - c: cell at index position
//   - logical: information about the logical rows calculated so far
func (bo *boundary) validNewlinePosition(wordWrapThreshold, index int, p2, p1, c *cell, logical *boundary) {
	if logical.widths+c.GetCellWidth() < wordWrapThreshold || p2.IsEmpty() || p1.IsEmpty() || c.IsEmpty() {
		return
	}

	is := func(class screen.CharClass, flag screen.CharClass) bool {
		return class&flag > 0
	}

	if ((is(p2.class, screen.PROHIBITED) && is(p1.class, screen.PROHIBITED) && is(c.class, screen.PROHIBITED)) ||
		(is(p1.class, screen.PROHIBITED) && !is(c.class, screen.PROHIBITED) && !((is(p2.class, screen.NUMBER) && !is(p2.class, screen.WIDECHAR)) && is(p1.class, screen.DECIMAL_SEPARATOR) && (is(c.class, screen.NUMBER) && !is(c.class, screen.WIDECHAR))))) ||
		(!is(p1.class, screen.PROHIBITED) && !is(c.class, screen.PROHIBITED)) && (!is(p1.class, c.class) || p1.width != c.width) {
		bo.index, bo.widths = index, logical.widths
	}
}

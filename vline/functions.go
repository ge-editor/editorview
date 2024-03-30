package vline

import (
	"github.com/ge-editor/gecore/define"
	"github.com/ge-editor/gecore/screen"

	"github.com/ge-editor/utils"

	"github.com/ge-editor/theme"
)

// Update cell width and class
// c_pos: cursor x position on screen
func updateCell(c *cell, ch rune, c_pos, tabWidth int) {
	c.class = screen.GetCharClass(ch)
	if ch == define.LF {
		c.SetCellWidth(theme.LF_WIDTH)
	} else if ch == '\t' {
		c.SetCellWidth(utils.TabWidth(c_pos, tabWidth))
	} else if ch == define.DEL { // 0x7f DEL ^?
		c.SetCellWidth(2)
	} else if ch < 32 {
		c.SetCellWidth(2)
	} else /* if ch >= 32 */ {
		c.SetCellWidth(utils.RuneWidth(ch))
	}
}

// Set the on-screen width of Tab to c.Width
// Tab width at logical end of line
/*
func updateTabWidth(c *cell, screenWidth int, isLastRune bool, logicalLineWidth int) {
	// verb.PP("updateTabWidth %d", c.width)
	w := 0

	if isLastRune {
		w = screenWidth - logicalLineWidth
	} else {
		w = screenWidth - 1 - logicalLineWidth
	}

	if w > 0 && w < c.GetCellWidth() {
		c.SetCellWidth(w)
	}
	// verb.PP("updateTabWidth %d", c.width)
}
*/

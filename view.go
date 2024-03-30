package te

import (
	"github.com/ge-editor/gecore/screen"
	"github.com/ge-editor/gecore/tree"
)

func NewView() tree.View {
	v := View{
		name:   "te",
		screen: screen.Get(),
	}
	return &v
}

// Implements View interface
type View struct {
	name   string
	screen *screen.Screen
}

// Return *te.Editor as *tree.Leaf interface
func (v *View) NewLeaf() *tree.Leaf {
	e := newEditor()
	e.parentView = v
	e.screen = screen.Get()

	var tv tree.Leaf = e
	return &tv
}

// Create a new tree.Leaf (Editor) from leaf *tree.Leaf information
// direction: "right", "bottom"
func (v *View) NewSiblingLeaf(direction string, leaf *tree.Leaf) *tree.Leaf {
	newEditor := newEditor()
	newEditor.parentView = v
	newEditor.screen = screen.Get()

	// Set the value of newEditor from leafEditor
	leafEditor := (*leaf).(*Editor)
	newEditor.File = leafEditor.File   // same pointer
	*newEditor.Meta = *leafEditor.Meta // copy value

	// Cast to tree.Leaf interface and return
	var tv tree.Leaf = newEditor
	return &tv
}

func (v *View) Name() string {
	return v.name
}

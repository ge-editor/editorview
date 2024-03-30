package file

import (
	"slices"
)

//----------------------------------------------------------------------------
// action
//
// A single entity of undo/redo history. All changes to contents of a buffer
// must be initiated by an action.
//----------------------------------------------------------------------------

type ActionClass int

const (
	Insert ActionClass = iota
	Delete
	DeleteBackward
	ActionSeparator
)

type Action struct {
	Class  ActionClass
	Before Cursor
	After  Cursor
	Data   []rune
}

//----------------------------------------------------------------------------
// action group
//----------------------------------------------------------------------------

type ActionGroup []Action

/*
type ActionGroup1 struct {
	actions []Action
	prev    *ActionGroup1
	next    *ActionGroup1
}
*/

func NewActionGroup() *ActionGroup {
	var ag ActionGroup = make([]Action, 0, 32)
	return &ag
}

func (ag *ActionGroup) Push(a Action) {
	// If an action exists in action group and the action to add is
	// the same as the last action (same class and cursor position),
	// update the last action.
	lastIndex := len(*ag) - 1
	if lastIndex != -1 && // exists and same action?
		(*ag)[lastIndex].Class == a.Class &&
		(*ag)[lastIndex].After.Equals(a.Before) {

		if a.Class == DeleteBackward {
			slices.Reverse(a.Data) // Reverse
			(*ag)[lastIndex].Data = append(a.Data, (*ag)[lastIndex].Data...)
			(*ag)[lastIndex].After = a.After
		} else { // Delete, Insert
			(*ag)[lastIndex].Data = append((*ag)[lastIndex].Data, a.Data...)
			(*ag)[lastIndex].After = a.After
		}
		return
	}

	// Add the new action
	(*ag) = append((*ag), a)
}

// try before isUndoEmpty() method
func (ag *ActionGroup) Pop() (lastAction Action, _ bool) {
	if ag.IsEmpty() {
		return lastAction, false
	}
	lastIndex := len((*ag)) - 1
	lastAction = (*ag)[lastIndex]
	(*ag) = (*ag)[:lastIndex] // remove last action
	return lastAction, true
}

func (ag *ActionGroup) IsEmpty() bool {
	return len(*ag) == 0
}

// This bad function
// Move Undo ActionGroup to Redo
func (undo *ActionGroup) MoveTo(redo *ActionGroup) {
	for {
		u, ok := undo.Pop()
		if !ok {
			break
		}
		switch u.Class {
		case Insert:
			u.Class = DeleteBackward
			a := u.After
			u.After = u.Before
			u.Before = a
		case Delete:
			u.Class = Insert
		case DeleteBackward:
			u.Class = Insert
			a := u.After
			u.After = u.Before
			u.Before = a
		}
		redo.Push(u)
	}
}

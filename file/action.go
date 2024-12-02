package file

import (
	"github.com/ge-editor/utils"
)

//----------------------------------------------------------------------------
// action
//
// A single entity of undo/redo history. All changes to contents of a buffer
// must be initiated by an action.
//----------------------------------------------------------------------------

type ActionClass int

const (
	INSERT ActionClass = iota
	DELETE
	DELETE_BACKWARD
	ACTION_SEPARATOR
)

type Action struct {
	Class  ActionClass
	Before Cursor
	After  Cursor
	Data   []byte
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
	// Is last action same action?
	if lastIndex >= 0 && (*ag)[lastIndex].Class == a.Class {
		if a.Class == DELETE_BACKWARD && (*ag)[lastIndex].After.Equals(a.Before) {
			utils.ReverseUTF8Bytes(a.Data)
			(*ag)[lastIndex].Data = append(a.Data, (*ag)[lastIndex].Data...)
			// (*ag)[lastIndex].Data = append(append(make([]byte, 0, len(a.Data)+len((*ag)[lastIndex].Data)), a.Data...), (*ag)[lastIndex].Data...)
			(*ag)[lastIndex].After = a.After
			return
		} else if a.Class == INSERT && (*ag)[lastIndex].After.Equals(a.Before) {
			(*ag)[lastIndex].Data = append((*ag)[lastIndex].Data, a.Data...)
			(*ag)[lastIndex].After = a.After
			return
		} else if a.Class == DELETE && (*ag)[lastIndex].After.Equals(a.Before) {
			(*ag)[lastIndex].Data = append((*ag)[lastIndex].Data, a.Data...)
			(*ag)[lastIndex].After = a.After
			return
		}
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
	*ag = (*ag)[:lastIndex] // remove last action
	return lastAction, true
}

func (ag *ActionGroup) IsEmpty() bool {
	return len(*ag) == 0
}

// This bad function
// Move Undo ActionGroup to RedoActionGroup
func (undo *ActionGroup) MoveTo(redo *ActionGroup) {
	for {
		u, ok := undo.Pop()
		if !ok {
			break
		}
		switch u.Class {
		case INSERT:
			u.Class = DELETE_BACKWARD
			a := u.After
			u.After = u.Before
			u.Before = a
		case DELETE:
			u.Class = INSERT
		case DELETE_BACKWARD:
			u.Class = INSERT
			a := u.After
			u.After = u.Before
			u.Before = a
		}
		redo.Push(u)
	}
}

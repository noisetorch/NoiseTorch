package nucular

import (
	"image"
	"image/color"
	"math"
	"runtime"
	"time"
	"unicode"

	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"

	"github.com/aarzilli/nucular/clipboard"
	"github.com/aarzilli/nucular/command"
	"github.com/aarzilli/nucular/font"
	"github.com/aarzilli/nucular/label"
	"github.com/aarzilli/nucular/rect"
	nstyle "github.com/aarzilli/nucular/style"
)

///////////////////////////////////////////////////////////////////////////////////
// TEXT EDITOR
///////////////////////////////////////////////////////////////////////////////////

type propertyStatus int

const (
	propertyDefault = propertyStatus(iota)
	propertyEdit
	propertyDrag
)

// TextEditor stores the state of a text editor.
// To add a text editor to a window create a TextEditor object with
// &TextEditor{}, store it somewhere then in the update function call
// the Edit method passing the window to it.
type TextEditor struct {
	win            *Window
	propertyStatus propertyStatus
	Cursor         int
	Buffer         []rune
	Filter         FilterFunc
	Flags          EditFlags
	CursorFollow   bool
	Redraw         bool

	Maxlen int

	PasswordChar rune // if non-zero all characters are displayed like this character

	Initialized            bool
	Active                 bool
	InsertMode             bool
	Scrollbar              image.Point
	SelectStart, SelectEnd int
	HasPreferredX          bool
	SingleLine             bool
	PreferredX             int
	Undo                   textUndoState

	drawchunks []drawchunk

	lastClickCoord  image.Point
	lastClickTime   time.Time
	clickCount      int
	trueSelectStart int

	needle []rune

	password []rune // support buffer for drawing PasswordChar!=0 fields
}

type drawchunk struct {
	rect.Rect
	start, end int
}

func (ed *TextEditor) init(win *Window) {
	if ed.Filter == nil {
		ed.Filter = FilterDefault
	}
	if !ed.Initialized {
		if ed.Flags&EditMultiline != 0 {
			ed.clearState(TextEditMultiLine)
		} else {
			ed.clearState(TextEditSingleLine)
		}

	}
	if ed.win == nil || ed.win != win {
		if ed.win == nil {
			if ed.Buffer == nil {
				ed.Buffer = []rune{}
			}
			ed.Filter = nil
			ed.Cursor = 0
		}
		ed.Redraw = true
		ed.win = win
	}
}

type EditFlags int

const (
	EditDefault  EditFlags = 0
	EditReadOnly EditFlags = 1 << iota
	EditAutoSelect
	EditSigEnter
	EditNoCursor
	EditSelectable
	EditClipboard
	EditCtrlEnterNewline
	EditNoHorizontalScroll
	EditAlwaysInsertMode
	EditMultiline
	EditNeverInsertMode
	EditFocusFollowsMouse
	EditNoContextMenu
	EditIbeamCursor

	EditSimple = EditAlwaysInsertMode
	EditField  = EditSelectable | EditClipboard | EditSigEnter
	EditBox    = EditSelectable | EditMultiline | EditClipboard
)

type EditEvents int

const (
	EditActive EditEvents = 1 << iota
	EditInactive
	EditActivated
	EditDeactivated
	EditCommitted
)

type TextEditType int

const (
	TextEditSingleLine TextEditType = iota
	TextEditMultiLine
)

type textUndoRecord struct {
	Where        int
	InsertLength int
	DeleteLength int
	Text         []rune
}

const _TEXTEDIT_UNDOSTATECOUNT = 99

type textUndoState struct {
	UndoRec   [_TEXTEDIT_UNDOSTATECOUNT]textUndoRecord
	UndoPoint int16
	RedoPoint int16
}

func strInsertText(str []rune, pos int, runes []rune) []rune {
	if cap(str) < len(str)+len(runes) {
		newcap := (cap(str) + 1) * 2
		if newcap < len(str)+len(runes) {
			newcap = len(str) + len(runes)
		}
		newstr := make([]rune, len(str), newcap)
		copy(newstr, str)
		str = newstr
	}
	str = str[:len(str)+len(runes)]
	copy(str[pos+len(runes):], str[pos:])
	copy(str[pos:], runes)
	return str
}

func strDeleteText(s []rune, pos int, dlen int) []rune {
	copy(s[pos:], s[pos+dlen:])
	s = s[:len(s)-dlen]
	return s
}

func (s *TextEditor) hasSelection() bool {
	return s.SelectStart != s.SelectEnd
}

func (edit *TextEditor) locateCoord(p image.Point, font font.Face, row_height int) int {
	x, y := p.X, p.Y

	var drawchunk *drawchunk

	for i := range edit.drawchunks {
		min := edit.drawchunks[i].Min()
		max := edit.drawchunks[i].Max()
		getprev := false
		if min.Y <= y && y <= max.Y {
			if min.X <= x && x <= max.X {
				drawchunk = &edit.drawchunks[i]
			}
			if min.X > x {
				getprev = true
			}
		} else if min.Y > y {
			getprev = true
		}
		if getprev {
			if i == 0 {
				drawchunk = &edit.drawchunks[0]
			} else {
				drawchunk = &edit.drawchunks[i-1]
			}
			break
		}
	}

	if drawchunk == nil {
		return len(edit.Buffer)
	}

	curx := drawchunk.X
	for i := drawchunk.start; i < drawchunk.end && i < len(edit.Buffer); i++ {
		curx += FontWidth(font, string(edit.Buffer[i:i+1]))
		if curx > x {
			return i
		}
	}

	if drawchunk.end >= len(edit.Buffer) {
		return len(edit.Buffer)
	}

	return drawchunk.end
}

func (edit *TextEditor) indexToCoord(index int, font font.Face, row_height int) image.Point {
	var drawchunk *drawchunk

	for i := range edit.drawchunks {
		if edit.drawchunks[i].start > index {
			if i == 0 {
				drawchunk = &edit.drawchunks[0]
			} else {
				drawchunk = &edit.drawchunks[i-1]
			}
			break
		}
	}

	if drawchunk == nil {
		if len(edit.drawchunks) == 0 {
			return image.Point{}
		}
		drawchunk = &edit.drawchunks[len(edit.drawchunks)-1]
	}

	x := drawchunk.X
	for i := drawchunk.start; i < drawchunk.end && i < len(edit.Buffer); i++ {
		if i >= index {
			break
		}
		x += FontWidth(font, string(edit.Buffer[i:i+1]))
	}
	if index >= len(edit.Buffer) && len(edit.Buffer) > 0 && edit.Buffer[len(edit.Buffer)-1] == '\n' {
		return image.Point{x, drawchunk.Y + drawchunk.H + drawchunk.H/2}
	}
	return image.Point{x, drawchunk.Y + drawchunk.H/2}
}

func (state *TextEditor) doubleClick(coord image.Point) bool {
	abs := func(x int) int {
		if x < 0 {
			return -x
		}
		return x
	}
	r := time.Since(state.lastClickTime) < 200*time.Millisecond && abs(state.lastClickCoord.X-coord.X) < 5 && abs(state.lastClickCoord.Y-coord.Y) < 5
	state.lastClickCoord = coord
	state.lastClickTime = time.Now()
	return r

}

func (state *TextEditor) click(coord image.Point, font font.Face, row_height int) {
	/* API click: on mouse down, move the cursor to the clicked location,
	 * and reset the selection */
	state.Cursor = state.locateCoord(coord, font, row_height)

	state.SelectStart = state.Cursor
	state.trueSelectStart = state.Cursor
	state.SelectEnd = state.Cursor
	state.HasPreferredX = false

	switch state.clickCount {
	case 2:
		state.selectWord(state.SelectEnd)
	case 3:
		state.selectLine(state.SelectEnd)
	}
}

func (state *TextEditor) drag(coord image.Point, font font.Face, row_height int) {
	/* API drag: on mouse drag, move the cursor and selection endpoint
	 * to the clicked location */
	var p int = state.locateCoord(coord, font, row_height)
	if state.SelectStart == state.SelectEnd {
		state.SelectStart = state.Cursor
		state.trueSelectStart = p
	}
	state.SelectEnd = p
	state.Cursor = state.SelectEnd

	switch state.clickCount {
	case 2:
		state.selectWord(p)
	case 3:
		state.selectLine(p)
	}
}

func (state *TextEditor) selectWord(end int) {
	state.SelectStart = state.trueSelectStart
	state.SelectEnd = end
	state.sortselection()
	state.SelectStart = state.towd(state.SelectStart, -1, false)
	state.SelectEnd = state.towd(state.SelectEnd, +1, true)
}

func (state *TextEditor) selectLine(end int) {
	state.SelectStart = state.trueSelectStart
	state.SelectEnd = end
	state.sortselection()
	state.SelectStart = state.tonl(state.SelectStart-1, -1)
	state.SelectEnd = state.tonl(state.SelectEnd, +1)
}

func (state *TextEditor) clamp() {
	/* make the selection/cursor state valid if client altered the string */
	if state.hasSelection() {
		if state.SelectStart > len(state.Buffer) {
			state.SelectStart = len(state.Buffer)
		}
		if state.SelectEnd > len(state.Buffer) {
			state.SelectEnd = len(state.Buffer)
		}

		/* if clamping forced them to be equal, move the cursor to match */
		if state.SelectStart == state.SelectEnd {
			state.Cursor = state.SelectStart
		}
	}

	if state.Cursor > len(state.Buffer) {
		state.Cursor = len(state.Buffer)
	}
}

// Deletes a chunk of text in the editor.
func (edit *TextEditor) Delete(where int, len int) {
	/* delete characters while updating undo */
	edit.makeundoDelete(where, len)

	edit.Buffer = strDeleteText(edit.Buffer, where, len)
	edit.HasPreferredX = false
}

// Deletes selection.
func (edit *TextEditor) DeleteSelection() {
	/* delete the section */
	edit.clamp()

	if edit.hasSelection() {
		if edit.SelectStart < edit.SelectEnd {
			edit.Delete(edit.SelectStart, edit.SelectEnd-edit.SelectStart)
			edit.Cursor = edit.SelectStart
			edit.SelectEnd = edit.Cursor
		} else {
			edit.Delete(edit.SelectEnd, edit.SelectStart-edit.SelectEnd)
			edit.Cursor = edit.SelectEnd
			edit.SelectStart = edit.Cursor
		}

		edit.HasPreferredX = false
	}
}

func (state *TextEditor) sortselection() {
	/* canonicalize the selection so start <= end */
	if state.SelectEnd < state.SelectStart {
		var temp int = state.SelectEnd
		state.SelectEnd = state.SelectStart
		state.SelectStart = temp
	}
}

func (state *TextEditor) moveToFirst() {
	/* move cursor to first character of selection */
	if state.hasSelection() {
		state.sortselection()
		state.Cursor = state.SelectStart
		state.SelectEnd = state.SelectStart
		state.HasPreferredX = false
	}
}

func (state *TextEditor) moveToLast() {
	/* move cursor to last character of selection */
	if state.hasSelection() {
		state.sortselection()
		state.clamp()
		state.Cursor = state.SelectEnd
		state.SelectStart = state.SelectEnd
		state.HasPreferredX = false
	}
}

// Moves to the beginning or end of a line
func (state *TextEditor) tonl(start int, dir int) int {
	sz := len(state.Buffer)

	i := start
	if i < 0 {
		return 0
	}
	if i >= sz {
		i = sz - 1
	}
	for ; (i >= 0) && (i < sz); i += dir {
		c := state.Buffer[i]

		if c == '\n' {
			if dir >= 0 {
				return i
			} else {
				return i + 1
			}
		}
	}
	if dir < 0 {
		return 0
	} else {
		return sz
	}
}

// Moves to the beginning or end of an alphanumerically delimited word
func (state *TextEditor) towd(start int, dir int, dontForceAdvance bool) int {
	first := (dir < 0)
	notfirst := !first
	var i int
	for i = start; (i >= 0) && (i < len(state.Buffer)); i += dir {
		c := state.Buffer[i]
		if !(unicode.IsLetter(c) || unicode.IsDigit(c) || (c == '_')) {
			if !first && !dontForceAdvance {
				i++
			}
			break
		}
		first = notfirst
	}
	if i < 0 {
		i = 0
	}
	return i
}

func (state *TextEditor) prepSelectionAtCursor() {
	/* update selection and cursor to match each other */
	if !state.hasSelection() {
		state.SelectEnd = state.Cursor
		state.SelectStart = state.SelectEnd
	} else {
		state.Cursor = state.SelectEnd
	}
}

func (edit *TextEditor) Cut() int {
	if edit.Flags&EditReadOnly != 0 {
		return 0
	}
	/* API cut: delete selection */
	if edit.hasSelection() {
		edit.DeleteSelection() /* implicitly clamps */
		edit.HasPreferredX = false
		return 1
	}

	return 0
}

// Paste from clipboard
func (edit *TextEditor) Paste(ctext string) {
	if edit.Flags&EditReadOnly != 0 {
		return
	}

	/* if there's a selection, the paste should delete it */
	edit.clamp()

	edit.DeleteSelection()

	text := []rune(ctext)

	edit.Buffer = strInsertText(edit.Buffer, edit.Cursor, text)

	edit.makeundoInsert(edit.Cursor, len(text))
	edit.Cursor += len(text)
	edit.HasPreferredX = false
}

func (edit *TextEditor) Text(text []rune) {
	if edit.Flags&EditReadOnly != 0 {
		return
	}

	for i := range text {
		/* can't add newline in single-line mode */
		if text[i] == '\n' && edit.SingleLine {
			break
		}

		/* can't add tab in single-line mode */
		if text[i] == '\t' && edit.SingleLine {
			break
		}

		/* filter incoming text */
		if edit.Filter != nil && !edit.Filter(text[i]) {
			continue
		}

		if edit.InsertMode && !edit.hasSelection() && edit.Cursor < len(edit.Buffer) {
			edit.makeundoReplace(edit.Cursor, 1, 1)
			edit.Buffer = strDeleteText(edit.Buffer, edit.Cursor, 1)
			edit.Buffer = strInsertText(edit.Buffer, edit.Cursor, text[i:i+1])
			edit.Cursor++
			edit.HasPreferredX = false
		} else {
			edit.DeleteSelection() /* implicitly clamps */
			edit.Buffer = strInsertText(edit.Buffer, edit.Cursor, text[i:i+1])
			edit.makeundoInsert(edit.Cursor, 1)
			edit.Cursor++
			edit.HasPreferredX = false
		}
	}
}

func (state *TextEditor) key(e key.Event, font font.Face, row_height int, area_height int) {
	readOnly := state.Flags&EditReadOnly != 0
retry:
	switch e.Code {
	case key.CodeZ:
		if readOnly {
			return
		}
		if e.Modifiers&key.ModControl != 0 {
			if e.Modifiers&key.ModShift != 0 {
				state.DoRedo()
				state.HasPreferredX = false

			} else {
				state.DoUndo()
				state.HasPreferredX = false
			}
		}

	case key.CodeK:
		if readOnly {
			return
		}
		if e.Modifiers&key.ModControl != 0 {
			state.trueSelectStart = state.Cursor
			state.selectLine(state.Cursor)
			state.DeleteSelection()
		}

	case key.CodeInsert:
		state.InsertMode = !state.InsertMode

	case key.CodeLeftArrow:
		if e.Modifiers&key.ModControl != 0 {
			if e.Modifiers&key.ModShift != 0 {
				if !state.hasSelection() {
					state.prepSelectionAtCursor()
				}
				state.Cursor = state.towd(state.Cursor-1, -1, false)
				state.SelectEnd = state.Cursor
				state.clamp()
			} else {
				if state.hasSelection() {
					state.moveToFirst()
				} else {
					state.Cursor = state.towd(state.Cursor-1, -1, false)
					state.clamp()
				}
			}
		} else {
			if e.Modifiers&key.ModShift != 0 {
				state.clamp()
				state.prepSelectionAtCursor()

				/* move selection left */
				if state.SelectEnd > 0 {
					state.SelectEnd--
				}
				state.Cursor = state.SelectEnd
				state.HasPreferredX = false
			} else {
				/* if currently there's a selection,
				 * move cursor to start of selection */
				if state.hasSelection() {
					state.moveToFirst()
				} else if state.Cursor > 0 {
					state.Cursor--
				}
				state.HasPreferredX = false
			}
		}

	case key.CodeRightArrow:
		if e.Modifiers&key.ModControl != 0 {
			if e.Modifiers&key.ModShift != 0 {
				if !state.hasSelection() {
					state.prepSelectionAtCursor()
				}
				state.Cursor = state.towd(state.Cursor, +1, false)
				state.SelectEnd = state.Cursor
				state.clamp()
			} else {
				if state.hasSelection() {
					state.moveToLast()
				} else {
					state.Cursor = state.towd(state.Cursor, +1, false)
					state.clamp()
				}
			}
		} else {
			if e.Modifiers&key.ModShift != 0 {
				state.prepSelectionAtCursor()

				/* move selection right */
				state.SelectEnd++

				state.clamp()
				state.Cursor = state.SelectEnd
				state.HasPreferredX = false
			} else {
				/* if currently there's a selection,
				 * move cursor to end of selection */
				if state.hasSelection() {
					state.moveToLast()
				} else {
					state.Cursor++
				}
				state.clamp()
				state.HasPreferredX = false
			}
		}
	case key.CodeDownArrow:
		if state.SingleLine {
			e.Code = key.CodeRightArrow
			goto retry
		}
		state.verticalCursorMove(e, font, row_height, +row_height)

	case key.CodeUpArrow:
		if state.SingleLine {
			e.Code = key.CodeRightArrow
			goto retry
		}
		state.verticalCursorMove(e, font, row_height, -row_height)

	case key.CodePageDown:
		if state.SingleLine {
			break
		}
		state.verticalCursorMove(e, font, row_height, +area_height/2)

	case key.CodePageUp:
		if state.SingleLine {
			break
		}
		state.verticalCursorMove(e, font, row_height, -area_height/2)

	case key.CodeDeleteForward:
		if readOnly {
			return
		}
		if state.hasSelection() {
			state.DeleteSelection()
		} else {
			if state.Cursor < len(state.Buffer) {
				state.Delete(state.Cursor, 1)
			}
		}

		state.HasPreferredX = false

	case key.CodeDeleteBackspace:
		if readOnly {
			return
		}
		switch {
		case state.hasSelection():
			state.DeleteSelection()
		case e.Modifiers&key.ModControl != 0:
			state.SelectEnd = state.Cursor
			state.SelectStart = state.towd(state.Cursor-1, -1, false)
			state.DeleteSelection()
		default:
			state.clamp()
			if state.Cursor > 0 {
				state.Delete(state.Cursor-1, 1)
				state.Cursor--
			}
		}
		state.HasPreferredX = false

	case key.CodeHome:
		if e.Modifiers&key.ModControl != 0 {
			if e.Modifiers&key.ModShift != 0 {
				state.prepSelectionAtCursor()
				state.SelectEnd = 0
				state.Cursor = state.SelectEnd
				state.HasPreferredX = false
			} else {
				state.SelectEnd = 0
				state.SelectStart = state.SelectEnd
				state.Cursor = state.SelectStart
				state.HasPreferredX = false
			}
		} else {
			state.clamp()
			start := state.tonl(state.Cursor-1, -1)
			if e.Modifiers&key.ModShift != 0 {
				state.clamp()
				state.prepSelectionAtCursor()
				state.SelectEnd = start
				state.Cursor = state.SelectEnd
				state.HasPreferredX = false
			} else {
				state.clamp()
				state.moveToFirst()
				state.Cursor = start
				state.HasPreferredX = false
			}
		}

	case key.CodeA:
		if e.Modifiers&key.ModControl != 0 {
			state.clamp()
			state.moveToFirst()
			state.Cursor = state.tonl(state.Cursor-1, -1)
			state.HasPreferredX = false
		}

	case key.CodeEnd:
		if e.Modifiers&key.ModControl != 0 {
			if e.Modifiers&key.ModShift != 0 {
				state.prepSelectionAtCursor()
				state.SelectEnd = len(state.Buffer)
				state.Cursor = state.SelectEnd
				state.HasPreferredX = false
			} else {
				state.Cursor = len(state.Buffer)
				state.SelectEnd = 0
				state.SelectStart = state.SelectEnd
				state.HasPreferredX = false
			}
		} else {
			state.clamp()
			end := state.tonl(state.Cursor, +1)
			if e.Modifiers&key.ModShift != 0 {
				state.clamp()
				state.prepSelectionAtCursor()
				state.HasPreferredX = false
				state.Cursor = end
				state.SelectEnd = state.Cursor
			} else {
				state.clamp()
				state.moveToFirst()
				state.HasPreferredX = false
				state.Cursor = end
			}
		}

	case key.CodeE:
		if e.Modifiers&key.ModControl != 0 {
			end := state.tonl(state.Cursor, +1)
			state.clamp()
			state.moveToFirst()
			state.HasPreferredX = false
			state.Cursor = end
		}
	}
}

func (state *TextEditor) verticalCursorMove(e key.Event, font font.Face, row_height int, offset int) {
	if e.Modifiers&key.ModShift != 0 {
		state.prepSelectionAtCursor()
	} else if state.hasSelection() {
		if offset < 0 {
			state.moveToFirst()
		} else {
			state.moveToLast()
		}
	}

	state.clamp()

	p := state.indexToCoord(state.Cursor, font, row_height)
	p.Y += offset

	if state.HasPreferredX {
		p.X = state.PreferredX
	} else {
		state.HasPreferredX = true
		state.PreferredX = p.X
	}
	state.Cursor = state.locateCoord(p, font, row_height)

	state.clamp()

	if e.Modifiers&key.ModShift != 0 {
		state.SelectEnd = state.Cursor
	}
}

func texteditFlushRedo(state *textUndoState) {
	state.RedoPoint = int16(_TEXTEDIT_UNDOSTATECOUNT)
}

func texteditDiscardUndo(state *textUndoState) {
	/* discard the oldest entry in the undo list */
	if state.UndoPoint > 0 {
		state.UndoPoint--
		copy(state.UndoRec[:], state.UndoRec[1:])
	}
}

func texteditCreateUndoRecord(state *textUndoState, numchars int) *textUndoRecord {
	/* any time we create a new undo record, we discard redo*/
	texteditFlushRedo(state)

	/* if we have no free records, we have to make room,
	 * by sliding the existing records down */
	if int(state.UndoPoint) == _TEXTEDIT_UNDOSTATECOUNT {
		texteditDiscardUndo(state)
	}

	r := &state.UndoRec[state.UndoPoint]
	state.UndoPoint++
	return r
}

func texteditCreateundo(state *textUndoState, pos int, insert_len int, delete_len int) *textUndoRecord {
	r := texteditCreateUndoRecord(state, insert_len)

	r.Where = pos
	r.InsertLength = insert_len
	r.DeleteLength = delete_len
	r.Text = nil

	return r
}

func (edit *TextEditor) DoUndo() {
	var s *textUndoState = &edit.Undo
	var u textUndoRecord
	var r *textUndoRecord
	if s.UndoPoint == 0 {
		return
	}

	/* we need to do two things: apply the undo record, and create a redo record */
	u = s.UndoRec[s.UndoPoint-1]

	r = &s.UndoRec[s.RedoPoint-1]
	r.Text = nil

	r.InsertLength = u.DeleteLength
	r.DeleteLength = u.InsertLength
	r.Where = u.Where

	if u.DeleteLength != 0 {
		r.Text = make([]rune, u.DeleteLength)
		copy(r.Text, edit.Buffer[u.Where:u.Where+u.DeleteLength])
		edit.Buffer = strDeleteText(edit.Buffer, u.Where, u.DeleteLength)
	}

	/* check type of recorded action: */
	if u.InsertLength != 0 {
		/* easy case: was a deletion, so we need to insert n characters */
		edit.Buffer = strInsertText(edit.Buffer, u.Where, u.Text)
	}

	edit.Cursor = u.Where + u.InsertLength

	s.UndoPoint--
	s.RedoPoint--
}

func (edit *TextEditor) DoRedo() {
	var s *textUndoState = &edit.Undo
	var u *textUndoRecord
	var r textUndoRecord
	if int(s.RedoPoint) == _TEXTEDIT_UNDOSTATECOUNT {
		return
	}

	/* we need to do two things: apply the redo record, and create an undo record */
	u = &s.UndoRec[s.UndoPoint]

	r = s.UndoRec[s.RedoPoint]

	/* we KNOW there must be room for the undo record, because the redo record
	was derived from an undo record */
	u.DeleteLength = r.InsertLength

	u.InsertLength = r.DeleteLength
	u.Where = r.Where
	u.Text = nil

	if r.DeleteLength != 0 {
		u.Text = make([]rune, r.DeleteLength)
		copy(u.Text, edit.Buffer[r.Where:r.Where+r.DeleteLength])
		edit.Buffer = strDeleteText(edit.Buffer, r.Where, r.DeleteLength)
	}

	if r.InsertLength != 0 {
		/* easy case: need to insert n characters */
		edit.Buffer = strInsertText(edit.Buffer, r.Where, r.Text)
	}

	edit.Cursor = r.Where + r.InsertLength

	s.UndoPoint++
	s.RedoPoint++
}

func (state *TextEditor) makeundoInsert(where int, length int) {
	texteditCreateundo(&state.Undo, where, 0, length)
}

func (state *TextEditor) makeundoDelete(where int, length int) {
	u := texteditCreateundo(&state.Undo, where, length, 0)
	u.Text = make([]rune, length)
	copy(u.Text, state.Buffer[where:where+length])
}

func (state *TextEditor) makeundoReplace(where int, old_length int, new_length int) {
	u := texteditCreateundo(&state.Undo, where, old_length, new_length)
	u.Text = make([]rune, old_length)
	copy(u.Text, state.Buffer[where:where+old_length])
}

func (state *TextEditor) clearState(type_ TextEditType) {
	/* reset the state to default */
	state.Undo.UndoPoint = 0

	state.Undo.RedoPoint = int16(_TEXTEDIT_UNDOSTATECOUNT)
	state.HasPreferredX = false
	state.PreferredX = 0
	//state.CursorAtEndOfLine = 0
	state.Initialized = true
	state.SingleLine = type_ == TextEditSingleLine
	state.InsertMode = false
}

func (edit *TextEditor) SelectAll() {
	edit.SelectStart = 0
	edit.SelectEnd = len(edit.Buffer)
}

func (edit *TextEditor) editDrawText(out *command.Buffer, style *nstyle.Edit, pos image.Point, x_margin int, text []rune, textOffset int, row_height int, f font.Face, background color.RGBA, foreground color.RGBA, is_selected bool) (posOut image.Point) {
	if len(text) == 0 {
		return pos
	}
	var line_offset int = 0
	var line_count int = 0
	var txt textWidget
	txt.Background = background
	txt.Text = foreground

	pos_x, pos_y := pos.X, pos.Y
	start := 0

	tabsz := glyphAdvance(f, ' ') * tabSizeInSpaces
	pwsz := glyphAdvance(f, '*')

	measureText := func(start, end int) int {
		if edit.PasswordChar != 0 {
			return pwsz * (end - start)
		}
		// XXX calculating text width here is slow figure out why
		return measureRunes(f, text[start:end])
	}

	getText := func(start, end int) string {
		if edit.PasswordChar != 0 {
			n := end - start
			if n >= len(edit.password) {
				edit.password = make([]rune, n)
				for i := range edit.password {
					edit.password[i] = edit.PasswordChar
				}
			}
			return string(edit.password[:n])
		}
		return string(text[start:end])
	}

	flushLine := func(index int) rect.Rect {
		// new line sepeator so draw previous line
		var lblrect rect.Rect
		lblrect.Y = pos_y + line_offset
		lblrect.H = row_height
		lblrect.W = nk_null_rect.W
		lblrect.X = pos_x

		if is_selected { // selection needs to draw different background color
			if index == len(text) || (index == start && start == 0) {
				lblrect.W = measureText(start, index)
			}
			out.FillRect(lblrect, 0, background)
		}
		edit.drawchunks = append(edit.drawchunks, drawchunk{lblrect, start + textOffset, index + textOffset})
		widgetText(out, lblrect, getText(start, index), &txt, "LC", f)

		pos_x = x_margin

		return lblrect
	}

	flushTab := func(index int) rect.Rect {
		var lblrect rect.Rect
		lblrect.Y = pos_y + line_offset
		lblrect.H = row_height
		lblrect.W = measureText(start, index)
		lblrect.X = pos_x

		lblrect.W = int(math.Floor(float64(lblrect.X+lblrect.W-x_margin)/float64(tabsz))+1)*tabsz + x_margin - lblrect.X

		if is_selected {
			out.FillRect(lblrect, 0, background)
		}
		edit.drawchunks = append(edit.drawchunks, drawchunk{lblrect, start + textOffset, index + textOffset})
		widgetText(out, lblrect, getText(start, index), &txt, "LC", f)

		pos_x += lblrect.W

		return lblrect
	}

	for index, glyph := range text {
		switch glyph {
		case '\t':
			flushTab(index)
			start = index + 1
		case '\n':
			flushLine(index)
			line_count++
			start = index + 1
			line_offset += row_height

		case '\r':
			// do nothing
		}
	}

	if start >= len(text) {
		return image.Point{pos_x, pos_y + line_offset}
	}

	// draw last line
	lblrect := flushLine(len(text))
	lblrect.W = measureText(start, len(text))

	return image.Point{lblrect.X + lblrect.W, lblrect.Y}
}

func (ed *TextEditor) doEdit(bounds rect.Rect, style *nstyle.Edit, inp *Input, cut, copy, paste bool) (ret EditEvents) {
	font := ed.win.ctx.Style.Font
	state := ed.win.widgets.PrevState(bounds)

	ed.clamp()

	// visible text area calculation
	var area rect.Rect
	area.X = bounds.X + style.Padding.X + style.Border
	area.Y = bounds.Y + style.Padding.Y + style.Border
	area.W = bounds.W - (2.0*style.Padding.X + 2*style.Border)
	area.H = bounds.H - (2.0*style.Padding.Y + 2*style.Border)
	if ed.Flags&EditMultiline != 0 {
		area.H = area.H - style.ScrollbarSize.Y
	}
	var row_height int
	if ed.Flags&EditMultiline != 0 {
		row_height = FontHeight(font) + style.RowPadding
	} else {
		row_height = area.H
	}

	/* update edit state */
	prev_state := ed.Active

	if ed.win.ctx.activateEditor != nil {
		if ed.win.ctx.activateEditor == ed {
			ed.Active = true
			if ed.win.flags&windowDocked != 0 {
				ed.win.ctx.dockedWindowFocus = ed.win.idx
			}
		} else {
			ed.Active = false
		}
	}

	is_hovered := inp.Mouse.HoveringRect(bounds)

	if ed.Flags&EditFocusFollowsMouse != 0 {
		if inp != nil {
			ed.Active = is_hovered
		}
	} else {
		if inp != nil && inp.Mouse.Buttons[mouse.ButtonLeft].Clicked && inp.Mouse.Buttons[mouse.ButtonLeft].Down {
			ed.Active = inp.Mouse.HoveringRect(bounds)
		}
	}

	/* (de)activate text editor */
	var select_all bool
	if !prev_state && ed.Active {
		type_ := TextEditSingleLine
		if ed.Flags&EditMultiline != 0 {
			type_ = TextEditMultiLine
		}
		ed.clearState(type_)
		if ed.Flags&EditAlwaysInsertMode != 0 {
			ed.InsertMode = true
		}
		if ed.Flags&EditAutoSelect != 0 {
			select_all = true
		}
	} else if !ed.Active {
		ed.InsertMode = false
	}

	if ed.Flags&EditNeverInsertMode != 0 {
		ed.InsertMode = false
	}

	if ed.Active {
		ret = EditActive
	} else {
		ret = EditInactive
	}
	if prev_state != ed.Active {
		if ed.Active {
			ret |= EditActivated
		} else {
			ret |= EditDeactivated
		}
	}

	/* handle user input */
	cursor_follow := ed.CursorFollow
	ed.CursorFollow = false
	if ed.Active && inp != nil {
		inpos := inp.Mouse.Pos
		indelta := inp.Mouse.Delta
		coord := image.Point{(inpos.X - area.X), (inpos.Y - area.Y)}

		var isHovered bool
		{
			areaWithoutScrollbar := area
			areaWithoutScrollbar.W -= style.ScrollbarSize.X
			isHovered = inp.Mouse.HoveringRect(areaWithoutScrollbar)
		}

		var autoscrollTop bool
		{
			a := area
			a.W -= style.ScrollbarSize.X
			a.H = FontHeight(font) / 2
			autoscrollTop = inp.Mouse.HoveringRect(a) && inp.Mouse.Buttons[mouse.ButtonLeft].Down
		}

		var autoscrollBot bool
		{
			a := area
			a.W -= style.ScrollbarSize.X
			a.Y = a.Y + a.H - FontHeight(font)/2
			a.H = FontHeight(font) / 2
			autoscrollBot = inp.Mouse.HoveringRect(a) && inp.Mouse.Buttons[mouse.ButtonLeft].Down
		}

		/* mouse click handler */
		if select_all {
			ed.SelectAll()
		} else if isHovered && inp.Mouse.Buttons[mouse.ButtonLeft].Down && inp.Mouse.Buttons[mouse.ButtonLeft].Clicked {
			if ed.doubleClick(coord) {
				ed.clickCount++
				if ed.clickCount > 3 {
					ed.clickCount = 3
				}
			} else {
				ed.clickCount = 1
			}
			ed.click(coord, font, row_height)
		} else if isHovered && inp.Mouse.Buttons[mouse.ButtonLeft].Down && (indelta.X != 0.0 || indelta.Y != 0.0) {
			ed.drag(coord, font, row_height)
			cursor_follow = true
		} else if autoscrollTop {
			coord1 := coord
			coord1.Y -= FontHeight(font)
			ed.drag(coord1, font, row_height)
			cursor_follow = true
		} else if autoscrollBot {
			coord1 := coord
			coord1.Y += FontHeight(font)
			ed.drag(coord1, font, row_height)
			cursor_follow = true
		}

		/* text input */
		if inp.Keyboard.Text != "" {
			ed.Text([]rune(inp.Keyboard.Text))
			cursor_follow = true
		}

		clipboardModifier := key.ModControl
		if runtime.GOOS == "darwin" {
			clipboardModifier = key.ModMeta
		}

		for _, e := range inp.Keyboard.Keys {
			switch e.Code {
			case key.CodeReturnEnter:
				if ed.Flags&EditCtrlEnterNewline != 0 && e.Modifiers&key.ModShift != 0 {
					ed.Text([]rune{'\n'})
					cursor_follow = true
				} else if ed.Flags&EditSigEnter != 0 {
					ret = EditInactive
					ret |= EditDeactivated
					if ed.Flags&EditReadOnly == 0 {
						ret |= EditCommitted
					}
					ed.Active = false
				}

			case key.CodeX:
				if e.Modifiers&clipboardModifier != 0 {
					cut = true
				}

			case key.CodeC:
				if e.Modifiers&clipboardModifier != 0 {
					copy = true
				}

			case key.CodeV:
				if e.Modifiers&clipboardModifier != 0 {
					paste = true
				}

			case key.CodeF:
				if e.Modifiers&clipboardModifier != 0 {
					ed.popupFind()
				}

			case key.CodeG:
				if e.Modifiers&clipboardModifier != 0 {
					ed.lookForward(true)
					cursor_follow = true
				}

			default:
				ed.key(e, font, row_height, area.H)
				cursor_follow = true
			}

		}

		/* cut & copy handler */
		if (copy || cut) && (ed.Flags&EditClipboard != 0) {
			var begin, end int
			if ed.SelectStart > ed.SelectEnd {
				begin = ed.SelectEnd
				end = ed.SelectStart
			} else {
				begin = ed.SelectStart
				end = ed.SelectEnd
			}
			clipboard.Set(string(ed.Buffer[begin:end]))
			if cut {
				ed.Cut()
				cursor_follow = true
			}
		}

		/* paste handler */
		if paste && (ed.Flags&EditClipboard != 0) {
			ed.Paste(clipboard.Get())
			cursor_follow = true
		}

	}

	/* set widget state */
	if ed.Active {
		state = nstyle.WidgetStateActive
	} else {
		state = nstyle.WidgetStateInactive
	}
	if is_hovered {
		state |= nstyle.WidgetStateHovered
	}

	var d drawableTextEditor

	/* text pointer positions */
	var selection_begin, selection_end int
	if ed.SelectStart < ed.SelectEnd {
		selection_begin = ed.SelectStart
		selection_end = ed.SelectEnd
	} else {
		selection_begin = ed.SelectEnd
		selection_end = ed.SelectStart
	}

	d.SelectionBegin, d.SelectionEnd = selection_begin, selection_end

	d.Edit = ed
	d.State = state
	d.Style = style
	d.Scaling = ed.win.ctx.Style.Scaling
	d.Bounds = bounds
	d.Area = area
	d.RowHeight = row_height
	d.hasInput = inp.Mouse.valid
	ed.win.widgets.Add(state, bounds)
	d.Draw(&ed.win.ctx.Style, &ed.win.cmds)

	/* scrollbar */
	if cursor_follow {
		cursor_pos := d.CursorPos
		/* update scrollbar to follow cursor */
		if ed.Flags&EditNoHorizontalScroll == 0 {
			/* horizontal scroll */
			scroll_increment := area.W / 2
			if (cursor_pos.X < ed.Scrollbar.X) || ((ed.Scrollbar.X+area.W)-cursor_pos.X < FontWidth(font, "i")) {
				ed.Scrollbar.X = max(0, cursor_pos.X-scroll_increment)
			}
		} else {
			ed.Scrollbar.X = 0
		}

		if ed.Flags&EditMultiline != 0 {
			/* vertical scroll */
			if cursor_pos.Y < ed.Scrollbar.Y {
				ed.Scrollbar.Y = max(0, cursor_pos.Y-row_height)
			}
			for (ed.Scrollbar.Y+area.H)-cursor_pos.Y < row_height {
				ed.Scrollbar.Y = ed.Scrollbar.Y + row_height
			}
		} else {
			ed.Scrollbar.Y = 0
		}
	}

	if !ed.SingleLine {
		/* scrollbar widget */
		var scroll rect.Rect
		scroll.X = (area.X + area.W) - style.ScrollbarSize.X
		scroll.Y = area.Y
		scroll.W = style.ScrollbarSize.X
		scroll.H = area.H

		scroll_offset := float64(ed.Scrollbar.Y)
		scroll_step := float64(scroll.H) * 0.1
		scroll_inc := float64(scroll.H) * 0.01
		scroll_target := float64(d.TextSize.Y + row_height)
		newy := int(doScrollbarv(ed.win, scroll, bounds, scroll_offset, scroll_target, scroll_step, scroll_inc, &style.Scrollbar, inp, font))
		if newy != ed.Scrollbar.Y {
			ed.win.ctx.trashFrame = true
		}
		ed.Scrollbar.Y = newy
	}

	return ret
}

type drawableTextEditor struct {
	Edit      *TextEditor
	State     nstyle.WidgetStates
	Style     *nstyle.Edit
	Scaling   float64
	Bounds    rect.Rect
	Area      rect.Rect
	RowHeight int
	hasInput  bool

	SelectionBegin, SelectionEnd int

	TextSize  image.Point
	CursorPos image.Point
}

func (d *drawableTextEditor) Draw(z *nstyle.Style, out *command.Buffer) {
	edit := d.Edit
	state := d.State
	style := d.Style
	bounds := d.Bounds
	font := z.Font
	area := d.Area
	row_height := d.RowHeight
	selection_begin := d.SelectionBegin
	selection_end := d.SelectionEnd
	if edit.drawchunks != nil {
		edit.drawchunks = edit.drawchunks[:0]
	}

	/* select background colors/images  */
	var old_clip rect.Rect = out.Clip
	{
		var background *nstyle.Item
		if state&nstyle.WidgetStateActive != 0 {
			background = &style.Active
		} else if state&nstyle.WidgetStateHovered != 0 {
			background = &style.Hover
		} else {
			background = &style.Normal
		}

		/* draw background frame */
		if background.Type == nstyle.ItemColor {
			out.FillRect(bounds, style.Rounding, style.BorderColor)
			out.FillRect(shrinkRect(bounds, style.Border), style.Rounding, background.Data.Color)
		} else {
			out.DrawImage(bounds, background.Data.Image)
		}
	}

	area.W -= FontWidth(font, "i")
	clip := unify(old_clip, area)
	out.PushScissor(clip)
	/* draw text */
	var background_color color.RGBA
	var text_color color.RGBA
	var sel_background_color color.RGBA
	var sel_text_color color.RGBA
	var cursor_color color.RGBA
	var cursor_text_color color.RGBA
	var background *nstyle.Item

	/* select correct colors to draw */
	if state&nstyle.WidgetStateActive != 0 {
		background = &style.Active
		text_color = style.TextActive
		sel_text_color = style.SelectedTextHover
		sel_background_color = style.SelectedHover
		cursor_color = style.CursorHover
		cursor_text_color = style.CursorTextHover
	} else if state&nstyle.WidgetStateHovered != 0 {
		background = &style.Hover
		text_color = style.TextHover
		sel_text_color = style.SelectedTextHover
		sel_background_color = style.SelectedHover
		cursor_text_color = style.CursorTextHover
		cursor_color = style.CursorHover
	} else {
		background = &style.Normal
		text_color = style.TextNormal
		sel_text_color = style.SelectedTextNormal
		sel_background_color = style.SelectedNormal
		cursor_color = style.CursorNormal
		cursor_text_color = style.CursorTextNormal
	}

	if background.Type == nstyle.ItemImage {
		background_color = color.RGBA{0, 0, 0, 0}
	} else {
		background_color = background.Data.Color
	}

	startPos := image.Point{area.X - edit.Scrollbar.X, area.Y - edit.Scrollbar.Y}
	pos := startPos
	x_margin := pos.X
	if edit.SelectStart == edit.SelectEnd {
		drawEolCursor := func() {
			cursor_pos := d.CursorPos
			/* draw cursor at end of line */
			var cursor rect.Rect
			if edit.Flags&EditIbeamCursor != 0 {
				cursor.W = int(d.Scaling)
				if cursor.W <= 0 {
					cursor.W = 1
				}
			} else {
				cursor.W = FontWidth(font, "i")
			}
			cursor.H = row_height
			cursor.X = area.X + cursor_pos.X - edit.Scrollbar.X
			cursor.Y = area.Y + cursor_pos.Y + row_height/2.0 - cursor.H/2.0
			cursor.Y -= edit.Scrollbar.Y
			out.FillRect(cursor, 0, cursor_color)
		}

		/* no selection so just draw the complete text */
		pos = edit.editDrawText(out, style, pos, x_margin, edit.Buffer[:edit.Cursor], 0, row_height, font, background_color, text_color, false)
		d.CursorPos = pos.Sub(startPos)
		if edit.Active && d.hasInput {
			if edit.Cursor < len(edit.Buffer) {
				cursorChar := edit.Buffer[edit.Cursor]
				if cursorChar == '\n' || cursorChar == '\t' || edit.Flags&EditIbeamCursor != 0 {
					pos = edit.editDrawText(out, style, pos, x_margin, edit.Buffer[edit.Cursor:edit.Cursor+1], edit.Cursor, row_height, font, background_color, text_color, true)
					drawEolCursor()
				} else {
					pos = edit.editDrawText(out, style, pos, x_margin, edit.Buffer[edit.Cursor:edit.Cursor+1], edit.Cursor, row_height, font, cursor_color, cursor_text_color, true)
				}
				pos = edit.editDrawText(out, style, pos, x_margin, edit.Buffer[edit.Cursor+1:], edit.Cursor+1, row_height, font, background_color, text_color, false)
			} else {
				drawEolCursor()
			}
		} else if edit.Cursor < len(edit.Buffer) {
			pos = edit.editDrawText(out, style, pos, x_margin, edit.Buffer[edit.Cursor:], edit.Cursor, row_height, font, background_color, text_color, false)
		}
	} else {
		/* edit has selection so draw 1-3 text chunks */
		if selection_begin > 0 {
			/* draw unselected text before selection */
			pos = edit.editDrawText(out, style, pos, x_margin, edit.Buffer[:selection_begin], 0, row_height, font, background_color, text_color, false)
		}

		if selection_begin == edit.SelectEnd {
			d.CursorPos = pos.Sub(startPos)
		}

		pos = edit.editDrawText(out, style, pos, x_margin, edit.Buffer[selection_begin:selection_end], selection_begin, row_height, font, sel_background_color, sel_text_color, true)

		if selection_begin != edit.SelectEnd {
			d.CursorPos = pos.Sub(startPos)
		}

		if selection_end < len(edit.Buffer) {
			pos = edit.editDrawText(out, style, pos, x_margin, edit.Buffer[selection_end:], selection_end, row_height, font, background_color, text_color, false)
		}
	}
	d.TextSize = pos.Sub(startPos)

	// fix rectangles in drawchunks by subtracting area from them
	for i := range edit.drawchunks {
		edit.drawchunks[i].X -= area.X
		edit.drawchunks[i].Y -= area.Y
	}

	out.PushScissor(old_clip)
}

func runeSliceEquals(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var clipboardModifier = func() key.Modifiers {
	if runtime.GOOS == "darwin" {
		return key.ModMeta
	}
	return key.ModControl
}()

func (edit *TextEditor) popupFind() {
	if edit.Flags&EditMultiline == 0 {
		return
	}
	var searchEd TextEditor
	searchEd.Flags = EditSigEnter | EditClipboard | EditSelectable
	searchEd.Buffer = append(searchEd.Buffer[:0], edit.needle...)
	searchEd.SelectStart = 0
	searchEd.SelectEnd = len(searchEd.Buffer)
	searchEd.Cursor = searchEd.SelectEnd
	searchEd.Active = true

	edit.SelectEnd = edit.SelectStart
	edit.Cursor = edit.SelectStart

	edit.win.Master().PopupOpen("Search...", WindowTitle|WindowNoScrollbar|WindowMovable|WindowBorder|WindowDynamic, rect.Rect{100, 100, 400, 500}, true, func(w *Window) {
		w.Row(30).Static()
		w.LayoutFitWidth(0, 30)
		w.Label("Search: ", "LC")
		w.LayoutSetWidth(150)
		ev := searchEd.Edit(w)
		if ev&EditCommitted != 0 {
			edit.Active = true
			w.Close()
		}
		w.LayoutSetWidth(100)
		if w.ButtonText("Done") {
			edit.Active = true
			w.Close()
		}
		kbd := &w.Input().Keyboard
		for _, k := range kbd.Keys {
			switch {
			case k.Modifiers == clipboardModifier && k.Code == key.CodeG:
				edit.lookForward(true)
			case k.Modifiers == 0 && k.Code == key.CodeEscape:
				edit.SelectEnd = edit.SelectStart
				edit.Cursor = edit.SelectStart
				edit.Active = true
				w.Close()
			}
		}
		if !runeSliceEquals(searchEd.Buffer, edit.needle) {
			edit.needle = append(edit.needle[:0], searchEd.Buffer...)
			edit.lookForward(false)
		}
	})
}

func (edit *TextEditor) lookForward(forceAdvance bool) {
	if edit.Flags&EditMultiline == 0 {
		return
	}
	if edit.hasSelection() {
		if forceAdvance {
			edit.SelectStart = edit.SelectEnd
		} else {
			edit.SelectEnd = edit.SelectStart
		}
		if edit.SelectEnd >= 0 {
			edit.Cursor = edit.SelectEnd
		}
	}
	for start := edit.Cursor; start < len(edit.Buffer); start++ {
		found := true
		for i := 0; i < len(edit.needle); i++ {
			if edit.needle[i] != edit.Buffer[start+i] {
				found = false
				break
			}
		}
		if found {
			edit.SelectStart = start
			edit.SelectEnd = start + len(edit.needle)
			edit.Cursor = edit.SelectEnd
			edit.CursorFollow = true
			return
		}
	}
	edit.SelectStart = 0
	edit.SelectEnd = 0
	edit.Cursor = 0
	edit.CursorFollow = true
}

// Adds text editor edit to win.
// Initial contents of the text editor will be set to text. If
// alwaysSet is specified the contents of the editor will be reset
// to text.
func (edit *TextEditor) Edit(win *Window) EditEvents {
	edit.init(win)
	if edit.Maxlen > 0 {
		if len(edit.Buffer) > edit.Maxlen {
			edit.Buffer = edit.Buffer[:edit.Maxlen]
		}
	}

	if edit.Flags&EditNoCursor != 0 {
		edit.Cursor = len(edit.Buffer)
	}
	if edit.Flags&EditSelectable == 0 {
		edit.SelectStart = edit.Cursor
		edit.SelectEnd = edit.Cursor
	}

	var bounds rect.Rect

	style := &edit.win.ctx.Style
	widget_state, bounds, _ := edit.win.widget()
	if !widget_state {
		return 0
	}
	in := edit.win.inputMaybe(widget_state)

	var cut, copy, paste bool

	if w := win.ContextualOpen(0, image.Point{}, bounds, nil); w != nil {
		w.Row(20).Dynamic(1)
		visible := false
		if edit.Flags&EditClipboard != 0 {
			visible = true
			if w.MenuItem(label.TA("Cut", "LC")) {
				cut = true
			}
			if w.MenuItem(label.TA("Copy", "LC")) {
				copy = true
			}
			if w.MenuItem(label.TA("Paste", "LC")) {
				paste = true
			}
		}
		if edit.Flags&EditMultiline != 0 {
			visible = true
			if w.MenuItem(label.TA("Find...", "LC")) {
				edit.popupFind()
			}
		}
		if !visible {
			w.Close()
		}
	}

	ev := edit.doEdit(bounds, &style.Edit, in, cut, copy, paste)
	return ev
}

// Package prompt is a dialog that asks for one line of text, such as the name
// of a new component.
package prompt

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/a11y"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/modal"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/pointer"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/theme"
	"github.com/meta-ui-labs/cuppa/libs/document/design"
)

const (
	boxW   = 50
	okText = "[ OK ]"
	noText = "[ Cancel ]"
)

// Model is one open prompt. Enter confirms a non-empty text; Esc cancels.
type Model struct {
	title, label string
	text         []rune
	hint         string
	// fresh is true until the first edit: the pre-filled text is selected, so
	// typing replaces it and Backspace clears it.
	fresh bool
	// pos is the text cursor, as a rune index into text.
	pos int

	// focus is 0 for the text field, 1 for OK and 2 for Cancel.
	focus   int
	rect    design.Rect
	hot     string
	outcome modal.Outcome
	done    bool
}

// New returns a prompt with the text pre-filled.
func New(title, label, initial string) *Model {
	return &Model{title: title, label: label, text: []rune(initial), pos: len([]rune(initial)), fresh: initial != ""}
}

// Place implements modal.Modal.
func (m *Model) Place(sw, sh int) {
	w := min(boxW, sw)
	m.rect = design.Rect{X: max((sw-w)/2, 0), Y: max((sh-7)/3, 0), W: w, H: 7}
}

// Rect implements modal.Modal.
func (m *Model) Rect() design.Rect { return m.rect }

// Lines implements modal.Modal.
func (m *Model) Lines() []string {
	inner := m.rect.W - 6
	field := theme.Fit(m.shown(inner), inner)
	hint := ""
	if m.hint != "" {
		hint = theme.Faded(m.hint)
	}
	row := m.button("ok", okText) + " " + m.button("no", noText)
	pad := max(m.rect.W-2-ansi.StringWidth(row)-1, 0)
	content := []string{" " + m.label, "  " + field, "  " + hint, "", strings.Repeat(" ", pad) + row}
	return theme.Panel(m.title, content, m.rect.W)
}

func (m *Model) button(id, label string) string {
	if m.hot == id || m.focus == 1 && id == "ok" || m.focus == 2 && id == "no" {
		return theme.Selected(label)
	}
	return theme.Button(label, true)
}

// buttonAt says which button the box column x is over.
func (m *Model) buttonAt(x int) string {
	noW, okW := ansi.StringWidth(noText), ansi.StringWidth(okText)
	noX := m.rect.W - 2 - noW
	okX := noX - 1 - okW
	switch {
	case x >= noX && x < noX+noW:
		return "no"
	case x >= okX && x < okX+okW:
		return "ok"
	}
	return ""
}

// Handle implements modal.Modal.
func (m *Model) Handle(e pointer.Event) {
	if m.done {
		return
	}
	x, y := e.X-m.rect.X, e.Y-m.rect.Y
	m.hot = ""
	if y != m.rect.H-2 {
		return
	}
	m.hot = m.buttonAt(x)
	if e.Phase == pointer.Down && e.Left {
		switch m.hot {
		case "ok":
			m.confirm()
		case "no":
			m.finish(true)
		}
	}
}

// Key implements modal.Modal.
func (m *Model) Key(text string, back, enter, esc bool) {
	switch {
	case esc:
		m.finish(true)
	case enter && m.focus == 2:
		m.finish(true)
	case enter:
		m.confirm()
	case m.focus != 0:
		// A button has the keyboard; typing goes nowhere.
	case back && m.fresh:
		m.text, m.pos, m.fresh = nil, 0, false
	case back:
		if m.pos > 0 {
			m.text = append(m.text[:m.pos-1], m.text[m.pos:]...)
			m.pos--
		}
	case text != "":
		if m.fresh {
			m.text, m.pos, m.fresh = nil, 0, false
		}
		t := []rune(text)
		m.text = append(m.text[:m.pos], append(t, m.text[m.pos:]...)...)
		m.pos += len(t)
	}
}

// moveCursor takes Left, Right, Home, End and Delete in the text field. With the
// text still selected, Left and Home go to its start, Right and End to its end,
// and Delete clears it.
func (m *Model) moveCursor(name string) {
	switch name {
	case "left":
		m.pos = max(m.pos-1, 0)
		if m.fresh {
			m.pos = 0
		}
	case "right":
		m.pos = min(m.pos+1, len(m.text))
		if m.fresh {
			m.pos = len(m.text)
		}
	case "home":
		m.pos = 0
	case "end":
		m.pos = len(m.text)
	case "delete":
		switch {
		case m.fresh:
			m.text, m.pos = nil, 0
		case m.pos < len(m.text):
			m.text = append(m.text[:m.pos], m.text[m.pos+1:]...)
		}
	}
	m.fresh = false
}

// shown is the text with its cursor, kept to width cells around the cursor, or
// highlighted while the next key will replace all of it.
func (m *Model) shown(width int) string {
	if m.fresh && m.focus == 0 && len(m.text) > 0 {
		return theme.Selected(tail(string(m.text), width))
	}
	r := m.text
	m.pos = min(max(m.pos, 0), len(r))
	start := max(m.pos-(width-2), 0)
	r, pos := r[start:], m.pos-start
	if start > 0 {
		r = append([]rune{'…'}, r[1:]...)
	}
	if pos == len(r) {
		return string(r) + "█"
	}
	return string(r[:pos]) + theme.Selected(string(r[pos])) + string(r[pos+1:])
}

// Nav implements modal.Navigator: Tab and Shift+Tab move between the text
// field, OK and Cancel; with a button focused the arrows move between them.
func (m *Model) Nav(name string) bool {
	switch name {
	case "tab":
		m.focus = (m.focus + 1) % 3
	case "shift+tab":
		m.focus = (m.focus + 2) % 3
	case "left", "right", "home", "end", "delete":
		if m.focus == 0 {
			m.moveCursor(name)
			return true
		}
		if name == "home" || name == "end" || name == "delete" {
			return false
		}
		m.focus = 3 - m.focus
	default:
		return false
	}
	return true
}

func (m *Model) confirm() {
	if strings.TrimSpace(string(m.text)) == "" {
		m.hint = "Type a name first"
		return
	}
	m.finish(false)
}

func (m *Model) finish(canceled bool) {
	m.done = true
	m.outcome = modal.Outcome{Button: "OK", Value: strings.TrimSpace(string(m.text)), Canceled: canceled}
}

// Outcome implements modal.Modal.
func (m *Model) Outcome() (modal.Outcome, bool) { return m.outcome, m.done }

// Describe reads the dialog out: its question, the text so far and the buttons.
func (m *Model) Describe() []a11y.Node {
	field := a11y.Field(m.label, string(m.text))
	field.Focused = m.focus == 0
	nodes := []a11y.Node{a11y.Heading(m.title), field}
	if m.hint != "" {
		nodes = append(nodes, a11y.Text(m.hint))
	}
	ok, cancel := a11y.Button("OK"), a11y.Button("Cancel")
	ok.Focused, cancel.Focused = m.focus == 1, m.focus == 2
	return append(nodes, ok, cancel)
}

// tail keeps the end of a long text, where the cursor is, with an ellipsis in front.
func tail(s string, width int) string {
	if ansi.StringWidth(s) <= width {
		return s
	}
	r := []rune(s)
	return "…" + string(r[len(r)-(width-1):])
}

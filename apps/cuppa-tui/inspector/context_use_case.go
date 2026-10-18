package inspector

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/x/ansi"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/pointer"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/theme"
	"github.com/meta-ui-labs/cuppa/libs/canvas/editor"
	"github.com/meta-ui-labs/cuppa/libs/document/design"
)

// ContextRows is how many lines the contextual bar takes above the canvas.
const ContextRows = 2

// Context renders the contextual bar of the select tool, ContextRows lines of
// w cells: what the selection is and where, and what can be done to it
// (order, copy, group), as the options bar of an image editor does. Nothing
// selected shows the snap setting only.
func (m *Model) Context(w int) []string {
	top, bottom := &builder{focus: m.focusStop()}, &builder{}
	// The two lines are built one after the other so the keyboard stops run
	// along the first line and then along the second.
	switch n, ok := m.ed.Primary(); {
	case !ok:
		top.text(theme.Title(" Select ")).text(theme.Dim("Nothing selected. Drag a component from the left bar onto the canvas.")).end()
	case len(m.ed.Selected()) > 1:
		top.text(theme.Title(" Select ")).text(theme.Bold(fmt.Sprintf("%d selected", len(m.ed.Selected())))).end()
	default:
		m.geometryRow(top, n)
	}
	bottom.focus, bottom.stops = m.focusStop(), top.stops
	bottom.regions = nil
	if _, ok := m.ed.Primary(); ok {
		m.arrangeRow(bottom)
	} else {
		bottom.text(" ")
		m.snapButton(bottom)
	}
	m.messageInline(bottom)
	bottom.end()

	// One region list for the bar, with the second line's rows after the first.
	lines := []string{top.lines[0], bottom.lines[0]}
	m.ctx = m.ctx[:0]
	m.ctx = append(m.ctx, top.regions...)
	for _, r := range bottom.regions {
		r.y = 1
		m.ctx = append(m.ctx, r)
	}
	m.ctxStops = bottom.stops
	return theme.Block(lines, w, ContextRows)
}

// geometryRow is the first line: the name and the place and size.
func (m *Model) geometryRow(b *builder, n design.Node) {
	b.text(theme.Title(" Select "))
	if m.editing == "name" {
		b.text(theme.Dim("Name ") + m.shown())
	} else {
		name := n.Name
		if n.Locked {
			name += " (locked)"
		}
		b.text(theme.Dim("Name "))
		b.add(theme.Bold(theme.Fit(ansi.Truncate(name, 16, "…"), 16)), func() { m.startEdit("name", n.Name) })
	}
	l := n.Layout
	b.text("  ")
	m.barNumeric(b, n.ID, "X", "x", n.Rect.X, l.X)
	m.barNumeric(b, n.ID, "Y", "y", n.Rect.Y, l.Y)
	m.barNumeric(b, n.ID, "W", "w", n.Rect.W, l.W)
	m.barNumeric(b, n.ID, "H", "h", n.Rect.H, l.H)
	b.end()
}

// barNumeric is one geometry field of the bar: its letter and the value, which
// is typed when pressed and steps with Left and Right on the keyboard. A value
// that follows the canvas shows its expression (typing a number makes it
// fixed again), and the last button switches the axis between fixed cells (%
// offered) and a percentage of the canvas (# offered).
func (m *Model) barNumeric(b *builder, id design.NodeID, label, field string, v int, expression string) {
	shown, initial := strconv.Itoa(v), strconv.Itoa(v)
	if expression != "" {
		shown, initial = expression, expression
	}
	b.text(theme.Dim(label + " "))
	if m.editing == field {
		b.text(m.shown() + " ")
		return
	}
	b.addStep(theme.Bold(shown), func() { m.startEdit(field, initial) }, func(d int) { m.nudge(field, d) })
	b.text(" ")
	b.addMouse(theme.Button("◂", true), func() { m.nudge(field, -1) })
	b.addMouse(theme.Button("▸", true), func() { m.nudge(field, 1) })
	unit := "%"
	if expression != "" {
		unit = "#"
	}
	b.text(" ").add(theme.Button(unit, true), func() { m.report(m.ed.ToggleLayoutUnit(id, field)) })
	b.text("  ")
}

// arrangeRow is the second line: order, copy, delete and group buttons and the
// snap checkbox.
func (m *Model) arrangeRow(b *builder) {
	button := func(label string, on bool, act func()) {
		b.add(theme.Button("["+label+"]", on), act)
		b.text(" ")
	}
	b.text(" ")
	button("Front", true, func() { m.ed.Reorder(editor.BringToFront) })
	button("Fwd", true, func() { m.ed.Reorder(editor.BringForward) })
	button("Bwd", true, func() { m.ed.Reorder(editor.SendBackward) })
	button("Back", true, func() { m.ed.Reorder(editor.SendToBack) })
	b.text(" ")
	button("Duplicate", true, func() { m.ed.Duplicate() })
	button("Delete", true, func() { m.ed.Delete() })
	b.text(" ")
	button("Group", m.ed.CanGroup(), func() { m.ed.Group() })
	button("Ungroup", m.ed.CanUngroup(), func() { m.ed.Ungroup() })
	b.text(" ")
	m.snapButton(b)
}

// snapButton is the snap-to-guides checkbox.
func (m *Model) snapButton(b *builder) {
	if m.snapGet == nil {
		return
	}
	mark := "[ ]"
	if m.snapGet() {
		mark = "[x]"
	}
	b.add(theme.Button(mark+" Snap", true), func() { m.snapSet(!m.snapGet()) })
}

// messageInline adds what the last edit said was wrong to the end of a line.
func (m *Model) messageInline(b *builder) {
	if m.message != "" {
		b.text(" " + theme.Fit(m.message, 60))
	}
}

// HandleContext takes a pointer event in the coordinates of the contextual
// bar and reports whether it pressed something.
func (m *Model) HandleContext(e pointer.Event) bool {
	if e.Phase != pointer.Down || !e.Left {
		return false
	}
	for _, r := range m.ctx {
		if r.y == e.Y && e.X >= r.x0 && e.X < r.x1 {
			m.cancelEdit()
			r.act()
			return true
		}
	}
	return false
}

// Package palette is the left bar: the catalog of components, grouped by
// what they are for, from which the designer drags components onto the canvas.
package palette

import (
	"strings"

	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/a11y"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/pointer"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/theme"
	"github.com/meta-ui-labs/cuppa/libs/catalog/definition"
)

// Catalog is the part of the component registry the palette lists.
type Catalog interface {
	Categories() []definition.Category
	ByCategory(definition.Category) []definition.Definition
	Search(query string) []definition.Definition
	Title(definition.Family) string
}

// headerRows is the number of fixed lines above the scrolling list.
const headerRows = 2

type row struct {
	category definition.Category // set on header rows
	def    definition.Definition
	header bool
}

// Model is the palette pane.
type Model struct {
	cat       Catalog
	collapsed map[definition.Category]bool
	rows      []row
	scroll    int
	hover     int // list index under the pointer, -1 if none
	focused   bool
	// cursor is the row the keyboard is on.
	cursor    int
	w, h      int
	query     string
	searching bool
}

// New returns a palette listing every component of cat.
func New(cat Catalog) *Model {
	m := &Model{cat: cat, collapsed: map[definition.Category]bool{}, hover: -1}
	m.rebuild()
	return m
}

// SetCatalog swaps what is listed, e.g. after a pack is switched on or off.
func (m *Model) SetCatalog(cat Catalog) {
	m.cat = cat
	m.hover = -1
	m.rebuild()
	m.clampScroll()
}

// SetSize sets the pane size in cells.
func (m *Model) SetSize(w, h int) {
	m.w, m.h = w, h
	m.clampScroll()
}

// FocusSearch gives the keyboard to the search box, as clicking it does.
func (m *Model) FocusSearch() { m.searching = true }

// EndSearch gives the keyboard back, keeping the query as typed.
func (m *Model) EndSearch() { m.searching = false }

// Searching reports whether the search box is taking keyboard input.
func (m *Model) Searching() bool { return m.searching }

func (m *Model) rebuild() {
	m.rows = m.rows[:0]
	if m.query != "" {
		for _, d := range m.cat.Search(m.query) {
			m.rows = append(m.rows, row{def: d})
		}
		return
	}
	for _, c := range m.cat.Categories() {
		m.rows = append(m.rows, row{category: c, header: true})
		if m.collapsed[c] {
			continue
		}
		for _, d := range m.cat.ByCategory(c) {
			m.rows = append(m.rows, row{def: d})
		}
	}
}

func (m *Model) listHeight() int { return max(m.h-headerRows, 0) }

func (m *Model) clampScroll() {
	m.scroll = min(max(m.scroll, 0), max(len(m.rows)-m.listHeight(), 0))
}

// listIndex maps a pane line to an index into rows, or -1.
func (m *Model) listIndex(y int) int {
	i := y - headerRows + m.scroll
	if y < headerRows || i < 0 || i >= len(m.rows) {
		return -1
	}
	return i
}

// Handle processes a pointer event in pane coordinates. It returns the
// component id when the event starts dragging a component, otherwise "".
func (m *Model) Handle(e pointer.Event) string {
	switch e.Phase {
	case pointer.Wheel:
		m.scroll += e.WheelY * 3
		m.clampScroll()
	case pointer.Move:
		m.hover = m.listIndex(e.Y)
	case pointer.Down:
		if !e.Left {
			return ""
		}
		if e.Y == 1 {
			m.searching = true
			return ""
		}
		i := m.listIndex(e.Y)
		if i < 0 {
			return ""
		}
		r := m.rows[i]
		if r.header {
			m.collapsed[r.category] = !m.collapsed[r.category]
			m.rebuild()
			m.clampScroll()
			return ""
		}
		return r.def.ID
	}
	return ""
}

// Key handles typing into the search box while it has focus.
func (m *Model) Key(text string, backspace, enter, escape bool) {
	switch {
	case escape:
		m.searching = false
		m.query = ""
	case enter:
		m.searching = false
	case backspace:
		if r := []rune(m.query); len(r) > 0 {
			m.query = string(r[:len(r)-1])
		}
	case text == "":
		return
	default:
		m.query += text
	}
	m.rebuild()
	m.scroll, m.cursor = 0, 0
}

// SetFocused marks the pane as the one the keyboard is on; its title shows it.
func (m *Model) SetFocused(on bool) { m.focused = on }

// Leave clears the hover highlight when the pointer leaves the pane.
func (m *Model) Leave() { m.hover = -1 }

// Lines renders the pane as exactly h lines of w cells.
func (m *Model) Lines() []string {
	title := theme.Title(" COMPONENTS")
	if m.focused {
		title = theme.Selected(" COMPONENTS ")
	}
	lines := []string{title}
	search := " ⌕ search components"
	switch {
	case m.searching:
		search = " ⌕ " + m.query + "█"
	case m.query != "":
		search = " ⌕ " + m.query
	}
	lines = append(lines, theme.Faded(search))
	end := min(m.scroll+m.listHeight(), len(m.rows))
	for i := m.scroll; i < end; i++ {
		lines = append(lines, m.renderRow(i))
	}
	if len(m.rows) == 0 {
		lines = append(lines, theme.Faded(" no matches"))
	}
	return theme.Block(lines, m.w, m.h)
}

func (m *Model) renderRow(i int) string {
	r := m.rows[i]
	var s string
	switch {
	case r.header:
		mark := "▾"
		if m.collapsed[r.category] {
			mark = "▸"
		}
		s = " " + theme.Bold(mark+" "+r.category.Title())
	default:
		suffix := ""
		if r.def.Status == definition.StatusPlaceholder {
			suffix = " ~"
		}
		indent := "   "
		if m.query != "" {
			indent = " "
		}
		s = indent + r.def.Name + suffix + " " + theme.Faded(strings.ToLower(m.cat.Title(r.def.Family)))
	}
	if m.focused && i == m.cursor {
		return theme.Selected(theme.Fit(s, m.w))
	}
	if i == m.hover {
		return theme.Hovered(theme.Fit(s, m.w))
	}
	return s
}

// rowText is how a row is read out.
func (m *Model) rowText(r row) string {
	switch {
	case r.header && m.collapsed[r.category]:
		return r.category.Title() + ", category, folded"
	case r.header:
		return r.category.Title() + ", category"
	}
	name := r.def.Name + ", " + m.cat.Title(r.def.Family)
	if r.def.Status == definition.StatusPlaceholder {
		name += ", approximate preview"
	}
	return name
}

// Current is the row under the keyboard cursor, as it is read out.
func (m *Model) Current() string {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return ""
	}
	return m.rowText(m.rows[m.cursor])
}

// Describe lists the components by category, with folded categories said as such.
func (m *Model) Describe() []a11y.Node {
	label := "Components"
	if m.query != "" {
		label = "Components matching " + m.query
	}
	var items []a11y.Node
	for i, r := range m.rows {
		item := a11y.Item(m.rowText(r), false)
		item.Focused = m.focused && i == m.cursor
		items = append(items, item)
	}
	return []a11y.Node{a11y.Heading("Components"), a11y.List(label, items...)}
}

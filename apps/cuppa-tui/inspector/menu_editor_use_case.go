package inspector

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/pointer"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/theme"
	"github.com/meta-ui-labs/cuppa/libs/catalog/definition"
	"github.com/meta-ui-labs/cuppa/libs/catalog/menuvalue"
	"github.com/meta-ui-labs/cuppa/libs/document/design"
)

// The rows of a menu property that are not an entry: entryHeading is the
// heading of a menu, entryAdd the row with the add buttons under its entries,
// and menuAdd the last row of the list, which adds a menu.
const (
	entryHeading = -1
	entryAdd     = -2
	menuAdd      = -3
)

// menuRef names a row of a menu property: a heading (entry is entryHeading),
// an entry of a menu, or one of the add rows.
type menuRef struct {
	node  design.NodeID
	key   string
	menu  int
	entry int
}

// menuDrag is a heading or an entry held by its grip. Moving the pointer to
// another row starts the drag and shows where it would land; letting go there
// moves it, and it may go into another menu.
type menuDrag struct {
	ref    menuRef
	startY int
	moved  bool
	// over is the row the pointer is on, when it is on one.
	over    menuRef
	hasOver bool
}

const (
	menuGrip   = "≡"
	menuMarker = "▸"
)

// menuProp is the editor of a menu property: each menu is a heading with its
// entries under it in two columns, the name and the shortcut, and rows that add
// an entry, a separator or a menu. The grip at the start of a row is held to
// drag it to another place, into another menu if it is an entry.
//
//	▾ ≡ File                    [x]
//	    ≡ New        Ctrl+N     [x]
//	    ≡ ───────────────────   [x]
//	    ≡ Quit       Ctrl+Q     [x]
//	    [+ Entry] [+ Separator]
//	  [+ Menu]
func (m *Model) menuProp(b *builder, n design.Node, p definition.PropSpec, value string) {
	menus, err := menuvalue.Parse(value)
	if err != nil {
		b.text("  " + theme.Faded("(not a list of menus)")).end()
		return
	}
	key, width := p.Key, m.content()
	text := func(menu, entry int, col string) string {
		return fmt.Sprintf("menutext:%s:%d:%d:%s", key, menu, entry, col)
	}
	// cell is a text that turns into a box when pressed: w cells wide.
	cell := func(field, shown, initial string, w int, style func(string) string) {
		if m.editing == field {
			r := []rune(m.buf)
			if len(r) > w-1 {
				r = r[len(r)-(w-1):]
			}
			if m.fresh && len(r) > 0 {
				b.text(theme.Selected(theme.Fit(string(r), w)))
				return
			}
			b.text(theme.Fit(string(r)+"█", w))
			return
		}
		b.add(theme.Fit(style(shown), w), func() { m.startEdit(field, initial) })
	}
	grip := func(ref menuRef, dim bool) {
		style := theme.Dim
		if dim {
			style = theme.Faded
		}
		b.add(style(menuGrip), func() {})
		ref2 := ref
		b.regions[len(b.regions)-1].menu = &ref2
		b.text(" ")
	}
	marker := func(ref menuRef) string {
		if d := m.mdrag; d != nil && d.moved && d.hasOver && d.over == ref {
			return theme.Title(menuMarker)
		}
		return " "
	}
	held := func(ref menuRef) bool {
		return m.mdrag != nil && m.mdrag.moved && m.mdrag.ref == ref
	}
	row := func(ref menuRef) { m.menuRows[len(b.lines)] = ref }

	// The two columns of an entry, named once above the list.
	left := max(width-5-4, 6)
	short := min(max(left/3, 6), 14)
	name := max(left-short-1, 4)
	b.text(strings.Repeat(" ", 5) + theme.Faded(theme.Fit("Name", name)+" "+"Shortcut")).end()

	for i, menu := range menus {
		hr := menuRef{n.ID, key, i, entryHeading}
		row(hr)
		b.text(marker(hr))
		grip(hr, held(hr))
		rest := max(width-1-2-4, 4)
		style := theme.Bold
		if held(hr) {
			style = theme.Faded
		}
		cell(text(i, entryHeading, "name"), menu.Name, menu.Name, rest, style)
		b.text(" ").addMouse(theme.Button("[x]", true), func() { m.removeMenu(n.ID, key, i) })
		b.end()

		for j, e := range menu.Entries {
			er := menuRef{n.ID, key, i, j}
			row(er)
			b.text(marker(er) + "  ")
			grip(er, held(er))
			if e.Separator {
				b.text(theme.Faded(strings.Repeat("─", left)))
			} else {
				entryStyle := plain
				if held(er) {
					entryStyle = theme.Faded
				}
				cell(text(i, j, "name"), e.Name, e.Name, name, entryStyle)
				b.text(" ")
				if e.Shortcut == "" {
					cell(text(i, j, "shortcut"), "–", "", short, theme.Faded)
				} else {
					cell(text(i, j, "shortcut"), e.Shortcut, e.Shortcut, short, theme.Dim)
				}
			}
			b.text(" ").addMouse(theme.Button("[x]", true), func() { m.removeEntry(n.ID, key, i, j) })
			b.end()
		}

		ar := menuRef{n.ID, key, i, entryAdd}
		row(ar)
		b.text(marker(ar) + "    ")
		sep := "[+ Separator]"
		if width < 5+len("[+ Entry]")+1+len(sep) {
			sep = "[+ ─]"
		}
		b.add(theme.Button("[+ Entry]", true), func() { m.addEntry(n.ID, key, i) }).text(" ")
		b.add(theme.Button(sep, true), func() { m.addSeparator(n.ID, key, i) })
		b.end()
	}
	mr := menuRef{n.ID, key, len(menus), menuAdd}
	row(mr)
	b.text(marker(mr) + " ").add(theme.Button("[+ Menu]", true), func() { m.addMenu(n.ID, key) })
	b.end()
}

// menusOf is the current value of a menu property of a node.
func (m *Model) menusOf(id design.NodeID, key string) (menuvalue.Menus, bool) {
	n, ok := m.ed.Document().Get(id)
	if !ok {
		return nil, false
	}
	value, set := n.Props[key]
	if !set {
		if def, ok := m.cat.Get(n.Component); ok {
			if spec, ok := def.Prop(key); ok {
				value = spec.Default
			}
		}
	}
	menus, err := menuvalue.Parse(value)
	return menus, err == nil
}

// setMenus stores a menu property, and says what is wrong when it is refused.
func (m *Model) setMenus(id design.NodeID, key string, menus menuvalue.Menus) {
	m.report(m.ed.SetProp(id, key, menus.Encode()))
}

func (m *Model) addMenu(id design.NodeID, key string) {
	menus, ok := m.menusOf(id, key)
	if !ok {
		return
	}
	m.setMenus(id, key, menus.AddMenu(""))
	m.startEdit(fmt.Sprintf("menutext:%s:%d:%d:name", key, len(menus), entryHeading), "")
}

func (m *Model) removeMenu(id design.NodeID, key string, i int) {
	if menus, ok := m.menusOf(id, key); ok {
		m.cancelEdit()
		m.setMenus(id, key, menus.RemoveMenu(i))
	}
}

func (m *Model) addEntry(id design.NodeID, key string, i int) {
	menus, ok := m.menusOf(id, key)
	if !ok || i >= len(menus) {
		return
	}
	at := len(menus[i].Entries)
	m.setMenus(id, key, menus.AddEntry(i, menuvalue.Entry{Name: "New entry"}))
	m.startEdit(fmt.Sprintf("menutext:%s:%d:%d:name", key, i, at), "")
}

func (m *Model) addSeparator(id design.NodeID, key string, i int) {
	if menus, ok := m.menusOf(id, key); ok {
		m.setMenus(id, key, menus.AddSeparator(i))
	}
}

func (m *Model) removeEntry(id design.NodeID, key string, i, j int) {
	if menus, ok := m.menusOf(id, key); ok {
		m.cancelEdit()
		m.setMenus(id, key, menus.RemoveEntry(i, j))
	}
}

// commitMenuText applies what was typed into a menu name, an entry name or a
// shortcut. The field is "menutext:<key>:<menu>:<entry>:<name|shortcut>".
func (m *Model) commitMenuText(id design.NodeID, field, value string) {
	parts := strings.Split(strings.TrimPrefix(field, "menutext:"), ":")
	if len(parts) != 4 {
		return
	}
	key, col := parts[0], parts[3]
	menu, err1 := strconv.Atoi(parts[1])
	entry, err2 := strconv.Atoi(parts[2])
	menus, ok := m.menusOf(id, key)
	if !ok || err1 != nil || err2 != nil || menu < 0 || menu >= len(menus) {
		return
	}
	var (
		out menuvalue.Menus
		err error
	)
	switch {
	case entry == entryHeading:
		out, err = menus.RenameMenu(menu, value)
	case entry >= 0 && entry < len(menus[menu].Entries):
		cur := menus[menu].Entries[entry]
		name, shortcut := cur.Name, cur.Shortcut
		if col == "shortcut" {
			shortcut = value
		} else {
			name = value
		}
		out, err = menus.SetEntry(menu, entry, name, shortcut)
	default:
		return
	}
	if err != nil {
		m.message = "A name is needed"
		return
	}
	m.setMenus(id, key, out)
}

// menuRowAt is the row of a menu property under bar row y. Above or below the
// panel it is the first or the last row, so a drag can be taken to either end.
func (m *Model) menuRowAt(y int) (menuRef, bool) {
	v, ok := m.propertiesView()
	if !ok || len(m.menuRows) == 0 {
		return menuRef{}, false
	}
	cy, on := m.contentY(y)
	if !on || y <= v.top || y > v.top+v.h {
		first, last := -1, -1
		for line := range m.menuRows {
			if first < 0 || line < first {
				first = line
			}
			last = max(last, line)
		}
		switch {
		case y <= v.top:
			cy = first
		case y > v.top+v.h:
			cy = last
		}
	}
	ref, ok := m.menuRows[cy]
	return ref, ok
}

// propertiesView is the Properties panel as it was last drawn.
func (m *Model) propertiesView() (panelView, bool) {
	for _, v := range m.views {
		if v.id == panelProperties && v.open {
			return v, true
		}
	}
	return panelView{}, false
}

// pressMenu starts watching for a drag of the row whose grip was pressed.
func (m *Model) pressMenu(ref menuRef, y int) {
	m.mdrag = &menuDrag{ref: ref, startY: y}
}

// dragMenu follows the pointer while a grip is held, showing the row the
// heading or entry would land on, and scrolls the panel when the pointer is
// above or under it.
func (m *Model) dragMenu(e pointer.Event) {
	d := m.mdrag
	if e.Y != d.startY {
		d.moved = true
	}
	if !d.moved {
		return
	}
	if v, ok := m.propertiesView(); ok {
		switch {
		case e.Y <= v.top:
			m.scrollPanel(v, -1)
		case e.Y > v.top+v.h:
			m.scrollPanel(v, 1)
		}
	}
	if ref, ok := m.menuRowAt(e.Y); ok && ref.key == d.ref.key && ref.node == d.ref.node {
		d.over, d.hasOver = ref, true
	}
}

// dropMenu ends the drag and moves the heading or entry to where it was let go.
// A press that never moved did nothing.
func (m *Model) dropMenu(e pointer.Event) {
	d := m.mdrag
	m.mdrag = nil
	if d == nil || !d.moved {
		return
	}
	if ref, ok := m.menuRowAt(e.Y); ok && ref.key == d.ref.key && ref.node == d.ref.node {
		d.over, d.hasOver = ref, true
	}
	if !d.hasOver {
		return
	}
	menus, ok := m.menusOf(d.ref.node, d.ref.key)
	if !ok {
		return
	}
	from, to := d.ref, d.over
	if from.entry == entryHeading {
		target := min(to.menu, len(menus)-1)
		m.setMenus(from.node, from.key, menus.MoveMenu(from.menu, target))
		return
	}
	toMenu, at := to.menu, to.entry
	switch to.entry {
	case entryHeading:
		at = 0
	case entryAdd:
		at = len(menus[toMenu].Entries)
	case menuAdd:
		toMenu = len(menus) - 1
		at = len(menus[toMenu].Entries)
	}
	if toMenu < 0 {
		return
	}
	// Dropped on its own place, or just below it: nothing moves.
	m.setMenus(from.node, from.key, menus.MoveEntry(from.menu, from.entry, toMenu, at))
}

// menuKey handles the keys of a menu row for the grip under the stop: Alt+Up
// and Alt+Down move it (an entry crosses into the next menu at the edge of its
// own) and Delete removes it.
func (m *Model) menuKey(name string, ref menuRef) bool {
	menus, ok := m.menusOf(ref.node, ref.key)
	if !ok {
		return false
	}
	dir := 0
	switch name {
	case "alt+up":
		dir = -1
	case "alt+down":
		dir = 1
	case "delete":
		if ref.entry == entryHeading {
			m.removeMenu(ref.node, ref.key, ref.menu)
		} else {
			m.removeEntry(ref.node, ref.key, ref.menu, ref.entry)
		}
		return true
	default:
		return false
	}
	if ref.entry == entryHeading {
		to := min(max(ref.menu+dir, 0), len(menus)-1)
		m.setMenus(ref.node, ref.key, menus.MoveMenu(ref.menu, to))
		f := menuRef{ref.node, ref.key, to, entryHeading}
		m.follow = &f
		return true
	}
	out, toMenu, to := menus.NudgeEntry(ref.menu, ref.entry, dir)
	m.setMenus(ref.node, ref.key, out)
	f := menuRef{ref.node, ref.key, toMenu, to}
	m.follow = &f
	return true
}

// followMenu puts the keyboard stop back on the grip of the row that was moved
// with the keyboard, which is somewhere else in the list now.
func (m *Model) followMenu() {
	f := m.follow
	if f == nil {
		return
	}
	m.follow = nil
	for i, r := range m.stopRegions() {
		if r.menu != nil && *r.menu == *f {
			m.stop = i
			return
		}
	}
}

// describeMenus is the menus as they are read out: "File: New, Ctrl+N; Open".
func describeMenus(value string) string {
	menus, err := menuvalue.Parse(value)
	if err != nil || len(menus) == 0 {
		return "none"
	}
	var parts []string
	for _, mn := range menus {
		var entries []string
		for _, e := range mn.Entries {
			switch {
			case e.Separator:
				entries = append(entries, "separator")
			case e.Shortcut != "":
				entries = append(entries, e.Name+" "+ansi.Strip(e.Shortcut))
			default:
				entries = append(entries, e.Name)
			}
		}
		parts = append(parts, mn.Name+": "+strings.Join(entries, ", "))
	}
	return strings.Join(parts, "; ")
}

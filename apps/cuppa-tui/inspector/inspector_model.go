// Package inspector is the right bar: details of the selection (position,
// size, layer order, properties) and the layer list.
package inspector

import (
	"strconv"
	"strings"

	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/pointer"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/theme"
	"github.com/meta-ui-labs/cuppa/libs/canvas/editor"
	"github.com/meta-ui-labs/cuppa/libs/catalog/definition"
	"github.com/meta-ui-labs/cuppa/libs/document/design"
)

// Catalog is what the inspector needs to know about components.
type Catalog interface {
	Get(id string) (definition.Definition, bool)
}

// region is a clickable span of one content line.
type region struct {
	y, x0, x1 int
	act       func()
	// step changes the value by one in a direction (-1 or +1), for Left and
	// Right on the keyboard stop; mouseOnly spans are not keyboard stops.
	step      func(delta int)
	mouseOnly bool
	// label is the stop as it is read out: the text before it on its line and
	// its own text.
	label string
	// layer is set on a layer name: pressing it can start a drag of that layer.
	layer design.NodeID
	// header is set on a panel's header: a click opens or closes the panel and
	// a drag moves the edge above it.
	header panelID
	// ctx is set on a span of the contextual bar, which the keyboard reaches
	// before the panels.
	ctx bool
	// menu is set on the grip of a row of a menu property: pressing it can
	// start a drag of that heading or entry.
	menu *menuRef
}

// Model is the inspector pane.
type Model struct {
	ed      *editor.Editor
	cat     Catalog
	w, h    int
	focused bool
	regions []region
	// ctx are the spans of the contextual bar (line 0 or 1) and ctxStops how
	// many keyboard stops they are.
	ctx      []region
	ctxStops int
	// stop is the keyboard stop that has the focus and stops how many there
	// were at the last render.
	stop, stops int

	editing string // field being typed into: "x", "y", "w", "h", "name" or "prop:<key>"
	buf     string
	pos     int  // the text cursor, as a rune index into buf
	fresh   bool // the shown value is still selected: the next key replaces it
	message string

	// previewing reads whether the design is being run, and setPreview starts
	// or stops it: the Editor and Preview buttons of the title row.
	previewing func() bool
	setPreview func(on bool)
	// modeX is where each button is on the title row: Editor then Preview.
	modeX [2][2]int

	// snap reads and changes the stage setting this pane exposes as a checkbox.
	snapGet func() bool
	snapSet func(bool)

	// pickColor opens the colour dialog; nil means colours are edited as text.
	pickColor ColorPicker
	// pickTheme opens the dialog that fills the theme from a colour scheme.
	pickTheme ThemePicker

	// panels is what each panel remembers, views where they were drawn at the
	// last render, press the header being held, and lastSel the layer that was
	// selected then (a new selection is scrolled into view in the layer list).
	panels  map[panelID]*panelState
	views   []panelView
	press   *headerPress
	lastSel design.NodeID
	// hover is the panel whose header the pointer is on: its rule lights up,
	// as the dividers between the panes do.
	hover panelID
	// menu is the dropdown that is open, if any.
	menu *dropdown

	// The layer list: a drag in progress, how long the list is (set at render),
	// and where the trash button is (columns; its row is the Layers header).
	drag             *layerDrag
	layerCount       int
	layerIDs         []design.NodeID
	trashX0, trashX1 int

	// The rows of a menu property as they were drawn, by content line, and
	// the grip being dragged. follow is the row to put the keyboard stop back
	// on after it was moved with the keys.
	menuRows map[int]menuRef
	mdrag    *menuDrag
	follow   *menuRef
}

// New returns an inspector editing through ed.
func New(ed *editor.Editor, cat Catalog) *Model { return &Model{ed: ed, cat: cat} }

// BindPreview connects the Editor and Preview buttons: get says whether the
// design is being run, set starts or stops it.
func (m *Model) BindPreview(get func() bool, set func(on bool)) {
	m.previewing, m.setPreview = get, set
}

// HandleMode takes a pointer event in pane coordinates while the design is
// being run, when the bar is otherwise idle: only the Editor button works. It
// reports whether it was pressed.
func (m *Model) HandleMode(e pointer.Event) bool {
	if e.Phase != pointer.Down || !e.Left || e.Y != 0 || m.setPreview == nil {
		return false
	}
	for i, x := range m.modeX {
		if e.X >= x[0] && e.X < x[1] {
			m.setPreview(i == 1)
			return true
		}
	}
	return false
}

// BindSnap connects the "snap to guides" checkbox to the stage setting.
func (m *Model) BindSnap(get func() bool, set func(bool)) { m.snapGet, m.snapSet = get, set }

// SetSize sets the pane size in cells.
func (m *Model) SetSize(w, h int) { m.w, m.h = w, h }

// ColorPicker opens a colour dialog for a property: it shows title with the
// current colour selected, and calls apply with the chosen one ("" for none).
// The shell provides it.
type ColorPicker func(title, current string, apply func(color string))

// ThemePicker opens the dialog that fills the theme from a colour scheme and
// calls apply with the background and the theme colours it chose. The shell
// provides it.
type ThemePicker func(apply func(background string, t design.Theme))

// BindThemePicker sets how the Theme section's scheme button opens the dialog.
// Without it the button is not shown.
func (m *Model) BindThemePicker(p ThemePicker) { m.pickTheme = p }

// BindColorPicker sets how clicking a colour opens the picker. Without it a
// click edits the colour as text.
func (m *Model) BindColorPicker(p ColorPicker) { m.pickColor = p }

// Editing reports whether a field is taking keyboard input.
// An open dropdown takes the keys too: Up and Down move through its options,
// Enter chooses and Esc closes it.
func (m *Model) Editing() bool { return m.editing != "" || m.menu != nil }

// Handle processes a pointer event in pane coordinates.
func (m *Model) Handle(e pointer.Event) {
	if m.menu != nil {
		m.handleMenu(e)
		return
	}
	switch e.Phase {
	case pointer.Wheel:
		// The wheel scrolls the panel under the pointer, not the whole bar.
		if v, ok := m.viewAt(e.Y); ok {
			m.scrollPanel(v, e.WheelY*2)
		}
	case pointer.Down:
		if !e.Left {
			return
		}
		cy, ok := m.contentY(e.Y)
		if ok {
			for _, r := range m.regions {
				if r.y == cy && e.X >= r.x0 && e.X < r.x1 {
					m.cancelEdit()
					switch {
					case r.layer != "":
						// Selecting changes what the bar shows above the list, which
						// would slide the list out from under a drag; so a layer is
						// selected on release, when it was not dragged.
						m.pressLayer(r.layer, e.Y, r.act)
					case r.header != "":
						m.pressHeader(r.header, e.Y, r.act)
					case r.menu != nil:
						m.pressMenu(*r.menu, e.Y)
					default:
						r.act()
					}
					return
				}
			}
		}
		m.cancelEdit()
	case pointer.Move:
		switch {
		case m.press != nil && e.Held:
			m.dragHeader(e)
		case m.drag != nil && e.Held:
			m.dragTo(e)
		case m.mdrag != nil && e.Held:
			m.dragMenu(e)
		default:
			m.hover = ""
			if v, ok := m.viewAt(e.Y); ok && e.Y == v.top {
				m.hover = v.id
			}
		}
	case pointer.Up:
		if m.press != nil {
			m.releaseHeader()
		}
		if m.drag != nil {
			m.drop(e)
		}
		if m.mdrag != nil {
			m.dropMenu(e)
		}
	}
}

// Leave tells the bar the pointer went to another pane.
func (m *Model) Leave() { m.hover = "" }

// Key handles typing into the field being edited.
func (m *Model) Key(text string, backspace, enter, escape bool) {
	if m.menu != nil {
		switch {
		case escape:
			m.menu = nil
		case enter || text == " ":
			m.chooseMenu(m.menu.at)
		}
		return
	}
	switch {
	case escape:
		m.cancelEdit()
	case enter:
		m.commit()
	case backspace:
		// The shown value starts out selected: Backspace clears all of it.
		r := []rune(m.buf)
		if m.fresh || m.pos == 0 {
			if m.fresh {
				m.buf, m.pos = "", 0
			}
		} else {
			m.buf, m.pos = string(r[:m.pos-1])+string(r[m.pos:]), m.pos-1
		}
		m.fresh = false
	case text != "":
		// The first character typed replaces the shown value.
		if m.fresh {
			m.buf, m.pos = "", 0
		}
		r := []rune(m.buf)
		t := []rune(text)
		m.buf, m.pos = string(r[:m.pos])+text+string(r[m.pos:]), m.pos+len(t)
		m.fresh = false
	}
}

// MoveCursor takes Left, Right, Home, End and Delete while a field is edited,
// and reports whether it used the key. With the value still selected, Left and
// Home go to its start and Right and End to its end, and Delete clears it.
func (m *Model) MoveCursor(name string) bool {
	if m.editing == "" || m.menu != nil {
		return false
	}
	r := []rune(m.buf)
	switch name {
	case "left":
		if m.fresh {
			m.pos = 0
		} else {
			m.pos = max(m.pos-1, 0)
		}
	case "right":
		if m.fresh {
			m.pos = len(r)
		} else {
			m.pos = min(m.pos+1, len(r))
		}
	case "home":
		m.pos = 0
	case "end":
		m.pos = len(r)
	case "delete":
		switch {
		case m.fresh:
			m.buf, m.pos = "", 0
		case m.pos < len(r):
			m.buf = string(r[:m.pos]) + string(r[m.pos+1:])
		}
	default:
		return false
	}
	m.fresh = false
	return true
}

// shown is the text being edited with its cursor, or highlighted while the
// next key will replace all of it.
func (m *Model) shown() string {
	if m.fresh && m.buf != "" {
		return theme.Selected(m.buf)
	}
	r := []rune(m.buf)
	m.pos = min(max(m.pos, 0), len(r))
	if m.pos == len(r) {
		return m.buf + "█"
	}
	return string(r[:m.pos]) + theme.Selected(string(r[m.pos])) + string(r[m.pos+1:])
}

func (m *Model) startEdit(field, initial string) {
	m.editing, m.buf, m.message, m.fresh = field, initial, "", true
	m.pos = len([]rune(initial))
}

func (m *Model) cancelEdit() { m.editing, m.buf, m.fresh = "", "", false }

func (m *Model) commit() {
	field, value := m.editing, m.buf
	m.cancelEdit()
	m.apply(field, value)
}

// Step adds delta to the number being edited and applies it at once, so the
// design follows the arrow keys; Enter then ends the edit. It reports whether
// the field holds a number.
func (m *Model) Step(delta int) bool {
	if m.menu != nil {
		// Up (a positive step) goes to the option above.
		if delta > 0 {
			m.menu.move(-1)
		} else if delta < 0 {
			m.menu.move(1)
		}
		return true
	}
	if m.editing == "" || m.editing == "background" || m.editing == "name" || strings.HasPrefix(m.editing, "menutext:") {
		return false
	}
	n, err := strconv.Atoi(strings.TrimSpace(m.buf))
	if err != nil {
		return false
	}
	m.buf = strconv.Itoa(n + delta)
	m.pos, m.fresh = len(m.buf), false
	m.apply(m.editing, m.buf)
	return true
}

// apply sets the field to value. The field is "background", "canvas-…",
// "name", "prop:key" or one of the geometry fields.
func (m *Model) apply(field, value string) {
	switch {
	case field == "background":
		m.report(m.ed.SetBackground(value))
		return
	case field == "swatch-new":
		m.report(m.ed.SetSwatch(value, "#ffffff"))
		return
	case strings.HasPrefix(field, "swatch:"):
		m.report(m.ed.SetSwatch(strings.TrimPrefix(field, "swatch:"), value))
		return
	case strings.HasPrefix(field, "theme-"):
		m.report(m.ed.SetThemeColor(strings.TrimPrefix(field, "theme-"), value))
		return
	case strings.HasPrefix(field, "pagename:"):
		if i, err := strconv.Atoi(strings.TrimPrefix(field, "pagename:")); err == nil {
			m.report(m.ed.RenamePage(i, value))
		}
		return
	case strings.HasPrefix(field, "canvas-"):
		m.commitCanvas(field, value)
		return
	}
	n, ok := m.ed.Primary()
	if !ok {
		return
	}
	switch {
	case field == "name":
		m.ed.Rename(n.ID, value)
	case strings.HasPrefix(field, "menutext:"):
		m.commitMenuText(n.ID, field, value)
	case strings.HasPrefix(field, "swatchname:"):
		key := strings.TrimPrefix(field, "swatchname:")
		current, _ := strings.CutPrefix(n.Props[key], design.TokenPrefix)
		m.nameSwatch(n.ID, key, current, value)
	case strings.HasPrefix(field, "prop:"):
		key := strings.TrimPrefix(field, "prop:")
		if def, ok := m.cat.Get(n.Component); ok {
			if spec, ok := def.Prop(key); ok && spec.Kind == definition.PropColor && !strings.HasPrefix(strings.TrimSpace(value), design.TokenPrefix) {
				m.report(m.ed.SetColour(n.ID, key, value))
				return
			}
		}
		m.report(m.ed.SetProp(n.ID, key, value))
	default:
		value = strings.TrimSpace(value)
		v, err := strconv.Atoi(value)
		switch {
		case err != nil:
			// Not a plain number: an expression such as 50% or 100% - 10.
			m.report(m.ed.SetLayout(n.ID, field, value))
		case layoutOf(m.ed.Layout(n.ID), field) != "":
			// A number typed over an expression makes the axis fixed.
			m.report(m.ed.SetLayout(n.ID, field, value))
		default:
			m.setGeometry(n, field, v)
		}
	}
}

func (m *Model) report(err error) {
	m.message = ""
	if err != nil {
		m.message = strings.TrimPrefix(err.Error(), "editor: ")
	}
}

func (m *Model) setGeometry(n design.Node, field string, v int) {
	r := n.Rect
	switch field {
	case "x":
		r.X = v
	case "y":
		r.Y = v
	case "w":
		r.W = v
	case "h":
		r.H = v
	}
	m.ed.SetRect(n.ID, r, true)
}

func (m *Model) nudge(field string, delta int) {
	n, ok := m.ed.Primary()
	if !ok {
		return
	}
	switch field {
	case "x":
		m.setGeometry(n, field, n.Rect.X+delta)
	case "y":
		m.setGeometry(n, field, n.Rect.Y+delta)
	case "w":
		m.setGeometry(n, field, n.Rect.W+delta)
	case "h":
		m.setGeometry(n, field, n.Rect.H+delta)
	}
}

func layoutOf(l design.Layout, field string) string {
	switch field {
	case "x":
		return l.X
	case "y":
		return l.Y
	case "w":
		return l.W
	case "h":
		return l.H
	}
	return ""
}

// nameSwatch sets the name of the palette colour a property uses: a name the
// palette has makes the property use that colour, another renames the colour
// everywhere.
func (m *Model) nameSwatch(id design.NodeID, key, current, to string) {
	to = design.CleanSwatchName(to)
	if to == "" || to == current {
		return
	}
	if _, ok := m.ed.Document().Theme.Colour(to); ok {
		m.report(m.ed.UseSwatch(id, key, to))
		return
	}
	m.report(m.ed.RenameSwatch(current, to))
}

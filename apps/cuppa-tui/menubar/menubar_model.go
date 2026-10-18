// Package menubar is the File / Edit / Export / Help bar along the top and the
// dropdowns under it. It only reports which action was chosen.
package menubar

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/a11y"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/pointer"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/theme"
)

const (
	brand    = " ☕ Cuppa "
	// iconBrand is the brand with the logo in place of the teacup character:
	// two private-use characters, the left and right half of the cup, that the
	// desktop app and the web page draw from a font they ship. They are one
	// cell each, so the bar keeps its width.
	// the two halves meet with no seam, like the box-drawing rules of the bar.
	iconBrand = " \ue000\ue001 Cuppa "
	gap      = 1
	itemPadX = 2
)

// Model is the bar. Row 0 of its own coordinates is the bar, the dropdown
// hangs below from row 1.
type Model struct {
	w int
	// open is the index of the open menu, or -1.
	open int
	// hover is the dropdown item under the pointer, or -1.
	hover int
	// hoverBar is the bar label under the pointer, or -1.
	hoverBar int
	disabled map[Action]bool
	// checked holds the menu items that are switches: present when it is one,
	// true when it is on.
	checked map[Action]bool
	// unavailable items look greyed out but still report clicks, so the
	// caller can explain why they do not work.
	unavailable map[Action]bool
	labelX   []int // left column of each label
	// command shows Cmd where the shortcuts say Ctrl (on a Mac).
	command bool
	// iconFont draws the logo from the icon font instead of the teacup character.
	iconFont bool
	// focused is true while the keyboard is on the bar; cursor is the menu it
	// is on. A dropdown that is open always has the cursor on it.
	focused bool
	cursor  int
}

// New returns a closed bar.
func New() *Model {
	return &Model{open: -1, hover: -1, hoverBar: -1, disabled: map[Action]bool{}, checked: map[Action]bool{}, unavailable: map[Action]bool{}}
}

// SetIconFont draws the logo in the bar from the Cuppa icon font. Only a host
// that ships the font (the desktop app, the web page) may ask for it; in a
// plain terminal the characters would show as empty boxes.
func (m *Model) SetIconFont(on bool) { m.iconFont = on }

// brandText is the brand as drawn.
func (m *Model) brandText() string {
	if m.iconFont {
		return iconBrand
	}
	return brand
}

// styledBrand is the brand with the logo in its own green and the name in the title style.
func (m *Model) styledBrand() string {
	if !m.iconFont {
		return theme.Title(brand)
	}
	return " " + theme.Logo("\ue000\ue001") + theme.Title(" Cuppa ")
}

// SetCommandKey makes the menus say Cmd where the shortcuts say Ctrl.
func (m *Model) SetCommandKey(on bool) { m.command = on }

// keys is a shortcut as shown: Ctrl+S, or Cmd+S on a Mac.
func (m *Model) keys(shortcut string) string {
	if m.command {
		return strings.ReplaceAll(shortcut, "Ctrl", "Cmd")
	}
	return shortcut
}

// SetWidth sets the screen width.
func (m *Model) SetWidth(w int) { m.w = w }

// SetEnabled greys out (or restores) one action.
func (m *Model) SetEnabled(a Action, enabled bool) { m.disabled[a] = !enabled }

// SetChecked makes the item a switch and says whether it is on; the menu draws
// a tick in front of it while it is.
func (m *Model) SetChecked(a Action, on bool) { m.checked[a] = on }

// labelOf is the item's label, with the tick of a switch that is on.
func (m *Model) labelOf(it item) string {
	on, isSwitch := m.checked[it.action]
	switch {
	case on:
		return "✓ " + it.label
	case isSwitch:
		return "  " + it.label
	}
	return it.label
}

// SetUnavailable greys out an action that still reports clicks.
func (m *Model) SetUnavailable(a Action, unavailable bool) { m.unavailable[a] = unavailable }

// Open reports whether a dropdown is showing.
func (m *Model) Open() bool { return m.open >= 0 }

// Close hides the dropdown.
func (m *Model) Close() { m.open, m.hover = -1, -1 }

// Current is what the keyboard is on, as it is read out: the highlighted item
// of the open menu, or the menu's name.
func (m *Model) Current() string {
	if m.open >= 0 && m.hover >= 0 {
		it := menus[m.open].items[m.hover]
		label := it.label
		if on, isSwitch := m.checked[it.action]; isSwitch {
			label += map[bool]string{true: ", on", false: ", off"}[on]
		}
		if it.shortcut != "" {
			label += ", " + m.keys(it.shortcut)
		}
		return label
	}
	if m.open >= 0 {
		return menus[m.open].label + " menu"
	}
	if m.focused {
		return menus[m.cursor].label + " menu"
	}
	return ""
}

// Focused reports whether the keyboard is on the bar.
func (m *Model) Focused() bool { return m.focused }

// Focus puts the keyboard on the bar (on the menu it was last on), or takes
// it off and closes the dropdown.
func (m *Model) Focus(on bool) {
	m.focused = on
	if !on {
		m.Close()
	}
}

// OpenMnemonic opens the menu whose Alt letter is r (Alt+F for File) and
// reports whether there is one.
func (m *Model) OpenMnemonic(r rune) bool {
	for i, mn := range menus {
		if mn.mnemonic == r {
			m.focused = true
			m.openMenu(i)
			return true
		}
	}
	return false
}

// OpenFirst opens the first menu, as F10 does.
func (m *Model) OpenFirst() {
	m.focused = true
	m.openMenu(m.cursor)
}

func (m *Model) openMenu(i int) {
	m.cursor, m.open = i, i
	m.hover = m.step(-1, 1)
}

// step is the next dropdown item from index from in direction dir that can be
// chosen: separators and disabled items are skipped, and it wraps. It is -1
// when there is none.
func (m *Model) step(from, dir int) int {
	items := menus[m.open].items
	n := len(items)
	for k := 1; k <= n; k++ {
		i := ((from+dir*k)%n + n) % n
		it := items[i]
		if it.label != separatorLabel && !m.disabled[it.action] {
			return i
		}
	}
	return -1
}

// Key takes a key while the bar has the keyboard: left and right change menu,
// down opens it (and moves through it), up moves back, home and end jump,
// enter runs the item or opens the menu, esc closes the dropdown and then
// gives the keyboard back. It returns the chosen action and whether the key
// was used.
func (m *Model) Key(name string) (Action, bool) {
	if !m.focused && !m.Open() {
		return nothing, false
	}
	m.focused = true
	switch name {
	case "left", "right":
		dir := 1
		if name == "left" {
			dir = -1
		}
		next := ((m.cursor+dir)%len(menus) + len(menus)) % len(menus)
		if m.Open() {
			m.openMenu(next)
		} else {
			m.cursor = next
		}
	case "down", "up":
		if !m.Open() {
			if name == "down" {
				m.openMenu(m.cursor)
			}
			break
		}
		dir := 1
		if name == "up" {
			dir = -1
		}
		from := m.hover
		if from < 0 && dir < 0 {
			from = 0
		}
		m.hover = m.step(from, dir)
	case "home":
		if m.Open() {
			m.hover = m.step(-1, 1)
		}
	case "end":
		if m.Open() {
			m.hover = m.step(0, -1)
		}
	case "enter":
		if !m.Open() {
			m.openMenu(m.cursor)
			break
		}
		if m.hover >= 0 {
			act := menus[m.open].items[m.hover].action
			m.Focus(false)
			return act, true
		}
	case "esc":
		if m.Open() {
			m.Close()
		} else {
			m.focused = false
		}
	default:
		return nothing, false
	}
	return nothing, true
}

// layout computes where each label sits.
func (m *Model) layout() {
	m.labelX = m.labelX[:0]
	x := ansi.StringWidth(m.brandText()) + 2
	for _, mn := range menus {
		m.labelX = append(m.labelX, x)
		x += ansi.StringWidth(mn.label) + 2*gap + 1
	}
}

// Line renders the bar, with right-aligned status text.
func (m *Model) Line(right string) string {
	m.layout()
	var b strings.Builder
	b.WriteString(m.styledBrand())
	b.WriteString(theme.Faded("│ "))
	for i, mn := range menus {
		label := " " + mn.label + " "
		switch {
		case i == m.open, i == m.cursor && m.focused:
			label = theme.Selected(label)
		case i == m.hoverBar:
			label = theme.Hovered(label)
		default:
			label = theme.Dim(label)
		}
		b.WriteString(label + " ")
	}
	left := b.String()
	pad := max(m.w-ansi.StringWidth(left)-ansi.StringWidth(right)-1, 1)
	return theme.Fit(left+strings.Repeat(" ", pad)+right, m.w)
}

// dropdown geometry --------------------------------------------------------

func (m *Model) dropdownWidth() int {
	w := 0
	for _, it := range menus[m.open].items {
		n := ansi.StringWidth(m.labelOf(it))
		if it.shortcut != "" {
			n += 3 + ansi.StringWidth(m.keys(it.shortcut))
		}
		w = max(w, n)
	}
	return w + 2*itemPadX
}

// Dropdown returns the open menu's lines and the column to draw them at.
// The first line belongs on screen row 1.
func (m *Model) Dropdown() (x int, lines []string) {
	if m.open < 0 {
		return 0, nil
	}
	m.layout()
	w := m.dropdownWidth()
	x = min(m.labelX[m.open], max(m.w-w-2, 0))
	items := menus[m.open].items
	body := make([]string, 0, len(items))
	for i, it := range items {
		body = append(body, m.itemLine(i, it, w))
	}
	return x, theme.Panel("", body, w+2)
}

func (m *Model) itemLine(i int, it item, w int) string {
	if it.label == separatorLabel {
		return theme.Faded(strings.Repeat("─", w))
	}
	shortcut := m.keys(it.shortcut)
	gapW := max(w-2*itemPadX-ansi.StringWidth(m.labelOf(it))-ansi.StringWidth(shortcut), 1)
	text := strings.Repeat(" ", itemPadX) + m.labelOf(it) + strings.Repeat(" ", gapW) + shortcut + strings.Repeat(" ", itemPadX)
	switch {
	case m.disabled[it.action] || m.unavailable[it.action]:
		return theme.Faded(text)
	case i == m.hover:
		return theme.Selected(text)
	}
	return text
}

// Handle takes a pointer event in screen coordinates. It returns the chosen
// action (or "") and whether the bar consumed the event. While a dropdown is
// open it consumes every press so a click elsewhere only closes it.
func (m *Model) Handle(e pointer.Event) (Action, bool) {
	m.layout()
	if e.Phase == pointer.Wheel || (e.Phase == pointer.Up && !m.Open()) {
		return nothing, false
	}
	onBar := e.Y == 0
	barIdx := m.labelAt(e.X)
	if onBar {
		m.hoverBar = barIdx
	} else {
		m.hoverBar = -1
	}
	itemIdx := m.itemAt(e.X, e.Y)
	switch e.Phase {
	case pointer.Move:
		if m.Open() {
			if onBar && barIdx >= 0 {
				m.open, m.hover = barIdx, -1
			}
			m.hover = itemIdx
		}
		return nothing, onBar && barIdx >= 0 || m.Open() && itemIdx >= 0
	case pointer.Down:
		return m.press(e, onBar, barIdx, itemIdx)
	case pointer.Up:
		return nothing, true
	}
	return nothing, false
}

func (m *Model) press(e pointer.Event, onBar bool, barIdx, itemIdx int) (Action, bool) {
	if !e.Left {
		return nothing, false
	}
	if onBar && barIdx >= 0 {
		if m.open == barIdx {
			m.Close()
		} else {
			m.open, m.hover = barIdx, -1
		}
		return nothing, true
	}
	if !m.Open() {
		return nothing, false
	}
	if itemIdx >= 0 {
		it := menus[m.open].items[itemIdx]
		if it.action == nothing || m.disabled[it.action] {
			return nothing, true
		}
		m.Close()
		return it.action, true
	}
	m.Close()
	return nothing, true
}

// labelAt is the bar label under column x, or -1.
func (m *Model) labelAt(x int) int {
	for i, mn := range menus {
		if x >= m.labelX[i] && x < m.labelX[i]+ansi.StringWidth(mn.label)+2*gap {
			return i
		}
	}
	return -1
}

// itemAt is the dropdown item under a screen cell, or -1.
func (m *Model) itemAt(x, y int) int {
	if m.open < 0 {
		return -1
	}
	w := m.dropdownWidth()
	left := min(m.labelX[m.open], max(m.w-w-2, 0))
	i := y - 2 // row 0 bar, row 1 box top border
	if x <= left || x >= left+w+1 || i < 0 || i >= len(menus[m.open].items) {
		return -1
	}
	return i
}

// Describe lists the menus and, when one is open, its items.
func (m *Model) Describe() []a11y.Node {
	var nodes []a11y.Node
	for i, mn := range menus {
		b := a11y.Button(mn.label + " menu")
		b.Selected = i == m.open
		nodes = append(nodes, b)
	}
	if m.open < 0 {
		return []a11y.Node{a11y.List("Menus", nodes...)}
	}
	var items []a11y.Node
	for _, it := range menus[m.open].items {
		if it.label == separatorLabel {
			continue
		}
		label := it.label
		if on, isSwitch := m.checked[it.action]; isSwitch {
			label += map[bool]string{true: ", on", false: ", off"}[on]
		}
		if it.shortcut != "" {
			label += ", " + m.keys(it.shortcut)
		}
		if m.disabled[it.action] || m.unavailable[it.action] {
			label += ", not available"
		}
		items = append(items, a11y.Item(label, false))
	}
	return []a11y.Node{a11y.List("Menus", nodes...), a11y.List(menus[m.open].label+" menu items", items...)}
}

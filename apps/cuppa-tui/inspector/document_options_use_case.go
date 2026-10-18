package inspector

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/theme"
	"github.com/meta-ui-labs/cuppa/libs/canvas/editor"
	"github.com/meta-ui-labs/cuppa/libs/document/design"
)

// canvasPanel is the canvas size and the buttons for the usual terminal sizes.
func (m *Model) canvasPanel(b *builder) {
	doc := m.ed.Document()
	m.canvasNumeric(b, "W", "canvas-w", doc.Width)
	m.canvasNumeric(b, "H", "canvas-h", doc.Height)
	m.sizePresets(b, doc)
	b.text(" " + theme.Faded(fmt.Sprintf("%d components", len(doc.Nodes)))).end()
	if len(doc.Nodes) == 0 {
		b.text(" " + theme.Faded("Drag a component from the")).end()
		b.text(" " + theme.Faded("left bar onto the canvas.")).end()
	}
	m.messageRow(b)
}

// themePanel is the colours components follow unless they set their own.
func (m *Model) themePanel(b *builder) {
	doc := m.ed.Document()
	if m.pickTheme != nil {
		b.text(" ").add(theme.Button("[Colour scheme…]", true), func() {
			m.pickTheme(func(background string, t design.Theme) { m.report(m.ed.SetTheme(background, t)) })
		}).end()
	}
	m.themeRow(b, "Background", "background", doc.Background, "terminal default", m.ed.SetBackground)
	for _, c := range []struct{ label, role, value string }{
		{"Text", editor.ThemeText, doc.Theme.Text},
		{"Muted", editor.ThemeMuted, doc.Theme.Muted},
		{"Border", editor.ThemeBorder, doc.Theme.Border},
		{"Secondary", editor.ThemeSecondary, doc.Theme.Secondary},
	} {
		role := c.role
		m.themeRow(b, c.label, "theme-"+role, c.value, "component default", func(v string) error { return m.ed.SetThemeColor(role, v) })
	}
	b.text(" " + theme.Faded("Components follow these unless")).end()
	b.text(" " + theme.Faded("they set their own.")).end()
}

// displayPanel is the effects over the canvas and how the terminal is shown.
func (m *Model) displayPanel(b *builder) {
	doc := m.ed.Document()
	m.optionRow(b, "Grid dots", editor.EffectGrid)
	m.optionRow(b, "Shadows", editor.EffectShadow)
	m.optionRow(b, "Scanlines", editor.EffectScanlines)
	m.optionRow(b, "Vignette", editor.EffectVignette)
	m.terminalRow(b, doc.Light)
	m.profileRow(b, doc.Profile)
	m.snapRow(b)
}

// canvasNumeric is a size field with [-] [+] and click-to-type, like a node's.
func (m *Model) canvasNumeric(b *builder, label, field string, v int) {
	b.text(" " + theme.Dim(label+"  "))
	b.addMouse(theme.Button("[-]", true), func() { m.nudgeCanvas(field, -1) }).text(" ")
	if m.editing == field {
		b.text(m.shown())
	} else {
		b.addStep(theme.Bold(fmt.Sprintf("%4d", v)), func() { m.startEdit(field, strconv.Itoa(v)) },
			func(d int) { m.nudgeCanvas(field, d) })
	}
	b.text(" ").addMouse(theme.Button("[+]", true), func() { m.nudgeCanvas(field, 1) }).end()
}

func (m *Model) nudgeCanvas(field string, delta int) {
	doc := m.ed.Document()
	m.setCanvas(field, map[string]int{"canvas-w": doc.Width, "canvas-h": doc.Height}[field]+delta)
}

func (m *Model) setCanvas(field string, v int) {
	doc := m.ed.Document()
	w, h := doc.Width, doc.Height
	if field == "canvas-w" {
		w = v
	} else {
		h = v
	}
	m.report(m.ed.SetCanvasSize(w, h))
}

func (m *Model) commitCanvas(field, value string) {
	v, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		m.message = "enter a whole number"
		return
	}
	m.setCanvas(field, v)
}

// themeRow is one colour of the theme on a single line: its name, a swatch and
// the value. Clicking opens the colour dialog (or a text box when none is
// bound); set stores the choice.
func (m *Model) themeRow(b *builder, label, field, value, none string, set func(string) error) {
	name := " " + theme.Dim(fmt.Sprintf("%-11s", label))
	if m.editing == field {
		b.text(name + m.shown()).end()
		return
	}
	swatch := theme.Faded("--")
	if value != "" {
		swatch = lipgloss.NewStyle().Background(lipgloss.Color(value)).Render("  ")
	}
	shown := value
	if shown == "" {
		shown = none
	}
	open := func() { m.startEdit(field, value) }
	if m.pickColor != nil {
		open = func() {
			m.pickColor(label, value, func(v string) { m.report(set(v)) })
		}
	}
	b.text(name+swatch+" ").add(shown+theme.Faded(" ✎"), open).end()
}

// profiles are the colour profiles in the order the arrows step through them.
var profiles = []struct{ value, name string }{
	{"", "true colour"}, {design.Profile256, "256 colours"}, {design.Profile16, "16 colours"}, {design.ProfileNone, "no colour"},
}

// terminalRow chooses whether the canvas previews a dark or a light terminal.
func (m *Model) terminalRow(b *builder, light bool) {
	pick := func(label string, active bool, set bool) {
		text := "[" + label + "]"
		if active {
			b.add(theme.Selected(text), func() { m.ed.SetLight(set) })
		} else {
			b.add(theme.Button(text, true), func() { m.ed.SetLight(set) })
		}
	}
	b.text(" " + theme.Dim("Preview on  "))
	pick("Dark", !light, false)
	b.text(" ")
	pick("Light", light, true)
	b.end()
}

// profileRow chooses the colour profile from a list.
func (m *Model) profileRow(b *builder, current string) {
	at := 0
	items := make([]menuItem, len(profiles))
	for i, p := range profiles {
		value := p.value
		if value == current {
			at = i
		}
		items[i] = menuItem{label: p.name, current: value == current, act: func() { m.report(m.ed.SetProfile(value)) }}
	}
	b.text(" " + theme.Dim("Colours  "))
	m.selectRow(b, profiles[at].name, "Colours", items, func(d int) {
		m.report(m.ed.SetProfile(profiles[((at+d)%len(profiles)+len(profiles))%len(profiles)].value))
	})
	b.end()
}

// optionRow is a checkbox for one canvas option.
func (m *Model) optionRow(b *builder, label, name string) {
	mark := "[ ]"
	if m.ed.Effect(name) {
		mark = "[x]"
	}
	b.text(" ").add(theme.Button(mark+" "+label, true), func() {
		m.report(m.ed.SetEffect(name, !m.ed.Effect(name)))
	}).end()
}

// terminalSizes are the terminal sizes the canvas can be set to in one click,
// to see how a layout with percentages behaves.
var terminalSizes = [...][2]int{{80, 24}, {120, 40}, {160, 50}}

// sizePresets is a row of buttons that set the canvas to a usual terminal size.
func (m *Model) sizePresets(b *builder, doc design.Document) {
	b.text(" ")
	for i, s := range terminalSizes {
		w, h := s[0], s[1]
		label := fmt.Sprintf("[%d×%d]", w, h)
		if i > 0 {
			b.text(" ")
		}
		b.add(theme.Button(label, doc.Width != w || doc.Height != h), func() { m.report(m.ed.SetCanvasSize(w, h)) })
	}
	b.end()
}

// paletteRows lists the design's named colours. Properties choose one by name,
// so the components that use a name follow it, and a Go screen exported from
// the design exposes each one to the program.
func (m *Model) paletteRows(b *builder, doc design.Document) {
	if len(doc.Theme.Palette) == 0 {
		b.text(" " + theme.Faded("A colour you set on a")).end()
		b.text(" " + theme.Faded("component is named after")).end()
		b.text(" " + theme.Faded("its property.")).end()
	}
	for _, s := range doc.Theme.Palette {
		name, color := s.Name, s.Color
		swatch := lipgloss.NewStyle().Background(lipgloss.Color(color)).Render("  ")
		open := func() { m.startEdit("swatch:"+name, color) }
		if m.pickColor != nil {
			open = func() { m.pickColor(name, color, func(v string) { m.report(m.ed.SetSwatch(name, v)) }) }
		}
		b.text(" ")
		if m.editing == "swatch:"+name {
			b.text(m.shown())
		} else {
			b.text(swatch+" ").add(theme.Bold(name)+" "+theme.Faded(color), open)
		}
		b.text(" ").addMouse(theme.Button("[x]", true), func() { m.report(m.ed.RemoveSwatch(name)) }).end()
	}
	b.text(" ")
	if m.editing == "swatch-new" {
		b.text(m.shown()).end()
		return
	}
	b.add(theme.Button("[+ colour]", true), func() { m.startEdit("swatch-new", "") }).end()
}

package inspector

import (
	"github.com/meta-ui-labs/cuppa/libs/document/container"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/theme"
	"github.com/meta-ui-labs/cuppa/libs/catalog/definition"
	"github.com/meta-ui-labs/cuppa/libs/document/design"
)

// builder assembles content lines and records which spans are clickable.
type builder struct {
	lines   []string
	regions []region
	cur     string
	x       int
	// focus is the keyboard stop to draw as focused (-1 for none) and stops
	// counts the stops added so far.
	focus, stops int
}

// spoken is how a span reads aloud: the text before it on its line (or, when
// there is none, the line above, which names a property) and its own text,
// without the stepper buttons.
func (b *builder) spoken(styled string) string {
	before := ansi.Strip(b.cur)
	if strings.TrimSpace(before) == "" && len(b.lines) > 0 {
		before = ansi.Strip(b.lines[len(b.lines)-1])
	}
	text := before + " " + ansi.Strip(styled)
	for _, junk := range []string{"[-]", "[+]", "◂", "▸", "█"} {
		text = strings.ReplaceAll(text, junk, "")
	}
	return strings.Join(strings.Fields(text), " ")
}

// add is a clickable span that is also a stop for the keyboard.
func (b *builder) add(styled string, act func()) *builder {
	return b.span(styled, act, nil, false)
}

// addStep is a stop that Left and Right (or - and +) also change by step(±1).
func (b *builder) addStep(styled string, act func(), step func(delta int)) *builder {
	return b.span(styled, act, step, false)
}

// addMouse is a clickable span the keyboard does not stop on, because
// another stop does what it does (the [-] and [+] of a number).
func (b *builder) addMouse(styled string, act func()) *builder {
	return b.span(styled, act, nil, true)
}

func (b *builder) span(styled string, act func(), step func(int), mouseOnly bool) *builder {
	w := lipgloss.Width(styled)
	label := b.spoken(styled)
	if act != nil {
		if !mouseOnly {
			if b.stops == b.focus {
				styled = theme.Selected(ansi.Strip(styled))
			}
			b.stops++
		}
		b.regions = append(b.regions, region{y: len(b.lines), x0: b.x, x1: b.x + w, act: act, step: step, mouseOnly: mouseOnly, label: label})
	}
	b.cur += styled
	b.x += w
	return b
}

func (b *builder) text(s string) *builder { return b.add(s, nil) }

func (b *builder) end() {
	b.lines = append(b.lines, b.cur)
	b.cur, b.x = "", 0
}

// wrap splits text into lines of at most width cells, breaking at spaces,
// commas and slashes, so a long list or path stays inside the pane.
func wrap(text string, width int) []string {
	return strings.Split(ansi.Wrap(text, max(width, 4), ",/"), "\n")
}

// block writes text wrapped to the pane, each line indented. When act is set,
// every wrapped line is clickable.
func (m *Model) block(b *builder, text string, indent int, style func(string) string, act func()) {
	for i, line := range wrap(text, m.content()-indent-1) {
		b.text(strings.Repeat(" ", indent))
		// One property is one stop for the keyboard, however many lines it wraps to.
		if i == 0 {
			b.add(style(line), act)
		} else {
			b.addMouse(style(line), act)
		}
		b.end()
	}
}

func plain(s string) string { return s }

// Lines renders the pane as exactly h lines of w cells: the title and the
// history buttons, then the panels, each a header and a body of its own
// height that scrolls inside itself.
func (m *Model) Lines() []string {
	b := &builder{focus: m.focusStop()}
	m.menuRows = map[int]menuRef{}
	if b.focus >= 0 {
		// The contextual bar comes first in the keyboard order.
		b.focus -= m.ctxStops
	}
	title := theme.Title(" DETAILS")
	if m.focused {
		title = theme.Selected(" DETAILS ")
	}
	b.text(title)
	m.modeButtons(b)
	b.end()

	specs := m.panelSpecs()
	m.views = make([]panelView, len(specs))
	for i, sp := range specs {
		st := m.state(sp.id)
		v := panelView{id: sp.id, start: len(b.lines), open: !st.closed}
		m.panelHeader(b, sp, st.closed)
		if v.open {
			sp.fill(b)
			v.total = len(b.lines) - v.start - 1
		}
		m.views[i] = v
	}
	m.regions, m.stops = b.regions, b.stops
	m.followMenu()
	m.reflow()
	m.revealSelection()

	out := make([]string, 0, m.h)
	out = append(out, b.lines[:fixedRows]...)
	for _, v := range m.views {
		out = append(out, b.lines[v.start])
		if !v.open {
			continue
		}
		st := m.state(v.id)
		st.scroll = min(max(st.scroll, 0), max(v.total-v.h, 0))
		barTop, barLen := thumb(v.h, v.total, st.scroll)
		for r := 0; r < v.h; r++ {
			line := ""
			if cy, ok := m.bodyRow(v, v.top+1+r); ok {
				line = b.lines[cy]
			}
			bar := " "
			if barLen > 0 {
				bar = theme.Faded("│")
				if r >= barTop && r < barTop+barLen {
					bar = theme.Dim("▐")
				}
			}
			out = append(out, theme.Fit(line, m.content())+bar)
		}
	}
	lines := theme.Block(out, m.w, m.h)
	if m.menu != nil {
		m.drawMenu(lines)
	}
	return lines
}

// panelSpecs are the panels to show for what is selected. What the selection
// is and where (name, place, size) and what is done to it (order, copy, group)
// are in the contextual bar above the canvas.
func (m *Model) panelSpecs() []panelSpec {
	layers := panelSpec{id: panelLayers, title: "Layers", fill: m.layers, trash: true}
	n, ok := m.ed.Primary()
	switch {
	case !ok:
		specs := []panelSpec{
			{id: panelCanvas, title: "Canvas", fill: m.canvasPanel},
			{id: panelTheme, title: "Theme", fill: m.themePanel},
			{id: panelColours, title: "Named colours", fill: func(b *builder) { m.paletteRows(b, m.ed.Document()) }},
			{id: panelDisplay, title: "Display", fill: m.displayPanel},
		}
		return append(specs, layers)
	case len(m.ed.Selected()) > 1:
		return []panelSpec{layers}
	}
	specs := []panelSpec{{id: panelProperties, title: "Properties", fill: func(b *builder) { m.properties(b, n) }}}
	if len(m.ed.PageNames()) > 1 {
		// What a click does is only a choice when there is a page to go to.
		specs = append(specs, panelSpec{id: panelBehaviour, title: "Behaviour", fill: func(b *builder) { m.behaviourPanel(b, n) }})
	}
	return append(specs, layers)
}

// panelHeader is the line that names a panel: an arrow that says whether it
// is open, the title and a rule. Clicking it opens or closes the panel;
// dragging it moves the edge above it.
func (m *Model) panelHeader(b *builder, sp panelSpec, closed bool) {
	arrow := "▾"
	if closed {
		arrow = "▸"
	}
	width := m.content()
	if sp.trash {
		width -= lipgloss.Width("[Trash]") + 1
	}
	label := arrow + " " + sp.title
	rule := strings.Repeat("─", max(width-lipgloss.Width(label)-2, 0))
	id := sp.id
	flip := func() { m.toggle(id, !m.state(id).closed) }
	// The rule lights up while the pointer is on the header or drags it, as a
	// divider between the panes does.
	if m.hover == id || m.press != nil && m.press.id == id {
		b.add(" "+theme.Title(label)+" "+theme.Title(rule), flip)
	} else {
		b.add(" "+theme.Bold(label)+" "+theme.Faded(rule), flip)
	}
	b.regions[len(b.regions)-1].header = id
	if sp.trash {
		b.text(" ")
		m.trashButton(b)
	}
	b.end()
}

// revealSelection scrolls the layer list to the selected layer when the
// selection has changed since the last render, so choosing a component on the
// canvas shows its layer.
func (m *Model) revealSelection() {
	n, ok := m.ed.Primary()
	var id design.NodeID
	if ok {
		id = n.ID
	}
	if id == m.lastSel {
		return
	}
	m.lastSel = id
	if id == "" {
		return
	}
	doc := m.ed.Document()
	for _, v := range m.views {
		if v.id == panelLayers && v.open {
			m.reveal(v.start + 1 + len(doc.Nodes) - 1 - doc.Index(id))
		}
	}
}

// snapRow is the "snap to guides" checkbox.
func (m *Model) snapRow(b *builder) {
	if m.snapGet == nil {
		return
	}
	mark := "[ ]"
	if m.snapGet() {
		mark = "[x]"
	}
	b.text(" ").add(theme.Button(mark+" Snap to guides", true), func() { m.snapSet(!m.snapGet()) }).end()
}

// messageRow shows what the last edit said was wrong.
func (m *Model) messageRow(b *builder) {
	if m.message != "" {
		b.text(" " + lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Warn)).Render(m.message)).end()
	}
}

// behaviourPanel is what a click on the component does in the preview.
func (m *Model) behaviourPanel(b *builder, n design.Node) {
	m.gotoRow(b, n)
}

// editable renders a value that turns into a text box when clicked.
func (m *Model) editable(b *builder, field, value, initial string) {
	if m.editing == field {
		b.text(m.shown())
		return
	}
	if value == "" {
		value = theme.Faded("(empty)")
	}
	b.add(value, func() { m.startEdit(field, initial) })
}

// properties lists the component-specific properties from the catalog schema.
func (m *Model) properties(b *builder, n design.Node) {
	m.pageRows(b, n)
	if n.Locked {
		b.text(" " + theme.Faded("Locked: unlock it in Layers to edit")).end()
	}
	m.messageRow(b)
	def, ok := m.cat.Get(n.Component)
	if !ok || len(def.Props) == 0 {
		b.text(" " + theme.Faded("No properties.")).end()
		return
	}
	doc := m.ed.Document()
	effective := def.Effective(n.Props, doc.Theme, doc.Background)
	for _, p := range def.Props {
		_, own := n.Props[p.Key]
		value := effective[p.Key]
		m.block(b, p.Label, 1, theme.Dim, nil)
		if p.Kind == definition.PropMenu {
			m.menuProp(b, n, p, value)
			continue
		}
		if p.Kind == definition.PropText || p.Kind == definition.PropFloat {
			m.textProp(b, "prop:"+p.Key, value)
		} else {
			b.text("  ")
			m.propValue(b, n.ID, p, value)
			if p.Role != definition.RoleNone {
				m.themeMark(b, n.ID, p, own)
			}
			b.end()
			if p.Kind == definition.PropColor {
				m.swatchRow(b, n, p)
			}
		}
	}
}

// textProp shows a text property wrapped to the pane; clicking any line edits it.
func (m *Model) textProp(b *builder, field, value string) {
	if m.editing == field {
		m.block(b, m.shown(), 2, plain, nil)
		return
	}
	edit := func() { m.startEdit(field, value) }
	if value == "" {
		m.block(b, "(empty)", 2, theme.Faded, edit)
		return
	}
	m.block(b, value, 2, plain, edit)
}

func (m *Model) propValue(b *builder, id design.NodeID, p definition.PropSpec, value string) {
	field := "prop:" + p.Key
	set := func(v string) func() {
		if p.Kind == definition.PropColor {
			// A colour is named after its property, so it can be reused.
			return func() { m.report(m.ed.SetColour(id, p.Key, v)) }
		}
		return func() { m.report(m.ed.SetProp(id, p.Key, v)) }
	}
	switch p.Kind {
	case definition.PropBool:
		mark := "[ ]"
		if value == "true" {
			mark = "[x]"
		}
		next := "true"
		if value == "true" {
			next = "false"
		}
		b.add(theme.Button(mark, true), set(next))
	case definition.PropChoice:
		i := slices.Index(p.Choices, value)
		count := len(p.Choices)
		items := make([]menuItem, count)
		for k, c := range p.Choices {
			items[k] = menuItem{label: c, current: k == i, act: set(c)}
		}
		m.selectRow(b, value, p.Label, items, func(d int) {
			set(p.Choices[((i+d)%count+count)%count])()
		})
	case definition.PropInt:
		n, _ := strconv.Atoi(value)
		b.addMouse(theme.Button("[-]", true), set(strconv.Itoa(n-1))).text(" ")
		if m.editing == field {
			b.text(m.shown())
		} else {
			b.addStep(theme.Bold(value), func() { m.startEdit(field, value) },
				func(d int) { set(strconv.Itoa(n + d))() })
		}
		b.text(" ").addMouse(theme.Button("[+]", true), set(strconv.Itoa(n+1)))
	case definition.PropColor:
		if m.editing == field {
			b.text(m.shown())
		} else {
			swatch := lipgloss.NewStyle().Background(lipgloss.Color(value)).Render("  ")
			if value == "" {
				swatch = theme.Faded("--")
			}
			open := func() { m.startEdit(field, value) }
			if m.pickColor != nil {
				open = func() { m.pickColor(p.Label, value, func(v string) { set(v)() }) }
			}
			b.text(swatch+" ").add(value+theme.Faded(" ✎"), open)
		}
	default:
		m.editable(b, field, value, value)
	}
}

// themeMark says whether a colour follows the theme or is the component's own,
// and for an own one offers to go back to the theme.
func (m *Model) themeMark(b *builder, id design.NodeID, p definition.PropSpec, own bool) {
	if !own {
		b.text(" " + theme.Faded("theme"))
		return
	}
	b.text(" ").add(theme.Button("[theme]", true), func() { m.ed.ClearProp(id, p.Key) })
}

// swatchRow shows which named colour a colour property uses and lists the
// palette: the properties that use the same name change together, and a
// program that shows the screen can change the colour by that name.
func (m *Model) swatchRow(b *builder, n design.Node, p definition.PropSpec) {
	palette := m.ed.Swatches()
	current, used := strings.CutPrefix(n.Props[p.Key], design.TokenPrefix)
	if len(palette) == 0 {
		return
	}
	field := "swatchname:" + p.Key
	b.text("  " + theme.Faded("name "))
	if m.editing == field {
		b.text(m.shown()).end()
		return
	}
	at := -1
	items := make([]menuItem, 0, len(palette)+1)
	for i, sw := range palette {
		name := sw.Name
		if used && name == current {
			at = i
		}
		items = append(items, menuItem{label: name, current: used && name == current,
			act: func() { m.report(m.ed.UseSwatch(n.ID, p.Key, name)) }})
	}
	rename := "+ New name…"
	if used {
		rename = "✎ Rename…"
	}
	items = append(items, menuItem{label: rename, act: func() { m.startEdit(field, current) }})
	shown := "(none)"
	if at >= 0 {
		shown = palette[at].Name
	}
	step := func(d int) {
		next := ((at+d)%len(palette) + len(palette)) % len(palette)
		if at < 0 && d < 0 {
			next = len(palette) - 1
		}
		m.report(m.ed.UseSwatch(n.ID, p.Key, palette[next].Name))
	}
	m.selectRow(b, shown, "Colour name", items, step)
	b.end()
}

// pageRows list the pages of a container: pressing a name shows that page,
// [x] removes it (with the components on it) and [+ Page] adds one.
func (m *Model) pageRows(b *builder, n design.Node) {
	if !n.IsContainer() {
		return
	}
	pages := container.Pages(n)
	for i, name := range pages {
		page := i
		mark := " "
		if container.Shown(n, i) {
			mark = theme.Title("▸")
		}
		b.text(" "+mark+" ").add(theme.Bold(theme.Fit(name, max(m.content()-10, 4))), func() { m.ed.PressContainerHeader(n.ID, page) })
		if len(pages) > 1 && !n.Locked {
			b.text(" ").addMouse(theme.Button("[x]", true), func() { m.report(m.ed.RemoveContainerPage(n.ID, page)) })
		}
		b.end()
	}
	if !n.Locked {
		b.text(" ").add(theme.Button("[+ Page]", true), func() { m.report(m.ed.AddContainerPage(n.ID, "")) }).end()
	}
}

// modeButtons are the Editor and Preview buttons at the right of the title
// row, a pair of which one is lit: while editing, a press on a tab or a header
// of the design only selects it; Preview makes the design respond.
func (m *Model) modeButtons(b *builder) {
	if m.setPreview == nil {
		return
	}
	on := m.previewing != nil && m.previewing()
	editor, preview := " Editor ", " Preview "
	pad := max(m.w-b.x-lipgloss.Width(editor)-lipgloss.Width(preview), 1)
	b.text(strings.Repeat(" ", pad))
	style := func(label string, active bool) string {
		if active {
			return theme.Selected(label)
		}
		return theme.Button(label, true)
	}
	m.modeX[0][0] = b.x
	b.add(style(editor, !on), func() { m.setPreview(false) })
	m.modeX[0][1], m.modeX[1][0] = b.x, b.x
	b.add(style(preview, on), func() { m.setPreview(true) })
	m.modeX[1][1] = b.x
}

// gotoRow chooses the page a click on the component shows, from the pages of
// the design (a design with one page has none to choose).
func (m *Model) gotoRow(b *builder, n design.Node) {
	names := m.ed.PageNames()
	if len(names) < 2 {
		return
	}
	items := []menuItem{{label: "(none)", current: n.Goto == "", act: func() { m.report(m.ed.SetGoto(n.ID, "")) }}}
	for i, name := range names {
		if i == m.ed.ActivePage() {
			continue
		}
		page := name
		items = append(items, menuItem{label: page, current: n.Goto == page, act: func() { m.report(m.ed.SetGoto(n.ID, page)) }})
	}
	shown := n.Goto
	if shown == "" {
		shown = "(none)"
	}
	b.text(" ").add(theme.Dim("Click goes to "), nil)
	b.end()
	b.text("  ")
	m.selectRow(b, shown, "Go to page", items, nil)
	b.end()
}


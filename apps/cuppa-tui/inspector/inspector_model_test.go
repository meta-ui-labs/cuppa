package inspector

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"

	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/pointer"
	"github.com/meta-ui-labs/cuppa/libs/canvas/editor"
	"github.com/meta-ui-labs/cuppa/libs/catalog/standard"
	"github.com/meta-ui-labs/cuppa/libs/document/design"
)

func setup(t *testing.T, comp string) (*Model, *editor.Editor, design.NodeID) {
	t.Helper()
	cat := standard.Default()
	ed := editor.New(cat, design.NewDocument("t", 100, 40))
	id, err := ed.Add(comp, 10, 5)
	if err != nil {
		t.Fatal(err)
	}
	m := New(ed, cat)
	m.SetSize(32, 200)
	expandAll(m)
	m.Lines() // renders once so clickable regions exist
	return m, ed, id
}

// expandAll opens every panel and gives it room, so a test can look at the
// whole bar in one list of lines.
func expandAll(m *Model) {
	for id := range panelDefaults {
		st := m.state(id)
		st.closed, st.want = false, 30
	}
}

// clickText clicks the first clickable span whose rendered line contains text.
func clickText(t *testing.T, m *Model, line, text string) {
	t.Helper()
	// The contextual bar above the canvas is part of the bar's controls.
	for y, l := range m.Context(120) {
		plain := stripANSI(l)
		if x := strings.Index(plain, text); strings.Contains(plain, line) && x >= 0 {
			m.HandleContext(pointer.Event{X: len([]rune(plain[:x])), Y: y, Phase: pointer.Down, Left: true})
			m.Lines()
			return
		}
	}
	lines := m.Lines()
	for y, l := range lines {
		plain := stripANSI(l)
		if !strings.Contains(plain, line) {
			continue
		}
		x := strings.Index(plain, text)
		if x < 0 {
			continue
		}
		col := len([]rune(plain[:x]))
		m.Handle(pointer.Event{X: col, Y: y, Phase: pointer.Down, Left: true})
		m.Lines()
		return
	}
	t.Fatalf("no line %q containing %q in:\n%s", line, text, strings.Join(lines, "\n"))
}

// barClick presses one part of a geometry field of the contextual bar: the
// "value" (to type it), "minus", "plus" or the "unit" button.
func barClick(t *testing.T, m *Model, label, part string) {
	t.Helper()
	plain := stripANSI(m.Context(120)[0])
	at := strings.Index(plain, " "+label+" ")
	if at < 0 {
		t.Fatalf("no %s field in the bar: %q", label, plain)
	}
	start := len([]rune(plain[:at])) + 1 + len(label) + 1
	end := start
	runes := []rune(plain)
	for end < len(runes) && runes[end] != ' ' {
		end++
	}
	x := map[string]int{"value": start, "minus": end + 1, "plus": end + 2, "unit": end + 4}[part]
	m.HandleContext(pointer.Event{X: x, Y: 0, Phase: pointer.Down, Left: true})
	m.Lines()
}

func stripANSI(s string) string {
	var b strings.Builder
	esc := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			esc = true
		case esc && r == 'm':
			esc = false
		case !esc:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func rect(ed *editor.Editor, id design.NodeID) design.Rect {
	n, _ := ed.Document().Get(id)
	return n.Rect
}

func TestPlusMinusNudgeGeometry(t *testing.T) {
	m, ed, id := setup(t, "lipgloss.box")
	barClick(t, m, "X", "plus")
	barClick(t, m, "Y", "minus")
	if got := rect(ed, id); got.X != 11 || got.Y != 4 {
		t.Fatalf("rect = %+v", got)
	}
}

func TestTypedValueCommitsOnEnter(t *testing.T) {
	m, ed, id := setup(t, "lipgloss.box")
	barClick(t, m, "W", "value")
	if !m.Editing() {
		t.Fatal("clicking the value should start editing")
	}
	for range "24" {
		m.Key("", true, false, false)
	}
	m.Key("3", false, false, false)
	m.Key("0", false, false, false)
	m.Key("", false, true, false)
	if got := rect(ed, id); got.W != 30 {
		t.Fatalf("W = %d", got.W)
	}
	if m.Editing() {
		t.Fatal("still editing after enter")
	}
}

func TestEscapeCancelsAndBadInputReports(t *testing.T) {
	m, ed, id := setup(t, "lipgloss.box")
	barClick(t, m, "H", "value")
	m.Key("9", false, false, false)
	m.Key("", false, false, true)
	if rect(ed, id).H != 6 || m.Editing() {
		t.Fatal("escape should discard the edit")
	}
	barClick(t, m, "H", "value")
	m.Key("", true, false, false)
	m.Key("x", false, false, false)
	m.Key("", false, true, false)
	if m.message == "" {
		t.Fatal("non-numeric input should produce a message")
	}
}

func TestLayerButtonsAndList(t *testing.T) {
	m, ed, a := setup(t, "lipgloss.box")
	m.SetSize(32, 80) // the box has many properties; the layer list is under them
	b, _ := ed.Add("lipgloss.label", 0, 0)
	ed.Select(a)
	m.Lines()
	clickText(t, m, "[Front]", "[Front]")
	doc := ed.Document()
	if doc.Nodes[len(doc.Nodes)-1].ID != a {
		t.Fatal("Front did not raise the node")
	}
	clickText(t, m, "Label 1", "Label 1")                  // layer list selects
	m.Handle(pointer.Event{Phase: pointer.Up, Left: true}) // a layer is selected when the press is released
	if sel := ed.Selected(); len(sel) != 1 || sel[0] != b {
		t.Fatalf("selection = %v", sel)
	}
}

func TestPropertiesEditThroughTheSchema(t *testing.T) {
	m, ed, id := setup(t, "lipgloss.box")
	clickText(t, m, "rounded", "rounded") // choice: opens the list of borders
	clickText(t, m, "  normal", "normal")
	n, _ := ed.Document().Get(id)
	if n.Props["border"] != "normal" {
		t.Fatalf("border = %q", n.Props["border"])
	}
	clickText(t, m, "  (empty)", "(empty)") // text prop (title)
	for _, r := range "Tea" {
		m.Key(string(r), false, false, false)
	}
	m.Key("", false, true, false)
	if n, _ = ed.Document().Get(id); n.Props["title"] != "Tea" {
		t.Fatalf("title = %q", n.Props["title"])
	}
}

func TestEmptyStateAndSizing(t *testing.T) {
	cat := standard.Default()
	ed := editor.New(cat, design.NewDocument("t", 100, 40))
	m := New(ed, cat)
	m.SetSize(30, 12)
	lines := m.Lines()
	if len(lines) != 12 || !strings.Contains(stripANSI(strings.Join(lines, "\n")), "Canvas") {
		t.Fatalf("empty state wrong:\n%s", strings.Join(lines, "\n"))
	}
}

func TestSnapCheckboxTogglesTheBoundSetting(t *testing.T) {
	cat := standard.Default()
	ed := editor.New(cat, design.NewDocument("t", 100, 40))
	m := New(ed, cat)
	m.SetSize(32, 200)
	expandAll(m)
	on := true
	m.BindSnap(func() bool { return on }, func(v bool) { on = v })
	m.Lines()
	clickText(t, m, "Snap to guides", "[x]")
	if on {
		t.Fatal("clicking the checkbox should switch snapping off")
	}
}

func TestLongTextPropertyWrapsInsidePaneAndStaysClickable(t *testing.T) {
	m, ed, id := setup(t, "huh.filepicker")
	long := "docs/,libs/,apps/,README.md,go.work,package.json,tsconfig.base.json,CLAUDE.md,commitlint.config.mjs"
	if err := ed.SetProp(id, "entries", long); err != nil {
		t.Fatal(err)
	}
	lines := m.Lines()
	first, last := -1, -1
	for y, l := range lines {
		if strings.Contains(l, "\n") {
			t.Fatalf("line %d contains a newline: %q", y, l)
		}
		if w := ansi.StringWidth(l); w != 32 {
			t.Fatalf("line %d is %d cells wide, want the pane width 32", y, w)
		}
		plain := strings.TrimSpace(stripANSI(l))
		if first < 0 && strings.HasPrefix(plain, "docs/,") {
			first = y
		}
		if strings.HasSuffix(plain, "mjs") {
			last = y
		}
	}
	if first < 0 || last-first < 2 {
		t.Fatalf("the long value should wrap over several lines (first=%d last=%d):\n%s", first, last, strings.Join(lines, "\n"))
	}
	// Clicking the last wrapped line starts editing, like clicking the first.
	m.Handle(pointer.Event{X: 4, Y: last, Phase: pointer.Down, Left: true})
	if !m.Editing() {
		t.Fatal("every wrapped line should start the edit")
	}
}

func TestFirstKeyReplacesTheShownValueAndBackspaceClearsIt(t *testing.T) {
	m := &Model{}
	m.startEdit("name", "Button")
	m.Key("", true, false, false)
	if m.buf != "" {
		t.Fatalf("buf = %q, want empty", m.buf)
	}
	m.startEdit("name", "Button")
	m.Key("x", false, false, false)
	m.Key("y", false, false, false)
	m.Key("", true, false, false)
	if m.buf != "x" {
		t.Fatalf("buf = %q, want x", m.buf)
	}
}

func TestFieldCursorMovesSoTextCanBeAddedAfterAComma(t *testing.T) {
	m := &Model{}
	m.startEdit("name", "General, Adv")
	m.MoveCursor("end") // leaves the selection
	m.Key("a", false, false, false)
	m.MoveCursor("left")
	m.MoveCursor("left")
	m.Key("X", false, false, false)
	if m.buf != "General, AdXva" {
		t.Fatalf("buf = %q", m.buf)
	}
	m.MoveCursor("home")
	m.MoveCursor("delete")
	if m.buf != "eneral, AdXva" {
		t.Fatalf("buf = %q after Delete", m.buf)
	}
}

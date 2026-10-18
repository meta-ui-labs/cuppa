package shell

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/menubar"
	"github.com/meta-ui-labs/cuppa/libs/document/design"
)

// place drops the first palette component on the canvas at canvas column x and
// returns the new node, which is left selected.
func place(t *testing.T, m *Model, x int) design.Node {
	t.Helper()
	send(m, click(5, firstComponentY))
	sx := m.layout.stage.X
	send(m, motion(sx+x, 8))
	send(m, release(sx+x, 8))
	n, ok := m.Editor().Primary()
	if !ok {
		t.Fatal("nothing was placed")
	}
	return n
}

func ctrlShift(r rune) tea.Msg { return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl | tea.ModShift} }

func TestCtrlDDuplicatesTheSelection(t *testing.T) {
	m := newShell(t)
	place(t, m, 5)
	send(m, ctrl('d'))
	if n := len(m.Editor().Document().Nodes); n != 2 {
		t.Fatalf("nodes = %d, want 2", n)
	}
}

func TestArrowsMoveTheSelectionByOneAndShiftArrowsByTen(t *testing.T) {
	m := newShell(t)
	n := place(t, m, 5)
	x0, y0 := n.Rect.X, n.Rect.Y
	send(m, tea.KeyPressMsg{Code: tea.KeyRight})
	send(m, tea.KeyPressMsg{Code: tea.KeyDown})
	got, _ := m.Editor().Primary()
	if got.Rect.X != x0+1 || got.Rect.Y != y0+1 {
		t.Fatalf("after arrows: (%d,%d), want (%d,%d)", got.Rect.X, got.Rect.Y, x0+1, y0+1)
	}
	send(m, tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	got, _ = m.Editor().Primary()
	if got.Rect.X != x0+11 {
		t.Fatalf("after shift+right: x = %d, want %d", got.Rect.X, x0+11)
	}
	send(m, tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift})
	send(m, tea.KeyPressMsg{Code: tea.KeyUp})
	got, _ = m.Editor().Primary()
	if got.Rect.X != x0+1 || got.Rect.Y != y0 {
		t.Fatalf("back again: (%d,%d), want (%d,%d)", got.Rect.X, got.Rect.Y, x0+1, y0)
	}
}

func TestEachArrowPressIsOneUndoStepAndNothingMovedLeavesNone(t *testing.T) {
	m := newShell(t)
	n := place(t, m, 5)
	send(m, tea.KeyPressMsg{Code: tea.KeyRight})
	send(m, tea.KeyPressMsg{Code: tea.KeyRight})
	send(m, ctrl('z'))
	got, _ := m.Editor().Primary()
	if got.Rect.X != n.Rect.X+1 {
		t.Fatalf("one undo should take back one press: x = %d, want %d", got.Rect.X, n.Rect.X+1)
	}
	// Pushed against the left edge, further presses move nothing and add no step.
	send(m, tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift})
	send(m, tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift})
	send(m, tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift})
	send(m, ctrl('z')) // undoes the move to the edge, not an empty step
	got, _ = m.Editor().Primary()
	if got.Rect.X != n.Rect.X+1 {
		t.Fatalf("undo after hitting the edge: x = %d, want %d", got.Rect.X, n.Rect.X+1)
	}
}

func TestArrowsDoNothingWithoutASelection(t *testing.T) {
	m := newShell(t)
	n := place(t, m, 5)
	send(m, tea.KeyPressMsg{Code: tea.KeyEscape}) // deselect
	send(m, tea.KeyPressMsg{Code: tea.KeyRight})
	got, _ := m.Editor().Document().Get(n.ID)
	if got.Rect != n.Rect {
		t.Fatalf("rect changed to %+v", got.Rect)
	}
}

func TestLayerOrderShortcuts(t *testing.T) {
	m := newShell(t)
	first := place(t, m, 5)
	place(t, m, 40)
	third := place(t, m, 70)
	index := func(id design.NodeID) int { return m.Editor().Document().Index(id) }
	m.Editor().Select(first.ID)

	send(m, ctrl(']'))
	if index(first.ID) != 1 {
		t.Fatalf("Ctrl+] : index %d, want 1", index(first.ID))
	}
	send(m, ctrlShift(']'))
	if index(first.ID) != 2 {
		t.Fatalf("Ctrl+Shift+] : index %d, want 2 (front)", index(first.ID))
	}
	send(m, ctrl('['))
	if index(first.ID) != 1 {
		t.Fatalf("Ctrl+[ : index %d, want 1", index(first.ID))
	}
	send(m, ctrlShift('['))
	if index(first.ID) != 0 || index(third.ID) != 2 {
		t.Fatalf("Ctrl+Shift+[ : first at %d, third at %d", index(first.ID), index(third.ID))
	}
}

func TestCtrlShiftSOpensSaveAsAndCtrlSDoesNotNeedAName(t *testing.T) {
	m := newShell(t)
	send(m, ctrlShift('s'))
	if m.flow.Modal() == nil {
		t.Fatal("Ctrl+Shift+S should open the Save As dialog")
	}
}

func TestCtrlFFocusesTheSearchBox(t *testing.T) {
	m := newShell(t)
	send(m, ctrl('f'))
	if !m.pal.Searching() {
		t.Fatal("Ctrl+F should focus the search")
	}
	send(m, tea.KeyPressMsg{Code: 'b', Text: "b"})
	if q := strings.ToLower(ansi.Strip(m.render())); !strings.Contains(q, "b") {
		t.Fatal("typing should reach the search box")
	}
}

func TestCmdActsAsCtrl(t *testing.T) {
	m := newShell(t)
	place(t, m, 5)
	send(m, tea.KeyPressMsg{Code: 'd', Mod: tea.ModSuper})
	if n := len(m.Editor().Document().Nodes); n != 2 {
		t.Fatalf("Cmd+D should duplicate: nodes = %d", n)
	}
	send(m, tea.KeyPressMsg{Code: 's', Mod: tea.ModSuper | tea.ModShift})
	if m.flow.Modal() == nil {
		t.Fatal("Cmd+Shift+S should open Save As")
	}
}

func TestMacShowsCmdInTheMenusAndTheShortcutsList(t *testing.T) {
	m := newShell(t)
	m.UseCommandKey()
	m.bar.Close()
	// Open the Edit menu with a click on its label (second after the brand).
	for x := 0; x < 40; x++ {
		send(m, click(x, 0))
		if _, lines := m.bar.Dropdown(); len(lines) > 0 && strings.Contains(ansi.Strip(strings.Join(lines, "\n")), "Duplicate") {
			break
		}
	}
	_, lines := m.bar.Dropdown()
	menu := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(menu, "Cmd+D") || strings.Contains(menu, "Ctrl") {
		t.Fatalf("menu should say Cmd and not Ctrl:\n%s", menu)
	}
	if text := shortcutsText(true); strings.Contains(text, "Ctrl") || !strings.Contains(text, "Cmd+Shift+S") {
		t.Fatalf("shortcuts list:\n%s", text)
	}
	if !strings.Contains(shortcutsText(false), "Ctrl+Shift+S") {
		t.Fatal("others still see Ctrl")
	}
	_ = menubar.EditDuplicate
}

func TestArrangeMenuItemsReorderTheSelection(t *testing.T) {
	m := newShell(t)
	first := place(t, m, 5)
	place(t, m, 40)
	m.Editor().Select(first.ID)
	m.perform(menubar.EditToFront)
	if got := m.Editor().Document().Index(first.ID); got != 1 {
		t.Fatalf("To front: index %d, want 1", got)
	}
	m.perform(menubar.EditToBack)
	if got := m.Editor().Document().Index(first.ID); got != 0 {
		t.Fatalf("To back: index %d, want 0", got)
	}
	m.perform(menubar.EditForward)
	if got := m.Editor().Document().Index(first.ID); got != 1 {
		t.Fatalf("Forward one: index %d, want 1", got)
	}
	m.perform(menubar.EditBackward)
	if got := m.Editor().Document().Index(first.ID); got != 0 {
		t.Fatalf("Back one: index %d, want 0", got)
	}
}

func TestBackspaceDeletesTheSelectedComponentOnTheCanvas(t *testing.T) {
	m := newShell(t)
	place(t, m, 5)
	send(m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if n := len(m.Editor().Document().Nodes); n != 0 {
		t.Fatalf("nodes = %d, want 0", n)
	}
}

func TestArrowsWalkTheSearchResults(t *testing.T) {
	m := newShell(t)
	send(m, ctrl('f'))
	send(m, tea.KeyPressMsg{Code: 'b', Text: "b"})
	send(m, tea.KeyPressMsg{Code: tea.KeyDown})
	if !m.pal.Searching() || m.pal.Current() == "" {
		t.Fatal("the search should stay open and move to a match")
	}
}

func TestClickingTheCanvasEndsTheSearchSoDeleteWorks(t *testing.T) {
	m := newShell(t)
	place(t, m, 5)
	send(m, ctrl('f'))
	n := m.Editor().Document().Nodes[0]
	send(m, click(m.layout.stage.X+n.Rect.X+1, m.layout.stage.Y+n.Rect.Y+1))
	if m.pal.Searching() {
		t.Fatal("a click outside the search box should end the search")
	}
	send(m, tea.KeyPressMsg{Code: tea.KeyDelete})
	if len(m.Editor().Document().Nodes) != 0 {
		t.Fatal("Delete should reach the canvas once the search is over")
	}
}

func TestCanvasCursorMenuItemTogglesAndRemembersTheChoice(t *testing.T) {
	m := newShell(t)
	m.restorePrefs(filepath.Join(t.TempDir(), "prefs.json"))
	m.perform(menubar.ViewCursor)
	if !m.cursorOff {
		t.Fatal("the menu item should hide the cursor")
	}
	again := newShell(t)
	again.restorePrefs(m.prefsFile)
	if !again.cursorOff {
		t.Fatal("the choice should be remembered")
	}
	m.perform(menubar.ViewCursor)
	if m.cursorOff {
		t.Fatal("the menu item should show the cursor again")
	}
}

func TestFirstMouseClickOffersToHideTheCursorOnce(t *testing.T) {
	m := newShell(t)
	m.restorePrefs(filepath.Join(t.TempDir(), "prefs.json"))
	send(m, click(m.layout.stage.X+30, m.layout.stage.Y+15))
	send(m, release(m.layout.stage.X+30, m.layout.stage.Y+15))
	if m.flow.Modal() == nil {
		t.Fatal("the first click should ask about the cursor")
	}
	clickText(t, m, "Hide")
	if !m.cursorOff {
		t.Fatal("Hide should switch the cursor off")
	}
	other := newShell(t)
	other.restorePrefs(m.prefsFile)
	send(other, click(5, 5))
	send(other, release(5, 5))
	if other.flow.Modal() != nil {
		t.Fatal("a question already answered is not asked again")
	}
}

func TestKeepingTheCursorIsRememberedAndNotAskedAgain(t *testing.T) {
	m := newShell(t)
	m.restorePrefs(filepath.Join(t.TempDir(), "prefs.json"))
	send(m, click(m.layout.stage.X+30, m.layout.stage.Y+15))
	send(m, release(m.layout.stage.X+30, m.layout.stage.Y+15))
	clickText(t, m, "Keep")
	if m.cursorOff {
		t.Fatal("Keep leaves the cursor on")
	}
	send(m, click(m.layout.stage.X+31, m.layout.stage.Y+15))
	send(m, release(m.layout.stage.X+31, m.layout.stage.Y+15))
	if m.flow.Modal() != nil {
		t.Fatal("asked twice")
	}
}

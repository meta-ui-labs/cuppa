package shell

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/pointer"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/theme"
)

// handleDivider takes the pointer when it presses a divider or is dragging one,
// and reports whether it used the event. Hovering a divider is tracked (so it
// can light up) without taking the event.
func (m *Model) handleDivider(e pointer.Event) bool {
	switch e.Phase {
	case pointer.Down:
		if e.Left && m.dragging == "" && m.owner == nowhere {
			if d := m.layout.dividerAt(e.X, e.Y); d != noDivider {
				m.grab, m.hover = d, d
				return true
			}
		}
	case pointer.Move:
		if m.grab != noDivider {
			if !e.Held { // the button was released out of our sight
				m.finishDividerDrag()
				return false
			}
			m.wantPalette, m.wantInspector = widthsFor(m.w, m.layout.palette.W, m.layout.inspector.W, m.grab, e.X)
			m.relayout()
			return true
		}
		m.hover = m.layout.dividerAt(e.X, e.Y)
	case pointer.Up:
		if m.grab != noDivider {
			m.finishDividerDrag()
			return true
		}
	}
	return false
}

func (m *Model) finishDividerDrag() {
	m.grab = noDivider
	m.saveLayout()
}

// separator draws the column between two panes, lit while it can be dragged.
func (m *Model) separator(d divider) string {
	if m.grab == d || m.hover == d {
		return theme.Title("┃")
	}
	return theme.Faded("│")
}

// savedLayout is what is remembered between runs.
type savedLayout struct {
	Palette   int `json:"palette"`
	Inspector int `json:"inspector"`
}

// RestoreLayout brings back the side bar widths and the enabled packs used last time and starts
// remembering changes. Front ends call it once at start; tests do not, so they
// never touch the user's settings.
func (m *Model) RestoreLayout() {
	dir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	m.layoutFile = filepath.Join(dir, "cuppa", "layout.json")
	m.restoreLayoutFrom(m.layoutFile)
	m.restorePacks()
	m.restorePrefs(filepath.Join(dir, "cuppa", "prefs.json"))
}

func (m *Model) restoreLayoutFrom(path string) {
	m.layoutFile = path
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var s savedLayout
	if json.Unmarshal(data, &s) != nil {
		return
	}
	m.wantPalette, m.wantInspector = s.Palette, s.Inspector
	if m.w > 0 {
		m.relayout()
	}
}

func (m *Model) saveLayout() {
	if m.layoutFile == "" {
		return
	}
	data, err := json.Marshal(savedLayout{Palette: m.layout.palette.W, Inspector: m.layout.inspector.W})
	if err != nil || os.MkdirAll(filepath.Dir(m.layoutFile), 0o755) != nil {
		return
	}
	_ = os.WriteFile(m.layoutFile, data, 0o644)
}

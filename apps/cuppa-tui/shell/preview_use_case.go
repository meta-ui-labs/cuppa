package shell

import (
	tea "charm.land/bubbletea/v2"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/menubar"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/pointer"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/preview"
)

// Previewing reports whether the design is being run instead of edited.
func (m *Model) Previewing() bool { return m.run != nil }

// togglePreview starts running the design, or goes back to editing.
func (m *Model) togglePreview() {
	if m.run != nil {
		m.stopPreview()
		return
	}
	m.ed.Clear()
	run, start := preview.Start(m.ed.Document(), m.cat)
	m.run = run
	m.stg.SetRenderer(run.Render)
	m.pending = append(m.pending, start)
	m.flow.SetStatus(previewStatus(run))
}

// previewStatus says what the preview can do with this design.
func previewStatus(run *preview.Session) string {
	if run.Count() == 0 {
		return "Preview: nothing here responds yet; Bubbles components do"
	}
	return "Preview: click a component to use it, Tab moves between them, Esc goes back to editing"
}

// stopPreview goes back to editing.
func (m *Model) stopPreview() {
	if m.run == nil {
		return
	}
	m.run.Stop()
	m.run = nil
	m.stg.SetRenderer(nil)
	m.flow.SetStatus("")
}

// keepsPreview is true for menu actions that do not change what is shown.
func keepsPreview(a menubar.Action) bool {
	switch a {
	case menubar.ViewPreview, menubar.ViewCursor, menubar.HelpShortcuts, menubar.HelpAbout,
		menubar.ExportANSI, menubar.ExportText, menubar.ExportPNG, menubar.ExportSVG, menubar.ExportWebP:
		return true
	}
	return false
}

// previewMouse hands pointer events over the canvas to the running design.
func (m *Model) previewMouse(e pointer.Event) {
	m.mouseX, m.mouseY = e.X, e.Y
	if m.layout.paneAt(e.X, e.Y) == inInspector {
		// The right bar is idle while the design runs, except for the button
		// that goes back to editing.
		m.ins.HandleMode(e.Translate(m.layout.inspector.X, m.layout.inspector.Y))
		return
	}
	if m.layout.paneAt(e.X, e.Y) != inStage {
		return
	}
	cx, cy, over := m.stageCell(e.X, e.Y)
	if !over {
		return
	}
	switch {
	case e.Phase == pointer.Down && e.Left:
		m.pending = append(m.pending, m.run.Press(cx, cy))
	case e.Phase == pointer.Wheel && e.WheelY != 0:
		m.pending = append(m.pending, m.run.Wheel(cx, cy, e.WheelY))
	}
}

// previewKey gives a key to the running design; Esc leaves the preview.
func (m *Model) previewKey(msg tea.KeyPressMsg) {
	if msg.Key().Code == tea.KeyEscape {
		m.stopPreview()
		return
	}
	_, cmd := m.run.Key(msg)
	m.pending = append(m.pending, cmd)
}

// takePending returns the commands the preview asked for since the last call.
func (m *Model) takePending() tea.Cmd {
	cmds := m.pending
	m.pending = nil
	return tea.Batch(cmds...)
}

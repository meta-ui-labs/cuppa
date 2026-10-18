package shell

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/confirm"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/modal"
)

// savedPrefs is what the user chose about the canvas cursor.
type savedPrefs struct {
	CursorOff   bool `json:"cursorOff"`
	CursorAsked bool `json:"cursorAsked"`
}

// restorePrefs brings back the canvas cursor choice and starts remembering it.
// RestoreLayout calls it, so tests never touch the user's settings (and are
// never asked about the cursor).
func (m *Model) restorePrefs(path string) {
	m.prefsFile = path
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var s savedPrefs
	if json.Unmarshal(data, &s) == nil {
		m.cursorOff, m.cursorAsked = s.CursorOff, s.CursorAsked
	}
}

func (m *Model) savePrefs() {
	if m.prefsFile == "" {
		return
	}
	data, err := json.Marshal(savedPrefs{CursorOff: m.cursorOff, CursorAsked: m.cursorAsked})
	if err != nil || os.MkdirAll(filepath.Dir(m.prefsFile), 0o755) != nil {
		return
	}
	_ = os.WriteFile(m.prefsFile, data, 0o644)
}

// setCursorOff shows or hides the canvas cursor and remembers it. Choosing it
// by hand also settles the question of asking.
func (m *Model) setCursorOff(off bool) {
	m.cursorOff, m.cursorAsked = off, true
	m.savePrefs()
	if off {
		m.say("Canvas cursor hidden")
	} else {
		m.say("Canvas cursor shown")
	}
}

// askAboutCursor offers to hide the canvas cursor the first time the mouse is
// used: someone clicking around is unlikely to place components from the
// keyboard, and the cursor then looks like a stray mark. It asks once.
func (m *Model) askAboutCursor() {
	if m.prefsFile == "" || m.cursorOff || m.cursorAsked || m.flow.Modal() != nil || m.run != nil || m.dragging != "" {
		return
	}
	m.cursorAsked = true
	m.savePrefs()
	body := "The ┼ on the canvas is the keyboard cursor: it marks where\n" +
		"a component is placed with Enter. Switch it off?\n\n" +
		"You can bring it back in View > Canvas cursor."
	m.flow.Show(confirm.New("Canvas cursor", body, "Hide", "Keep"), func(o modal.Outcome) {
		if !o.Canceled && o.Button == "Hide" {
			m.setCursorOff(true)
		}
	})
}

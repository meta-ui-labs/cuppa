package prompt

import "testing"

func TestTabMovesToCancelAndEnterCancels(t *testing.T) {
	m := New("Name", "Name", "thing")
	m.Place(80, 24)
	m.Nav("tab") // OK
	m.Nav("tab") // Cancel
	m.Key("", false, true, false)
	out, done := m.Outcome()
	if !done || !out.Canceled {
		t.Fatalf("outcome %+v, want canceled", out)
	}
}

func TestTypingWhileAButtonHasTheKeyboardChangesNothing(t *testing.T) {
	m := New("Name", "Name", "ab")
	m.Place(80, 24)
	m.Nav("tab")
	m.Key("x", false, false, false)
	if string(m.text) != "ab" {
		t.Fatalf("text = %q", string(m.text))
	}
	m.Nav("shift+tab") // back to the field
	m.Key("c", false, false, false)
	if string(m.text) != "c" {
		t.Fatalf("text = %q, want c (the first key replaces the prefilled text)", string(m.text))
	}
}

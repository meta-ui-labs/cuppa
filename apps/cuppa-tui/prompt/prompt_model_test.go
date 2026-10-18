package prompt

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/meta-ui-labs/cuppa/apps/cuppa-tui/pointer"
)

func open(initial string) *Model {
	m := New("Save as component", "Name", initial)
	m.Place(100, 40)
	return m
}

func click(m *Model, x, y int) {
	m.Handle(pointer.Event{X: m.Rect().X + x, Y: m.Rect().Y + y, Phase: pointer.Down, Left: true})
}

func TestTypingAndEnterReturnTheText(t *testing.T) {
	m := open("Card")
	m.Key("s", false, false, false) // replaces the prefilled "Card"
	m.Key("a", false, false, false)
	m.Key("", true, false, false)
	m.Key("!", false, false, false)
	m.Key("", false, true, false)
	o, done := m.Outcome()
	if !done || o.Canceled || o.Value != "s!" {
		t.Fatalf("outcome = %+v %v", o, done)
	}
}

func TestAnEmptyNameIsNotAccepted(t *testing.T) {
	m := open("")
	m.Key("", false, true, false)
	if _, done := m.Outcome(); done {
		t.Fatal("empty text must not confirm")
	}
	if !strings.Contains(ansi.Strip(strings.Join(m.Lines(), "\n")), "Type a name first") {
		t.Fatal("it says why")
	}
}

func TestEscAndCancelEndWithoutAValue(t *testing.T) {
	m := open("x")
	m.Key("", false, false, true)
	if o, done := m.Outcome(); !done || !o.Canceled {
		t.Fatalf("Esc: %+v", o)
	}
	m = open("x")
	click(m, m.Rect().W-6, m.Rect().H-2)
	if o, done := m.Outcome(); !done || !o.Canceled {
		t.Fatalf("Cancel button: %+v", o)
	}
}

func TestTheOKButtonConfirms(t *testing.T) {
	m := open("Card")
	click(m, m.Rect().W-2-ansi.StringWidth(noText)-1-3, m.Rect().H-2)
	if o, done := m.Outcome(); !done || o.Canceled || o.Value != "Card" {
		t.Fatalf("OK button: %+v %v", o, done)
	}
}

func TestALongTextShowsItsEndWhereTheCursorIs(t *testing.T) {
	m := open("/a/very/long/folder/that/does/not/fit/in/the/field/untitled-app")
	shown := ansi.Strip(strings.Join(m.Lines(), "\n"))
	if !strings.Contains(shown, "untitled-app") || !strings.Contains(shown, "…") {
		t.Fatalf("the end of the text stays visible:\n%s", shown)
	}
}

func TestBackspaceClearsThePrefilledTextAtOnce(t *testing.T) {
	m := open("Card")
	m.Key("", true, false, false)
	if len(m.text) != 0 {
		t.Fatalf("text = %q, want empty", string(m.text))
	}
	m.Key("a", false, false, false)
	m.Key("b", false, false, false)
	m.Key("", true, false, false)
	if string(m.text) != "a" {
		t.Fatalf("text = %q, want a", string(m.text))
	}
}

func TestTheCursorMovesSoTextCanBeAddedInTheMiddle(t *testing.T) {
	m := open("Card")
	m.Nav("left") // deselects, cursor at the start
	m.Nav("end")
	m.Nav("left")
	m.Nav("left")
	m.Key("X", false, false, false)
	if string(m.text) != "CaXrd" {
		t.Fatalf("text = %q, want CaXrd", string(m.text))
	}
	m.Nav("delete")
	m.Key("", true, false, false)
	if string(m.text) != "Cad" {
		t.Fatalf("text = %q, want Cad", string(m.text))
	}
	m.Nav("home")
	m.Key(">", false, false, false)
	if string(m.text) != ">Cad" {
		t.Fatalf("text = %q, want >Cad", string(m.text))
	}
}

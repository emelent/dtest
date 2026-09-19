package ui

import (
	"strings"
	"testing"
)

// The keys a shipped dtest starts with must not collide, since a collision
// would silently swallow whichever action lost.
func TestDefaultKeysDoNotClash(t *testing.T) {
	if _, err := buildKeymap(defaultKeys); err != nil {
		t.Fatal(err)
	}
	for act := range defaultKeys {
		if len(defaultKeys[act]) == 0 {
			t.Errorf("%s is bound to nothing", act)
		}
	}
}

// A config rebinds an action outright: the keys it names replace the
// default, and the old ones stop working.
func TestApplyKeys(t *testing.T) {
	t.Cleanup(resetKeys)
	if err := ApplyKeys(map[string][]string{"quit": {"Q", "ctrl+q"}}); err != nil {
		t.Fatal(err)
	}
	if action("Q") != actQuit || action("ctrl+q") != actQuit {
		t.Errorf("Q should quit now")
	}
	if action("q") != actNone {
		t.Errorf("q should be free again, got %q", action("q"))
	}
	if action("j") != actDown {
		t.Errorf("everything else keeps its keys")
	}
	// The model obeys the new binding.
	m := newTestModel(t)
	if _, cmd := m.handleKey(keyPress("Q")); cmd == nil {
		t.Error("Q should now ask to quit")
	}
	// Binding an action to nothing unbinds it, and the help says so.
	if err := ApplyKeys(map[string][]string{"quit": {}}); err != nil {
		t.Fatal(err)
	}
	if action("q") != actNone || keyFor(actQuit) != "–" {
		t.Errorf("quit should be unbound, help shows %q", keyFor(actQuit))
	}
}

// A config that names an action or a key wrongly is refused whole, so a
// typo cannot leave half a keymap behind.
func TestApplyKeysRefusesAndKeepsTheOldMap(t *testing.T) {
	t.Cleanup(resetKeys)
	for name, binds := range map[string]map[string][]string{
		"unknown action": {"teleport": {"z"}},
		"clash":          {"quit": {"j"}},
		"empty key":      {"quit": {""}},
	} {
		err := ApplyKeys(binds)
		if err == nil {
			t.Errorf("%s should be refused", name)
		}
		if action("q") != actQuit || action("j") != actDown {
			t.Errorf("%s: the old bindings should still stand", name)
		}
	}
}

// The help screen prints the keys that are bound now, not the ones that
// were bound when it was written.
func TestHelpFollowsTheBindings(t *testing.T) {
	t.Cleanup(resetKeys)
	m := newTestModel(t)
	if err := ApplyKeys(map[string][]string{"run-all": {"Z"}}); err != nil {
		t.Fatal(err)
	}
	press(m, "?")
	v := view(m)
	if !strings.Contains(v, "press Z") {
		t.Errorf("the help should show the new key:\n%s", v)
	}
	if strings.Contains(v, "press A ") {
		t.Errorf("and not the old one:\n%s", v)
	}
}

// A config repaints the palette, and every style is rebuilt from it.
func TestApplyColors(t *testing.T) {
	t.Cleanup(resetTheme)
	before := styleFailed.Render("x")
	if err := ApplyColors(map[string]string{"failed": "#ff5f87", "selection": "17"}); err != nil {
		t.Fatal(err)
	}
	if styleFailed.Render("x") == before {
		t.Error("the failed colour should have changed")
	}
	if !strings.Contains(styleFailed.Render("x"), "255;95;135") {
		t.Errorf("a hex colour should render as truecolor: %q", styleFailed.Render("x"))
	}
	if bgSelected != "\x1b[48;5;17m" {
		t.Errorf("selection background = %q", bgSelected)
	}
	// Styles derived from the same colour follow it, and so do the rules
	// that colour raw dotnet output.
	if !strings.Contains(styleLogError.Render("x"), "255;95;135") {
		t.Error("log errors share the failed colour")
	}
	if colorLine("Program.cs(3,5): error CS1002: ; expected") == "Program.cs(3,5): error CS1002: ; expected" {
		t.Error("the log rules should still style after a repaint")
	}
}

func TestApplyColorsRefuses(t *testing.T) {
	t.Cleanup(resetTheme)
	before := styleFailed.Render("x")
	for name, colors := range map[string]map[string]string{
		"unknown role": {"chartreuse": "1"},
		"not a colour": {"failed": "puce"},
		"out of range": {"failed": "999"},
		"short hex":    {"failed": "#fff"},
	} {
		if err := ApplyColors(colors); err == nil {
			t.Errorf("%s should be refused", name)
		}
		if styleFailed.Render("x") != before {
			t.Errorf("%s: the palette should be untouched", name)
		}
	}
}

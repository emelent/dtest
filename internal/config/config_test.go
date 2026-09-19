package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad(t *testing.T) {
	path := write(t, `
# a comment, and a table that is not there at all is fine
[keys]
quit = "Q"
down = ["j", "down"]

[colors]
failed = "#ff5f87"
passed = "34"
`)
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(c.Keys["quit"], ","); got != "Q" {
		t.Errorf("a lone key reads as a list of one, got %q", got)
	}
	if got := strings.Join(c.Keys["down"], ","); got != "j,down" {
		t.Errorf("down = %q", got)
	}
	if c.Colors["failed"] != "#ff5f87" || c.Colors["passed"] != "34" {
		t.Errorf("colors = %v", c.Colors)
	}
}

// Having no config file is the normal case, not an error.
func TestLoadMissing(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nothing.toml"))
	if err != nil || c.Keys != nil || c.Colors != nil {
		t.Errorf("missing file: %v %v", c, err)
	}
	if c, err := Load(""); err != nil || c.Keys != nil {
		t.Errorf("no path: %v %v", c, err)
	}
}

func TestLoadBroken(t *testing.T) {
	for name, body := range map[string]string{
		"not toml":    "[keys\nquit = ",
		"wrong shape": "[keys]\nquit = 3\n",
		"mixed list":  "[keys]\nquit = [\"q\", 3]\n",
	} {
		if _, err := Load(write(t, body)); err == nil {
			t.Errorf("%s should be an error", name)
		}
	}
}

func TestPath(t *testing.T) {
	t.Setenv("DTEST_CONFIG", "/somewhere/else.toml")
	if got := Path(); got != "/somewhere/else.toml" {
		t.Errorf("DTEST_CONFIG should win, got %q", got)
	}
	t.Setenv("DTEST_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got := Path(); got != filepath.Join("/xdg", "dtest", "config.toml") {
		t.Errorf("XDG path = %q", got)
	}
}

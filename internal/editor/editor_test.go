package editor

import (
	"os/exec"
	"reflect"
	"testing"
)

func TestResolveSocket(t *testing.T) {
	t.Setenv("nvim_sock", "")
	t.Setenv("NVIM_SOCK", "")
	if got := ResolveSocket(""); got != "" {
		t.Errorf("nothing set = %q, want empty", got)
	}
	t.Setenv("NVIM_SOCK", "/tmp/upper.sock")
	if got := ResolveSocket(""); got != "/tmp/upper.sock" {
		t.Errorf("$NVIM_SOCK = %q", got)
	}
	t.Setenv("nvim_sock", "/tmp/x.sock")
	if got := ResolveSocket(""); got != "/tmp/x.sock" {
		t.Errorf("$nvim_sock = %q", got)
	}
	if got := ResolveSocket("/tmp/explicit"); got != "/tmp/explicit" {
		t.Errorf("explicit = %q", got)
	}
	if HasServer("/nonexistent/socket") || HasServer("") {
		t.Error("no server should exist")
	}
}

func TestEditExpr(t *testing.T) {
	if got, want := EditExpr("/src/it's here/a b.cs", 7), "execute('edit +7 ' . fnameescape('/src/it''s here/a b.cs'))"; got != want {
		t.Fatalf("EditExpr = %q, want %q", got, want)
	}
	if got, want := EditExpr("/src/a.cs", 0), "execute('edit ' . fnameescape('/src/a.cs'))"; got != want {
		t.Fatalf("EditExpr = %q, want %q", got, want)
	}
}

func TestOpen(t *testing.T) {
	var calls [][]string
	fail := false
	execCommand = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string{name}, args...))
		if fail {
			return exec.Command("sh", "-c", "echo 'E247: no server' >&2; exit 1")
		}
		return exec.Command("true")
	}
	t.Cleanup(func() { execCommand = exec.Command })

	t.Setenv("TMUX", "")
	if err := Open("/tmp/s.sock", "/src/a.cs", 0); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"nvim", "--server", "/tmp/s.sock", "--remote-expr", "execute('edit ' . fnameescape('/src/a.cs'))"}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	calls = nil
	t.Setenv("TMUX", "/tmp/tmux-501/default,1,0")
	if err := Open("/tmp/s.sock", "/src/a.cs", 42); err != nil {
		t.Fatal(err)
	}
	want = [][]string{
		{"nvim", "--server", "/tmp/s.sock", "--remote-expr", "execute('edit +42 ' . fnameescape('/src/a.cs'))"},
		{"tmux", "select-window", "-t", ":code"},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	calls, fail = nil, true
	err := Open("/tmp/s.sock", "x", 1)
	if err == nil || err.Error() != "nvim --remote-expr: E247: no server" || len(calls) != 1 {
		t.Fatalf("err=%v calls=%v", err, calls)
	}
	execCommand = exec.Command
	if cmd := LaunchCmd("/src/a.cs", 12); !reflect.DeepEqual(cmd.Args[1:], []string{"+12", "/src/a.cs"}) {
		t.Errorf("LaunchCmd args = %v", cmd.Args)
	}
}

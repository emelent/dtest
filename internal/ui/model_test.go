package ui

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"dtest/internal/dotnet"
	"dtest/internal/tree"
)

const projA = "/src/Alpha.Tests/Alpha.Tests.csproj"

// newTestModel returns a 120x30 model with one project listed and no
// dotnet calls made.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	m := New(Config{Target: "/src/Sample.slnx", Socket: "/tmp/nvim.Sample.sock"})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	p := m.tree.AddProject(projA)
	m.tree.SetTests(p, []string{
		"Alpha.Tests.MathTests.Adds",
		"Alpha.Tests.MathTests.Fails",
		"Alpha.Tests.MathTests.Theory(n: 1)",
		"Alpha.Tests.MathTests.Theory(n: 2)",
		"Alpha.Tests.SlowTests.Waits",
	})
	m.rebuildRows()
	m.refreshLog()
	return m
}

func press(m *Model, keys ...string) tea.Cmd {
	var last tea.Cmd
	for _, k := range keys {
		msg := tea.KeyPressMsg{Text: k}
		if len(k) == 1 {
			msg.Code = rune(k[0])
		}
		switch k {
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		case "tab":
			msg = tea.KeyPressMsg{Code: tea.KeyTab}
		case "space":
			msg = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
		case "backspace":
			msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
		case "ctrl+d":
			msg = tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
		case "ctrl+u":
			msg = tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
		}
		_, last = m.handleKey(msg)
	}
	return last
}

func view(m *Model) string { return ansi.Strip(m.View().Content) }

func TestNavigationAndFolding(t *testing.T) {
	m := newTestModel(t)
	// rows: project, namespace, MathTests, Adds, Fails, Theory, SlowTests, Waits
	if len(m.rows) != 8 {
		t.Fatalf("rows = %d", len(m.rows))
	}
	press(m, "j", "j", "j")
	if m.current().Name != "Adds" {
		t.Fatalf("cursor on %q", m.current().Name)
	}
	press(m, "G")
	if m.current().Name != "Waits" {
		t.Fatalf("G -> %q", m.current().Name)
	}
	press(m, "g", "g")
	if m.cursor != 0 {
		t.Fatalf("gg -> %d", m.cursor)
	}
	press(m, "j", "j", "h") // collapse MathTests
	if len(m.rows) != 5 || m.current().Name != "MathTests" {
		t.Fatalf("after h: %d rows, on %q", len(m.rows), m.current().Name)
	}
	press(m, "h") // to parent namespace
	if m.current().Kind != tree.KindNamespace {
		t.Fatalf("h on collapsed node should go to parent, got %v", m.current().Kind)
	}
	press(m, "j", "l") // expand again
	if len(m.rows) != 8 {
		t.Fatalf("after l: %d rows", len(m.rows))
	}
	press(m, "l") // into first child
	if m.current().Name != "Adds" {
		t.Fatalf("l on expanded -> %q", m.current().Name)
	}
	press(m, "j", "j", "enter") // expand theory rows
	if !strings.Contains(view(m), "(n: 1)") {
		t.Fatal("theory rows should show after enter")
	}
	press(m, "H")
	if len(m.rows) != 2 {
		t.Fatalf("H -> %d rows", len(m.rows))
	}
	press(m, "L")
	if len(m.rows) != 10 {
		t.Fatalf("L -> %d rows", len(m.rows))
	}
	press(m, "ctrl+d")
	if m.cursor == 0 {
		t.Fatal("ctrl+d should move")
	}
	press(m, "ctrl+u")
	if m.cursor != 0 {
		t.Fatalf("ctrl+u -> %d", m.cursor)
	}
}

func TestSearch(t *testing.T) {
	m := newTestModel(t)
	press(m, "/", "w", "a", "i")
	if !m.searching || m.query != "wai" {
		t.Fatalf("searching=%v query=%q", m.searching, m.query)
	}
	if len(m.rows) != 4 || m.rows[3].Name != "Waits" {
		t.Fatalf("filtered rows = %d", len(m.rows))
	}
	if v := view(m); !strings.Contains(v, "/wai") || strings.Contains(v, "\t") {
		t.Fatalf("status bar should show the query and rows must not contain tabs:\n%s", v)
	}
	press(m, "x", "y", "z")
	if len(m.rows) != 0 || !strings.Contains(view(m), "No tests match /waixyz") {
		t.Fatalf("no-match state: %d rows\n%s", len(m.rows), view(m))
	}
	press(m, "backspace", "backspace", "backspace")
	// Capitals arrive with the shift modifier set and must still be typed.
	m.handleKey(tea.KeyPressMsg{Code: 'T', Text: "T", Mod: tea.ModShift})
	m.handleKey(tea.KeyPressMsg{Code: 'x', Text: "x", Mod: tea.ModCtrl})
	if m.query != "waiT" {
		t.Fatalf("query after shift+T and ctrl+x = %q", m.query)
	}
	press(m, "backspace", "backspace", "enter")
	if m.searching || m.query != "wa" {
		t.Fatalf("after enter: searching=%v query=%q", m.searching, m.query)
	}
	press(m, "esc")
	if m.query != "" || len(m.rows) != 8 {
		t.Fatalf("esc should clear the filter: %q %d", m.query, len(m.rows))
	}
}

// fakeRun replaces dotnet.Run, recording the invocation and letting the test
// feed events.
type fakeRun struct {
	project, filter string
	emit            func(dotnet.Event)
	started         chan struct{}
}

func stubRun(t *testing.T) *fakeRun {
	t.Helper()
	f := &fakeRun{started: make(chan struct{}, 8)}
	runTests = func(ctx context.Context, project, filter string, opts dotnet.Options, emit func(dotnet.Event)) {
		f.project, f.filter, f.emit = project, filter, emit
		f.started <- struct{}{}
	}
	t.Cleanup(func() { runTests = dotnet.Run })
	return f
}

func TestRunFlow(t *testing.T) {
	f := stubRun(t)
	m := newTestModel(t)
	press(m, "j", "j") // MathTests
	press(m, "r")
	<-f.started
	if f.project != projA || f.filter != "FullyQualifiedName~Alpha.Tests.MathTests." {
		t.Fatalf("run %q %q", f.project, f.filter)
	}
	class := m.current()
	if class.Status() != tree.StatusRunning || class.Counts().Running != 4 {
		t.Fatalf("class should be running: %+v", class.Counts())
	}
	if !strings.Contains(view(m), "running Alpha.Tests: MathTests") {
		t.Fatalf("header should show the run:\n%s", view(m))
	}
	// Events arrive through the channel; deliver them as Update would, with
	// the cursor sitting on Adds so its detail pane must update in place.
	press(m, "j")
	if v := view(m); !strings.Contains(v, "Test: Adds") || !strings.Contains(v, "Running…") {
		t.Fatalf("detail while running:\n%s", v)
	}
	f.emit(dotnet.LineEvent{Text: "  Starting: Alpha.Tests"})
	f.emit(dotnet.ResultEvent{Result: dotnet.Result{Name: "Alpha.Tests.MathTests.Adds", Outcome: dotnet.OutcomePassed}})
	for i := 0; i < 2; i++ {
		m.Update(<-m.runEvents())
	}
	adds := m.tree.Lookup(class.Project(), "Alpha.Tests.MathTests.Adds")
	if adds.Status() != tree.StatusPassed {
		t.Fatal("Adds should be passed after the result line")
	}
	if v := view(m); !strings.Contains(v, "Test: Adds") || !strings.Contains(v, "✓ Passed in") {
		t.Fatalf("detail after result:\n%s", v)
	}
	press(m, "k")
	f.emit(dotnet.DoneEvent{Results: []dotnet.Result{
		{Name: "Alpha.Tests.MathTests.Adds", Outcome: dotnet.OutcomePassed},
		{Name: "Alpha.Tests.MathTests.Fails", Outcome: dotnet.OutcomeFailed, Message: "Expected 5", StackTrace: "   at X in /src/Alpha.Tests/UnitTest1.cs:line 12"},
		{Name: "Alpha.Tests.MathTests.Theory(n: 1)", Outcome: dotnet.OutcomePassed},
		{Name: "Alpha.Tests.MathTests.Theory(n: 2)", Outcome: dotnet.OutcomeSkipped},
		{Name: "Alpha.Tests.MathTests.Extra", Outcome: dotnet.OutcomePassed},
	}})
	m.Update(<-m.runEvents())
	if m.run != nil {
		t.Fatal("run should be over")
	}
	c := class.Counts()
	if c.Passed != 3 || c.Failed != 1 || c.Skipped != 1 || c.Running != 0 {
		t.Fatalf("counts = %+v", c)
	}
	if m.tree.Lookup(class.Project(), "Alpha.Tests.MathTests.Extra") == nil {
		t.Fatal("an unlisted test in the results should be added")
	}
	if !strings.Contains(m.status, "3 passed, 1 failed, 1 skipped") || !m.statusErr {
		t.Fatalf("status = %q err=%v", m.status, m.statusErr)
	}
	if v := view(m); strings.Contains(v, "✓3") || strings.Contains(v, "✗1 ") {
		t.Fatalf("rows must not show pass/fail counts:\n%s", v)
	}
	// f jumps to the failed test and the detail pane shows its message.
	press(m, "g", "g", "f")
	if m.current().Name != "Fails" {
		t.Fatalf("f -> %q", m.current().Name)
	}
	if v := view(m); !strings.Contains(v, "Test: Fails") || !strings.Contains(v, "Expected 5") || !strings.Contains(v, "✗ 1") {
		t.Fatalf("detail pane:\n%s", v)
	}
	// s steps through skipped tests the same way.
	press(m, "g", "g", "s")
	if m.current().Name != "(n: 2)" || m.current().Status() != tree.StatusSkipped {
		t.Fatalf("s -> %q", m.current().Name)
	}
	press(m, "S")
	if m.current().Name != "(n: 2)" {
		t.Fatalf("S with one skipped test should stay put, got %q", m.current().Name)
	}
	// e re-runs the failed tests only.
	press(m, "e")
	<-f.started
	if f.filter != "FullyQualifiedName=Alpha.Tests.MathTests.Fails" {
		t.Fatalf("rerun-failed filter = %q", f.filter)
	}
	press(m, "x")
	f.emit(dotnet.DoneEvent{Err: context.Canceled})
	m.Update(<-m.runEvents())
	// Marked nodes across the tree run together in one filter per project.
	press(m, "g", "g", "j", "j", "j", "m", "G", "m")
	if got := len(m.tree.Marked()); got != 2 {
		t.Fatalf("marked = %d", got)
	}
	press(m, "r")
	<-f.started
	if f.filter != "FullyQualifiedName=Alpha.Tests.MathTests.Adds|FullyQualifiedName=Alpha.Tests.SlowTests.Waits" {
		t.Fatalf("marked filter = %q", f.filter)
	}
	// A second request queues behind the active run; R runs whole projects.
	press(m, "R")
	if len(m.queue) != 1 || m.queue[0].filter != "" {
		t.Fatalf("queue = %+v", m.queue)
	}
	press(m, "x")
	if len(m.queue) != 0 {
		t.Fatal("x should drop the queue")
	}
	f.emit(dotnet.DoneEvent{Err: context.Canceled})
	_, _ = m.Update(<-m.runEvents())
	if m.run != nil || m.tree.Lookup(class.Project(), "Alpha.Tests.SlowTests.Waits").Status() != tree.StatusNone {
		t.Fatal("cancelled leaves should return to StatusNone")
	}
}

func TestRerunFailedWithNothingFailed(t *testing.T) {
	m := newTestModel(t)
	press(m, "e")
	if !strings.Contains(m.status, "No failed tests to re-run") {
		t.Fatalf("status = %q", m.status)
	}
	press(m, "s")
	if !strings.Contains(m.status, "No skipped tests") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestColorLog(t *testing.T) {
	lines := []string{
		"$ dotnet test x",
		"  Passed Ns.C.M [1 ms]",
		"  Failed Ns.C.N [1 ms]",
		"  Skipped Ns.C.O",
		"[xUnit.net 00:00:00.01]   Starting: X",
		"Program.cs(3,5): error CS1002: ; expected",
		"Program.cs(3,5): warning CS0168: unused",
		"   at Ns.C.N() in /x.cs:line 3",
		"Build succeeded.",
		"just some output",
	}
	out := colorLog(lines)
	for i, l := range lines {
		if ansi.Strip(out[i]) != l {
			t.Errorf("line %d text changed: %q", i, ansi.Strip(out[i]))
		}
	}
	if out[9] != lines[9] {
		t.Errorf("plain output must stay unstyled: %q", out[9])
	}
	for i := 0; i < 9; i++ {
		if out[i] == lines[i] {
			t.Errorf("line %d should be styled: %q", i, lines[i])
		}
	}
	if out[1] == out[2] || out[2] != styleFailed.Render(lines[2]) || out[5] != styleLogError.Render(lines[5]) {
		t.Error("result and error lines should carry their own styles")
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{
		0:                              "0.000s",
		32 * time.Millisecond:          "0.032s",
		999 * time.Millisecond:         "0.999s",
		1500 * time.Millisecond:        "1.5s",
		9949 * time.Millisecond:        "9.9s",
		35 * time.Second:               "35s",
		35400 * time.Millisecond:       "35s",
		2*time.Minute + 34*time.Second: "2m34s",
		12*time.Minute + 5*time.Second: "12m05s",
		time.Hour + 2*time.Minute:      "1h02m",
		3*time.Hour + 59*time.Minute + 40*time.Second: "4h00m",
	}
	for d, want := range cases {
		if got := formatDuration(d); got != want {
			t.Errorf("formatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestDurationsInTree(t *testing.T) {
	m := newTestModel(t)
	p := m.tree.Projects[0]
	adds := m.tree.Lookup(p, "Alpha.Tests.MathTests.Adds")
	adds.Result = &dotnet.Result{Name: adds.FQN, Outcome: dotnet.OutcomePassed, Duration: 32 * time.Millisecond}
	adds.SetStatus(tree.StatusPassed)
	waits := m.tree.Lookup(p, "Alpha.Tests.SlowTests.Waits")
	waits.Result = &dotnet.Result{Name: waits.FQN, Outcome: dotnet.OutcomePassed, Duration: 2*time.Minute + 34*time.Second}
	waits.SetStatus(tree.StatusPassed)
	m.refreshLog()
	v := view(m)
	for _, want := range []string{"Adds", "0.032s", "Waits", "2m34s", "2m34s"} {
		if !strings.Contains(v, want) {
			t.Fatalf("view should contain %q:\n%s", want, v)
		}
	}
	// The project row sums its tests; the class without results shows none.
	for _, line := range strings.Split(v, "\n") {
		switch {
		case strings.Contains(line, "▾ ✓ Alpha.Tests") && strings.Contains(line, "✓2"):
			if !strings.Contains(line, "2m34s") {
				t.Fatalf("project row should show the summed time: %q", line)
			}
		case strings.Contains(line, "Fails"):
			treePart, _, _ := strings.Cut(line, "│")
			if !strings.HasSuffix(strings.TrimRight(treePart, " "), "Fails") {
				t.Fatalf("unrun test must show no time: %q", line)
			}
		}
	}
}

func TestOpenInEditor(t *testing.T) {
	m := newTestModel(t)
	var opened []string
	hasServer = func(string) bool { return true }
	openInServer = func(sock, path string, line int) error {
		opened = append(opened, sock, path)
		return nil
	}
	locate = func(dir, class, method string) (dotnet.Location, bool) {
		return dotnet.Location{File: dir + "/UnitTest1.cs", Line: 7}, class == "Alpha.Tests.MathTests"
	}
	t.Cleanup(func() { hasServer, openInServer, locate = nil, nil, dotnet.Locate })

	press(m, "j", "j", "j", "o") // Adds
	if len(opened) != 2 || opened[0] != "/tmp/nvim.Sample.sock" || opened[1] != "/src/Alpha.Tests/UnitTest1.cs" {
		t.Fatalf("opened = %v", opened)
	}
	if !strings.Contains(m.status, "Sent UnitTest1.cs:7") {
		t.Fatalf("status = %q", m.status)
	}
	press(m, "g", "g", "j", "o") // namespace: nothing to open
	if !m.statusErr {
		t.Fatalf("namespace should report no location, status=%q", m.status)
	}
	press(m, "g", "g", "o") // project opens its file
	if opened[len(opened)-1] != projA {
		t.Fatalf("project open = %v", opened)
	}
	// Without a listening server, nvim is launched in the terminal (an
	// ExecProcess command), unless nvim is not installed.
	hasServer = func(string) bool { return false }
	calls := len(opened)
	lookPath = func(string) (string, error) { return "/usr/bin/nvim", nil }
	t.Cleanup(func() { lookPath = exec.LookPath })
	if cmd := press(m, "o"); cmd == nil || len(opened) != calls {
		t.Fatal("o without a server should launch nvim in place")
	}
	lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	press(m, "o")
	if !m.statusErr || !strings.Contains(m.status, "nvim not found") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestLogPaneAndHelp(t *testing.T) {
	m := newTestModel(t)
	m.logs[buildLogKey] = []string{"$ dotnet build", "Build succeeded."}
	m.refreshLog()
	if v := view(m); !strings.Contains(v, "Log: build") || !strings.Contains(v, "Build succeeded.") {
		t.Fatalf("build log should show:\n%s", v)
	}
	press(m, "tab")
	if m.focus != paneLog {
		t.Fatal("tab should focus the log")
	}
	press(m, "h")
	if m.focus != paneTree {
		t.Fatal("h should return to the tree")
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40}) // the full help needs the room
	press(m, "?")
	if v := view(m); !strings.Contains(v, "run the marked nodes") || !strings.Contains(v, "/tmp/nvim.Sample.sock") {
		t.Fatalf("help:\n%s", v)
	}
	m.socket = ""
	if v := view(m); !strings.Contains(v, "$nvim_sock is not set, so o opens nvim in this terminal") {
		t.Fatalf("help:\n%s", v)
	}
	press(m, "q")
	if m.help {
		t.Fatal("any key should close help")
	}
	// Narrow terminals show one pane.
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	if v := view(m); strings.Contains(v, "Log: build") {
		t.Fatal("narrow view should hide the log pane")
	}
	press(m, "tab")
	if v := view(m); !strings.Contains(v, "Log: build") || strings.Contains(v, " Tests ") {
		t.Fatalf("narrow view with log focused:\n%s", v)
	}
}

func (m *Model) runEvents() <-chan tea.Msg { return m.run.events }

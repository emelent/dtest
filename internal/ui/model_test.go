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

// newTestModel returns a 120x40 model with one project listed and no
// dotnet calls made.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	m := New(Config{Target: "/src/Sample.slnx", Socket: "/tmp/nvim.Sample.sock"})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	p := m.tree.AddProject(projA)
	m.tree.SetTests(p, []string{
		"Alpha.Tests.MathTests.Adds",
		"Alpha.Tests.MathTests.Fails",
		"Alpha.Tests.MathTests.Theory(n: 1)",
		"Alpha.Tests.MathTests.Theory(n: 2)",
		"Alpha.Tests.SlowTests.Waits",
	})
	m.refresh()
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

// markedLine is the visible report line carrying the cursor marker.
func markedLine(m *Model) string {
	for _, l := range strings.Split(view(m), "\n") {
		if strings.HasPrefix(l, iconArrow) {
			return l
		}
	}
	return ""
}

func (m *Model) node() *tree.Node {
	r, _ := m.current()
	return r.node
}

func TestNavigationAndFolding(t *testing.T) {
	m := newTestModel(t)
	// Fresh: project, then its two classes collapsed.
	if len(m.rows) != 3 || m.rows[1].node.Name != "MathTests" {
		t.Fatalf("rows = %d", len(m.rows))
	}
	press(m, "j", "l") // expand MathTests
	if len(m.rows) != 6 || m.node().Name != "MathTests" {
		t.Fatalf("after l: %d rows on %q", len(m.rows), m.node().Name)
	}
	press(m, "l") // into first child
	if m.node().Name != "Adds" {
		t.Fatalf("l on expanded -> %q", m.node().Name)
	}
	press(m, "G")
	if m.node().Name != "SlowTests" {
		t.Fatalf("G -> %q", m.node().Name)
	}
	press(m, "g", "g")
	if m.cursor != 0 {
		t.Fatalf("gg -> %d", m.cursor)
	}
	press(m, "j", "j", "j", "j", "space") // Theory: toggle open
	if m.node().Name != "Theory" || !strings.Contains(view(m), "(n: 1)") {
		t.Fatalf("space should expand the theory, on %q", m.node().Name)
	}
	press(m, "h") // collapse it
	if strings.Contains(view(m), "(n: 1)") {
		t.Fatal("h should collapse the theory")
	}
	press(m, "h") // to the parent class
	if m.node().Name != "MathTests" {
		t.Fatalf("h on a collapsed node should select its parent, got %q", m.node().Name)
	}
	press(m, "ctrl+d")
	if m.cursor == 1 {
		t.Fatal("ctrl+d should move")
	}
	press(m, "ctrl+u")
	if m.cursor != 0 {
		t.Fatalf("ctrl+u -> %d", m.cursor)
	}
	v := view(m)
	for _, want := range []string{"DTEST", "Alpha.Tests", "(5 tests)", "MathTests (4 tests)", "Test Projects", "Tests", "Start at", "Duration", "5 tests ready.", "press ? for help"} {
		if !strings.Contains(v, want) {
			t.Errorf("view should contain %q:\n%s", want, v)
		}
	}
}

func TestFilter(t *testing.T) {
	m := newTestModel(t)
	press(m, "t", "w", "a", "i")
	if !m.filtering || m.query != "wai" {
		t.Fatalf("filtering=%v query=%q", m.filtering, m.query)
	}
	if len(m.rows) != 3 || m.rows[2].node.Name != "Waits" {
		t.Fatalf("filtered rows = %d", len(m.rows))
	}
	v := view(m)
	if !strings.Contains(v, "filter: wai") || strings.Contains(v, "\t") {
		t.Fatalf("status should show the filter and rows must not contain tabs:\n%s", v)
	}
	m.handleKey(tea.KeyPressMsg{Code: 'T', Text: "T", Mod: tea.ModShift})
	m.handleKey(tea.KeyPressMsg{Code: 'x', Text: "x", Mod: tea.ModCtrl})
	if m.query != "waiT" {
		t.Fatalf("query after shift+T and ctrl+x = %q", m.query)
	}
	press(m, "x", "y")
	if len(m.rows) != 0 || !strings.Contains(view(m), "No tests match") {
		t.Fatalf("no-match state: %d rows\n%s", len(m.rows), view(m))
	}
	press(m, "backspace", "backspace", "backspace", "enter")
	if m.filtering || m.query != "wai" || !strings.Contains(view(m), "filter: wai") {
		t.Fatalf("after enter: filtering=%v query=%q", m.filtering, m.query)
	}
	press(m, "esc")
	if m.query != "" || len(m.rows) != 3 {
		t.Fatalf("esc should clear the filter: %q %d", m.query, len(m.rows))
	}
	press(m, "/", "a", "l", "p", "h", "a", ".", "t", "esc")
	if m.query != "" {
		t.Fatal("esc while typing clears the filter")
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

func (m *Model) runEvents() <-chan tea.Msg { return m.run.events }

func TestRunFlow(t *testing.T) {
	f := stubRun(t)
	m := newTestModel(t)
	clock := time.Date(2026, 9, 17, 19, 10, 48, 0, time.Local)
	now = func() time.Time { return clock }
	t.Cleanup(func() { now = time.Now })

	press(m, "j", "r") // MathTests
	<-f.started
	if f.project != projA || f.filter != "FullyQualifiedName~Alpha.Tests.MathTests." {
		t.Fatalf("run %q %q", f.project, f.filter)
	}
	class := m.node()
	if class.Status() != tree.StatusRunning || class.Counts().Running != 4 {
		t.Fatalf("class should be running: %+v", class.Counts())
	}
	v := view(m)
	if !strings.Contains(v, "RUN") || !strings.Contains(v, "Alpha.Tests › MathTests") || !strings.Contains(v, "4 running") || !strings.Contains(v, "19:10:48") {
		t.Fatalf("running view:\n%s", v)
	}
	f.emit(dotnet.LineEvent{Text: "  Starting: Alpha.Tests"})
	f.emit(dotnet.ResultEvent{Result: dotnet.Result{Name: "Alpha.Tests.MathTests.Adds", Outcome: dotnet.OutcomePassed, Duration: 32 * time.Millisecond}})
	for i := 0; i < 2; i++ {
		m.Update(<-m.runEvents())
	}
	if m.tree.Lookup(class.Project(), "Alpha.Tests.MathTests.Adds").Status() != tree.StatusPassed {
		t.Fatal("Adds should be passed after the result line")
	}
	clock = clock.Add(2*time.Minute + 34*time.Second)
	f.emit(dotnet.DoneEvent{Results: []dotnet.Result{
		{Name: "Alpha.Tests.MathTests.Adds", Outcome: dotnet.OutcomePassed, Duration: 32 * time.Millisecond},
		{Name: "Alpha.Tests.MathTests.Fails", Outcome: dotnet.OutcomeFailed, Duration: time.Millisecond, Message: "Assert.Equal() Failure: Values differ\nExpected: 5\nActual:   4", StackTrace: "   at Alpha.Tests.MathTests.Fails() in /src/Alpha.Tests/UnitTest1.cs:line 12"},
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
	// The report folds by result: the failing class is open, the theory
	// (one skipped, none failed) closed; the failure section and summary
	// carry the details.
	v = view(m)
	for _, want := range []string{
		"× MathTests (5 tests | 1 failed | 1 skipped)",
		"× Fails 0.001s",
		"→ Assert.Equal() Failure: Values differ",
		"Failed Tests 1",
		"FAIL  Alpha.Tests › MathTests › Fails",
		"Expected: 5",
		"Actual:   4",
		"❯ /src/Alpha.Tests/UnitTest1.cs:12",
		"Test Projects  1 failed (1)",
		"Tests  1 failed | 3 passed | 1 skipped | 1 not run (6)",
		"Duration  2m34s",
		"FAIL  Tests failed.",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("view should contain %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "(n: 1)") {
		t.Error("theory without failures should be folded")
	}
	// n jumps to the failed test and the marker moves with it; G reaches
	// the failure entry, which also selects that test.
	press(m, "g", "g", "n")
	if m.node().Name != "Fails" {
		t.Fatalf("n -> %q", m.node().Name)
	}
	if marked := markedLine(m); !strings.Contains(marked, "× Fails") {
		t.Fatalf("marker should be on Fails, got %q", marked)
	}
	press(m, "G")
	if r, _ := m.current(); !r.failure || r.node.Name != "Fails" {
		t.Fatalf("G should land on the failure entry: %+v", r)
	}
	if marked := markedLine(m); !strings.HasPrefix(marked, "❯  FAIL  Alpha.Tests › MathTests › Fails") {
		t.Fatalf("marker should be on the failure entry, got %q", marked)
	}
	press(m, "g", "g")
	if marked := markedLine(m); !strings.HasPrefix(marked, "❯ × Alpha.Tests (6 tests") {
		t.Fatalf("marker on the project row = %q", marked)
	}
	// f re-runs the failed tests only; a runs whole projects and queues.
	press(m, "f")
	<-f.started
	if f.filter != "FullyQualifiedName=Alpha.Tests.MathTests.Fails" {
		t.Fatalf("rerun-failed filter = %q", f.filter)
	}
	press(m, "a")
	if len(m.queue) != 1 || m.queue[0].filter != "" {
		t.Fatalf("queue = %+v", m.queue)
	}
	press(m, "x")
	if len(m.queue) != 0 {
		t.Fatal("x should drop the queue")
	}
	f.emit(dotnet.DoneEvent{Err: context.Canceled})
	m.Update(<-m.runEvents())
	if m.run != nil || m.tree.Lookup(class.Project(), "Alpha.Tests.MathTests.Fails").Status() != tree.StatusNone {
		t.Fatal("cancelled leaves should return to StatusNone")
	}
}

func TestRunFailedWithNothingFailed(t *testing.T) {
	m := newTestModel(t)
	press(m, "f")
	if !strings.Contains(m.status, "No failed tests to re-run") {
		t.Fatalf("status = %q", m.status)
	}
	press(m, "n")
	if !strings.Contains(m.status, "No failed tests") {
		t.Fatalf("status = %q", m.status)
	}
	press(m, "x")
	if !strings.Contains(m.status, "Nothing is running") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestOutputToggleAndHelp(t *testing.T) {
	m := newTestModel(t)
	m.logs[buildLogKey] = []string{"$ dotnet build", "  Determining projects to restore...", "Build succeeded."}
	m.lastLog = buildLogKey
	m.refresh()
	if v := view(m); strings.Contains(v, "Determining projects") {
		t.Fatalf("output hidden by default:\n%s", v)
	}
	press(m, "v")
	if v := view(m); !strings.Contains(v, "Output  build") || !strings.Contains(v, "Determining projects") || !strings.Contains(v, "Build succeeded.") {
		t.Fatalf("output shown:\n%s", v)
	}
	press(m, "v")
	if strings.Contains(view(m), "Determining projects") {
		t.Fatal("v should hide the output again")
	}
	press(m, "?")
	if v := view(m); !strings.Contains(v, "press f") || !strings.Contains(v, "rerun only the failed tests") || !strings.Contains(v, "/tmp/nvim.Sample.sock") {
		t.Fatalf("help:\n%s", v)
	}
	press(m, "j")
	if m.help || m.cursor != 0 {
		t.Fatal("any key closes help without acting")
	}
	m.socket = ""
	press(m, "?")
	if !strings.Contains(view(m), "$nvim_sock is not set") {
		t.Fatal("help should explain the missing socket")
	}
}

func TestOpenInEditor(t *testing.T) {
	m := newTestModel(t)
	var opened []string
	var lines []int
	hasServer = func(string) bool { return true }
	openInServer = func(sock, path string, line int) error {
		opened = append(opened, sock, path)
		lines = append(lines, line)
		return nil
	}
	locate = func(dir, class, method string) (dotnet.Location, bool) {
		return dotnet.Location{File: dir + "/UnitTest1.cs", Line: 7}, class == "Alpha.Tests.MathTests"
	}
	t.Cleanup(func() { hasServer, openInServer, locate = nil, nil, dotnet.Locate })

	press(m, "j", "l", "j", "o") // Adds
	if len(opened) != 2 || opened[0] != "/tmp/nvim.Sample.sock" || opened[1] != "/src/Alpha.Tests/UnitTest1.cs" || lines[0] != 7 {
		t.Fatalf("opened = %v lines = %v", opened, lines)
	}
	if !strings.Contains(m.status, "Sent UnitTest1.cs:7") {
		t.Fatalf("status = %q", m.status)
	}
	press(m, "g", "g", "o") // project opens its file
	if opened[len(opened)-1] != projA {
		t.Fatalf("project open = %v", opened)
	}
	// A failure entry opens the failing line from the stack trace.
	fails := m.tree.Lookup(m.tree.Projects[0], "Alpha.Tests.MathTests.Fails")
	fails.Result = &dotnet.Result{Outcome: dotnet.OutcomeFailed, Message: "boom", StackTrace: "   at X in /src/Alpha.Tests/UnitTest1.cs:line 12"}
	fails.SetStatus(tree.StatusFailed)
	m.refresh()
	press(m, "G", "o")
	if lines[len(lines)-1] != 12 {
		t.Fatalf("failure entry should open line 12, got %v", lines)
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

func TestColorAssertion(t *testing.T) {
	green, red := styleExpected.Render, styleActual.Render
	cases := map[string]string{
		"Expected: 5":                green("Expected: 5"),
		"Actual:   4":                red("Actual:   4"),
		"  But was:  4":              "  " + red("But was:  4"),
		"Expected:<5>. Actual:<4>. ": green("Expected:<5>. ") + red("Actual:<4>. "),
		"Assert.Equal() Failure":     styleLogError.Render("Assert.Equal() Failure"),
	}
	for in, want := range cases {
		if got := colorMessage(in); got != want {
			t.Errorf("colorMessage(%q) = %q, want %q", in, got, want)
		}
	}
	lines := []string{"$ dotnet test x", "  Passed Ns.C.M [1 ms]", "  Failed Ns.C.N [1 ms]", "[xUnit.net 00:00:00.01] x", "Program.cs(3,5): error CS1002: ; expected", "plain"}
	out := colorLog(lines)
	for i := range lines {
		if ansi.Strip(out[i]) != lines[i] {
			t.Errorf("text changed: %q", out[i])
		}
		if (out[i] == lines[i]) != (i == len(lines)-1) {
			t.Errorf("styling of %q wrong", lines[i])
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{
		0:                              "0.000s",
		32 * time.Millisecond:          "0.032s",
		1500 * time.Millisecond:        "1.5s",
		35400 * time.Millisecond:       "35s",
		2*time.Minute + 34*time.Second: "2m34s",
		time.Hour + 2*time.Minute:      "1h02m",
		3*time.Hour + 59*time.Minute + 40*time.Second: "4h00m",
	}
	for d, want := range cases {
		if got := formatDuration(d); got != want {
			t.Errorf("formatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

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
		}
		if strings.HasPrefix(k, "ctrl+") {
			msg = tea.KeyPressMsg{Code: rune(k[len(k)-1]), Mod: tea.ModCtrl}
		}
		_, last = m.handleKey(msg)
	}
	return last
}

func view(m *Model) string { return ansi.Strip(m.View().Content) }

// markedLines are the visible lines carrying the cursor marker.
func markedLines(m *Model) []string {
	var out []string
	for _, l := range strings.Split(view(m), "\n") {
		if strings.HasPrefix(l, iconArrow) {
			out = append(out, l)
		}
	}
	return out
}

func containsAll(t *testing.T, v string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(v, w) {
			t.Errorf("view should contain %q:\n%s", w, v)
		}
	}
}

func TestLayoutAndNavigation(t *testing.T) {
	m := newTestModel(t)
	// Fresh: project, then its two classes collapsed.
	if len(m.rows) != 3 || m.rows[1].Name != "MathTests" {
		t.Fatalf("rows = %d", len(m.rows))
	}
	v := view(m)
	containsAll(t, v, "DTEST", "⎯⎯ Log  Alpha.Tests ⎯", "⎯⎯ Tests ⎯", "▾ Alpha.Tests (5 tests)", "▸ MathTests (4 tests)", "5 not run (5)", "Not run yet",
		"Test Projects  (1)", "Tests  5", "Failed  0", "Skipped  0", "Passed  0", "Start at  –", "Duration  –",
		"press ? to show help, press q to quit")
	if strings.Contains(v, "Summary") || strings.Contains(v, "│") || strings.Contains(v, "Ready") || strings.Count(v, "DTEST") != 1 {
		t.Errorf("one bottom pane and no extra text before the first run:\n%s", v)
	}
	// The summary block sits flush with the right edge of the bottom pane,
	// its widest line ending at the last column.
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, "press ? to show help") && !strings.HasSuffix(l, "press q to quit") {
			t.Errorf("hint should end at the right edge: %q", l)
		}
	}
	if strings.Contains(v, "(5 tests) 0.000s") {
		t.Error("unrun groups show no duration")
	}
	// The summary block is bottom-aligned: the hint is the last line.
	lines := strings.Split(strings.TrimRight(v, "\n"), "\n")
	if last := lines[len(lines)-1]; !strings.HasSuffix(last, "press q to quit") {
		t.Errorf("last line should be the hint, got %q", last)
	}
	// The state line sits above the block, so the block keeps its place
	// when a run starts: the Tests line stays on the same screen row.
	row := func() int {
		for i, l := range strings.Split(view(m), "\n") {
			if strings.Contains(l, "Tests  ") && !strings.Contains(l, "⎯") {
				return i
			}
		}
		return -1
	}
	before := row()
	col := func() int {
		for _, l := range strings.Split(view(m), "\n") {
			if i := strings.Index(l, "Tests  ("); i >= 0 {
				return i
			}
		}
		return -1
	}
	beforeCol := col()
	m.building = true
	m.refresh()
	if after := row(); after != before || col() != beforeCol || !strings.Contains(view(m), "Building Sample.slnx…") {
		t.Errorf("Tests line moved from row %d/col %d to row %d/col %d when building started", before, beforeCol, after, col())
	}
	m.building = false
	m.refresh()
	// The log pane is about 70% of the height: header + title + log + title + bottom = 40.
	g := m.layout()
	if g.logH < 24 || g.logH > 27 || g.bottomH+g.logH+headerH+2*titleH != 40 || g.statsW == 0 || g.treeW+g.statsW+2 != 120 {
		t.Fatalf("geometry = %+v", g)
	}

	press(m, "j", "l") // expand MathTests
	if len(m.rows) != 6 || m.current().Name != "MathTests" {
		t.Fatalf("after l: %d rows on %q", len(m.rows), m.current().Name)
	}
	containsAll(t, view(m), "▾ MathTests (4 tests)", "· Adds", "▸ Theory (2 tests)")
	press(m, "l")
	if m.current().Name != "Adds" {
		t.Fatalf("l on expanded -> %q", m.current().Name)
	}
	press(m, "G")
	if m.current().Name != "SlowTests" {
		t.Fatalf("G -> %q", m.current().Name)
	}
	press(m, "g", "g")
	if m.cursor != 0 {
		t.Fatalf("gg -> %d", m.cursor)
	}
	press(m, "j", "j", "j", "j", "space")
	if m.current().Name != "Theory" || !strings.Contains(view(m), "(n: 1)") {
		t.Fatalf("space should expand the theory, on %q", m.current().Name)
	}
	press(m, "h")
	if strings.Contains(view(m), "(n: 1)") {
		t.Fatal("h should collapse the theory")
	}
	press(m, "h")
	if m.current().Name != "MathTests" {
		t.Fatalf("h on a collapsed node selects its parent, got %q", m.current().Name)
	}
	press(m, "ctrl+d")
	if m.cursor == 1 {
		t.Fatal("ctrl+d should move")
	}
	press(m, "ctrl+u")
	if m.cursor != 0 {
		t.Fatalf("ctrl+u -> %d", m.cursor)
	}
	if marked := markedLines(m); len(marked) != 1 || !strings.Contains(marked[0], "Alpha.Tests (5 tests)") {
		t.Fatalf("tree marker = %v", marked)
	}
}

func TestPaneFocusAndLogScroll(t *testing.T) {
	m := newTestModel(t)
	if m.focus != paneTree {
		t.Fatal("the tree is focused first")
	}
	press(m, "ctrl+k")
	if m.focus != paneLog {
		t.Fatal("ctrl+k focuses the log")
	}
	press(m, "ctrl+k")
	if m.focus != paneTree {
		t.Fatal("ctrl+k wraps back to the tree")
	}
	press(m, "ctrl+j", "ctrl+j")
	if m.focus != paneTree {
		t.Fatal("ctrl+j wraps too")
	}
	press(m, "tab")
	if m.focus != paneLog || !strings.Contains(view(m), "⎯⎯ Log") {
		t.Fatal("tab switches panes too")
	}
	m.handleKey(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.focus != paneTree {
		t.Fatal("shift+tab switches back")
	}
	// Give the log more lines than fit and scroll it from the log pane.
	fails := m.tree.Lookup(m.tree.Projects[0], "Alpha.Tests.MathTests.Fails")
	fails.Result = &dotnet.Result{Outcome: dotnet.OutcomeFailed, Message: "boom", StackTrace: strings.Repeat("   at X in /src/a.cs:line 1\n", 60)}
	fails.SetStatus(tree.StatusFailed)
	press(m, "j", "l", "j", "j") // select Fails
	if m.current() != fails || m.log.YOffset() != 0 {
		t.Fatalf("selecting a node shows its log from the top: %q offset %d", m.current().Name, m.log.YOffset())
	}
	press(m, "ctrl+j", "j", "j")
	if m.focus != paneLog || m.log.YOffset() != 2 || m.current() != fails {
		t.Fatalf("j in the log scrolls it: focus=%v offset=%d", m.focus, m.log.YOffset())
	}
	press(m, "G")
	if !m.log.AtBottom() {
		t.Fatal("G scrolls to the bottom")
	}
	press(m, "g", "g")
	if m.log.YOffset() != 0 {
		t.Fatal("gg scrolls to the top")
	}
	press(m, "ctrl+d")
	if m.log.YOffset() == 0 {
		t.Fatal("ctrl+d scrolls half a page")
	}
}

func TestFilter(t *testing.T) {
	m := newTestModel(t)
	press(m, "t", "w", "a", "i")
	if !m.filtering || m.query != "wai" {
		t.Fatalf("filtering=%v query=%q", m.filtering, m.query)
	}
	if len(m.rows) != 3 || m.rows[2].Name != "Waits" {
		t.Fatalf("filtered rows = %d", len(m.rows))
	}
	v := view(m)
	if !strings.Contains(v, "? Filter › wai") || strings.Contains(v, "\t") {
		t.Fatalf("tree title should show the prompt and rows must not contain tabs:\n%s", v)
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
	if m.filtering || m.query != "wai" || !strings.Contains(view(m), "Tests  filter: wai") {
		t.Fatalf("after enter: filtering=%v query=%q", m.filtering, m.query)
	}
	press(m, "esc")
	if m.query != "" || len(m.rows) != 3 {
		t.Fatalf("esc should clear the filter: %q %d", m.query, len(m.rows))
	}
	press(m, "/", "a", "esc")
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
	class := m.current()
	if class.Status() != tree.StatusRunning || class.Counts().Running != 4 {
		t.Fatalf("class should be running: %+v", class.Counts())
	}
	v := view(m)
	containsAll(t, v, "(5 tests | 4 running)", "19:10:48", "Tests  5", "Failed  0", "Passed  0")
	// Only the project row spins; the running class beneath keeps its arrow.
	_, bottom, _ := strings.Cut(v, "⎯⎯ Tests")
	spinners := 0
	for _, l := range strings.Split(bottom, "\n") {
		if strings.ContainsAny(l, "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏") {
			spinners++
			if !strings.Contains(l, "Alpha.Tests (5 tests") {
				t.Errorf("only the project row should spin: %q", l)
			}
		}
	}
	if spinners != 1 || !strings.Contains(bottom, "▸ MathTests (4 tests | 4 running)") {
		t.Errorf("spinners = %d; class should show its arrow:\n%s", spinners, bottom)
	}
	for _, l := range strings.Split(v, "\n") {
		summary := strings.Contains(l, "Test Projects  ") || strings.Contains(l, "   Tests  ")
		if strings.Contains(v, "RUN") || strings.Contains(v, "Running") || (summary && strings.Contains(l, "running")) {
			t.Errorf("no RUN badge or running text in the summary while tests run: %q", l)
		}
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
	// The cursor is on MathTests, so the log aggregates the class: its
	// counts and every failure beneath it, without stack traces.
	v = view(m)
	containsAll(t, v,
		"⎯⎯ Log  Alpha.Tests › MathTests ⎯",
		"× Alpha.Tests › MathTests 0.033s",
		"FAIL  Alpha.Tests › MathTests › Fails 0.001s",
		"Expected: 5",
		"Actual:   4",
		"❯ /src/Alpha.Tests/UnitTest1.cs:12",
		"▾ MathTests (5 tests | 1 failed | 1 skipped)",
		"× Fails 0.001s",
		"▸ Theory (2 tests | 1 skipped)",
		"Test Projects  1 failed (1)",
		"Tests  6",
		"Failed  1",
		"Skipped  1",
		"Passed  3",
		"Duration  2m34s (tests 0.033s)",
	)
	if strings.Contains(v, "Tests failed.") || strings.Contains(v, "Tests passed.") {
		t.Error("no result line in the stats")
	}
	if strings.Contains(v, "(n: 1)") {
		t.Error("theory without failures should be folded")
	}
	if strings.Contains(v, "Stack trace") {
		t.Error("a group's log leaves the stack traces out")
	}
	// The line under the section header is its tally.
	vlines := strings.Split(v, "\n")
	for i, l := range vlines {
		if strings.Contains(l, "× Alpha.Tests › MathTests 0.033s") {
			if !strings.Contains(vlines[i+1], "1 failed | 3 passed | 1 skipped (5)") {
				t.Errorf("tally line = %q", vlines[i+1])
			}
		}
	}
	// n selects the failed test; its own log adds the stack trace. A passed
	// test shows its verdict.
	press(m, "g", "g", "n")
	if m.current().Name != "Fails" {
		t.Fatalf("n -> %q", m.current().Name)
	}
	containsAll(t, view(m), "⎯⎯ Log  Alpha.Tests › MathTests › Fails ⎯", "Stack trace", "at Alpha.Tests.MathTests.Fails()")
	if strings.Contains(view(m), "× MathTests") {
		t.Error("groups use fold arrows, not the test glyphs")
	}
	if marked := markedLines(m); len(marked) != 1 || !strings.Contains(marked[0], "× Fails") {
		t.Fatalf("markers = %v", marked)
	}
	press(m, "k", "k") // past Extra, which sorts before Fails
	containsAll(t, view(m), "⎯⎯ Log  Alpha.Tests › MathTests › Adds ⎯", "✓ Passed in 0.032s")
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
	// Tests waiting in the queued run show as queued; the one still
	// running in the current run keeps running.
	waits := m.tree.Lookup(class.Project(), "Alpha.Tests.SlowTests.Waits")
	if waits.Status() != tree.StatusQueued || m.tree.Lookup(class.Project(), "Alpha.Tests.MathTests.Fails").Status() != tree.StatusRunning {
		t.Fatalf("Waits=%v Fails=%v", waits.Status(), m.tree.Lookup(class.Project(), "Alpha.Tests.MathTests.Fails").Status())
	}
	press(m, "G", "l")
	containsAll(t, view(m), "○ Waits", "1 queued")
	press(m, "g", "g", "j", "j") // back inside MathTests, on Adds
	press(m, "x")
	if len(m.queue) != 0 || waits.Status() != tree.StatusNone {
		t.Fatalf("x should drop the queue and clear queued marks, Waits=%v", waits.Status())
	}
	f.emit(dotnet.DoneEvent{Err: context.Canceled})
	m.Update(<-m.runEvents())
	if m.run != nil || m.tree.Lookup(class.Project(), "Alpha.Tests.MathTests.Fails").Status() != tree.StatusNone {
		t.Fatal("cancelled leaves should return to StatusNone")
	}
	// With no failures left the class folded, and the cursor moved to it
	// rather than jumping to the top.
	if m.current() != class {
		t.Fatalf("cursor should rest on the folded class, got %q", m.current().Name)
	}
	// enter from the log pane runs the tree's selection too.
	press(m, "l", "j", "j", "j", "ctrl+k", "enter")
	<-f.started
	if f.filter != "FullyQualifiedName=Alpha.Tests.MathTests.Fails" {
		t.Fatalf("log-pane enter filter = %q", f.filter)
	}
}

func TestOutputDrawnOnTick(t *testing.T) {
	f := stubRun(t)
	m := newTestModel(t)
	press(m, "v", "r") // raw output on, run the project
	<-f.started
	f.emit(dotnet.LineEvent{Text: "  Determining projects to restore..."})
	m.Update(<-m.runEvents())
	if strings.Contains(view(m), "Determining projects") {
		t.Fatal("a single output line must not rebuild the panes by itself")
	}
	m.Update(m.spin.Tick())
	if !strings.Contains(view(m), "Determining projects") {
		t.Fatal("the spinner tick draws the lines that arrived")
	}
	f.emit(dotnet.DoneEvent{})
	m.Update(<-m.runEvents())
	if m.busy() {
		t.Fatal("run should be over")
	}
}

func TestNothingToDo(t *testing.T) {
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
	m.logs[buildLogKey] = []string{"$ dotnet build", "  Determining projects to restore...", "Program.cs(3,5): error CS1002: ; expected", "Build FAILED."}
	m.lastLog = buildLogKey
	m.refresh()
	v := view(m)
	if strings.Contains(v, "Determining projects") || !strings.Contains(v, "BUILD  Sample.slnx") || !strings.Contains(v, "error CS1002") {
		t.Fatalf("build errors show, the rest is hidden:\n%s", v)
	}
	press(m, "v")
	containsAll(t, view(m), "⎯⎯ Output  build ⎯", "Determining projects", "Build FAILED.")
	m.logs[projA] = []string{"$ dotnet test", "  Passed X [1 ms]"}
	m.refresh()
	containsAll(t, view(m), "⎯⎯ Output  Alpha.Tests ⎯", "Passed X")
	press(m, "v")
	if strings.Contains(view(m), "Determining projects") {
		t.Fatal("v should hide the output again")
	}
	press(m, "?")
	containsAll(t, view(m), "⎯⎯ Usage ⎯", "press f", "rerun only the failed tests", "/tmp/nvim.Sample.sock")
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
	// o on a failed test opens the failing line from the stack trace, from
	// either pane; once it passes again, the declaration.
	fails := m.tree.Lookup(m.tree.Projects[0], "Alpha.Tests.MathTests.Fails")
	fails.Result = &dotnet.Result{Outcome: dotnet.OutcomeFailed, Message: "boom", StackTrace: "   at X in /src/Alpha.Tests/UnitTest1.cs:line 12"}
	fails.SetStatus(tree.StatusFailed)
	press(m, "j", "j", "j", "o")
	if m.current() != fails || lines[len(lines)-1] != 12 {
		t.Fatalf("failing line expected from the tree, got %v on %q", lines, m.current().Name)
	}
	press(m, "ctrl+k", "o")
	if lines[len(lines)-1] != 12 {
		t.Fatalf("failing line expected from the log, got %v", lines)
	}
	fails.SetStatus(tree.StatusPassed)
	press(m, "o")
	if lines[len(lines)-1] != 7 {
		t.Fatalf("a passing test opens at its declaration, got %v", lines)
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

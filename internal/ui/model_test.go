package ui

import (
	"context"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"dtest/internal/dotnet"
	"dtest/internal/tree"
)

const projA = "/src/Alpha.Tests/Alpha.Tests.csproj"

// newTestModel returns a 140x40 model (wide enough for the stats to show)
// with one project listed and no dotnet calls made.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	m := New(Config{Version: "v1.2.3", Target: "/src/Sample.slnx", Socket: "/tmp/nvim.Sample.sock"})
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	p := m.tree.AddProject(projA)
	m.tree.SetTests(p, []string{
		"Alpha.Tests.MathTests.Adds",
		"Alpha.Tests.MathTests.Fails",
		"Alpha.Tests.MathTests.Theory(n: 1)",
		"Alpha.Tests.MathTests.Theory(n: 2)",
		"Alpha.Tests.SlowTests.Waits",
	})
	p.Expanded = true // most of these tests navigate inside the project
	m.refresh()
	return m
}

// A fresh tree shows the solution and its projects, nothing more: projects
// are collapsed until they are asked for.
func TestProjectsStartCollapsed(t *testing.T) {
	m := New(Config{Target: "/src/Sample.slnx"})
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	p := m.tree.AddProject(projA)
	m.tree.SetTests(p, []string{"Alpha.Tests.MathTests.Adds", "Alpha.Tests.SlowTests.Waits"})
	m.refresh()
	want := []string{"▾ Sample (1 project | 2 tests)", "└─ ▸ Alpha.Tests (2 tests)"}
	if got := treeRows(m); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("fresh tree = %q", got)
	}
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

// markedLines are the visible lines carrying the cursor background, as
// plain text.
func markedLines(m *Model) []string {
	var out []string
	for _, l := range strings.Split(m.View().Content, "\n") {
		if strings.Contains(l, bgFocused) || strings.Contains(l, bgUnfocused) {
			out = append(out, ansi.Strip(l))
		}
	}
	return out
}

// footerRow is the last line of the screen, as plain text: what dtest is
// doing, or the summary of the last batch of runs.
func footerRow(m *Model) string { return strings.TrimRight(ansi.Strip(m.renderFooter()), " ") }

// clipboardOf reports the text a command would put on the clipboard.
// bubbletea carries it in an unexported message whose underlying type is a
// string, so its kind is what identifies it. The walk stops at the first
// one, which also keeps it from running the five-second status tick that
// rides along in the same batch.
func clipboardOf(t *testing.T, cmd tea.Cmd) string {
	t.Helper()
	if cmd == nil {
		return ""
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, sub := range msg {
			if s := clipboardOf(t, sub); s != "" {
				return s
			}
		}
	default:
		if v := reflect.ValueOf(msg); v.Kind() == reflect.String {
			return v.String()
		}
	}
	return ""
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
	// Fresh: the root, the project, then its two classes collapsed.
	if len(m.rows) != 4 || m.rows[2].Name != "MathTests" {
		t.Fatalf("rows = %d", len(m.rows))
	}
	v := view(m)
	containsAll(t, v, "DTEST  Sample  v1.2.3", "⎯⎯ Log  Sample ⎯", "⎯⎯ Tests ⎯", "▾ Sample (1 project | 5 tests)", "▾ Alpha.Tests (5 tests)", "▸ MathTests (4 tests)", "Not run yet",
		"nothing run yet", "press ? for help")
	if strings.Contains(v, "Summary") || strings.Contains(v, "│") || strings.Contains(v, "Ready") || strings.Count(v, "DTEST") != 1 {
		t.Errorf("one bottom pane and no extra text before the first run:\n%s", v)
	}
	if strings.Contains(v, "(5 tests) 0.000s") {
		t.Error("unrun groups show no duration")
	}
	// The summary is one row, directly under the tree pane's title and above
	// the root, with the hint at the right edge.
	lines := strings.Split(v, "\n")
	title := 0
	for i, l := range lines {
		if strings.HasPrefix(l, rule+rule+" Tests") {
			title = i
		}
	}
	// The tree starts right under its title, and the summary is the last
	// line of the screen with the hint at its right edge.
	if !strings.Contains(lines[title+1], "▾ Sample") {
		t.Errorf("the root should follow the title, got %q", lines[title+1])
	}
	if got := strings.TrimRight(lines[len(lines)-1], " "); got != footerRow(m) || !strings.HasSuffix(got, "press ? for help") {
		t.Errorf("footer = %q", got)
	}
	// What dtest is doing takes the footer while it is doing it.
	m.building = true
	m.refresh()
	if got := footerRow(m); !strings.Contains(got, "Building Sample.slnx…") || strings.Contains(got, "nothing run yet") {
		t.Errorf("footer while building = %q", got)
	}
	m.building = false
	m.refresh()
	// The log pane is about 70% of what is left: header + title + log +
	// title + summary + tree + status = 40, both panes the full width.
	g := m.layout()
	if g.logH < 23 || g.logH > 27 || g.treeH+g.logH+headerH+2*titleH+footerH != 40 {
		t.Fatalf("geometry = %+v", g)
	}
	if m.treeView.Width() != 140 || m.log.Width() != 140 {
		t.Fatalf("widths = %d %d", m.treeView.Width(), m.log.Width())
	}

	press(m, "j", "j", "l") // expand MathTests
	if len(m.rows) != 7 || m.current().Name != "MathTests" {
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
	press(m, "j", "j", "j", "j", "j", "space")
	if m.current().Name != "Theory" || !strings.Contains(view(m), "(n: 1)") {
		t.Fatalf("space should expand the theory, on %q", m.current().Name)
	}
	// The highlighted tree row keeps its colours and spans the tree width.
	for _, l := range strings.Split(m.View().Content, "\n") {
		if strings.Contains(l, bgFocused) && strings.Contains(l, "Theory") {
			if !strings.Contains(l, "\x1b[90m") { // the dim arrow's colour survives
				t.Errorf("highlighted row lost its colours: %q", l)
			}
			if strings.Count(l, bgFocused) < 2 {
				t.Errorf("background should be re-applied after resets: %q", l)
			}
		}
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
	down := m.cursor
	if down <= 2 {
		t.Fatalf("ctrl+d should move down from MathTests, cursor = %d", down)
	}
	press(m, "ctrl+u")
	if m.cursor >= down {
		t.Fatalf("ctrl+u should move back up, %d -> %d", down, m.cursor)
	}
	press(m, "g", "g")
	// One highlighted row in the log (dim, unfocused), one in the tree.
	if marked := markedLines(m); len(marked) != 2 || !strings.Contains(marked[1], "Sample (1 project | 5 tests)") {
		t.Fatalf("markers = %v", marked)
	}
	if !strings.Contains(m.View().Content, bgUnfocused) || !strings.Contains(m.View().Content, bgFocused) {
		t.Fatal("the unfocused pane's cursor is dimmer than the focused one's")
	}
}

func TestShortTerminal(t *testing.T) {
	// With little height the summary row still shows, above whatever the
	// tree has room for.
	m := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 150, Height: 12})
	m.building = true
	m.refresh()
	if g := m.layout(); g.treeH < 1 || g.logH < 1 {
		t.Fatalf("geometry = %+v", g)
	}
	if v := view(m); !strings.Contains(v, "Building Sample.slnx…") || !strings.Contains(v, "press ? for help") {
		t.Fatalf("short terminal should keep the summary row:\n%s", v)
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
	press(m, "j", "j", "l", "j", "j") // select Fails
	if m.current() != fails || m.log.YOffset() != 0 {
		t.Fatalf("selecting a node shows its log from the top: %q offset %d", m.current().Name, m.log.YOffset())
	}
	press(m, "ctrl+j", "j", "j")
	if m.focus != paneLog || m.logCursor != 2 || m.current() != fails {
		t.Fatalf("j in the log moves its cursor: focus=%v cursor=%d", m.focus, m.logCursor)
	}
	if v := view(m); !strings.Contains(v, fmt.Sprintf(" 3/%d ⎯", len(m.logLines))) {
		t.Fatalf("title should show the position:\n%s", v)
	}
	press(m, "G")
	if m.logCursor != len(m.logLines)-1 || !m.log.AtBottom() {
		t.Fatal("G moves to the last line and scrolls there")
	}
	press(m, "g", "g")
	if m.logCursor != 0 || m.log.YOffset() != 0 {
		t.Fatal("gg moves to the top")
	}
	press(m, "ctrl+d")
	if m.logCursor != m.log.Height()/2 {
		t.Fatalf("ctrl+d moves half a page, cursor = %d", m.logCursor)
	}
	press(m, "ctrl+d", "ctrl+d")
	if m.log.YOffset() == 0 {
		t.Fatal("moving the cursor below the pane scrolls it")
	}
	// Long lines wrap instead of being cut off.
	long := strings.Repeat("word ", 40)
	fails.Result.Message = long
	press(m, "g", "g")
	m.logSig = "" // force a rebuild with the new message
	m.refresh()
	if v := view(m); !strings.Contains(v, "word word") || strings.Contains(v, "…word") {
		t.Fatalf("message should wrap:\n%s", v)
	}
	if strings.Count(view(m), "word") != 40 {
		t.Fatalf("every word of the wrapped message should be visible and whole, got %d", strings.Count(view(m), "word"))
	}
	// The position counts logical lines, not wrapped rows, and G still
	// reaches the last one.
	press(m, "G")
	if m.logCursor != len(m.logLines)-1 || !strings.Contains(view(m), fmt.Sprintf(" %d/%d ⎯", len(m.logLines), len(m.logLines))) {
		t.Fatalf("position after G: cursor %d of %d", m.logCursor, len(m.logLines))
	}
}

func TestLocationInLine(t *testing.T) {
	cases := map[string]dotnet.Location{
		"   at Shop.Core.Pricing.Discount.Percent() in /src/Shop.Core/Pricing/Discount.cs:line 9":               {File: "/src/Shop.Core/Pricing/Discount.cs", Line: 9},
		" ❯ /src/tests/DiscountTests.cs:38":                                                                     {File: "/src/tests/DiscountTests.cs", Line: 38},
		"[xUnit.net 00:00:00.06]         /src/Alpha.Tests/UnitTest1.cs(12,0): at Alpha.Tests.MathTests.Fails()": {File: "/src/Alpha.Tests/UnitTest1.cs", Line: 12},
	}
	for line, want := range cases {
		if got, ok := locationInLine(line); !ok || got != want {
			t.Errorf("locationInLine(%q) = %+v %v, want %+v", line, got, ok, want)
		}
	}
	for _, line := range []string{"Expected: 5", "  × Fails 0.001s", "   at System.Reflection.MethodBaseInvoker.InterpretedInvoke_Method(Object obj, IntPtr* args)"} {
		if _, ok := locationInLine(line); ok {
			t.Errorf("%q should have no location", line)
		}
	}
}

func TestFilter(t *testing.T) {
	m := newTestModel(t)
	press(m, "t", "w", "a", "i")
	if !m.filtering || m.query != "wai" {
		t.Fatalf("filtering=%v query=%q", m.filtering, m.query)
	}
	if len(m.rows) != 4 || m.rows[3].Name != "Waits" {
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
	if m.query != "" || len(m.rows) != 4 {
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

	press(m, "j", "j", "r") // MathTests
	<-f.started
	if f.project != projA || f.filter != "FullyQualifiedName~Alpha.Tests.MathTests." {
		t.Fatalf("run %q %q", f.project, f.filter)
	}
	class := m.current()
	if class.Status() != tree.StatusRunning || class.Counts().Running != 4 {
		t.Fatalf("class should be running: %+v", class.Counts())
	}
	v := view(m)
	containsAll(t, v, "(5 tests | 4 running)")
	if got := footerRow(m); !strings.Contains(got, "Ran 0 tests in 0.000s at 19:10:48  ·  0 failed | 0 passed | 0 skipped") {
		t.Errorf("a run in flight keeps the same shape: %q", got)
	}
	// The timer runs on its own, without waiting for a result to land.
	clock = clock.Add(3 * time.Second)
	if got := footerRow(m); !strings.Contains(got, "in 3.0s") {
		t.Errorf("the timer should keep moving mid-run: %q", got)
	}
	clock = clock.Add(-3 * time.Second)
	// Only the project row spins; the running class beneath keeps its arrow,
	// and nothing in the log spins at all.
	const dots = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"
	top, bottom, _ := strings.Cut(v, "⎯⎯ Tests")
	if strings.ContainsAny(top, dots) {
		t.Errorf("the log should hold still while tests run:\n%s", top)
	}
	spinners := 0
	for _, l := range strings.Split(bottom, "\n") {
		if strings.ContainsAny(l, dots) {
			spinners++
			if !strings.Contains(l, "Alpha.Tests (5 tests") {
				t.Errorf("only the project row should spin: %q", l)
			}
		}
	}
	if spinners != 1 || !strings.Contains(bottom, "▸ MathTests (4 tests | 4 running)") {
		t.Errorf("spinners = %d; class should show its arrow:\n%s", spinners, bottom)
	}
	if strings.Contains(v, badgeInfo.Render("RUN")) || strings.Contains(v, badgeFail.Render("RUN")) {
		t.Errorf("no RUN badge while tests run:\n%s", v)
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
		// The clock ran on for 2m34s between starting and finishing, and
		// that is what the footer reports.
		"Ran 5 tests in 2m34s at 19:10:48  ·  1 failed | 3 passed | 1 skipped",
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
	// The section header is followed straight by its failures: the tally is
	// on the tree row the section was selected from, not repeated here.
	vlines := strings.Split(v, "\n")
	for i, l := range vlines {
		if strings.Contains(l, "× Alpha.Tests › MathTests 0.033s") {
			if strings.Contains(vlines[i+1], "passed") || !strings.Contains(vlines[i+2], "FAIL") {
				t.Errorf("after the header: %q then %q", vlines[i+1], vlines[i+2])
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
	if marked := markedLines(m); len(marked) != 2 || !strings.Contains(marked[1], "× Fails") {
		t.Fatalf("markers = %v", marked)
	}
	press(m, "k", "k") // past Extra, which sorts before Fails
	containsAll(t, view(m), "⎯⎯ Log  Alpha.Tests › MathTests › Adds ⎯", "✓ Passed in 0.032s")
	// f narrows the tree to failures, s to skipped tests, esc widens it.
	press(m, "f")
	if len(m.rows) != 4 || m.rows[3].Name != "Fails" || !strings.Contains(view(m), "Tests  failed only") {
		t.Fatalf("failed-only rows = %d\n%s", len(m.rows), view(m))
	}
	press(m, "s")
	if len(m.rows) != 5 || m.rows[4].Name != "(n: 2)" || !strings.Contains(view(m), "skipped only") {
		t.Fatalf("skipped-only rows = %d", len(m.rows))
	}
	press(m, "s")
	if m.statusFilter != tree.StatusNone {
		t.Fatal("s again clears the status filter")
	}
	press(m, "f", "esc")
	if m.statusFilter != tree.StatusNone {
		t.Fatal("esc clears the status filter")
	}
	press(m, "f", "a")
	if m.statusFilter != tree.StatusNone || len(m.rows) < 6 {
		t.Fatal("a shows all tests again")
	}
	// F re-runs the failed tests only; A runs whole projects and queues.
	press(m, "F")
	<-f.started
	if f.filter != "FullyQualifiedName=Alpha.Tests.MathTests.Fails" {
		t.Fatalf("rerun-failed filter = %q", f.filter)
	}
	press(m, "A")
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
	containsAll(t, view(m), iconQueued+" Waits", "1 queued")
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
	press(m, "F")
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
	press(m, "f")
	if len(m.rows) != 0 || !strings.Contains(view(m), "No failed tests") {
		t.Fatalf("failed-only with nothing failed: %d rows", len(m.rows))
	}
	press(m, "esc")
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
	m.logs[projA], m.lastLog = []string{"$ dotnet test", "  Passed X [1 ms]"}, projA
	m.refresh()
	containsAll(t, view(m), "⎯⎯ Output  Alpha.Tests ⎯", "Passed X")
	press(m, "v")
	if strings.Contains(view(m), "Determining projects") {
		t.Fatal("v should hide the output again")
	}
	press(m, "?")
	containsAll(t, view(m), "⎯⎯ Usage ⎯", "press A", "press F", "rerun only the failed tests", "press f / s", "press a", "/tmp/nvim.Sample.sock")
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

	press(m, "j", "j", "l", "j", "o") // Adds
	if len(opened) != 2 || opened[0] != "/tmp/nvim.Sample.sock" || opened[1] != "/src/Alpha.Tests/UnitTest1.cs" || lines[0] != 7 {
		t.Fatalf("opened = %v lines = %v", opened, lines)
	}
	if !strings.Contains(m.status, "Sent UnitTest1.cs:7") {
		t.Fatalf("status = %q", m.status)
	}
	// The root is the solution, so it opens the solution file.
	press(m, "g", "g", "o")
	if opened[len(opened)-1] != "/src/Sample.slnx" {
		t.Fatalf("root open = %v", opened)
	}
	press(m, "j", "o") // the project opens its own file
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
	// With the log cursor on a deeper stack frame, o opens that frame.
	fails.Result.StackTrace = "   at Shop.Core.Discount.Percent() in /src/Shop.Core/Discount.cs:line 9\n   at X in /src/Alpha.Tests/UnitTest1.cs:line 12"
	m.refresh()
	for i, l := range m.logLines {
		if strings.Contains(l, "Discount.cs:line 9") {
			m.logCursor = i
		}
	}
	press(m, "o")
	if opened[len(opened)-1] != "/src/Shop.Core/Discount.cs" || lines[len(lines)-1] != 9 {
		t.Fatalf("frame under the cursor expected, got %v %v", opened[len(opened)-1], lines[len(lines)-1])
	}
	press(m, "ctrl+k")
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

// A time is drawn in the quick colour up to the slow threshold and the slow
// one past it, with the unit a faded shade of whichever it got. The
// expectations are built from the styles rather than spelled out as escape
// codes, so repainting the palette does not break the test that guards the
// rule.
func TestRenderDuration(t *testing.T) {
	quick, slow := styleQuick, styleSlow
	cases := map[time.Duration]string{
		3 * time.Millisecond:           quick.Render("0.003") + quick.Faint(true).Render("s"),
		slowDuration:                   quick.Render("0.300") + quick.Faint(true).Render("s"),
		slowDuration + time.Second/100: slow.Render("0.310") + slow.Faint(true).Render("s"),
		1201 * time.Millisecond:        slow.Render("1.2") + slow.Faint(true).Render("s"),
		2*time.Minute + 34*time.Second: slow.Render("2m34s"), // one unit inside the number
	}
	for d, want := range cases {
		if got := renderDuration(d); got != want {
			t.Errorf("renderDuration(%v) = %q, want %q", d, got, want)
		}
	}
	if quick.Render("x") == slow.Render("x") {
		t.Error("quick and slow should not look the same")
	}
	if quick.Render("s") == quick.Faint(true).Render("s") {
		t.Error("the unit should be a shade back from the number")
	}
}

// The rule around a pane title is fainter than the title, and the line
// still fills the width exactly.
func TestPaneTitleRule(t *testing.T) {
	m := newTestModel(t)
	var titles []string
	for _, l := range strings.Split(m.View().Content, "\n") {
		if strings.Contains(ansi.Strip(l), rule) {
			titles = append(titles, l)
		}
	}
	if len(titles) != 2 {
		t.Fatalf("two pane titles, got %d", len(titles))
	}
	for _, l := range titles {
		if w := ansi.StringWidth(l); w != m.width {
			t.Errorf("title is %d cells wide, want %d: %q", w, m.width, ansi.Strip(l))
		}
		if !strings.Contains(l, styleRule.Render(rule+rule)) {
			t.Errorf("the rule should be faint: %q", l)
		}
	}
	if strings.Contains(titles[1], styleRule.Render("Tests")) {
		t.Error("the focused title itself should not be faint")
	}
}

// finishRun feeds a run to completion with the given results.
func finishRun(t *testing.T, m *Model, f *fakeRun, results ...dotnet.Result) {
	t.Helper()
	<-f.started
	f.emit(dotnet.DoneEvent{Results: results})
	m.Update(<-m.runEvents())
}

// The summary beside the tree describes the last batch of runs, not the
// whole solution: running one class after everything reports that class.
func TestSummaryCountsLastRun(t *testing.T) {
	f := stubRun(t)
	m := newTestModel(t)
	pass := func(name string) dotnet.Result {
		return dotnet.Result{Name: name, Outcome: dotnet.OutcomePassed, Duration: time.Millisecond}
	}
	fail := func(name string) dotnet.Result {
		return dotnet.Result{Name: name, Outcome: dotnet.OutcomeFailed, Message: "boom"}
	}
	// Everything: one of the five fails.
	press(m, "A")
	finishRun(t, m, f,
		pass("Alpha.Tests.MathTests.Adds"), fail("Alpha.Tests.MathTests.Fails"),
		pass("Alpha.Tests.MathTests.Theory(n: 1)"), pass("Alpha.Tests.MathTests.Theory(n: 2)"),
		pass("Alpha.Tests.SlowTests.Waits"))
	if got := footerRow(m); !strings.Contains(got, "Ran 5 tests") || !strings.Contains(got, "1 failed | 4 passed") {
		t.Errorf("after running everything: %q", got)
	}

	// Then just SlowTests, which passes. The failure elsewhere is still in
	// the tree, but this run had none of it.
	press(m, "G", "r")
	finishRun(t, m, f, pass("Alpha.Tests.SlowTests.Waits"))
	v := view(m)
	// The counts and the time are this run's, not every test that has run.
	if got := footerRow(m); !strings.Contains(got, "Ran 1 test") || !strings.Contains(got, "0 failed | 1 passed") {
		t.Errorf("after running one class: %q", got)
	}
	if !strings.Contains(v, "× Fails") {
		t.Error("the earlier failure should still be in the tree")
	}
	if c := m.batchCounts(); c.Total != 1 || c.Passed != 1 {
		t.Errorf("batch = %+v", c)
	}
}

// What dtest is doing, and anything it has to say, takes the last line of
// the screen; the rest of the time that line carries the run summary.
func TestFooterLine(t *testing.T) {
	m := newTestModel(t)
	last := func() string {
		lines := strings.Split(view(m), "\n")
		return strings.TrimRight(lines[len(lines)-1], " ")
	}
	if !strings.Contains(last(), "nothing run yet") || last() != footerRow(m) {
		t.Errorf("idle footer = %q", last())
	}
	m.building = true
	m.refresh()
	if got := last(); !strings.Contains(got, "Building Sample.slnx…") || strings.Contains(got, "nothing run yet") {
		t.Errorf("building = %q", got)
	}
	m.building = false
	m.loading = 1
	m.refresh()
	if !strings.Contains(last(), "Listing tests…") {
		t.Errorf("listing = %q", last())
	}
	m.loading = 0
	m.status, m.statusErr = "No failed tests to re-run", false
	m.refresh()
	if got := last(); !strings.Contains(got, "INFO") || !strings.Contains(got, "No failed tests to re-run") {
		t.Errorf("status = %q", got)
	}
	m.statusErr = true
	m.refresh()
	if !strings.Contains(last(), "FAIL") {
		t.Errorf("an error carries the FAIL badge: %q", last())
	}
	// The hint stays put through all of it.
	if !strings.HasSuffix(last(), "press ? for help") {
		t.Errorf("the hint should hold its place: %q", last())
	}
	m.status = ""
	m.refresh()
	if !strings.Contains(last(), "nothing run yet") {
		t.Errorf("the summary comes back: %q", last())
	}
}

// The tree draws branch lines down its left: a tee for a child with more
// siblings below, a corner for the last one, and a bar carried down through
// every level that has not ended yet.
func TestTreeGuides(t *testing.T) {
	m := newTestModel(t)
	press(m, "j", "j", "l", "j", "j", "j", "space") // expand MathTests, then Theory
	want := []string{
		"▾ Sample (1 project | 5 tests)",
		"└─ ▾ Alpha.Tests (5 tests)",
		"   ├─ ▾ MathTests (4 tests)",
		"   │  ├─ · Adds",
		"   │  ├─ · Fails",
		"   │  └─ ▾ Theory (2 tests)",
		"   │     ├─ · (n: 1)",
		"   │     └─ · (n: 2)",
		"   └─ ▸ SlowTests (1 test)",
	}
	got := treeRows(m)
	if len(got) != len(want) {
		t.Fatalf("got %d rows:\n%s", len(got), strings.Join(got, "\n"))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %q, want %q", i, got[i], want[i])
		}
	}
	// Filtering hides rows, and the guides join up the ones that are left.
	press(m, "/", "W", "a", "i", "enter")
	wantFiltered := []string{"▾ Sample (1 project | 5 tests)", "└─ ▾ Alpha.Tests (5 tests)", "   └─ ▾ SlowTests (1 test)", "      └─ · Waits"}
	if got := treeRows(m); strings.Join(got, "|") != strings.Join(wantFiltered, "|") {
		t.Errorf("filtered guides = %q", got)
	}
}

// treeRows are the tree pane's rows as plain text, without the summary
// block beside them or the indent every row carries.
func treeRows(m *Model) []string {
	lines := strings.Split(view(m), "\n")
	start := 0
	for i, l := range lines {
		if strings.HasPrefix(l, rule+rule+" Tests") {
			start = i + 1
		}
	}
	var out []string
	for _, l := range lines[start : len(lines)-1] { // the last line is the footer
		if l = strings.TrimRight(ansi.Truncate(l, m.treeView.Width(), ""), " "); l != "" {
			out = append(out, strings.TrimPrefix(l, "  "))
		}
	}
	return out
}

// Everything waiting for its run is greyed out, name included, so a queued
// test reads as waiting rather than as a result. A project keeps its weight
// so the tree still has headings.
func TestQueuedIsGrey(t *testing.T) {
	f := stubRun(t)
	m := newTestModel(t)
	p := m.tree.AddProject("/src/Beta.Tests/Beta.Tests.csproj")
	m.tree.SetTests(p, []string{"Beta.Tests.ApiTests.Pings"})
	// Run the first project, then ask for the second while that is going:
	// the second one's tests wait their turn.
	press(m, "g", "g", "j", "r")
	<-f.started
	press(m, "j", "j", "j", "r")
	if p.Status() != tree.StatusQueued {
		t.Fatalf("the second project should be queued, got %v", p.Status())
	}
	press(m, "l", "l", "l") // open the project and its class so its tests show
	want := map[string]string{
		"Beta.Tests": styleQueued.Bold(true).Render("Beta.Tests"),
		"ApiTests":   styleQueued.Render("ApiTests"),
		"Pings":      styleQueued.Render("Pings"),
	}
	guides := treeGuides(m.rows)
	seen := map[string]bool{}
	for i, n := range m.rows {
		rendered, ok := want[n.Name]
		if !ok {
			continue
		}
		seen[n.Name] = true
		if l := m.renderNode(n, guides[i]); !strings.Contains(l, rendered) {
			t.Errorf("%s should be grey: %q", n.Name, l)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("no tree row for %s", name)
		}
	}
	if strings.Contains(styleQueued.Render("x"), "36") {
		t.Error("queued should be grey, not cyan")
	}
}

// L and H fold the whole tree at once, and the cursor comes to rest on the
// nearest row still visible.
func TestExpandAndCollapseAll(t *testing.T) {
	m := newTestModel(t)
	press(m, "L")
	want := []string{
		"▾ Sample (1 project | 5 tests)",
		"└─ ▾ Alpha.Tests (5 tests)",
		"   ├─ ▾ MathTests (4 tests)",
		"   │  ├─ · Adds",
		"   │  ├─ · Fails",
		"   │  └─ ▾ Theory (2 tests)",
		"   │     ├─ · (n: 1)",
		"   │     └─ · (n: 2)",
		"   └─ ▾ SlowTests (1 test)",
		"      └─ · Waits",
	}
	if got := treeRows(m); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("after L:\n%s", strings.Join(got, "\n"))
	}
	// From a deep row, H collapses everything and the cursor rides up to
	// the project that is still on screen.
	press(m, "G")
	if m.current().Name != "Waits" {
		t.Fatalf("G -> %q", m.current().Name)
	}
	press(m, "H")
	if got := treeRows(m); len(got) != 1 || got[0] != "▸ Sample (1 project | 5 tests)" {
		t.Errorf("after H: %q", got)
	}
	if m.cursor != 0 || m.current().Kind != tree.KindRoot {
		t.Errorf("cursor = %d on %q", m.cursor, m.current().Name)
	}
	// Folding is off while the tree is filtered, so neither key disturbs it.
	press(m, "L", "/", "W", "a", "i", "enter")
	before := treeRows(m)
	press(m, "H")
	if got := treeRows(m); strings.Join(got, "|") != strings.Join(before, "|") {
		t.Errorf("H while filtered changed the tree:\n%s", strings.Join(got, "\n"))
	}
}

// The root is the solution: it rolls up every project, running it is one
// `dotnet test` over the whole thing, and each result is filed under the
// project that listed that test.
func TestRootNode(t *testing.T) {
	f := stubRun(t)
	m := newTestModel(t)
	beta := m.tree.AddProject("/src/Beta.Tests/Beta.Tests.csproj")
	m.tree.SetTests(beta, []string{"Beta.Tests.ApiTests.Pings"})
	m.refresh()
	root := m.rows[0]
	if root.Kind != tree.KindRoot || root.Name != "Sample" || root.Path != "/src/Sample.slnx" || root.Counts().Total != 6 {
		t.Fatalf("root = %q %q %+v", root.Name, root.Path, root.Counts())
	}
	press(m, "g", "g", "enter")
	<-f.started
	if f.project != "/src/Sample.slnx" || f.filter != "" {
		t.Fatalf("run = %q %q", f.project, f.filter)
	}
	if len(m.queue) != 0 {
		t.Fatalf("the solution runs in one go, but %d more were queued", len(m.queue))
	}
	f.emit(dotnet.DoneEvent{Results: []dotnet.Result{
		{Name: "Alpha.Tests.MathTests.Fails", Outcome: dotnet.OutcomeFailed, Message: "alpha boom"},
		{Name: "Beta.Tests.ApiTests.Pings", Outcome: dotnet.OutcomeFailed, Message: "beta boom"},
		{Name: "Beta.Tests.ApiTests.Added", Outcome: dotnet.OutcomePassed},
	}})
	m.Update(<-m.runEvents())
	for _, c := range []struct {
		project *tree.Node
		name    string
	}{
		{m.tree.Projects[0], "Alpha.Tests.MathTests.Fails"},
		{beta, "Beta.Tests.ApiTests.Pings"},
		{beta, "Beta.Tests.ApiTests.Added"}, // never listed, placed by its name
	} {
		if l := m.tree.Lookup(c.project, c.name); l == nil || l.Result == nil {
			t.Errorf("%s should have landed under %s", c.name, c.project.Name)
		}
	}
	// Back on the root, its log reports the failures from both projects.
	press(m, "g", "g")
	v := view(m)
	containsAll(t, v, "⎯⎯ Log  Sample ⎯", "× Sample",
		"FAIL  Alpha.Tests › MathTests › Fails", "alpha boom",
		"FAIL  Beta.Tests › ApiTests › Pings", "beta boom")
	if strings.Contains(v, "Sample › Alpha") {
		t.Error("the root should not pad the breadcrumbs beneath it")
	}
	// Its own row carries the total and nothing else: no tally of how the
	// run went, and no time, both of which belong below it.
	rows := treeRows(m)
	if rows[0] != "▾ Sample (2 projects | 7 tests)" {
		t.Errorf("root row = %q", rows[0])
	}
	if !strings.Contains(rows[1], "1 failed") {
		t.Errorf("the project below it still reports its run: %q", rows[1])
	}
}

// The log holds a node's time back until its tests have all reported, so a
// running total never reads as a result.
func TestLogDurationWaitsForTheRun(t *testing.T) {
	f := stubRun(t)
	m := newTestModel(t)
	press(m, "j", "j", "r") // MathTests
	<-f.started
	// One of its four tests is in; the class itself is still going.
	f.emit(dotnet.ResultEvent{Result: dotnet.Result{
		Name: "Alpha.Tests.MathTests.Adds", Outcome: dotnet.OutcomePassed, Duration: 32 * time.Millisecond}})
	m.Update(<-m.runEvents())
	if got := m.logLines[0]; strings.Contains(got, "0.032s") || !strings.Contains(got, "MathTests") {
		t.Errorf("mid-run header = %q", got)
	}
	// The finished test does show its own time, from its own row.
	press(m, "l", "l")
	if got := m.logLines[0]; !strings.Contains(got, "Adds") || !strings.Contains(got, "0.032s") {
		t.Errorf("finished test header = %q", got)
	}
	// Once the run ends the class reports its total.
	press(m, "h", "h")
	f.emit(dotnet.DoneEvent{Results: []dotnet.Result{{
		Name: "Alpha.Tests.MathTests.Adds", Outcome: dotnet.OutcomePassed, Duration: 32 * time.Millisecond}}})
	m.Update(<-m.runEvents())
	if got := m.logLines[0]; !strings.Contains(got, "MathTests") || !strings.Contains(got, "0.032s") {
		t.Errorf("finished header = %q", got)
	}
}

// The footer counts results in as they land and keeps one shape doing it:
// the same line from the first result to the last, with the outcomes that
// are still zero greyed rather than missing. The clock time is grey, the
// time the tests took cyan.
func TestSummaryLiveThenFinal(t *testing.T) {
	f := stubRun(t)
	m := newTestModel(t)
	clock := time.Date(2026, 9, 19, 21, 36, 37, 0, time.Local)
	now = func() time.Time { return clock }
	t.Cleanup(func() { now = time.Now })

	press(m, "A")
	<-f.started
	if got := footerRow(m); !strings.Contains(got, "Ran 0 tests in 0.000s at 21:36:37  ·  0 failed | 0 passed | 0 skipped") {
		t.Errorf("nothing reported yet: %q", got)
	}
	// One result lands, and the footer moves with it.
	clock = clock.Add(2300 * time.Millisecond)
	f.emit(dotnet.ResultEvent{Result: dotnet.Result{
		Name: "Alpha.Tests.MathTests.Adds", Outcome: dotnet.OutcomePassed, Duration: time.Millisecond}})
	m.Update(<-m.runEvents())
	if got := footerRow(m); !strings.Contains(got, "Ran 1 test in 2.3s at 21:36:37  ·  0 failed | 1 passed | 0 skipped") {
		t.Errorf("mid-run summary = %q", got)
	}
	f.emit(dotnet.DoneEvent{Results: []dotnet.Result{{
		Name: "Alpha.Tests.MathTests.Adds", Outcome: dotnet.OutcomePassed, Duration: time.Millisecond}}})
	m.Update(<-m.runEvents())
	if got := footerRow(m); !strings.Contains(got, "Ran 1 test in 2.3s at 21:36:37  ·  0 failed | 1 passed | 0 skipped") {
		t.Errorf("the finished line reads the same: %q", got)
	}
	// The timer stopped with the batch: it does not run on afterwards.
	clock = clock.Add(time.Minute)
	if got := footerRow(m); !strings.Contains(got, "in 2.3s") {
		t.Errorf("the timer should stop when the batch does: %q", got)
	}
	// A zero outcome is grey; one that happened is bold in its colour.
	line := m.renderFooter()
	if !strings.Contains(line, styleDim.Render("0 failed")) || !strings.Contains(line, stylePassed.Bold(true).Render("1 passed")) {
		t.Errorf("outcome colours: %q", line)
	}
	if !strings.Contains(line, styleDim.Render(" at 21:36:37")) {
		t.Errorf("the clock time should be grey: %q", ansi.Strip(line))
	}
	if !strings.Contains(line, styleElapsed.Render("2.3s")) {
		t.Errorf("the elapsed time should be quiet cyan: %q", ansi.Strip(line))
	}
	if styleElapsed.Render("2.3s") == styleDim.Render("2.3s") {
		t.Error("the two times should not look the same")
	}
}

// V starts a linewise selection the motions extend, y copies it, and the
// text goes out as an OSC 52 clipboard escape.
func TestSelectAndCopyLog(t *testing.T) {
	m := newTestModel(t)
	fails := m.tree.Lookup(m.tree.Projects[0], "Alpha.Tests.MathTests.Fails")
	fails.Result = &dotnet.Result{Outcome: dotnet.OutcomeFailed, Message: "one\ntwo\nthree\nfour"}
	fails.SetStatus(tree.StatusFailed)
	press(m, "j", "j", "l", "j", "j") // select Fails in the tree
	press(m, "ctrl+j")                // focus the log
	if m.focus != paneLog || m.selAnchor != -1 {
		t.Fatal("the log starts with no selection")
	}
	// V anchors, j extends, and the selected rows carry the selection's own
	// background rather than the cursor's.
	press(m, "j", "j", "V", "j")
	from, to := m.selection()
	if from != 2 || to != 3 {
		t.Fatalf("selection = %d..%d of %q", from, to, m.logLines)
	}
	marked := 0
	for _, l := range strings.Split(m.View().Content, "\n") {
		if strings.Contains(l, bgSelected) {
			marked++
		}
	}
	if marked != 2 {
		t.Errorf("two rows should be selected, %d are", marked)
	}
	// y copies them and clears the selection.
	cmd := press(m, "y")
	if m.selAnchor != -1 || !strings.Contains(m.status, "Copied 2 lines") {
		t.Fatalf("after y: anchor=%d status=%q", m.selAnchor, m.status)
	}
	if want := strings.Join(m.logLines[2:4], "\n"); clipboardOf(t, cmd) != want {
		t.Errorf("clipboard = %q, want %q", clipboardOf(t, cmd), want)
	}
	// With nothing selected, y takes the cursor's line alone.
	cmd = press(m, "y")
	if got := clipboardOf(t, cmd); got != m.logLines[m.logCursor] {
		t.Errorf("single-line copy = %q", got)
	}
	// esc drops a selection without copying.
	press(m, "V", "j", "esc")
	if m.selAnchor != -1 {
		t.Error("esc should drop the selection")
	}
}

// Dragging the mouse over the log selects the lines it covers and copies
// them on release; a plain click only moves the cursor.
func TestMouseSelectLog(t *testing.T) {
	m := newTestModel(t)
	fails := m.tree.Lookup(m.tree.Projects[0], "Alpha.Tests.MathTests.Fails")
	fails.Result = &dotnet.Result{Outcome: dotnet.OutcomeFailed, Message: "one\ntwo\nthree\nfour"}
	fails.SetStatus(tree.StatusFailed)
	press(m, "j", "j", "l", "j", "j")
	top := headerH + titleH
	m.Update(tea.MouseClickMsg{Y: top + 1, Button: tea.MouseLeft})
	if m.focus != paneLog || m.logCursor != 1 || m.selAnchor != 1 {
		t.Fatalf("click: focus=%v cursor=%d anchor=%d", m.focus, m.logCursor, m.selAnchor)
	}
	m.Update(tea.MouseMotionMsg{Y: top + 3, Button: tea.MouseLeft})
	if from, to := m.selection(); from != 1 || to != 3 {
		t.Fatalf("drag selection = %d..%d", from, to)
	}
	_, cmd := m.Update(tea.MouseReleaseMsg{Y: top + 3, Button: tea.MouseLeft})
	if want := strings.Join(m.logLines[1:4], "\n"); clipboardOf(t, cmd) != want {
		t.Errorf("clipboard = %q, want %q", clipboardOf(t, cmd), want)
	}
	if m.selAnchor != -1 || !strings.Contains(m.status, "Copied 3 lines") {
		t.Errorf("after release: anchor=%d status=%q", m.selAnchor, m.status)
	}
	// A click with no drag just moves the cursor.
	m.Update(tea.MouseClickMsg{Y: top + 2, Button: tea.MouseLeft})
	_, cmd = m.Update(tea.MouseReleaseMsg{Y: top + 2, Button: tea.MouseLeft})
	if m.logCursor != 2 || m.selAnchor != -1 || cmd != nil {
		t.Errorf("plain click: cursor=%d anchor=%d cmd=%v", m.logCursor, m.selAnchor, cmd)
	}
}

// The tree wears its colours a shade back so the footer's carry, and how
// many tests a row holds is grey rather than plain.
func TestTreeColoursAreDimmerThanTheFooter(t *testing.T) {
	f := stubRun(t)
	m := newTestModel(t)
	press(m, "A")
	<-f.started
	f.emit(dotnet.DoneEvent{Results: []dotnet.Result{
		{Name: "Alpha.Tests.MathTests.Adds", Outcome: dotnet.OutcomePassed, Duration: 32 * time.Millisecond},
		{Name: "Alpha.Tests.MathTests.Fails", Outcome: dotnet.OutcomeFailed, Message: "boom"},
	}})
	m.Update(<-m.runEvents())

	guides := treeGuides(m.rows)
	var project string
	for i, n := range m.rows {
		if n.Name == "Alpha.Tests" {
			project = m.renderNode(n, guides[i])
		}
	}
	if project == "" {
		t.Fatal("no project row")
	}
	if !strings.Contains(project, inTree(styleFailed).Render("1 failed")) {
		t.Errorf("the tree's failures should be a shade back: %q", project)
	}
	if !strings.Contains(project, styleDim.Render("5 tests")) {
		t.Errorf("how many tests there are should be grey: %q", project)
	}
	footer := m.renderFooter()
	if !strings.Contains(footer, styleFailed.Bold(true).Render("1 failed")) {
		t.Errorf("the footer's failures should carry: %q", ansi.Strip(footer))
	}
	if inTree(styleFailed).Render("x") == styleFailed.Bold(true).Render("x") {
		t.Error("the two should not look the same")
	}
	// A duration in the tree is a shade back from the same time in the log.
	if treeDuration(time.Second) == renderDuration(time.Second) {
		t.Error("a tree duration should be dimmer than a log one")
	}
}

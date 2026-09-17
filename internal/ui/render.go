package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"dtest/internal/tree"
)

const (
	headerH = 1
	bottomH = 8 // blank, four summary rows, blank, status, hint
)

// bottomHeight is the height of the block under the report: the summary
// and status, or the usage list while help is shown.
func (m *Model) bottomHeight() int {
	if m.help {
		return len(helpRows) + 5
	}
	return bottomH
}

// layout sizes the report viewport to the window.
func (m *Model) layout() {
	m.report.SetWidth(max(1, m.width))
	m.report.SetHeight(max(1, m.height-headerH-m.bottomHeight()))
}

// View draws the whole screen.
func (m *Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("")
	}
	m.layout()
	bottom := m.renderSummary() + "\n" + m.renderStatus()
	if m.help {
		bottom = m.renderHelp()
	}
	content := lipgloss.JoinVertical(lipgloss.Left, m.renderHeader(), m.report.View(), bottom)
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "dtest " + m.name
	return v
}

// refresh rebuilds the report from the tree, keeps the cursor on its row
// and scrolls it into view.
func (m *Model) refresh() {
	var keep row
	if r, ok := m.current(); ok {
		keep = r
	}
	lines, rows := m.buildReport()
	m.rows = rows
	m.cursor = 0
	for i, r := range rows {
		if r.node == keep.node && r.failure == keep.failure {
			m.cursor = i
			break
		}
	}
	m.clampCursor()
	if len(rows) > 0 {
		// Every row line starts with a space reserved for the cursor marker.
		at := rows[m.cursor].line
		lines[at] = styleKey.Render(iconArrow) + strings.TrimPrefix(lines[at], " ")
	}
	sig := strings.Join(lines, "\n")
	if sig != m.reportSig {
		m.reportSig = sig
		m.report.SetContentLines(lines)
	}
	if len(rows) > 0 {
		m.report.EnsureVisible(rows[m.cursor].line, 0, 0)
	}
}

// buildReport lays out the report: the tree, then the failed tests with
// their messages, then (on request) the raw dotnet output. It returns the
// lines and the selectable rows pointing into them.
func (m *Model) buildReport() ([]string, []row) {
	var lines []string
	var rows []row
	nodes := m.tree.Visible(m.query)
	if len(nodes) == 0 {
		switch {
		case m.building:
			lines = append(lines, " "+m.spin.View()+" Building "+filepath.Base(m.cfg.Target)+"…")
		case m.loading > 0:
			lines = append(lines, " "+m.spin.View()+" Listing tests…")
		case m.query != "":
			lines = append(lines, styleDim.Render(" No tests match "+m.query))
		default:
			lines = append(lines, styleDim.Render(" No tests found"))
		}
	}
	for _, n := range nodes {
		rows = append(rows, row{node: n, line: len(lines)})
		lines = append(lines, m.renderNode(n))
		if n.IsLeaf() && n.Status() == tree.StatusFailed && n.Result != nil {
			if first := firstLine(n.Result.Message); first != "" {
				lines = append(lines, indent(n.Depth()+1)+styleFailed.Render("→ "+first))
			}
		}
	}

	var failed []*tree.Node
	for _, l := range m.tree.Leaves() {
		if l.Status() == tree.StatusFailed && l.Result != nil {
			failed = append(failed, l)
		}
	}
	if len(failed) > 0 {
		lines = append(lines, "", m.titledRule(fmt.Sprintf("Failed Tests %d", len(failed)), styleFailed.Bold(true)), "")
		for i, l := range failed {
			rows = append(rows, row{node: l, failure: true, line: len(lines)})
			lines = append(lines, "  "+badgeFail.Render("FAIL")+" "+styleBold.Render(breadcrumb(l)))
			for _, ml := range strings.Split(strings.TrimRight(l.Result.Message, "\n"), "\n") {
				lines = append(lines, colorMessage(ml))
			}
			if loc, ok := l.Result.FailureLocation(); ok {
				lines = append(lines, styleLocation.Render(fmt.Sprintf(" %s %s:%d", iconArrow, displayPath(loc.File), loc.Line)))
			}
			if i < len(failed)-1 {
				lines = append(lines, "", styleDim.Render(strings.Repeat(rule, m.width)), "")
			}
		}
	}

	if m.showOutput {
		title := "Output"
		if m.lastLog != "" && m.lastLog != buildLogKey {
			title = "Output  " + strings.TrimSuffix(filepath.Base(m.lastLog), filepath.Ext(m.lastLog))
		} else if m.lastLog == buildLogKey {
			title = "Output  build"
		}
		lines = append(lines, "", m.titledRule(title, styleBold), "")
		out := m.logs[m.lastLog]
		if len(out) == 0 {
			lines = append(lines, styleDim.Render("  (no output yet)"))
		}
		lines = append(lines, colorLog(out)...)
	}
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, m.width, "…")
	}
	return lines, rows
}

// renderNode draws one tree line: glyph, name and, for groups, the counts
// in parentheses, then the duration.
func (m *Model) renderNode(n *tree.Node) string {
	status := n.Status()
	name := n.Name
	var tail []string
	if !n.IsLeaf() {
		tail = append(tail, m.renderCounts(n.Counts()))
	}
	switch {
	case status == tree.StatusFailed && n.IsLeaf():
		name = styleFailed.Render(name)
	case status == tree.StatusSkipped && n.IsLeaf():
		name = styleDim.Render(name) + " " + styleSkipped.Render("[skipped]")
	case n.Kind == tree.KindProject:
		name = styleBold.Render(name)
	}
	if d := n.Duration(); d > 0 || (n.IsLeaf() && n.Result != nil && status != tree.StatusSkipped) {
		tail = append(tail, styleDim.Render(formatDuration(d)))
	}
	line := indent(n.Depth()) + m.statusIcon(status) + " " + name
	if len(tail) > 0 {
		line += " " + strings.Join(tail, " ")
	}
	return line
}

// renderCounts is vitest's "(4 tests | 1 failed | 1 skipped)".
func (m *Model) renderCounts(c tree.Counts) string {
	parts := []string{fmt.Sprintf("%d tests", c.Total)}
	if c.Total == 1 {
		parts[0] = "1 test"
	}
	if c.Running > 0 {
		parts = append(parts, styleRunning.Render(fmt.Sprintf("%d running", c.Running)))
	}
	if c.Failed > 0 {
		parts = append(parts, styleFailed.Bold(true).Render(fmt.Sprintf("%d failed", c.Failed)))
	}
	if c.Skipped > 0 {
		parts = append(parts, styleSkipped.Render(fmt.Sprintf("%d skipped", c.Skipped)))
	}
	sep := styleDim.Render(" | ")
	return styleDim.Render("(") + strings.Join(parts, sep) + styleDim.Render(")")
}

func (m *Model) statusIcon(s tree.Status) string {
	switch s {
	case tree.StatusRunning:
		return m.spin.View()
	case tree.StatusPassed:
		return stylePassed.Render(iconPassed)
	case tree.StatusFailed:
		return styleFailed.Bold(true).Render(iconFailed)
	case tree.StatusSkipped:
		return styleSkipped.Render(iconSkipped)
	}
	return styleDim.Render(iconNone)
}

// breadcrumb is "Project › Class › Method(args)" for a failure header.
func breadcrumb(l *tree.Node) string {
	var parts []string
	for n := l; n != nil; n = n.Parent {
		parts = append([]string{n.Name}, parts...)
	}
	return strings.Join(parts, " › ")
}

// displayPath shortens a source path relative to the working directory.
func displayPath(path string) string {
	if wd, err := filepath.Abs("."); err == nil {
		if rel, err := filepath.Rel(wd, path); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return path
}

// titledRule is a full-width ⎯⎯⎯ Title ⎯⎯⎯ line.
func (m *Model) titledRule(title string, style lipgloss.Style) string {
	side := max(0, (m.width-ansi.StringWidth(title)-2)/2)
	left := strings.Repeat(rule, side)
	right := strings.Repeat(rule, max(0, m.width-side-ansi.StringWidth(title)-2))
	return style.Render(left + " " + title + " " + right)
}

// Header, summary and status.

func (m *Model) renderHeader() string {
	left := badgeInfo.Render("DTEST") + " " + styleBold.Render(m.name)
	right := ""
	switch {
	case m.building:
		right = badgeRun.Render("RUN") + " building"
	case m.loading > 0:
		right = badgeRun.Render("RUN") + " listing tests"
	case m.run != nil:
		right = badgeRun.Render("RUN") + " " + m.run.req.label
		if len(m.queue) > 0 {
			right += styleDim.Render(fmt.Sprintf("  +%d queued", len(m.queue)))
		}
	}
	right = ansi.Truncate(right, max(10, m.width/2), "…")
	return fitLine(" "+left, right+" ", m.width)
}

// renderSummary is the fixed block at the bottom, as vitest prints after a
// run: projects, tests, start time and duration. Before the first run it is
// blank.
func (m *Model) renderSummary() string {
	if m.batchStart.IsZero() {
		return strings.Repeat("\n", 4)
	}
	var pc tree.Counts
	for _, p := range m.tree.Projects {
		pc.Total++
		switch p.Status() {
		case tree.StatusRunning:
			pc.Running++
		case tree.StatusFailed:
			pc.Failed++
		case tree.StatusPassed:
			pc.Passed++
		case tree.StatusSkipped:
			pc.Skipped++
		}
	}
	tc := m.tree.Counts()
	end := m.batchEnd
	if end.IsZero() {
		end = now()
	}
	var tests time.Duration
	for _, p := range m.tree.Projects {
		tests += p.Duration()
	}
	dur := formatDuration(end.Sub(m.batchStart)) + styleDim.Render(fmt.Sprintf(" (tests %s)", formatDuration(tests)))
	label := func(s string) string { return styleDim.Render(fmt.Sprintf("%14s", s)) }
	return strings.Join([]string{
		"",
		label("Test Projects") + "  " + summaryCounts(pc),
		label("Tests") + "  " + summaryCounts(tc),
		label("Start at") + "  " + m.batchStart.Format("15:04:05"),
		label("Duration") + "  " + dur,
	}, "\n")
}

// summaryCounts is "1 failed | 1 passed | 2 skipped (4)", listing only the
// non-zero groups, each bold in its colour as vitest prints them.
func summaryCounts(c tree.Counts) string {
	var parts []string
	if c.Running > 0 {
		parts = append(parts, styleRunning.Bold(true).Render(fmt.Sprintf("%d running", c.Running)))
	}
	if c.Failed > 0 {
		parts = append(parts, styleFailed.Bold(true).Render(fmt.Sprintf("%d failed", c.Failed)))
	}
	if c.Passed > 0 {
		parts = append(parts, stylePassed.Bold(true).Render(fmt.Sprintf("%d passed", c.Passed)))
	}
	if c.Skipped > 0 {
		parts = append(parts, styleSkipped.Bold(true).Render(fmt.Sprintf("%d skipped", c.Skipped)))
	}
	if rest := c.Total - c.Running - c.Failed - c.Passed - c.Skipped; rest > 0 {
		parts = append(parts, styleDim.Render(fmt.Sprintf("%d not run", rest)))
	}
	if len(parts) == 0 {
		return styleDim.Render(fmt.Sprintf("(%d)", c.Total))
	}
	return strings.Join(parts, styleDim.Render(" | ")) + styleDim.Render(fmt.Sprintf(" (%d)", c.Total))
}

// renderStatus is the two closing lines, as vitest prints them: a badge
// with the state, then "press ? to show help, press q to quit" aligned
// under the text. While filtering the second line is the prompt.
func (m *Model) renderStatus() string {
	var line string
	switch {
	case m.status != "":
		badge := badgeInfo.Render("INFO")
		if m.statusErr {
			badge = badgeFail.Render("FAIL")
		}
		line = " " + badge + " " + m.status
	case m.building:
		line = " " + badgeRun.Render("RUN") + " Building " + filepath.Base(m.cfg.Target) + "…"
	case m.loading > 0:
		line = " " + badgeRun.Render("RUN") + " Listing tests…"
	case m.run != nil:
		line = " " + badgeRun.Render("RUN") + " Running " + m.run.req.label + "…"
	case m.runsDone && m.tree.Counts().Failed > 0:
		line = " " + badgeFail.Render("FAIL") + " Tests failed."
	case m.runsDone:
		line = " " + badgePass.Render("PASS") + " Tests passed."
	default:
		c := m.tree.Counts()
		line = " " + badgeInfo.Render("DTEST") + fmt.Sprintf(" Ready. %d tests in %d projects.", c.Total, len(m.tree.Projects))
	}
	hint := strings.Repeat(" ", 7) + styleDim.Render("press ? to show help, press q to quit")
	switch {
	case m.filtering:
		hint = " " + styleKey.Render("?") + " Filter by test or project name " + styleDim.Render("›") + " " + m.query + "▏"
	case m.query != "":
		hint += styleDim.Render("  filter: " + m.query + " (esc clears)")
	}
	return "\n" + fit(line, m.width) + "\n" + fit(hint, m.width)
}

// Help.

type helpRow struct{ keys, desc string }

var helpRows = []helpRow{
	{"j / k", "to move"},
	{"gg / G", "to jump to the first / last line"},
	{"ctrl+d / ctrl+u", "to move half a page"},
	{"l / h", "to expand / collapse a project, class or theory"},
	{"enter or r", "to run the selected project, class or test"},
	{"a", "to run every project"},
	{"f", "to rerun only the failed tests"},
	{"x", "to cancel the running tests"},
	{"n / N", "to jump to the next / previous failure"},
	{"o", "to open the selected test in Neovim (a failure entry opens the failing line)"},
	{"t or /", "to filter by test or project name (enter keeps it, esc clears it)"},
	{"v", "to show or hide the raw dotnet output"},
	{"ctrl+r", "to rebuild and list the tests again"},
	{"q", "to quit"},
}

// renderHelp is vitest's "Watch Usage" list, drawn in place of the summary
// and status; any key closes it.
func (m *Model) renderHelp() string {
	var b strings.Builder
	b.WriteString("\n " + styleBold.Render("Usage") + "\n")
	for _, r := range helpRows {
		fmt.Fprintf(&b, " %s %s %s\n", styleDim.Render("press"), styleKey.Render(fmt.Sprintf("%-16s", r.keys)), r.desc)
	}
	nvim := "o sends the file and line to the Neovim listening on " + m.socket + " (from $nvim_sock or --nvim-socket)"
	if m.socket == "" {
		nvim = "$nvim_sock is not set, so o opens nvim in this terminal; set it to the socket of a Neovim started with --listen"
	}
	b.WriteString("\n " + styleDim.Render(nvim) + "\n")
	b.WriteString(" " + styleDim.Render("press any key to close this help"))
	return b.String()
}

// Output colouring.

// logRules classify raw dotnet lines, in order; the first match styles it.
var logRules = []struct {
	match func(string) bool
	style lipgloss.Style
}{
	{func(s string) bool { return strings.HasPrefix(s, "$ ") }, styleCommand},
	{func(s string) bool { return strings.HasPrefix(strings.TrimLeft(s, " "), "Passed ") }, stylePassed},
	{func(s string) bool { return strings.HasPrefix(strings.TrimLeft(s, " "), "Failed ") }, styleFailed.Bold(true)},
	{func(s string) bool { return strings.HasPrefix(strings.TrimLeft(s, " "), "Skipped ") }, styleSkipped},
	{func(s string) bool { return strings.HasPrefix(s, "[xUnit.net") }, styleDim},
	{func(s string) bool { return strings.HasPrefix(strings.TrimLeft(s, " "), "at ") }, styleDim},
	{func(s string) bool {
		return strings.Contains(s, "error ") || strings.Contains(s, "Error Message") || strings.Contains(s, "Test Run Failed") ||
			strings.Contains(s, "Build FAILED") || strings.Contains(s, "[FAIL]") || strings.HasPrefix(strings.TrimLeft(s, " "), "Failed:") ||
			strings.Contains(s, "Exception") || strings.Contains(s, "Assert.") || strings.Contains(s, "Failure")
	}, styleLogError},
	{func(s string) bool {
		return strings.Contains(s, "warning ") || strings.Contains(s, "[SKIP]") || strings.HasPrefix(strings.TrimLeft(s, " "), "Skipped:")
	}, styleSkipped},
	{func(s string) bool {
		return strings.Contains(s, "Test Run Successful") || strings.Contains(s, "Build succeeded") || strings.HasPrefix(strings.TrimLeft(s, " "), "Passed:")
	}, stylePassed},
}

// colorLog styles raw dotnet output for reading: results green, red and
// yellow, expected values green and actual values red, errors red, commands
// cyan, runner chatter and stack frames dim, everything else in the
// terminal's default colour.
func colorLog(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = colorLine(line)
	}
	return out
}

func colorLine(line string) string {
	if s, ok := colorAssertion(line); ok {
		return s
	}
	for _, r := range logRules {
		if r.match(line) {
			return r.style.Render(line)
		}
	}
	return line
}

// colorAssertion highlights the two sides of a failed comparison: the
// expected value green and the actual value red. It understands xUnit and
// NUnit ("Expected: …" then "Actual: …" or "But was: …" on their own lines)
// and MSTest ("Expected:<…>. Actual:<…>." on one line).
func colorAssertion(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(trimmed)]
	switch {
	case strings.HasPrefix(trimmed, "Expected"):
		if i := strings.Index(trimmed, "Actual"); i > 0 {
			return indent + styleExpected.Render(trimmed[:i]) + styleActual.Render(trimmed[i:]), true
		}
		return indent + styleExpected.Render(trimmed), true
	case strings.HasPrefix(trimmed, "Actual"), strings.HasPrefix(trimmed, "But was"):
		return indent + styleActual.Render(trimmed), true
	}
	return "", false
}

// colorMessage styles one line of a failure message: assertion sides in
// their colours, anything else red.
func colorMessage(line string) string {
	if s, ok := colorAssertion(line); ok {
		return s
	}
	return styleLogError.Render(line)
}

// Text helpers.

var spaces = regexp.MustCompile(`\s+`)

// firstLine is the first non-empty line of s, whitespace collapsed.
func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return spaces.ReplaceAllString(l, " ")
		}
	}
	return ""
}

// indent is the left margin of a tree line: two columns (the first holds
// the cursor marker) plus two per level.
func indent(depth int) string { return "  " + strings.Repeat("  ", depth) }

// formatDuration renders a test time compactly: 0.032s below a second, 1.5s
// below ten, 35s below a minute, then 2m34s and 1h02m.
func formatDuration(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%.3fs", d.Seconds())
	case d < 10*time.Second:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Round(time.Second).Seconds()))
	case d < time.Hour:
		d = d.Round(time.Second)
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	d = d.Round(time.Minute)
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

// fit truncates or pads s to exactly width cells.
func fit(s string, width int) string {
	s = ansi.Truncate(s, width, "")
	if pad := width - ansi.StringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// fitLine lays left and right at the edges of a line width cells wide.
func fitLine(left, right string, width int) string {
	rw := ansi.StringWidth(right)
	left = ansi.Truncate(left, max(0, width-rw-1), "…")
	pad := width - ansi.StringWidth(left) - rw
	if pad < 1 {
		return fit(left+right, width)
	}
	return left + strings.Repeat(" ", pad) + right
}

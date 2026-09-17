package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"dtest/internal/tree"
)

const (
	headerH  = 1
	titleH   = 1  // each pane has a title line
	minWidth = 40 // below this the bottom panes stack one above the other
)

// geometry is the current pane arrangement.
type geometry struct {
	logH          int // log pane body height (top, full width)
	treeW, statsW int // bottom pane widths
	bottomH       int // bottom pane body height
}

// layout splits the window: the log takes about 70% of the height, the
// tree and the stats share the rest side by side.
func (m *Model) layout() geometry {
	body := max(4, m.height-headerH-2*titleH)
	g := geometry{}
	g.logH = max(1, body*7/10)
	g.bottomH = max(1, body-g.logH)
	g.treeW = m.width / 2
	g.statsW = m.width - g.treeW - 1 // one column for the separator
	if g.statsW < 1 {
		g.treeW, g.statsW = m.width, 0
	}
	m.log.SetWidth(max(1, m.width))
	m.log.SetHeight(g.logH)
	m.treeView.SetWidth(max(1, g.treeW))
	m.treeView.SetHeight(g.bottomH)
	return g
}

// View draws the whole screen.
func (m *Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("")
	}
	g := m.layout()
	top := lipgloss.JoinVertical(lipgloss.Left, m.paneTitle(m.logTitle(), m.focus == paneLog, m.width), m.log.View())
	if m.help {
		top = lipgloss.JoinVertical(lipgloss.Left, m.paneTitle("Usage", true, m.width), m.renderHelp(g.logH))
	}
	left := lipgloss.JoinVertical(lipgloss.Left, m.paneTitle(m.treeTitle(), m.focus == paneTree, g.treeW), m.treeView.View())
	bottom := left
	if g.statsW > 0 {
		sep := strings.TrimRight(strings.Repeat(styleDim.Render("│")+"\n", g.bottomH+titleH), "\n")
		right := lipgloss.JoinVertical(lipgloss.Left, m.paneTitle("Summary", m.focus == paneStats, g.statsW), m.renderStats(g.statsW, g.bottomH))
		bottom = lipgloss.JoinHorizontal(lipgloss.Top, left, sep, right)
	}
	content := lipgloss.JoinVertical(lipgloss.Left, m.renderHeader(), top, bottom)
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "dtest " + m.name
	return v
}

// paneTitle is a ⎯⎯ Title ⎯⎯⎯ line, bright when the pane is focused.
func (m *Model) paneTitle(title string, focused bool, width int) string {
	style := styleDim
	if focused {
		style = styleKey
	}
	text := rule + rule + " " + title + " "
	return style.Render(text + strings.Repeat(rule, max(0, width-ansi.StringWidth(text))))
}

func (m *Model) logTitle() string {
	if m.showOutput {
		name := "build"
		if m.lastLog != "" && m.lastLog != buildLogKey {
			name = strings.TrimSuffix(filepath.Base(m.lastLog), filepath.Ext(m.lastLog))
		}
		return "Output  " + name
	}
	if n := len(m.fails); n > 0 {
		return fmt.Sprintf("Failed Tests %d", n)
	}
	return "Log"
}

func (m *Model) treeTitle() string {
	switch {
	case m.filtering:
		return "? Filter › " + m.query + "▏"
	case m.query != "":
		return "Tests  filter: " + m.query
	}
	return "Tests"
}

// refresh rebuilds both panes from the tree and scrolls their cursors into
// view.
func (m *Model) refresh() {
	m.layout()
	m.refreshTree()
	m.refreshLog()
}

func (m *Model) refreshTree() {
	keep := m.current()
	m.rows = m.tree.Visible(m.query)
	m.cursor = 0
	for i, r := range m.rows {
		if r == keep {
			m.cursor = i
			break
		}
	}
	m.cursor = clamp(m.cursor, len(m.rows))
	lines := make([]string, 0, len(m.rows))
	for i, n := range m.rows {
		lines = append(lines, ansi.Truncate(m.renderNode(n, i == m.cursor), m.treeView.Width(), "…"))
	}
	if len(m.rows) == 0 {
		lines = append(lines, m.emptyTreeMessage())
	}
	sig := strings.Join(lines, "\n")
	if sig != m.treeSig {
		m.treeSig = sig
		m.treeView.SetContentLines(lines)
	}
	if len(m.rows) > 0 {
		m.treeView.EnsureVisible(m.cursor, 0, 0)
	}
}

func (m *Model) emptyTreeMessage() string {
	switch {
	case m.building:
		return "  " + m.spin.View() + " Building " + filepath.Base(m.cfg.Target) + "…"
	case m.loading > 0:
		return "  " + m.spin.View() + " Listing tests…"
	case m.query != "":
		return styleDim.Render("  No tests match " + m.query)
	}
	return styleDim.Render("  No tests found")
}

// refreshLog lays out the failure entries (or the raw output) and scrolls
// the selected entry into view.
func (m *Model) refreshLog() {
	keep := m.currentFailure()
	m.fails = m.fails[:0]
	for _, l := range m.tree.Leaves() {
		if l.Status() == tree.StatusFailed && l.Result != nil {
			m.fails = append(m.fails, l)
		}
	}
	m.failCursor = 0
	for i, f := range m.fails {
		if f == keep {
			m.failCursor = i
			break
		}
	}
	m.failCursor = clamp(m.failCursor, len(m.fails))

	var lines []string
	m.failLines = m.failLines[:0]
	if m.showOutput {
		out := m.logs[m.lastLog]
		if len(out) == 0 {
			lines = append(lines, styleDim.Render("  (no output yet)"))
		}
		lines = append(lines, colorLog(out)...)
	} else {
		lines = m.renderFailures()
	}
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, m.log.Width(), "…")
	}
	sig := strings.Join(lines, "\n")
	if sig != m.logSig {
		m.logSig = sig
		m.log.SetContentLines(lines)
	}
	if !m.showOutput && m.failCursor < len(m.failLines) {
		m.log.EnsureVisible(m.failLines[m.failCursor], 0, 0)
	}
}

// renderFailures is the minimal log: build errors, then every failed test
// with its message and location, or a line saying there is nothing to show.
func (m *Model) renderFailures() []string {
	var lines []string
	if errs := buildErrors(m.logs[buildLogKey]); len(errs) > 0 {
		lines = append(lines, "  "+badgeFail.Render("BUILD")+" "+styleBold.Render(filepath.Base(m.cfg.Target)))
		lines = append(lines, colorLog(errs)...)
		lines = append(lines, "")
	}
	if len(m.fails) == 0 {
		switch {
		case len(lines) > 0:
		case m.run != nil:
			lines = append(lines, "  "+m.spin.View()+" Running "+m.run.req.label+"…")
		case m.runsDone:
			lines = append(lines, "  "+stylePassed.Render(iconPassed+" No failed tests."))
		default:
			lines = append(lines, styleDim.Render("  Failures appear here. Select a project, class or test below and press enter to run it, a for everything."))
		}
		return lines
	}
	for i, l := range m.fails {
		if i > 0 {
			lines = append(lines, "", styleDim.Render(strings.Repeat(rule, m.log.Width())), "")
		}
		m.failLines = append(m.failLines, len(lines))
		marker := " "
		if i == m.failCursor {
			marker = m.marker(m.focus == paneLog)
		}
		lines = append(lines, marker+" "+badgeFail.Render("FAIL")+" "+styleBold.Render(breadcrumb(l)))
		for _, ml := range strings.Split(strings.TrimRight(l.Result.Message, "\n"), "\n") {
			lines = append(lines, colorMessage(ml))
		}
		if loc, ok := l.Result.FailureLocation(); ok {
			lines = append(lines, styleLocation.Render(fmt.Sprintf(" %s %s:%d", iconArrow, displayPath(loc.File), loc.Line)))
		}
	}
	return lines
}

// buildErrors keeps the error lines of a build log.
func buildErrors(log []string) []string {
	var out []string
	for _, l := range log {
		if strings.Contains(l, ": error ") || strings.Contains(l, "Build FAILED") || strings.HasPrefix(l, "dotnet build failed") {
			out = append(out, l)
		}
	}
	return out
}

// marker is the cursor glyph, bright in the focused pane and dim elsewhere.
func (m *Model) marker(focused bool) string {
	if focused {
		return styleKey.Render(iconArrow)
	}
	return styleDim.Render(iconArrow)
}

// renderNode draws one tree line: marker, glyph, name and, for groups, the
// counts in parentheses, then the duration.
func (m *Model) renderNode(n *tree.Node, selected bool) string {
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
	marker := " "
	if selected {
		marker = m.marker(m.focus == paneTree)
	}
	line := marker + " " + strings.Repeat("  ", n.Depth()) + m.statusIcon(status) + " " + name
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

// Header and stats.

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

// renderStats fills the bottom-right pane: vitest's summary block, then
// the state badge and the key hint.
func (m *Model) renderStats(width, height int) string {
	var lines []string
	if !m.batchStart.IsZero() {
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
		end := m.batchEnd
		if end.IsZero() {
			end = now()
		}
		var tests time.Duration
		for _, p := range m.tree.Projects {
			tests += p.Duration()
		}
		lines = append(lines, wrapSummary("Test Projects", summaryParts(pc), width)...)
		lines = append(lines, wrapSummary("Tests", summaryParts(m.tree.Counts()), width)...)
		lines = append(lines,
			summaryLabel("Start at")+m.batchStart.Format("15:04:05"),
			summaryLabel("Duration")+formatDuration(end.Sub(m.batchStart))+styleDim.Render(fmt.Sprintf(" (tests %s)", formatDuration(tests))),
			"")
	} else {
		c := m.tree.Counts()
		lines = append(lines, summaryLabel("Tests")+styleDim.Render(fmt.Sprintf("(%d)", c.Total)), "")
	}
	lines = append(lines, m.renderStatusLines()...)
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, l := range lines {
		lines[i] = fit(l, width)
	}
	return strings.Join(lines[:height], "\n")
}

// summaryLabelW is the width of the summary's label column: the longest
// label, right-aligned, plus two spaces.
const summaryLabelW = 16

func summaryLabel(s string) string {
	return styleDim.Render(fmt.Sprintf("%*s", summaryLabelW-2, s)) + "  "
}

// wrapSummary lays a labelled list of parts out as "label  a | b | c",
// continuing on indented lines when the pane is too narrow for one.
func wrapSummary(label string, parts []string, width int) []string {
	sep := styleDim.Render(" | ")
	var lines []string
	cur := summaryLabel(label)
	curW := summaryLabelW
	first := true
	for _, p := range parts {
		pw := ansi.StringWidth(p)
		if !first && curW+3+pw > width {
			lines = append(lines, cur)
			cur, curW, first = strings.Repeat(" ", summaryLabelW), summaryLabelW, true
		}
		if !first {
			cur += sep
			curW += 3
		}
		cur += p
		curW += pw
		first = false
	}
	return append(lines, cur)
}

// summaryParts is vitest's "1 failed | 1 passed | 2 skipped (4)" as its
// pieces, listing only the non-zero groups, each bold in its colour, with
// the total attached to the last one.
func summaryParts(c tree.Counts) []string {
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
		return []string{styleDim.Render(fmt.Sprintf("(%d)", c.Total))}
	}
	parts[len(parts)-1] += styleDim.Render(fmt.Sprintf(" (%d)", c.Total))
	return parts
}

// renderStatusLines is vitest's closing pair: a badge with the state, then
// "press ? to show help, press q to quit" aligned under the text.
func (m *Model) renderStatusLines() []string {
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
	return []string{line, strings.Repeat(" ", 7) + styleDim.Render("press ? to show help, press q to quit")}
}

// Help.

type helpRow struct{ keys, desc string }

var helpRows = []helpRow{
	{"ctrl+k / ctrl+j", "to focus the log above / the tree below"},
	{"ctrl+h / ctrl+l", "to focus the tree / the summary"},
	{"j / k", "to move (in the log: between failures)"},
	{"gg / G", "to jump to the first / last entry"},
	{"ctrl+d / ctrl+u", "to move half a page"},
	{"l / h", "to expand / collapse a project, class or theory"},
	{"enter or r", "to run the selected project, class or test"},
	{"a", "to run every project"},
	{"f", "to rerun only the failed tests"},
	{"x", "to cancel the running tests"},
	{"n / N", "to jump to the next / previous failure"},
	{"o", "to open the selection in Neovim (from the log: the failing line)"},
	{"t or /", "to filter the tree by test or project name (enter keeps it, esc clears it)"},
	{"v", "to show or hide the raw dotnet output in the log pane"},
	{"ctrl+r", "to rebuild and list the tests again"},
	{"q", "to quit"},
}

// renderHelp is vitest's "Watch Usage" list, drawn in the log pane; any
// key closes it.
func (m *Model) renderHelp(height int) string {
	var b strings.Builder
	for _, r := range helpRows {
		fmt.Fprintf(&b, " %s %s %s\n", styleDim.Render("press"), styleKey.Render(fmt.Sprintf("%-16s", r.keys)), r.desc)
	}
	nvim := "o sends the file and line to the Neovim listening on " + m.socket + " (from $nvim_sock or --nvim-socket)"
	if m.socket == "" {
		nvim = "$nvim_sock is not set, so o opens nvim in this terminal; set it to the socket of a Neovim started with --listen"
	}
	b.WriteString("\n " + styleDim.Render(nvim) + "\n")
	b.WriteString(" " + styleDim.Render("press any key to close this help"))
	return lipgloss.NewStyle().Width(m.width).Height(height).MaxHeight(height).Render(b.String())
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

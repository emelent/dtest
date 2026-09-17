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
	headerH = 1
	titleH  = 1 // each pane has a title line
)

// geometry is the current pane arrangement.
type geometry struct {
	logH    int // log pane body height (top, full width)
	bottomH int // bottom pane body height
	treeW   int // width left for the tree beside the summary
	statsW  int // width of the right-aligned summary; 0 when it does not fit
}

// layout splits the window: the log takes about 70% of the height; the
// bottom pane holds the tree with the summary right-aligned beside it.
func (m *Model) layout() geometry {
	body := max(4, m.height-headerH-2*titleH)
	g := geometry{}
	g.logH = max(1, body*7/10)
	g.bottomH = max(1, body-g.logH)
	// The block is as wide as its summary lines; the state line on top is
	// truncated to that, so a long "Running …" label cannot shift the block.
	stats := m.statsLines()
	for _, l := range stats[1:] {
		g.statsW = max(g.statsW, ansi.StringWidth(l))
	}
	g.treeW = m.width - g.statsW - 2
	if g.treeW < 30 {
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
	top := lipgloss.JoinVertical(lipgloss.Left, m.paneTitle(m.logTitle(), m.focus == paneLog, m.width, m.logPosition()), m.log.View())
	if m.help {
		top = lipgloss.JoinVertical(lipgloss.Left, m.paneTitle("Usage", true, m.width, ""), m.renderHelp(g.logH))
	}
	bottom := lipgloss.JoinVertical(lipgloss.Left, m.paneTitle(m.treeTitle(), m.focus == paneTree, m.width, ""), m.renderBottom(g))
	content := lipgloss.JoinVertical(lipgloss.Left, m.renderHeader(), top, bottom)
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "dtest " + m.name
	return v
}

// paneTitle is a ⎯⎯ Title ⎯⎯⎯ line, bright when the pane is focused, with
// an optional note such as the cursor position near the right end.
func (m *Model) paneTitle(title string, focused bool, width int, note string) string {
	style := styleDim
	if focused {
		style = styleKey
	}
	text := rule + rule + " " + title + " "
	tail := rule + rule
	if note != "" {
		tail = " " + note + " " + rule + rule
	}
	return style.Render(text + strings.Repeat(rule, max(0, width-ansi.StringWidth(text)-ansi.StringWidth(tail))) + tail)
}

// logPosition is the log cursor's line over the line count, "12/80".
func (m *Model) logPosition() string {
	if len(m.logLines) == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", m.logCursor+1, len(m.logLines))
}

func (m *Model) logTitle() string {
	n := m.current()
	if m.showOutput {
		name := "build"
		if n != nil && m.logs[n.Project().Path] != nil {
			name = n.Project().Name
		}
		return "Output  " + name
	}
	if n == nil {
		return "Log"
	}
	return "Log  " + breadcrumb(n)
}

func (m *Model) treeTitle() string {
	title := "Tests"
	switch m.statusFilter {
	case tree.StatusFailed:
		title += "  failed only"
	case tree.StatusSkipped:
		title += "  skipped only"
	}
	switch {
	case m.filtering:
		return "? Filter › " + m.query + "▏"
	case m.query != "":
		return title + "  filter: " + m.query
	}
	return title
}

// refresh rebuilds both panes from the tree: the rows around the cursor,
// and the log for the selected node.
func (m *Model) refresh() {
	m.layout()
	m.refreshTree()
	m.refreshLog()
}

func (m *Model) refreshTree() {
	keep := m.current()
	m.rows = m.tree.Visible(m.query, m.statusFilter)
	m.cursor = 0
	// Keep the cursor on its node or, when a fold hid it, its nearest
	// visible ancestor.
	for n := keep; n != nil; n = n.Parent {
		if i := indexOf(m.rows, n); i >= 0 {
			m.cursor = i
			break
		}
	}
	m.cursor = clamp(m.cursor, len(m.rows))
	lines := make([]string, 0, len(m.rows))
	for i, n := range m.rows {
		line := ansi.Truncate(m.renderNode(n), m.treeView.Width(), "…")
		if i == m.cursor {
			line = highlight(line, m.treeView.Width(), m.focus == paneTree)
		}
		lines = append(lines, line)
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

func indexOf(rows []*tree.Node, n *tree.Node) int {
	for i, r := range rows {
		if r == n {
			return i
		}
	}
	return -1
}

func (m *Model) emptyTreeMessage() string {
	switch {
	case m.building:
		return "  " + m.spin.View() + " Building " + filepath.Base(m.cfg.Target) + "…"
	case m.loading > 0:
		return "  " + m.spin.View() + " Listing tests…"
	case m.query != "":
		return styleDim.Render("  No tests match " + m.query)
	case m.statusFilter == tree.StatusFailed:
		return styleDim.Render("  No failed tests")
	case m.statusFilter == tree.StatusSkipped:
		return styleDim.Render("  No skipped tests")
	}
	return styleDim.Render("  No tests found")
}

// refreshLog shows the results for the node under the tree cursor (or the
// raw output of its project), with the cursor line marked. Selecting
// another node starts at the top; a log that is being appended to keeps
// following its tail.
func (m *Model) refreshLog() {
	n := m.current()
	var lines []string
	if m.showOutput {
		key := buildLogKey
		if n != nil && m.logs[n.Project().Path] != nil {
			key = n.Project().Path
		}
		out := m.logs[key]
		if len(out) == 0 {
			lines = append(lines, styleDim.Render("  (no output yet)"))
		}
		lines = append(lines, colorLog(out)...)
	} else {
		lines = m.renderNodeLog(n)
	}
	changed := n != m.logNode
	if changed {
		m.logCursor = 0
	}
	m.logCursor = clamp(m.logCursor, len(lines))
	// Wrap each line at word boundaries, remembering where each starts;
	// every row of the cursor line is highlighted.
	m.logLines, m.logStarts = m.logLines[:0], m.logStarts[:0]
	var display []string
	width := max(1, m.log.Width())
	for i, l := range lines {
		m.logLines = append(m.logLines, ansi.Strip(l))
		m.logStarts = append(m.logStarts, len(display))
		for _, seg := range strings.Split(ansi.Wrap(l, width, ""), "\n") {
			if i == m.logCursor {
				seg = highlight(seg, width, m.focus == paneLog)
			}
			display = append(display, seg)
		}
	}
	sig := strings.Join(display, "\n")
	if sig == m.logSig && !changed {
		return
	}
	follow := m.log.AtBottom() || m.log.PastBottom()
	m.logSig, m.logNode = sig, n
	m.log.SetContentLines(display)
	switch {
	case changed:
		m.log.GotoTop()
	case m.showOutput && follow:
		m.log.GotoBottom()
	}
}

// renderNodeLog is the minimal log for a node: build errors first, then
// for a test its result, message, stack trace and output; for a group its
// passed / failed / skipped tally and every failure beneath it.
func (m *Model) renderNodeLog(n *tree.Node) []string {
	var lines []string
	if errs := buildErrors(m.logs[buildLogKey]); len(errs) > 0 {
		lines = append(lines, "  "+badgeFail.Render("BUILD")+" "+styleBold.Render(filepath.Base(m.cfg.Target)))
		lines = append(lines, colorLog(errs)...)
		lines = append(lines, "")
	}
	if n == nil {
		if len(lines) == 0 {
			lines = append(lines, styleDim.Render("  Select a project, class or test below; its results show here."))
		}
		return lines
	}
	if n.IsLeaf() {
		return append(lines, m.renderLeafLog(n)...)
	}
	c := n.Counts()
	head := "  " + m.statusIcon(n.Status()) + " " + styleBold.Render(breadcrumb(n))
	if d := n.Duration(); d > 0 {
		head += " " + styleDim.Render(formatDuration(d))
	}
	// Then the section's tally, as the summary prints it.
	lines = append(lines, head, "    "+strings.Join(summaryParts(c, true), styleDim.Render(" | ")))
	var failed []*tree.Node
	for _, l := range n.Leaves() {
		if l.Status() == tree.StatusFailed && l.Result != nil {
			failed = append(failed, l)
		}
	}
	switch {
	case len(failed) > 0:
		for _, l := range failed {
			lines = append(lines, "")
			lines = append(lines, m.renderFailure(l, false)...)
		}
	case c.Running > 0, c.Queued > 0:
		// The tally above already says how many are running or queued.
	case c.Passed+c.Skipped > 0:
		lines = append(lines, "", "  "+stylePassed.Render(iconPassed+" No failed tests."))
	default:
		lines = append(lines, "", styleDim.Render("  Not run yet. Press enter to run it, a for everything."))
	}
	return lines
}

// renderLeafLog describes one test's latest result.
func (m *Model) renderLeafLog(n *tree.Node) []string {
	r := n.Result
	head := "  " + m.statusIcon(n.Status()) + " " + styleBold.Render(breadcrumb(n))
	switch {
	case n.Status() == tree.StatusRunning, n.Status() == tree.StatusQueued:
		return []string{head} // the glyph in the header says it all
	case r == nil:
		return []string{head, "", styleDim.Render("  Not run yet. Press enter to run it, o to open it in nvim.")}
	case n.Status() == tree.StatusFailed:
		return append([]string{head, ""}, m.renderFailure(n, true)...)
	}
	lines := []string{head + " " + styleDim.Render(formatDuration(r.Duration)), ""}
	switch n.Status() {
	case tree.StatusSkipped:
		lines = append(lines, "  "+styleSkipped.Render(iconSkipped+" Skipped"))
		if r.Message != "" {
			lines = append(lines, styleDim.Render("  "+r.Message))
		}
	default:
		lines = append(lines, "  "+stylePassed.Render(iconPassed+" Passed in "+formatDuration(r.Duration)))
	}
	return append(lines, outputLines(r.Output)...)
}

// renderFailure is one failed test: FAIL badge and breadcrumb, the message
// with its assertion coloured, the failing location and, in full, the
// stack trace and captured output.
func (m *Model) renderFailure(l *tree.Node, full bool) []string {
	r := l.Result
	lines := []string{"  " + badgeFail.Render("FAIL") + " " + styleBold.Render(breadcrumb(l)) + " " + styleDim.Render(formatDuration(r.Duration))}
	for _, ml := range strings.Split(strings.TrimRight(r.Message, "\n"), "\n") {
		lines = append(lines, colorMessage(ml))
	}
	if loc, ok := r.FailureLocation(); ok {
		lines = append(lines, styleLocation.Render(fmt.Sprintf(" %s %s:%d", iconArrow, displayPath(loc.File), loc.Line)))
	}
	if full && r.StackTrace != "" {
		lines = append(lines, "", styleDim.Render("  Stack trace"))
		for _, sl := range strings.Split(r.StackTrace, "\n") {
			lines = append(lines, styleDim.Render(sl))
		}
	}
	if full {
		lines = append(lines, outputLines(r.Output)...)
	}
	return lines
}

func outputLines(output string) []string {
	if output == "" {
		return nil
	}
	lines := []string{"", styleDim.Render("  Output")}
	return append(lines, strings.Split(output, "\n")...)
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

// highlight paints the cursor line's background across the full width,
// brighter in the focused pane. Styled text resets the attributes after
// each coloured span, so the background is re-applied after every reset to
// keep the whole row lit while the colours stay.
func highlight(line string, width int, focused bool) string {
	bg := bgUnfocused
	if focused {
		bg = bgFocused
	}
	s := fit(line, width)
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+bg)
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[m"+bg)
	return bg + s + "\x1b[0m"
}

// renderBottom draws the tree with the summary right-aligned beside it: the
// summary block keeps its label column and sits flush with the right edge.
func (m *Model) renderBottom(g geometry) string {
	tree := m.treeView.View()
	if g.statsW == 0 {
		return tree
	}
	stats := m.statsLines()
	// Bottom-aligned: pad above, or keep the last lines when it is taller
	// than the pane.
	for len(stats) < g.bottomH {
		stats = append([]string{""}, stats...)
	}
	stats = stats[len(stats)-g.bottomH:]
	for i, l := range stats {
		stats[i] = fit(ansi.Truncate(l, g.statsW, "…"), g.statsW)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, tree, "  ", strings.Join(stats, "\n"))
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
	line := "  " + strings.Repeat("  ", n.Depth()) + m.treeIcon(n) + " " + name
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
	if c.Queued > 0 {
		parts = append(parts, styleQueued.Render(fmt.Sprintf("%d queued", c.Queued)))
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

// treeIcon is a node's glyph in the tree: a fold arrow for a project, class
// or theory, a status glyph for a test, each in the colour of its status.
// While tests run, only the top-most running node (the project, since
// running rolls up) spins; everything running beneath it is simply drawn
// in the running colour, so the tree does not flicker all over.
func (m *Model) treeIcon(n *tree.Node) string {
	status := n.Status()
	if status == tree.StatusRunning {
		if n.Parent == nil || n.Parent.Status() != tree.StatusRunning {
			return m.spin.View()
		}
		if n.IsLeaf() {
			return styleRunning.Render(iconNone)
		}
		return styleRunning.Render(m.arrow(n))
	}
	if n.IsLeaf() {
		return m.statusIcon(status)
	}
	switch status {
	case tree.StatusQueued:
		return styleQueued.Render(m.arrow(n))
	case tree.StatusPassed:
		return stylePassed.Render(m.arrow(n))
	case tree.StatusFailed:
		return styleFailed.Bold(true).Render(m.arrow(n))
	case tree.StatusSkipped:
		return styleSkipped.Render(m.arrow(n))
	}
	return styleDim.Render(m.arrow(n))
}

// arrow is the fold glyph for a group: open when expanded or filtered.
func (m *Model) arrow(n *tree.Node) string {
	if n.Expanded || m.filtered() {
		return iconOpen
	}
	return iconClosed
}

func (m *Model) statusIcon(s tree.Status) string {
	switch s {
	case tree.StatusRunning:
		return m.spin.View()
	case tree.StatusQueued:
		return styleQueued.Render(iconQueued)
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
	line := " " + badgeInfo.Render("DTEST") + " " + styleBold.Render(m.name)
	if m.cfg.Version != "" {
		line += "  " + styleDim.Render(m.cfg.Version)
	}
	return fit(line, m.width)
}

// statsLines is the block beside the tree: the state line, then the
// counts stacked one per row, the start time and duration, and the key
// hint. Every row is always present so the block never changes shape.
func (m *Model) statsLines() []string {
	var pc tree.Counts
	for _, p := range m.tree.Projects {
		pc.Total++
		switch p.Status() {
		case tree.StatusFailed:
			pc.Failed++
		case tree.StatusPassed:
			pc.Passed++
		case tree.StatusSkipped:
			pc.Skipped++
		}
	}
	c := m.tree.Counts()
	start, dur := styleDim.Render("–"), styleDim.Render("–")
	if !m.batchStart.IsZero() {
		end := m.batchEnd
		if end.IsZero() {
			end = now()
		}
		var tests time.Duration
		for _, p := range m.tree.Projects {
			tests += p.Duration()
		}
		start = m.batchStart.Format("15:04:05")
		dur = formatDuration(end.Sub(m.batchStart)) + styleDim.Render(fmt.Sprintf(" (tests %s)", formatDuration(tests)))
	}
	count := func(n int, style lipgloss.Style) string {
		if n == 0 {
			return styleDim.Render("0")
		}
		return style.Bold(true).Render(fmt.Sprint(n))
	}
	return []string{
		m.stateLine(),
		"",
		summaryLabel("Test Projects") + strings.Join(summaryParts(pc, false), styleDim.Render(" | ")),
		summaryLabel("Tests") + fmt.Sprint(c.Total),
		summaryLabel("Failed") + count(c.Failed, styleFailed),
		summaryLabel("Skipped") + count(c.Skipped, styleSkipped),
		summaryLabel("Passed") + count(c.Passed, stylePassed),
		summaryLabel("Start at") + start,
		summaryLabel("Duration") + dur,
		"",
		strings.Repeat(" ", 7) + styleDim.Render("press ? to show help, press q to quit"),
	}
}

// summaryLabelW is the width of the summary's label column: the longest
// label, right-aligned, plus two spaces.
const summaryLabelW = 16

func summaryLabel(s string) string {
	return styleDim.Render(fmt.Sprintf("%*s", summaryLabelW-2, s)) + "  "
}

// summaryParts is vitest's "1 failed | 1 passed | 2 skipped (4)" as its
// pieces, listing only the non-zero groups, each bold in its colour, with
// the total attached to the last one. With notRun the tests without a
// result are counted too (running ones included, so the line does not keep
// changing during a run); the bottom summary leaves them out.
func summaryParts(c tree.Counts, notRun bool) []string {
	var parts []string
	if c.Failed > 0 {
		parts = append(parts, styleFailed.Bold(true).Render(fmt.Sprintf("%d failed", c.Failed)))
	}
	if c.Passed > 0 {
		parts = append(parts, stylePassed.Bold(true).Render(fmt.Sprintf("%d passed", c.Passed)))
	}
	if c.Skipped > 0 {
		parts = append(parts, styleSkipped.Bold(true).Render(fmt.Sprintf("%d skipped", c.Skipped)))
	}
	if rest := c.Total - c.Failed - c.Passed - c.Skipped; notRun && rest > 0 {
		parts = append(parts, styleDim.Render(fmt.Sprintf("%d not run", rest)))
	}
	if len(parts) == 0 {
		return []string{styleDim.Render(fmt.Sprintf("(%d)", c.Total))}
	}
	parts[len(parts)-1] += styleDim.Render(fmt.Sprintf(" (%d)", c.Total))
	return parts
}

// stateLine is what dtest is doing: a status message, or building or
// listing as plain text. It is empty otherwise, before, during and after
// runs: the tree's spinners and the stacked counts already say it, and
// changing text here made the block jump.
func (m *Model) stateLine() string {
	switch {
	case m.status != "":
		badge := badgeInfo.Render("INFO")
		if m.statusErr {
			badge = badgeFail.Render("FAIL")
		}
		return " " + badge + " " + m.status
	case m.building:
		return "       Building " + filepath.Base(m.cfg.Target) + "…"
	case m.loading > 0:
		return "       Listing tests…"
	}
	return ""
}

// Help.

type helpRow struct{ keys, desc string }

var helpRows = []helpRow{
	{"ctrl+j / ctrl+k", "to switch between the log and the tree (tab works too)"},
	{"j / k", "to move through the tree, or the lines of the log"},
	{"gg / G", "to jump to the top / bottom"},
	{"ctrl+d / ctrl+u", "to move half a page (ctrl+e / ctrl+y scroll the log without moving)"},
	{"l / h", "to expand / collapse a project, class or theory"},
	{"enter or r", "to run the selected project, class or test"},
	{"A", "to run every project"},
	{"F", "to rerun only the failed tests"},
	{"f / s", "to show only the failed / skipped tests"},
	{"a", "to show all tests again (esc does too)"},
	{"x", "to cancel the running tests"},
	{"n / N", "to jump to the next / previous failure"},
	{"o", "to open in Neovim: a stack frame under the log cursor, else a failed test's failing line, else the declaration"},
	{"t or /", "to filter the tree by name (enter keeps it, esc clears every filter)"},
	{"v", "to show the raw dotnet output of the selected project instead"},
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

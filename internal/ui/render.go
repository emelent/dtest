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
	footerH = 1 // the status and summary line along the bottom of the screen
)

// geometry is the current pane arrangement.
type geometry struct {
	logH  int // log pane body height (top, full width)
	treeH int // tree height, below the summary row
}

// layout splits the window: what is left over once the header, the two pane
// titles and the footer have their rows is split between the panes, the log
// taking about 70% of it. Both run the full width.
func (m *Model) layout() geometry {
	body := max(4, m.height-headerH-2*titleH-footerH)
	g := geometry{}
	g.logH = max(1, body*7/10)
	g.treeH = max(1, body-g.logH)
	m.log.SetWidth(max(1, m.width))
	m.log.SetHeight(g.logH)
	m.treeView.SetWidth(max(1, m.width))
	m.treeView.SetHeight(g.treeH)
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
	bottom := lipgloss.JoinVertical(lipgloss.Left, m.paneTitle(m.treeTitle(), m.focus == paneTree, m.width, ""), m.treeView.View())
	content := lipgloss.JoinVertical(lipgloss.Left, m.renderHeader(), top, bottom, m.renderFooter())
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "dtest " + m.name
	return v
}

// paneTitle is a ⎯⎯ Title ⎯⎯⎯ line with an optional note such as the cursor
// position near the right end. The rule itself is faint so it only frames
// the pane; the title carries the colour, bright when the pane is focused.
func (m *Model) paneTitle(title string, focused bool, width int, note string) string {
	style := styleDim
	if focused {
		style = styleKey
	}
	head := rule + rule + " "
	tail := rule + rule
	if note != "" {
		tail = " " + note + " " + rule + rule
	}
	fill := max(0, width-ansi.StringWidth(head)-ansi.StringWidth(title)-1-ansi.StringWidth(tail))
	line := styleRule.Render(head) + style.Render(title) + " " + styleRule.Render(strings.Repeat(rule, fill))
	if note != "" {
		return line + " " + styleDim.Render(note) + " " + styleRule.Render(rule+rule)
	}
	return line + styleRule.Render(tail)
}

// logPosition is the log cursor's line over the line count, "12/80".
func (m *Model) logPosition() string {
	if len(m.logLines) == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", m.logCursor+1, len(m.logLines))
}

// outputFor picks the raw dotnet output to show for a node: the run it came
// from, which is its own project's or, after a solution-wide run, the
// root's. A node no run has covered falls back to whatever ran last, and
// then to the build log.
func (m *Model) outputFor(n *tree.Node) (key, name string) {
	for c := n; c != nil; c = c.Parent {
		if c.Path != "" && m.logs[c.Path] != nil {
			return c.Path, c.Name
		}
	}
	if n != nil && m.logs[m.lastLog] != nil {
		if c := m.nodeByPath(m.lastLog); c != nil {
			return c.Path, c.Name
		}
	}
	return buildLogKey, "build"
}

// nodeByPath is the root or the project that `dotnet test` was pointed at.
func (m *Model) nodeByPath(path string) *tree.Node {
	if path == "" || path == buildLogKey {
		return nil
	}
	if m.tree.Root.Path == path {
		return m.tree.Root
	}
	for _, p := range m.tree.Projects {
		if p.Path == path {
			return p
		}
	}
	return nil
}

func (m *Model) logTitle() string {
	n := m.current()
	if m.showOutput {
		_, name := m.outputFor(n)
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
	guides := treeGuides(m.rows)
	for i, n := range m.rows {
		line := ansi.Truncate(m.renderNode(n, guides[i]), m.treeView.Width(), "…")
		if i == m.cursor {
			line = highlight(line, m.treeView.Width(), cursorBg(m.focus == paneTree))
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
		key, _ := m.outputFor(n)
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
		m.logCursor, m.selAnchor = 0, -1 // a different log, so the selection is void
	}
	m.logCursor = clamp(m.logCursor, len(lines))
	// The selected range is worked out up front, against the lines that are
	// about to be drawn: m.logLines is rebuilt below, so it cannot be asked
	// how long it is until that is done.
	selFrom, selTo := -1, -2
	if m.selAnchor >= 0 {
		m.selAnchor = clamp(m.selAnchor, len(lines))
		selFrom, selTo = min(m.selAnchor, m.logCursor), max(m.selAnchor, m.logCursor)
	}
	// Wrap each line at word boundaries, remembering where each starts;
	// every row of the cursor line, and of a selected one, is highlighted.
	m.logLines, m.logStarts = m.logLines[:0], m.logStarts[:0]
	var display []string
	width := max(1, m.log.Width())
	for i, l := range lines {
		m.logLines = append(m.logLines, ansi.Strip(l))
		m.logStarts = append(m.logStarts, len(display))
		bg := ""
		switch {
		case i >= selFrom && i <= selTo:
			bg = bgSelected
		case i == m.logCursor:
			bg = cursorBg(m.focus == paneLog)
		}
		for _, seg := range strings.Split(ansi.Wrap(l, width, ""), "\n") {
			if bg != "" {
				seg = highlight(seg, width, bg)
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
// for a test its result, message, stack trace and output; for a group every
// failure beneath it. The group's tally is not repeated here, since the row
// it is selected from carries it.
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
	status := n.Status()
	head := "  " + statusIcon(status) + " " + styleBold.Render(breadcrumb(n))
	// A time only goes up once its tests have all reported: until then it
	// would be a running total pretending to be a result.
	if d := n.Duration(); d > 0 && !inFlight(status) {
		head += " " + renderDuration(d)
	}
	lines = append(lines, head)
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
		// Nothing to report yet; the glyph in the header says it is going.
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
	head := "  " + statusIcon(n.Status()) + " " + styleBold.Render(breadcrumb(n))
	switch {
	case inFlight(n.Status()):
		return []string{head} // the glyph in the header says it all
	case r == nil:
		return []string{head, "", styleDim.Render("  Not run yet. Press enter to run it, o to open it in nvim.")}
	case n.Status() == tree.StatusFailed:
		return append([]string{head, ""}, m.renderFailure(n, true)...)
	}
	lines := []string{head + " " + renderDuration(r.Duration), ""}
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

// inFlight reports whether a node's tests are still running or waiting to,
// so nothing about them is settled yet.
func inFlight(s tree.Status) bool {
	return s == tree.StatusRunning || s == tree.StatusQueued
}

// renderFailure is one failed test: FAIL badge and breadcrumb, the message
// with its assertion coloured, the failing location and, in full, the
// stack trace and captured output.
func (m *Model) renderFailure(l *tree.Node, full bool) []string {
	r := l.Result
	lines := []string{"  " + badgeFail.Render("FAIL") + " " + styleBold.Render(breadcrumb(l)) + " " + renderDuration(r.Duration)}
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

// cursorBg is the cursor line's background, brighter in the focused pane.
func cursorBg(focused bool) string {
	if focused {
		return bgFocused
	}
	return bgUnfocused
}

// highlight paints bg across the full width of a line. Styled text resets
// the attributes after each coloured span, so the background is re-applied
// after every reset to keep the whole row lit while the colours stay.
func highlight(line string, width int, bg string) string {
	s := fit(line, width)
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+bg)
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[m"+bg)
	return bg + s + "\x1b[0m"
}

// treeGuides draws the branch lines down the left of the tree, one per row:
// a corner or a tee at the row's own level, and above it a bar for every
// level that carries on further down. Projects are roots, so they get none.
// It works off the visible rows, so a filtered tree still joins up.
func treeGuides(rows []*tree.Node) []string {
	depth := make([]int, len(rows))
	deepest := 0
	for i, n := range rows {
		depth[i] = n.Depth()
		deepest = max(deepest, depth[i])
	}
	// Backwards: a row is the last of its siblings unless a row at the same
	// depth was already seen without dropping shallower in between.
	last := make([]bool, len(rows))
	more := make([]bool, deepest+2)
	for i := len(rows) - 1; i >= 0; i-- {
		d := depth[i]
		last[i] = !more[d]
		more[d] = true
		for k := d + 1; k < len(more); k++ {
			more[k] = false
		}
	}
	// Forwards: every ancestor has been seen by the time its children are,
	// so carries[k] is whether this row's ancestor at depth k has siblings
	// still to come.
	out := make([]string, len(rows))
	carries := make([]bool, deepest+2)
	for i, d := range depth {
		carries[d] = !last[i]
		var b strings.Builder
		for k := 1; k < d; k++ {
			if carries[k] {
				b.WriteString(guideBar)
			} else {
				b.WriteString(guideGap)
			}
		}
		if d > 0 {
			if last[i] {
				b.WriteString(guideLast)
			} else {
				b.WriteString(guideBranch)
			}
		}
		out[i] = styleDim.Render(b.String())
	}
	return out
}

// renderNode draws one tree line: the branch guide, then the glyph, name
// and, for groups, the counts in parentheses, then the duration. The root
// is the exception: it names how many tests the solution has and stops
// there, since how the last run went is what the summary beside the tree is
// for.
func (m *Model) renderNode(n *tree.Node, guide string) string {
	status := n.Status()
	name := n.Name
	var tail []string
	switch {
	case n.Kind == tree.KindRoot:
		tail = append(tail, styleDim.Render("("+plural(len(n.Children), "project")+" | "+testCount(n.Counts().Total)+")"))
	case !n.IsLeaf():
		tail = append(tail, m.renderCounts(n.Counts()))
	}
	switch {
	case status == tree.StatusFailed && n.IsLeaf():
		name = inTree(styleFailed).Render(name)
	case status == tree.StatusSkipped && n.IsLeaf():
		name = styleDim.Render(name) + " " + inTree(styleSkipped).Render("[skipped]")
	case status == tree.StatusQueued:
		// Everything waiting for its run is greyed out, name included; the
		// root and the projects keep their weight so the tree still has
		// headings.
		name = styleQueued.Bold(isHeading(n)).Render(name)
	case isHeading(n):
		name = styleBold.Render(name)
	}
	if d := n.Duration(); n.Kind != tree.KindRoot && (d > 0 || (n.IsLeaf() && n.Result != nil && status != tree.StatusSkipped)) {
		tail = append(tail, treeDuration(d))
	}
	line := "  " + guide + m.treeIcon(n) + " " + name
	if len(tail) > 0 {
		line += " " + strings.Join(tail, " ")
	}
	return line
}

// isHeading reports whether a node is one of the tree's headings: the root
// or a project, which are drawn in bold rather than in a status colour.
func isHeading(n *tree.Node) bool {
	return n.Kind == tree.KindRoot || n.Kind == tree.KindProject
}

// testCount is "4 tests", or "1 test".
func testCount(n int) string { return plural(n, "test") }

// plural counts things: "2 projects", "1 project".
func plural(n int, thing string) string {
	if n == 1 {
		return "1 " + thing
	}
	return fmt.Sprintf("%d %ss", n, thing)
}

// renderCounts is vitest's "(4 tests | 1 failed | 1 skipped)". How many
// there are is grey, being a fact about the tree rather than a result, and
// the outcomes are a shade back from the footer's, which is the line meant
// to be read at a glance.
func (m *Model) renderCounts(c tree.Counts) string {
	parts := []string{styleDim.Render(testCount(c.Total))}
	if c.Running > 0 {
		parts = append(parts, inTree(styleRunning).Render(fmt.Sprintf("%d running", c.Running)))
	}
	if c.Queued > 0 {
		parts = append(parts, styleQueued.Render(fmt.Sprintf("%d queued", c.Queued)))
	}
	if c.Failed > 0 {
		parts = append(parts, inTree(styleFailed).Render(fmt.Sprintf("%d failed", c.Failed)))
	}
	if c.Skipped > 0 {
		parts = append(parts, inTree(styleSkipped).Render(fmt.Sprintf("%d skipped", c.Skipped)))
	}
	sep := styleDim.Render(" | ")
	return styleDim.Render("(") + strings.Join(parts, sep) + styleDim.Render(")")
}

// treeIcon is a node's glyph in the tree: a fold arrow for the root, a
// project, a class or a theory, a status glyph for a test, each in the
// colour of its status. While tests run only the running project spins;
// everything running beneath it, and the root above it, is simply drawn in
// the running colour, so the tree does not flicker all over.
func (m *Model) treeIcon(n *tree.Node) string {
	status := n.Status()
	if status == tree.StatusRunning {
		if n.Kind == tree.KindProject {
			return m.spin.View()
		}
		if n.IsLeaf() {
			return inTree(styleRunning).Render(iconNone)
		}
		return inTree(styleRunning).Render(m.arrow(n))
	}
	if n.IsLeaf() {
		switch status {
		case tree.StatusQueued:
			return styleQueued.Render(iconQueued)
		case tree.StatusPassed:
			return inTree(stylePassed).Render(iconPassed)
		case tree.StatusFailed:
			return inTree(styleFailed).Render(iconFailed)
		case tree.StatusSkipped:
			return inTree(styleSkipped).Render(iconSkipped)
		}
		return styleDim.Render(iconNone)
	}
	switch status {
	case tree.StatusQueued:
		return styleQueued.Render(m.arrow(n))
	case tree.StatusPassed:
		return inTree(stylePassed).Render(m.arrow(n))
	case tree.StatusFailed:
		return inTree(styleFailed).Render(m.arrow(n))
	case tree.StatusSkipped:
		return inTree(styleSkipped).Render(m.arrow(n))
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

// statusIcon is a node's glyph outside the tree. It never spins: the log
// header sits still while its lines are read, and the tree is where a run
// shows its progress.
func statusIcon(s tree.Status) string {
	switch s {
	case tree.StatusRunning:
		return styleRunning.Render(iconNone)
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

// breadcrumb is "Project › Class › Method(args)" for a failure header. The
// root is left out: it is above every test, so naming it says nothing.
func breadcrumb(l *tree.Node) string {
	var parts []string
	for n := l; n != nil && n.Kind != tree.KindRoot; n = n.Parent {
		parts = append([]string{n.Name}, parts...)
	}
	if len(parts) == 0 {
		return l.Name // the root itself
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

// renderFooter is the last line of the screen: what dtest is doing or has
// to say and, the rest of the time, how the last batch of runs went, with
// the key hint at the right edge.
func (m *Model) renderFooter() string {
	left := m.stateLine()
	if left == "" {
		left = m.summary()
	}
	return fitLine(" "+left, styleDim.Render("press ? for help"), m.width)
}

// summary is how the tests of the last batch of runs are going: what has
// been run of it so far and how that went. It keeps one shape from the
// first result to the last, so the line settles rather than changing form
// when the batch ends. What the solution holds is not here; the root of the
// tree carries it.
func (m *Model) summary() string {
	if m.batchStart.IsZero() {
		return styleDim.Render("nothing run yet")
	}
	ran := m.batchCounts()
	done := ran.Failed + ran.Passed + ran.Skipped
	// The clock time is a footnote, so it stays grey; how long the tests
	// took is worth a glance, so it gets a quiet cyan.
	head := styleDim.Render("Ran "+testCount(done)+" in ") +
		styleElapsed.Render(formatDuration(m.batchDuration())) +
		styleDim.Render(" at "+m.batchStart.Format("15:04:05"))
	return head + styleDim.Render("  ·  ") + strings.Join(outcomeParts(ran), styleDim.Render(" | "))
}

// outcomeParts is the three outcomes of a tally, each bold in its colour,
// or grey while it is still zero, so the line keeps its shape as a run
// fills it in.
func outcomeParts(c tree.Counts) []string {
	part := func(n int, label string, style lipgloss.Style) string {
		if n == 0 {
			style = styleDim
		}
		return style.Bold(n > 0).Render(fmt.Sprintf("%d %s", n, label))
	}
	return []string{
		part(c.Failed, "failed", styleFailed),
		part(c.Passed, "passed", stylePassed),
		part(c.Skipped, "skipped", styleSkipped),
	}
}

// stateLine is what dtest is doing: a status message, or building or
// listing as plain text. It is empty otherwise, before, during and after
// runs, since the tree's spinner and the summary row already say it.
func (m *Model) stateLine() string {
	switch {
	case m.status != "":
		badge := badgeInfo.Render("INFO")
		if m.statusErr {
			badge = badgeFail.Render("FAIL")
		}
		return badge + " " + m.status
	case m.building:
		return "Building " + filepath.Base(m.cfg.Target) + "…"
	case m.loading > 0:
		return "Listing tests…"
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
	{"V then y", "to select lines of the log and copy them; the mouse selects too"},
	{"y", "to copy the log line under the cursor"},
	{"l / h", "to expand / collapse a project, class or theory"},
	{"L / H", "to expand / collapse the whole tree"},
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

// slowDuration is vitest's slowTestThreshold: anything that took longer is
// worth a second look, so its time is drawn in the warning colour.
const slowDuration = 300 * time.Millisecond

// inTree is a style as the tree wears it: the same colour a shade back, so
// the tree stays the quiet half of the screen and the footer's colours,
// which are the ones meant to be read at a glance, carry.
func inTree(s lipgloss.Style) lipgloss.Style { return s.Faint(true) }

// renderDuration colours a time as vitest does: green while it is quick,
// yellow once it passes the slow threshold, with the unit a faded shade of
// that same colour so the number reads first.
func renderDuration(d time.Duration) string { return duration(d, false) }

// treeDuration is the same, a shade back for the tree.
func treeDuration(d time.Duration) string { return duration(d, true) }

func duration(d time.Duration, faint bool) string {
	style := styleQuick
	if d > slowDuration {
		style = styleSlow
	}
	if faint {
		style = inTree(style)
	}
	s := formatDuration(d)
	// The plain forms (0.032s, 1.5s, 35s) end in their unit, which is faded;
	// the compound ones (2m34s, 1h02m) carry a unit inside the number, so
	// they are left in one shade.
	if i := strings.IndexFunc(s, isLetter); i > 0 && strings.IndexFunc(s[i:], func(r rune) bool { return !isLetter(r) }) < 0 {
		return style.Render(s[:i]) + style.Faint(true).Render(s[i:])
	}
	return style.Render(s)
}

func isLetter(r rune) bool { return r >= 'a' && r <= 'z' }

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

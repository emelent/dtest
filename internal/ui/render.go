package ui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"dtest/internal/tree"
)

const (
	twoPaneMin = 80 // narrower terminals show one pane at a time
	headerH    = 1
	titleH     = 1
	statusH    = 1
)

// bodyHeight is the number of rows available to the panes.
func (m *Model) bodyHeight() int {
	return max(1, m.height-headerH-titleH-statusH)
}

// paneWidths returns the tree and log widths; the log width is 0 when only
// one pane fits.
func (m *Model) paneWidths() (treeW, logW int) {
	if m.width < twoPaneMin {
		if m.focus == paneLog {
			return 0, m.width
		}
		return m.width, 0
	}
	treeW = min(max(m.width*2/5, 34), 70)
	return treeW, m.width - treeW - 1
}

// layout resizes the log viewport to the current window.
func (m *Model) layout() {
	_, logW := m.paneWidths()
	m.log.SetWidth(max(1, logW))
	m.log.SetHeight(m.bodyHeight())
}

// View draws the whole screen.
func (m *Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("")
	}
	var body string
	if m.help {
		body = m.renderHelp()
	} else {
		body = m.renderPanes()
	}
	content := lipgloss.JoinVertical(lipgloss.Left, m.renderHeader(), body, m.renderStatus())
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "dtest " + m.name
	return v
}

func (m *Model) renderHeader() string {
	c := m.tree.Counts()
	left := fmt.Sprintf(" dtest  %s   %s %d  %s %d  %s %d   %d tests",
		m.name,
		stylePassed.Render(iconPassed), c.Passed,
		styleFailed.Render(iconFailed), c.Failed,
		styleSkipped.Render(iconSkipped), c.Skipped,
		c.Total)
	right := ""
	switch {
	case m.building:
		right = m.spin.View() + " building"
	case m.loading > 0:
		right = m.spin.View() + " listing tests"
	case m.run != nil:
		right = m.spin.View() + " running " + m.run.req.label
		if len(m.queue) > 0 {
			right += fmt.Sprintf("  (+%d queued)", len(m.queue))
		}
	}
	// Long run labels must not push the counts off the line.
	right = ansi.Truncate(right, max(10, m.width/2), "…")
	return styleHeader.Render(fitLine(left, right, m.width))
}

func (m *Model) renderStatus() string {
	if m.status != "" {
		if m.statusErr {
			return styleError.Render(fit(" "+m.status, m.width))
		}
		return styleBar.Render(fit(" "+m.status, m.width))
	}
	if m.searching {
		return styleBar.Render(fit(" /"+m.query+"▏", m.width))
	}
	hints := []string{"j/k move", "h/l fold", "r run", "R all", "e rerun failed", "m mark", "o nvim", "f/s fail/skip", "/ find", "v full log", "tab log", "? help", "q quit"}
	if m.focus == paneLog {
		hints = []string{"j/k scroll", "g/G top/bottom", "ctrl+d/u page", "v full log", "h back", "r run", "x cancel", "? help", "q quit"}
	}
	var b strings.Builder
	for _, h := range hints {
		k, rest, _ := strings.Cut(h, " ")
		b.WriteString(" " + styleKey.Render(k) + " " + rest + " ")
	}
	return styleBar.Render(fit(b.String(), m.width))
}

// renderPanes draws the tree and the log side by side, or the focused one
// alone on a narrow terminal.
func (m *Model) renderPanes() string {
	treeW, logW := m.paneWidths()
	h := m.bodyHeight()
	var cols []string
	if treeW > 0 {
		title := paneTitle(" Tests ", m.focus == paneTree, treeW)
		cols = append(cols, lipgloss.JoinVertical(lipgloss.Left, title, m.renderTree(treeW, h)))
	}
	if logW > 0 {
		title := paneTitle(" "+m.logTitle()+" ", m.focus == paneLog, logW)
		log := lipgloss.NewStyle().Width(logW).Height(h).MaxHeight(h).Render(m.log.View())
		cols = append(cols, lipgloss.JoinVertical(lipgloss.Left, title, log))
	}
	if len(cols) == 2 {
		sep := strings.TrimRight(strings.Repeat(styleDim.Render("│")+"\n", h+titleH), "\n")
		return lipgloss.JoinHorizontal(lipgloss.Top, cols[0], sep, cols[1])
	}
	return cols[0]
}

func paneTitle(text string, focused bool, width int) string {
	if focused {
		return fit(styleFocus.Render(text), width)
	}
	return fit(styleTitle.Render(text), width)
}

// renderTree draws the visible rows around the cursor.
func (m *Model) renderTree(width, height int) string {
	if len(m.rows) == 0 {
		msg := "  No tests found"
		switch {
		case m.building || m.loading > 0:
			msg = "  " + m.spin.View() + " Loading…"
		case m.query != "":
			msg = "  No tests match /" + m.query
		}
		return lipgloss.NewStyle().Width(width).Height(height).Render(styleDim.Render(msg))
	}
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+height {
		m.top = m.cursor - height + 1
	}
	if m.top > len(m.rows)-1 {
		m.top = max(0, len(m.rows)-1)
	}
	lines := make([]string, 0, height)
	for i := m.top; i < len(m.rows) && len(lines) < height; i++ {
		lines = append(lines, m.renderRow(m.rows[i], i == m.cursor, width))
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

// renderRow draws one node: mark, indent, fold arrow, status icon, name and,
// for interior nodes, a count.
func (m *Model) renderRow(n *tree.Node, selected bool, width int) string {
	mark := " "
	if n.Marked {
		mark = styleMark.Render(iconMark)
	}
	indent := strings.Repeat("  ", n.Depth())
	arrow := "  "
	if !n.IsLeaf() {
		arrow = "▸ "
		if n.Expanded || m.query != "" {
			arrow = "▾ "
		}
	}
	status := n.Status()
	name := n.Name
	if n.Kind == tree.KindNamespace && name == "" {
		name = "(global namespace)"
	}
	if status == tree.StatusFailed && n.IsLeaf() {
		name = styleFailed.Render(name)
	}
	tail := ""
	if d := n.Duration(); d > 0 || (n.IsLeaf() && n.Result != nil) {
		tail = styleDim.Render(formatDuration(d))
	}
	head := mark + indent + arrow + m.statusIcon(status) + " "
	nameW := width - ansi.StringWidth(head) - ansi.StringWidth(tail) - 2
	if nameW < 5 {
		tail = ""
		nameW = max(1, width-ansi.StringWidth(head)-1)
	}
	line := head + ansi.Truncate(name, nameW, "…")
	line = fitLine(line, tail+" ", width)
	if selected {
		return styleCursor.Render(line)
	}
	return line
}

func (m *Model) statusIcon(s tree.Status) string {
	switch s {
	case tree.StatusRunning:
		return m.spin.View()
	case tree.StatusPassed:
		return stylePassed.Render(iconPassed)
	case tree.StatusFailed:
		return styleFailed.Render(iconFailed)
	case tree.StatusSkipped:
		return styleSkipped.Render(iconSkipped)
	}
	return styleDim.Render(iconNone)
}

// Log pane.

// logTitle names what the log pane shows.
func (m *Model) logTitle() string {
	n := m.current()
	mode := " (minimal)"
	if m.fullLog {
		mode = " (full)"
	}
	switch {
	case n != nil && n.IsLeaf():
		return "Test: " + n.Name
	case n != nil && m.logs[n.Project().Path] != nil:
		return "Log: " + n.Project().Name + mode
	}
	return "Log: build" + mode
}

// refreshLog points the viewport at the content for the cursor: a test's
// detail for a leaf, otherwise the latest run log of its project (or the
// build log). A log that is being appended to keeps following its tail.
func (m *Model) refreshLog() {
	key, lines := m.logContent()
	sig := contentSignature(key, lines)
	if key == m.logKey && sig == m.logSig {
		return
	}
	follow := m.log.AtBottom() || m.log.PastBottom()
	changed := key != m.logKey
	m.logKey, m.logSig = key, sig
	m.log.SetContentLines(lines)
	switch {
	case changed && strings.HasPrefix(key, "test:"):
		m.log.GotoTop()
	case changed || follow:
		m.log.GotoBottom()
	}
}

// contentSignature cheaply identifies content so the viewport is only reset
// when it changes: a full run log is append-only, so its length is enough;
// a test detail or a minimal log is short and may change without growing
// (the totals line updates in place as summary lines stream in).
func contentSignature(key string, lines []string) string {
	if strings.HasPrefix(key, "log:full:") {
		return fmt.Sprint(len(lines))
	}
	return strings.Join(lines, "\n")
}

// logContent picks the lines to show and a key identifying them.
func (m *Model) logContent() (string, []string) {
	n := m.current()
	if n != nil && n.IsLeaf() {
		return "test:" + n.Project().Path + "/" + n.FQN, m.renderDetail(n)
	}
	mode := "minimal:"
	if m.fullLog {
		mode = "full:"
	}
	if n != nil {
		if lines, ok := m.logs[n.Project().Path]; ok {
			return "log:" + mode + n.Project().Path, m.presentLog(lines)
		}
	}
	return "log:" + mode + buildLogKey, m.presentLog(m.logs[buildLogKey])
}

// presentLog prepares raw output for the pane: reduced to failures and
// totals unless the full log is on, then coloured.
func (m *Model) presentLog(lines []string) []string {
	if !m.fullLog {
		lines = resultLines(lines)
	}
	return colorLog(lines)
}

// resultLines reduces dotnet output to the minimum: for each failed test
// its name and the failing assertion or exception (the message, without the
// stack trace), one summary line per run such as "33 tests: 30 passed,
// 2 failed, 1 skipped", and build errors so a broken build is not silent.
func resultLines(lines []string) []string {
	var out []string
	inFailure := false
	var summary *runSummary
	flush := func() {
		if summary != nil {
			out = append(out, summary.String())
			summary = nil
		}
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if summary != nil && !summary.take(trimmed) {
			flush()
		}
		switch {
		case inFailure && (trimmed == "" || strings.HasPrefix(trimmed, "Stack Trace:")):
			inFailure = false
		case inFailure:
			if !strings.HasPrefix(trimmed, "Error Message:") {
				out = append(out, line)
			}
		case strings.HasPrefix(trimmed, "Failed "):
			inFailure = true
			out = append(out, line)
		case strings.HasPrefix(trimmed, "Total tests:"):
			summary = &runSummary{}
			summary.take(trimmed)
		case strings.Contains(line, ": error "), strings.Contains(line, "Build FAILED"), strings.HasPrefix(trimmed, "dotnet test failed"):
			out = append(out, line)
		}
	}
	flush()
	return out
}

// runSummary gathers the console logger's summary counters into one line.
type runSummary struct{ total, passed, failed, skipped string }

// take records a summary line, reporting whether it was one.
func (s *runSummary) take(trimmed string) bool {
	for prefix, dst := range map[string]*string{"Total tests:": &s.total, "Passed:": &s.passed, "Failed:": &s.failed, "Skipped:": &s.skipped} {
		if strings.HasPrefix(trimmed, prefix) {
			*dst = strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
			return true
		}
	}
	return false
}

func (s *runSummary) String() string {
	or := func(v string) string {
		if v == "" {
			return "0"
		}
		return v
	}
	return fmt.Sprintf("%s tests: %s passed, %s failed, %s skipped", or(s.total), or(s.passed), or(s.failed), or(s.skipped))
}

// summaryLine matches the line runSummary produces.
var summaryLine = regexp.MustCompile(`^\d+ tests: \d+ passed, (\d+) failed, \d+ skipped$`)

// Log line classes, matched in order; the first match styles the line.
var logRules = []struct {
	match func(string) bool
	style lipgloss.Style
}{
	{func(s string) bool { return strings.HasPrefix(s, "$ ") }, styleCommand},
	{func(s string) bool { return strings.HasPrefix(strings.TrimLeft(s, " "), "Passed ") }, stylePassed},
	{func(s string) bool { return strings.HasPrefix(strings.TrimLeft(s, " "), "Failed ") }, styleFailed},
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
	if m := summaryLine.FindStringSubmatch(line); m != nil {
		if m[1] == "0" {
			return stylePassed.Render(line)
		}
		return styleFailed.Render(line)
	}
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

// renderDetail describes one test's latest result.
func (m *Model) renderDetail(n *tree.Node) []string {
	lines := []string{styleTitle.Render(n.FQN), ""}
	switch n.Status() {
	case tree.StatusRunning:
		return append(lines, styleRunning.Render("Running…"))
	case tree.StatusNone:
		if n.Result == nil {
			return append(lines, styleDim.Render("Not run yet. Press r to run it, o to open it in nvim."))
		}
	}
	r := n.Result
	if r == nil {
		return lines
	}
	verdict := fmt.Sprintf("%s %s in %s", m.statusIcon(n.Status()), r.Outcome, formatDuration(r.Duration))
	lines = append(lines, verdict)
	if r.Message != "" {
		lines = append(lines, "", styleFailed.Render("Message"))
		for _, l := range strings.Split(r.Message, "\n") {
			lines = append(lines, colorMessage(l))
		}
	}
	if r.StackTrace != "" {
		lines = append(lines, "", styleTitle.Render("Stack trace"))
		for _, l := range strings.Split(r.StackTrace, "\n") {
			lines = append(lines, styleDim.Render(l))
		}
	}
	if r.Output != "" {
		lines = append(lines, "", styleTitle.Render("Output"))
		lines = append(lines, strings.Split(r.Output, "\n")...)
	}
	return lines
}

// Help.

type helpRow struct{ keys, desc string }

var helpRows = []helpRow{
	{"j / k, ↓ / ↑", "move the cursor"},
	{"gg / G", "first / last row"},
	{"ctrl+d / ctrl+u", "half page down / up"},
	{"ctrl+f / ctrl+b", "page down / up"},
	{"h / l", "collapse or go to parent / expand or enter"},
	{"enter", "toggle a folder; run a test"},
	{"H / L", "collapse / expand everything"},
	{"m", "mark the node for a run (● in the margin)"},
	{"u, esc", "clear marks (esc first clears the / filter)"},
	{"r", "run the marked nodes, or the node under the cursor"},
	{"R", "run every project"},
	{"e", "re-run every test that failed"},
	{"x", "cancel the running tests and drop the queue"},
	{"f / F", "next / previous failed test"},
	{"s / S", "next / previous skipped test"},
	{"/", "filter the tree by name; enter keeps it, esc clears it"},
	{"o", "open the test's source in Neovim (see below)"},
	{"v", "log pane: minimal (default: failures and totals) or the full dotnet output"},
	{"tab", "focus the log pane; h, esc or tab come back"},
	{"ctrl+r", "rebuild and list tests again"},
	{"?", "this help"},
	{"q, ctrl+c", "quit"},
}

func (m *Model) renderHelp() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(" Keys") + "\n\n")
	for _, r := range helpRows {
		fmt.Fprintf(&b, "  %s  %s\n", styleKey.Render(fmt.Sprintf("%-16s", r.keys)), r.desc)
	}
	b.WriteString("\n" + styleTitle.Render(" Neovim") + "\n\n")
	if m.socket == "" {
		b.WriteString("  $nvim_sock is not set, so o opens nvim in this terminal and dtest resumes\n  when it exits. Set it (or pass --nvim-socket) to the socket of a Neovim\n  started with nvim --listen <socket> to send files there instead.\n")
	} else {
		fmt.Fprintf(&b, "  o sends the file and line to the Neovim listening on\n    %s\n", m.socket)
		b.WriteString("  (from $nvim_sock or --nvim-socket). Inside tmux the window named \"code\" is\n  selected too. With nothing listening, nvim opens in this terminal instead.\n")
	}
	b.WriteString("\n" + styleDim.Render("  Any key closes this help."))
	return lipgloss.NewStyle().Width(m.width).Height(m.bodyHeight()).MaxHeight(m.bodyHeight()).Render(b.String())
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

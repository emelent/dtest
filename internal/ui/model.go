// Package ui is the bubbletea front end, laid out in two panes in the
// style of vitest: the log of the selected tests on top, and below it the
// test tree with the run statistics along its right-hand side.
package ui

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"dtest/internal/dotnet"
	"dtest/internal/editor"
	"dtest/internal/tree"
)

// Externals, as package variables so tests can stub them.
var (
	findProjects = dotnet.Projects
	listTests    = dotnet.ListTests
	buildAll     = dotnet.Build
	runTests     = dotnet.Run
	locate       = dotnet.Locate
	readExcerpt  = dotnet.ReadExcerpt
	hasServer    = editor.HasServer
	openInServer = editor.Open
	lookPath     = exec.LookPath
	now          = time.Now
)

// Config is what main passes in.
type Config struct {
	Version string         // shown in the header; "dev" for local builds
	Target  string         // solution or project file
	Options dotnet.Options // configuration and --no-build
	Socket  string         // explicit Neovim server socket; empty to use $nvim_sock
}

const (
	statusTimeout = 5 * time.Second
	buildLogKey   = "build"
)

// runRequest is one queued `dotnet test` invocation. The target is what the
// command is pointed at: a project, or the root, which is the solution.
type runRequest struct {
	target *tree.Node
	filter string
	label  string       // shown while running
	leaves []*tree.Node // marked running until results arrive
}

// activeRun is the invocation in progress.
type activeRun struct {
	req    runRequest
	cancel context.CancelFunc
	events chan tea.Msg
}

// pane is one of the two regions of the screen.
type pane int

const (
	paneLog  pane = iota // top: the selected node's results, or the raw output
	paneTree             // bottom: the test tree, with the summary beside it
)

// Model is the root bubbletea model.
type Model struct {
	cfg    Config
	name   string // solution or project name without extension
	socket string

	tree   *tree.Tree
	rows   []*tree.Node // the tree pane's rows
	cursor int
	zoom   *tree.Node // the subtree the view is focused on; nil for all of it

	width, height int
	focus         pane
	log           viewport.Model
	logSig        string
	logNode       *tree.Node // whose results the log shows
	logLines      []string   // the log's lines, unstyled, for the cursor and o
	logCursor     int        // the log's cursor line
	logStarts     []int      // first display row of each log line, once wrapped
	selAnchor     int        // where a log selection started; -1 when there is none
	dragging      bool       // the mouse button is down over the log
	treeView      viewport.Model
	treeSig       string
	spin          spinner.Model

	logs     map[string][]string // raw dotnet output by project path, plus buildLogKey
	lastLog  string              // key of the most recent output
	queue    []runRequest
	run      *activeRun
	loading  int // projects whose test list is outstanding
	building bool
	events   chan tea.Msg // build events

	batchStart time.Time           // when the current sequence of runs began
	batchEnd   time.Time           // when it finished; zero while it is going
	batch      map[*tree.Node]bool // the leaves of that sequence, for the summary
	runsDone   bool                // at least one run has finished

	status    string
	statusErr bool
	statusID  int

	help         bool
	showOutput   bool // append the raw dotnet output to the report
	filtering    bool
	query        string
	statusFilter tree.Status // show only leaves in this status; StatusNone for all
	pendingG     bool
	fatal        error

	locations map[string]dotnet.Location
	excerpts  map[string]cachedExcerpt // source around a failure, by "file:line"
}

// cachedExcerpt remembers one lookup, a miss included: a file that is not
// there is worth not reopening on every redraw either.
type cachedExcerpt struct {
	ex dotnet.Excerpt
	ok bool
}

// New returns a model for the solution or project in cfg.Target.
func New(cfg Config) *Model {
	name := strings.TrimSuffix(filepath.Base(cfg.Target), filepath.Ext(cfg.Target))
	m := &Model{
		cfg:       cfg,
		name:      name,
		socket:    editor.ResolveSocket(cfg.Socket),
		tree:      tree.New(name, cfg.Target),
		logs:      map[string][]string{},
		locations: map[string]dotnet.Location{},
		spin:      spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(styleRunning)),
		log:       viewport.New(),
		treeView:  viewport.New(),
		focus:     paneTree,
		selAnchor: -1,
	}
	m.log.MouseWheelEnabled = true
	m.treeView.MouseWheelEnabled = true
	return m
}

// Fatal is the error that made the program stop, if any.
func (m *Model) Fatal() error { return m.fatal }

// Messages.
type (
	projectsMsg struct {
		paths []string
		err   error
	}
	buildLineMsg struct{ text string }
	buildDoneMsg struct{ err error }
	listMsg      struct {
		project *tree.Node
		names   []string
		err     error
	}
	runEventMsg struct {
		target *tree.Node
		event  dotnet.Event
	}
	statusClearMsg struct{ id int }
	editorDoneMsg  struct{ err error }
)

// Init starts the spinner and the discovery of test projects.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, m.loadProjects())
}

func (m *Model) loadProjects() tea.Cmd {
	target := m.cfg.Target
	return func() tea.Msg {
		paths, err := findProjects(target)
		return projectsMsg{paths: paths, err: err}
	}
}

// waitMsg delivers the next message from ch.
func waitMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

// startBuild builds the whole target once, streaming output to the build
// log, so the per-project listings that follow can use --no-build.
func (m *Model) startBuild() tea.Cmd {
	m.building = true
	m.logs[buildLogKey] = nil
	m.lastLog = buildLogKey
	ch := make(chan tea.Msg, 256)
	m.events = ch
	target, opts := m.cfg.Target, m.cfg.Options
	go func() {
		err := buildAll(context.Background(), target, opts, func(e dotnet.LineEvent) { ch <- buildLineMsg{text: e.Text} })
		ch <- buildDoneMsg{err: err}
	}()
	return waitMsg(ch)
}

// listWorkers caps how many `dotnet test --list-tests` run at once; each is
// a full dotnet process, and a solution can have dozens of test projects.
const listWorkers = 4

// startListing lists the tests of every project, a few at a time.
func (m *Model) startListing() tea.Cmd {
	opts := m.cfg.Options
	opts.NoBuild = true // just built, or the user asked for no builds
	sem := make(chan struct{}, listWorkers)
	var cmds []tea.Cmd
	for _, p := range m.tree.Projects {
		p := p
		m.loading++
		cmds = append(cmds, func() tea.Msg {
			sem <- struct{}{}
			defer func() { <-sem }()
			names, err := listTests(context.Background(), p.Path, opts)
			return listMsg{project: p, names: names, err: err}
		})
	}
	return tea.Batch(cmds...)
}

// busy reports whether dotnet is doing something on our behalf.
func (m *Model) busy() bool {
	return m.building || m.loading > 0 || m.run != nil
}

// reload rebuilds (unless --no-build) and lists the tests again.
func (m *Model) reload() tea.Cmd {
	if m.building || m.loading > 0 {
		return m.setStatus("Already loading", true)
	}
	m.zoom = nil // listing replaces the nodes it was pointing at
	if m.cfg.Options.NoBuild {
		return m.startListing()
	}
	return m.startBuild()
}

// Update handles every message.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		m.refresh()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		// While dotnet is busy the tick drives the redraw: spinners and the
		// duration advance, and output lines that arrived since the last
		// tick are drawn in one go instead of one rebuild per line.
		if m.busy() {
			m.refresh()
		}
		return m, cmd

	case projectsMsg:
		if msg.err != nil {
			m.fatal = msg.err
			return m, tea.Quit
		}
		for _, p := range msg.paths {
			m.tree.AddProject(p)
		}
		m.refresh()
		if m.cfg.Options.NoBuild {
			return m, m.startListing()
		}
		return m, m.startBuild()

	case buildLineMsg:
		m.logs[buildLogKey] = append(m.logs[buildLogKey], msg.text)
		return m, waitMsg(m.events)

	case buildDoneMsg:
		m.building = false
		var cmd tea.Cmd
		if msg.err != nil {
			cmd = m.setStatus(msg.err.Error()+" (press v for the output)", true)
		}
		return m, tea.Batch(cmd, m.startListing())

	case listMsg:
		m.loading--
		var cmd tea.Cmd
		if msg.err != nil {
			m.logs[msg.project.Path] = append(m.logs[msg.project.Path], msg.err.Error())
			cmd = m.setStatus(msg.project.Name+": "+msg.err.Error(), true)
		} else {
			m.tree.SetTests(msg.project, msg.names)
		}
		m.refresh()
		return m, cmd

	case runEventMsg:
		return m, m.handleRunEvent(msg)

	case statusClearMsg:
		if msg.id == m.statusID {
			m.status = ""
		}
		return m, nil

	case editorDoneMsg:
		if msg.err != nil {
			return m, m.setStatus("nvim: "+msg.err.Error(), true)
		}
		return m, nil

	case tea.MouseWheelMsg:
		// The wheel scrolls whatever is under the pointer, focused or not,
		// so a glance at the other pane costs nothing.
		var cmd tea.Cmd
		if m.paneAt(msg.Y) == paneLog {
			m.log, cmd = m.log.Update(msg)
		} else {
			m.treeView, cmd = m.treeView.Update(msg)
		}
		return m, cmd

	case tea.MouseClickMsg:
		return m, m.mouseDown(msg.Y)

	case tea.MouseMotionMsg:
		return m, m.dragSelect(msg.Y)

	case tea.MouseReleaseMsg:
		return m, m.endSelect()

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// setStatus shows text in the status line for a while.
func (m *Model) setStatus(text string, isErr bool) tea.Cmd {
	m.status, m.statusErr = text, isErr
	m.statusID++
	id := m.statusID
	return tea.Tick(statusTimeout, func(time.Time) tea.Msg { return statusClearMsg{id: id} })
}

// Running.

// enqueue queues a run for each project that nodes belong to and starts the
// first when nothing else is running.
func (m *Model) enqueue(nodes []*tree.Node) tea.Cmd {
	// Nothing running means this starts a new batch, so the summary above
	// the tree describes these runs and not the last ones.
	if m.run == nil {
		m.batchStart, m.batchEnd, m.batch = now(), time.Time{}, nil
	}
	// The root is the solution, so running it is one `dotnet test` over the
	// whole thing rather than one invocation per project.
	for _, n := range nodes {
		if n.Kind == tree.KindRoot {
			m.queueRun(runRequest{target: n, leaves: n.Leaves(), label: n.Name + " › all"})
			return m.pump()
		}
	}
	byProject := map[*tree.Node][]*tree.Node{}
	var order []*tree.Node
	for _, n := range nodes {
		p := n.Project()
		if _, ok := byProject[p]; !ok {
			order = append(order, p)
		}
		byProject[p] = append(byProject[p], n)
	}
	for _, p := range order {
		req := runRequest{target: p}
		var filters, labels []string
		for _, n := range byProject[p] {
			req.leaves = append(req.leaves, n.Leaves()...)
			if n.Kind == tree.KindProject {
				filters, labels = nil, []string{"all"}
				req.leaves = p.Leaves()
				break
			}
			filters = append(filters, n.Filter())
			labels = append(labels, n.Name)
		}
		req.filter = dotnet.JoinFilters(filters)
		req.label = p.Name + " › " + strings.Join(labels, ", ")
		m.queueRun(req)
	}
	return m.pump()
}

// queueRun adds a request to the queue and marks its tests as waiting. Ones
// already running, from another request for the same tests, keep spinning.
func (m *Model) queueRun(req runRequest) {
	for _, l := range req.leaves {
		if l.Status() != tree.StatusRunning {
			l.SetStatus(tree.StatusQueued)
		}
	}
	m.addToBatch(req.leaves...)
	m.queue = append(m.queue, req)
}

// addToBatch records leaves as part of the batch in progress. A project and
// one of its classes can both be queued in the same batch, and a test that
// was never listed shows up only when its result arrives, so the batch is a
// set that either can add to.
func (m *Model) addToBatch(leaves ...*tree.Node) {
	if m.batch == nil {
		m.batch = map[*tree.Node]bool{}
	}
	for _, l := range leaves {
		m.batch[l] = true
	}
}

// batchDuration is how long the batch has been going, and how long it took
// once it is over. It is wall-clock time rather than the sum of the tests'
// own: a sum only moves when a result lands, so it sits still through every
// slow test, and a timer that stops for seconds at a time reads as a stuck
// program rather than a slow one.
func (m *Model) batchDuration() time.Duration {
	if m.batchStart.IsZero() {
		return 0
	}
	end := m.batchEnd
	if end.IsZero() {
		end = now()
	}
	return end.Sub(m.batchStart)
}

// batchCounts tallies the batch by the state its tests are in now. A test
// whose run was cancelled or dropped has no state and falls out, so the
// rows always add up to the total above them.
func (m *Model) batchCounts() tree.Counts {
	var c tree.Counts
	for l := range m.batch {
		switch l.Status() {
		case tree.StatusRunning:
			c.Running++
		case tree.StatusQueued:
			c.Queued++
		case tree.StatusPassed:
			c.Passed++
		case tree.StatusFailed:
			c.Failed++
		case tree.StatusSkipped:
			c.Skipped++
		default:
			continue
		}
		c.Total++
	}
	return c
}

// dropQueue forgets the queued runs and clears their tests' queued marks.
func (m *Model) dropQueue() {
	for _, req := range m.queue {
		for _, l := range req.leaves {
			if l.Status() == tree.StatusQueued {
				l.SetStatus(tree.StatusNone)
			}
		}
	}
	m.queue = nil
}

// pump starts the next queued run when none is active.
func (m *Model) pump() tea.Cmd {
	if m.run != nil || len(m.queue) == 0 {
		return nil
	}
	req := m.queue[0]
	m.queue = m.queue[1:]
	// The sources may have been edited since the last run, which is usually
	// why this one is happening, so the code frames are read again.
	m.excerpts = nil
	for _, l := range req.leaves {
		l.SetStatus(tree.StatusRunning)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan tea.Msg, 256)
	m.run = &activeRun{req: req, cancel: cancel, events: ch}
	m.logs[req.target.Path] = nil
	m.lastLog = req.target.Path
	target, opts := req.target, m.cfg.Options
	go func() {
		runTests(ctx, target.Path, req.filter, opts, func(e dotnet.Event) { ch <- runEventMsg{target: target, event: e} })
	}()
	m.refresh()
	return waitMsg(ch)
}

func (m *Model) handleRunEvent(msg runEventMsg) tea.Cmd {
	if m.run == nil || msg.target != m.run.req.target {
		return nil
	}
	key := msg.target.Path
	switch e := msg.event.(type) {
	case dotnet.LineEvent:
		m.logs[key] = append(m.logs[key], e.Text)
		return waitMsg(m.run.events) // drawn on the next spinner tick

	case dotnet.ResultEvent:
		m.apply(msg.target, e.Result)
		return waitMsg(m.run.events)

	case dotnet.DoneEvent:
		for _, r := range e.Results {
			m.apply(msg.target, r)
		}
		for _, l := range m.run.req.leaves {
			if l.Status() == tree.StatusRunning {
				l.SetStatus(tree.StatusNone)
			}
		}
		label := m.run.req.label
		m.run.cancel()
		m.run = nil
		m.tree.FoldByResult(msg.target)
		var cmd tea.Cmd
		if e.Err != nil {
			m.dropQueue()
			cmd = m.setStatus(label+": "+e.Err.Error(), true)
		}
		if len(m.queue) == 0 {
			m.batchEnd, m.runsDone = now(), true
		}
		m.refresh()
		return tea.Batch(cmd, m.pump())
	}
	return nil
}

// apply records one result on its leaf, creating the leaf for a test that
// was not listed. A solution-wide run reports tests from every project, so
// the tree decides which one each belongs to.
func (m *Model) apply(target *tree.Node, r dotnet.Result) {
	leaf := m.tree.LeafIn(target, r.Name)
	if leaf == nil {
		return
	}
	m.addToBatch(leaf) // it may not have been listed, so not queued either
	res := r
	leaf.Result = &res
	switch r.Outcome {
	case dotnet.OutcomePassed:
		leaf.SetStatus(tree.StatusPassed)
	case dotnet.OutcomeFailed:
		leaf.SetStatus(tree.StatusFailed)
	case dotnet.OutcomeSkipped:
		leaf.SetStatus(tree.StatusSkipped)
	default:
		leaf.SetStatus(tree.StatusNone)
	}
}

// runFailed re-runs every test that failed last time. Theory rows share
// their method's filter, so each method is included once.
func (m *Model) runFailed() tea.Cmd {
	var nodes []*tree.Node
	seen := map[*tree.Node]bool{}
	for _, l := range m.root().Leaves() {
		if l.Status() != tree.StatusFailed {
			continue
		}
		unit := l
		if l.Kind == tree.KindCase {
			unit = l.Parent
		}
		if !seen[unit] {
			seen[unit] = true
			nodes = append(nodes, unit)
		}
	}
	if len(nodes) == 0 {
		return m.setStatus("No failed tests to re-run", false)
	}
	return m.enqueue(nodes)
}

// cancelRun stops the active run and drops the queue.
func (m *Model) cancelRun() tea.Cmd {
	if m.run == nil {
		return m.setStatus("Nothing is running", false)
	}
	m.dropQueue()
	m.run.cancel()
	return m.setStatus("Cancelling "+m.run.req.label+"…", false)
}

// Shutdown kills any running dotnet process; main calls it before exiting.
func (m *Model) Shutdown() {
	if m.run != nil {
		m.run.cancel()
	}
}

// Editor.

// openInEditor sends a source position to Neovim: from the log pane, the
// file and line named on the cursor's line when it is a stack frame or
// location; otherwise the offending line of a failed test, or the
// declaration of whatever is selected. Without a listening server nvim is
// opened in the terminal instead.
func (m *Model) openInEditor() tea.Cmd {
	n := m.current()
	if n == nil {
		return nil
	}
	var loc dotnet.Location
	ok := false
	if m.focus == paneLog && m.logCursor < len(m.logLines) {
		loc, ok = locationInLine(m.logLines[m.logCursor])
	}
	if !ok && n.Status() == tree.StatusFailed && n.Result != nil {
		loc, ok = n.Result.FailureLocation()
	}
	if !ok {
		loc, ok = m.locateNode(n)
	}
	if !ok {
		return m.setStatus("No source location for "+n.Name, true)
	}
	if hasServer(m.socket) {
		if err := openInServer(m.socket, loc.File, loc.Line); err != nil {
			return m.setStatus(err.Error(), true)
		}
		return m.setStatus(fmt.Sprintf("Sent %s:%d to nvim", filepath.Base(loc.File), loc.Line), false)
	}
	if _, err := lookPath("nvim"); err != nil {
		return m.setStatus("nvim not found in PATH", true)
	}
	return tea.ExecProcess(editor.LaunchCmd(loc.File, loc.Line), func(err error) tea.Msg { return editorDoneMsg{err: err} })
}

// Source positions as they appear in log lines: .NET stack frames
// ("at X in /path/file.cs:line 12"), xUnit's own frames
// ("/path/file.cs(12,0): at X") and dtest's location lines ("❯ path:12").
var (
	frameLocation = regexp.MustCompile(` in (.+?):line (\d+)`)
	parenLocation = regexp.MustCompile(`(\S+?\.\w+)\((\d+),\d+\)`)
	arrowLocation = regexp.MustCompile(iconArrow + ` (\S+):(\d+)$`)
)

// locationInLine extracts a file and line from a log line, resolving a
// relative path against the working directory.
func locationInLine(line string) (dotnet.Location, bool) {
	for _, re := range []*regexp.Regexp{frameLocation, arrowLocation, parenLocation} {
		if m := re.FindStringSubmatch(strings.TrimRight(line, " ")); m != nil {
			n, err := strconv.Atoi(m[2])
			if err != nil {
				continue
			}
			path := m[1]
			if abs, err := filepath.Abs(path); err == nil {
				path = abs
			}
			return dotnet.Location{File: path, Line: n}, true
		}
	}
	return dotnet.Location{}, false
}

// locateNode finds the source position for a node: the project file for a
// project, the class or method declaration otherwise, falling back to the
// failure position from a stack trace.
func (m *Model) locateNode(n *tree.Node) (dotnet.Location, bool) {
	// The root is the solution file, a project is its own.
	if n.Kind == tree.KindRoot || n.Kind == tree.KindProject {
		return dotnet.Location{File: n.Path}, true
	}
	class, method := n.Class()
	key := class + "#" + method
	if loc, ok := m.locations[key]; ok {
		return loc, true
	}
	loc, ok := locate(filepath.Dir(n.Project().Path), class, method)
	if !ok && n.Result != nil {
		loc, ok = n.Result.FailureLocation()
	}
	if ok {
		m.locations[key] = loc
	}
	return loc, ok
}

// Selecting and copying log lines.

// logTop and treeTop are the first screen row of each pane's body: the
// header and a title above the log, then the log and another title above
// the tree.
func (m *Model) logTop() int  { return headerH + titleH }
func (m *Model) treeTop() int { return m.logTop() + m.log.Height() + titleH }

// paneAt reports which pane covers a screen row, counting each pane's
// title as part of it, and falls back to whichever pane has focus for the
// header, the footer and anything off the screen.
func (m *Model) paneAt(y int) pane {
	switch {
	case y >= m.logTop()-titleH && y < m.logTop()+m.log.Height():
		return paneLog
	case y >= m.treeTop()-titleH && y < m.treeTop()+m.treeView.Height():
		return paneTree
	}
	return m.focus
}

// logLineAt maps a screen row to the log line drawn on it, or -1 when the
// row is not part of the log's body. A line that wrapped covers several
// rows, so the last one that starts at or above this row is the answer.
func (m *Model) logLineAt(y int) int {
	top := m.logTop()
	if y < top || y >= top+m.log.Height() || len(m.logStarts) == 0 {
		return -1
	}
	row := y - top + m.log.YOffset()
	line := 0
	for i, start := range m.logStarts {
		if start > row {
			break
		}
		line = i
	}
	return line
}

// treeRowAt maps a screen row to the tree row drawn on it, or -1. Tree rows
// are truncated rather than wrapped, so they are one row each.
func (m *Model) treeRowAt(y int) int {
	top := m.treeTop()
	if y < top || y >= top+m.treeView.Height() {
		return -1
	}
	row := y - top + m.treeView.YOffset()
	if row >= len(m.rows) {
		return -1
	}
	return row
}

// mouseDown moves focus to the pane that was clicked and puts its cursor on
// the row under the pointer. In the log that also anchors a selection, so a
// drag from here picks out lines. In the tree, clicking the row that is
// already selected folds it, which is how the mouse opens a project without
// reaching for a key.
func (m *Model) mouseDown(y int) tea.Cmd {
	if m.help {
		m.help = false
	}
	switch m.paneAt(y) {
	case paneLog:
		m.focus = paneLog
		if line := m.logLineAt(y); line >= 0 {
			m.dragging = true
			m.selAnchor, m.logCursor = line, line
		}
	case paneTree:
		m.focus = paneTree
		if row := m.treeRowAt(y); row >= 0 {
			if row == m.cursor {
				if n := m.current(); n != nil && !n.IsLeaf() && !m.filtered() {
					n.Expanded = !n.Expanded
				}
			}
			m.cursor = row
		}
	}
	m.refresh()
	return nil
}

// dragSelect extends a selection to the line under the mouse.
func (m *Model) dragSelect(y int) tea.Cmd {
	if !m.dragging {
		return nil
	}
	if line := m.logLineAt(y); line >= 0 {
		m.moveLog(line - m.logCursor)
		m.refresh()
	}
	return nil
}

// endSelect finishes a drag. A drag across more than one line copies what it
// covered, the way selecting in a terminal does; a plain click just leaves
// the cursor where it landed.
func (m *Model) endSelect() tea.Cmd {
	if !m.dragging {
		return nil
	}
	m.dragging = false
	if m.selAnchor == m.logCursor {
		m.selAnchor = -1
		m.refresh()
		return nil
	}
	return m.copyLog()
}

// selection is the range of log lines a copy would take, the anchor to the
// cursor while selecting, or the cursor's line alone.
func (m *Model) selection() (from, to int) {
	if len(m.logLines) == 0 {
		return -1, -1
	}
	from, to = m.logCursor, m.logCursor
	if m.selAnchor >= 0 {
		from, to = min(m.selAnchor, m.logCursor), max(m.selAnchor, m.logCursor)
	}
	return clamp(from, len(m.logLines)), clamp(to, len(m.logLines))
}

// copyLog puts the selected lines, or the cursor's line, on the system
// clipboard. It goes out as an OSC 52 escape, which the terminal acts on, so
// it reaches the clipboard of whatever machine the terminal is running on
// even when dtest is on the far side of an ssh session.
func (m *Model) copyLog() tea.Cmd {
	from, to := m.selection()
	if from < 0 {
		return m.setStatus("Nothing to copy", true)
	}
	text := strings.Join(m.logLines[from:to+1], "\n")
	m.selAnchor = -1
	m.refresh()
	return tea.Batch(tea.SetClipboard(text), m.setStatus("Copied "+plural(to-from+1, "line"), false))
}

// Cursor.

// current is the tree node under the cursor; the log shows its results.
func (m *Model) current() *tree.Node {
	if m.cursor < len(m.rows) {
		return m.rows[m.cursor]
	}
	return nil
}

func (m *Model) move(delta int) {
	m.cursor = clamp(m.cursor+delta, len(m.rows))
}

// moveLog moves the log's cursor line and scrolls it into view.
func (m *Model) moveLog(delta int) {
	m.logCursor = clamp(m.logCursor+delta, len(m.logLines))
	m.refreshLog()
	if m.logCursor < len(m.logStarts) {
		m.log.EnsureVisible(m.logStarts[m.logCursor], 0, 0)
	}
}

func clamp(i, n int) int {
	if i >= n {
		i = n - 1
	}
	if i < 0 {
		i = 0
	}
	return i
}

// selectNode moves the tree cursor to n, expanding its ancestors.
func (m *Model) selectNode(n *tree.Node) {
	for p := n.Parent; p != nil; p = p.Parent {
		p.Expanded = true
	}
	m.rows = m.visibleRows()
	for i, r := range m.rows {
		if r == n {
			m.cursor = i
			break
		}
	}
	m.refresh()
}

// root is the node the view is rooted at: the subtree in focus, or the
// solution when there is none.
func (m *Model) root() *tree.Node {
	if m.zoom != nil {
		return m.zoom
	}
	return m.tree.Root
}

// visibleRows are the tree pane's rows, within whatever is in focus.
func (m *Model) visibleRows() []*tree.Node {
	return m.tree.VisibleFrom(m.root(), m.query, m.statusFilter)
}

// focusNode narrows the view to the selected node, so everything from the
// rows on screen to what "run all" runs treats its tests as the only ones.
// A leaf has nothing below it to focus on.
func (m *Model) focusNode() tea.Cmd {
	n := m.current()
	switch {
	case n == nil:
		return nil
	case n.IsLeaf():
		return m.setStatus("Nothing to focus on under "+n.Name, true)
	case n == m.zoom:
		return m.setStatus("Already focused on "+n.Name, false)
	}
	m.zoom = n
	n.Expanded = true
	m.cursor = 0
	m.refresh()
	return nil
}

// unfocusNode steps back out one level, to the parent of what is in focus,
// and leaves the cursor on the subtree that was being looked at.
func (m *Model) unfocusNode() tea.Cmd {
	if m.zoom == nil {
		return m.setStatus("Not focused on anything", false)
	}
	was := m.zoom
	if m.zoom = was.Parent; m.zoom == m.tree.Root {
		m.zoom = nil
	}
	m.selectNode(was)
	return nil
}

// filtered reports whether the tree is narrowed by a query or a status, in
// which case folding is off since matches are always shown.
func (m *Model) filtered() bool {
	return m.query != "" || m.statusFilter != tree.StatusNone
}

// toggleStatusFilter narrows the tree to leaves in status, or widens it
// again when that filter is already on.
func (m *Model) toggleStatusFilter(status tree.Status) {
	if m.statusFilter == status {
		m.statusFilter = tree.StatusNone
	} else {
		m.statusFilter = status
	}
}

// nextFailed moves the tree cursor to the next (or previous) failed test in
// tree order, wrapping around.
func (m *Model) nextFailed(forward bool) tea.Cmd {
	leaves := m.root().Leaves()
	start := -1
	if cur := m.current(); cur != nil {
		for i, l := range leaves {
			if l == cur || (!cur.IsLeaf() && isUnder(l, cur)) {
				start = i
				if forward {
					break
				}
			}
		}
	}
	n := len(leaves)
	for step := 1; step <= n; step++ {
		i := start + step
		if !forward {
			i = start - step
		}
		l := leaves[((i%n)+n)%n]
		if l.Status() == tree.StatusFailed {
			m.selectNode(l)
			return nil
		}
	}
	return m.setStatus("No failed tests", false)
}

func isUnder(n, ancestor *tree.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p == ancestor {
			return true
		}
	}
	return false
}

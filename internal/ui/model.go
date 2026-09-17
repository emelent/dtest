// Package ui is the bubbletea front end, laid out in two panes in the
// style of vitest: the log of the selected tests on top, and below it the
// test tree with the run statistics right-aligned beside it.
package ui

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
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
	hasServer    = editor.HasServer
	openInServer = editor.Open
	lookPath     = exec.LookPath
	now          = time.Now
)

// Config is what main passes in.
type Config struct {
	Target  string         // solution or project file
	Options dotnet.Options // configuration and --no-build
	Socket  string         // explicit Neovim server socket; empty to use $nvim_sock
}

const (
	statusTimeout = 5 * time.Second
	buildLogKey   = "build"
)

// runRequest is one queued `dotnet test` invocation.
type runRequest struct {
	project *tree.Node
	filter  string
	label   string       // shown while running
	leaves  []*tree.Node // marked running until results arrive
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

	width, height int
	focus         pane
	log           viewport.Model
	logSig        string
	logNode       *tree.Node // whose results the log shows
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

	batchStart time.Time // when the current sequence of runs began
	batchEnd   time.Time // when it finished; zero while running
	runsDone   bool      // at least one run has finished

	status    string
	statusErr bool
	statusID  int

	help       bool
	showOutput bool // append the raw dotnet output to the report
	filtering  bool
	query      string
	pendingG   bool
	fatal      error

	locations map[string]dotnet.Location
}

// New returns a model for the solution or project in cfg.Target.
func New(cfg Config) *Model {
	name := strings.TrimSuffix(filepath.Base(cfg.Target), filepath.Ext(cfg.Target))
	m := &Model{
		cfg:       cfg,
		name:      name,
		socket:    editor.ResolveSocket(cfg.Socket),
		tree:      tree.New(),
		logs:      map[string][]string{},
		locations: map[string]dotnet.Location{},
		spin:      spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(styleRunning)),
		log:       viewport.New(),
		treeView:  viewport.New(),
		focus:     paneTree,
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
		project *tree.Node
		event   dotnet.Event
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
		var cmd tea.Cmd
		if m.focus == paneLog {
			m.log, cmd = m.log.Update(msg)
		} else {
			m.treeView, cmd = m.treeView.Update(msg)
		}
		return m, cmd

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
		req := runRequest{project: p}
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
		m.queue = append(m.queue, req)
	}
	if m.run == nil {
		m.batchStart, m.batchEnd = now(), time.Time{}
	}
	return m.pump()
}

// pump starts the next queued run when none is active.
func (m *Model) pump() tea.Cmd {
	if m.run != nil || len(m.queue) == 0 {
		return nil
	}
	req := m.queue[0]
	m.queue = m.queue[1:]
	for _, l := range req.leaves {
		l.SetStatus(tree.StatusRunning)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan tea.Msg, 256)
	m.run = &activeRun{req: req, cancel: cancel, events: ch}
	m.logs[req.project.Path] = nil
	m.lastLog = req.project.Path
	project, opts := req.project, m.cfg.Options
	go func() {
		runTests(ctx, project.Path, req.filter, opts, func(e dotnet.Event) { ch <- runEventMsg{project: project, event: e} })
	}()
	m.refresh()
	return waitMsg(ch)
}

func (m *Model) handleRunEvent(msg runEventMsg) tea.Cmd {
	if m.run == nil || msg.project != m.run.req.project {
		return nil
	}
	key := msg.project.Path
	switch e := msg.event.(type) {
	case dotnet.LineEvent:
		m.logs[key] = append(m.logs[key], e.Text)
		return waitMsg(m.run.events) // drawn on the next spinner tick

	case dotnet.ResultEvent:
		m.apply(msg.project, e.Result)
		return waitMsg(m.run.events)

	case dotnet.DoneEvent:
		for _, r := range e.Results {
			m.apply(msg.project, r)
		}
		for _, l := range m.run.req.leaves {
			if l.Status() == tree.StatusRunning {
				l.SetStatus(tree.StatusNone)
			}
		}
		label := m.run.req.label
		m.run.cancel()
		m.run = nil
		m.tree.FoldByResult(msg.project)
		var cmd tea.Cmd
		if e.Err != nil {
			m.queue = nil
			cmd = m.setStatus(label+": "+e.Err.Error(), true)
		}
		if len(m.queue) == 0 {
			m.batchEnd = now()
			m.runsDone = true
		}
		m.refresh()
		return tea.Batch(cmd, m.pump())
	}
	return nil
}

// apply records one result on its leaf, creating the leaf for a test that
// was not listed.
func (m *Model) apply(project *tree.Node, r dotnet.Result) {
	leaf := m.tree.Leaf(project, r.Name)
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
	for _, l := range m.tree.Leaves() {
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
	m.queue = nil
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

// openInEditor sends the source for the selected node to Neovim: the
// offending line of a failed test, otherwise the declaration. Without a
// listening server nvim is opened in the terminal instead.
func (m *Model) openInEditor() tea.Cmd {
	n := m.current()
	if n == nil {
		return nil
	}
	var loc dotnet.Location
	ok := false
	if n.Status() == tree.StatusFailed && n.Result != nil {
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

// locateNode finds the source position for a node: the project file for a
// project, the class or method declaration otherwise, falling back to the
// failure position from a stack trace.
func (m *Model) locateNode(n *tree.Node) (dotnet.Location, bool) {
	if n.Kind == tree.KindProject {
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
	m.rows = m.tree.Visible(m.query)
	for i, r := range m.rows {
		if r == n {
			m.cursor = i
			break
		}
	}
	m.refresh()
}

// nextFailed moves the tree cursor to the next (or previous) failed test in
// tree order, wrapping around.
func (m *Model) nextFailed(forward bool) tea.Cmd {
	leaves := m.tree.Leaves()
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

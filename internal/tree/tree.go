// Package tree arranges tests into project → class → method → case nodes
// and tracks their status.
package tree

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dtest/internal/dotnet"
)

// Kind is the level of a node in the tree.
type Kind int

const (
	KindProject Kind = iota
	KindClass
	KindMethod
	KindCase // one data row of a theory / parameterised test
)

// Status is a node's most recent test status. Interior nodes derive theirs
// from their leaves with [Node.Status].
type Status int

const (
	StatusNone Status = iota
	StatusRunning
	StatusQueued // scheduled in a run that has not started yet
	StatusPassed
	StatusFailed
	StatusSkipped
)

// Node is one entry of the tree.
type Node struct {
	Kind     Kind
	Name     string // label shown in the tree; a class drops the project-name prefix of its namespace
	FQN      string // fully qualified name: "Ns.Class", "Ns.Class.Method", or the case display name
	Path     string // project file (project nodes only)
	Parent   *Node
	Children []*Node
	Expanded bool

	// Leaf state; see IsLeaf.
	status Status
	Result *dotnet.Result
}

// IsLeaf reports whether the node is a runnable unit with its own result: a
// case, or a method without cases.
func (n *Node) IsLeaf() bool { return len(n.Children) == 0 && n.Kind >= KindMethod }

// Project returns the project node the node belongs to.
func (n *Node) Project() *Node {
	for n.Parent != nil {
		n = n.Parent
	}
	return n
}

// Depth is the number of ancestors of the node.
func (n *Node) Depth() int {
	d := 0
	for p := n.Parent; p != nil; p = p.Parent {
		d++
	}
	return d
}

// Leaves returns every leaf under (or at) the node.
func (n *Node) Leaves() []*Node {
	if n.IsLeaf() {
		return []*Node{n}
	}
	var out []*Node
	for _, c := range n.Children {
		out = append(out, c.Leaves()...)
	}
	return out
}

// SetStatus sets a leaf's status. It is ignored on interior nodes.
func (n *Node) SetStatus(s Status) {
	if n.IsLeaf() {
		n.status = s
	}
}

// Status is the leaf's own status or, for an interior node, the roll-up of
// its leaves: running beats queued beats failed beats passed beats skipped;
// a node whose leaves have never run is StatusNone.
func (n *Node) Status() Status {
	if n.IsLeaf() {
		return n.status
	}
	c := n.Counts()
	switch {
	case c.Running > 0:
		return StatusRunning
	case c.Queued > 0:
		return StatusQueued
	case c.Failed > 0:
		return StatusFailed
	case c.Passed > 0:
		return StatusPassed
	case c.Skipped > 0:
		return StatusSkipped
	}
	return StatusNone
}

// Counts tallies leaf statuses.
type Counts struct{ Total, Running, Queued, Passed, Failed, Skipped int }

// Counts tallies the leaves under the node.
func (n *Node) Counts() Counts {
	var c Counts
	for _, l := range n.Leaves() {
		c.Total++
		switch l.status {
		case StatusRunning:
			c.Running++
		case StatusQueued:
			c.Queued++
		case StatusPassed:
			c.Passed++
		case StatusFailed:
			c.Failed++
		case StatusSkipped:
			c.Skipped++
		}
	}
	return c
}

// Duration is how long the node's tests took in their latest run: a leaf's
// own time, or the sum over the leaves beneath that have a result. Tests
// that have not run contribute nothing.
func (n *Node) Duration() time.Duration {
	var total time.Duration
	for _, l := range n.Leaves() {
		if l.Result != nil {
			total += l.Result.Duration
		}
	}
	return total
}

// Filter is the dotnet --filter expression selecting the node's tests; it is
// empty for a project, which runs unfiltered. A case shares its method's
// filter, since data rows cannot be addressed by fully qualified name.
func (n *Node) Filter() string {
	switch n.Kind {
	case KindProject:
		return ""
	case KindClass:
		return dotnet.FilterPrefix(n.FQN + ".")
	case KindMethod:
		return dotnet.FilterExact(n.FQN)
	case KindCase:
		return n.Parent.Filter()
	}
	return ""
}

// Class returns the fully qualified class name and method name that declare
// the node's tests, for locating them in source. Both are empty for a
// project.
func (n *Node) Class() (class, method string) {
	switch n.Kind {
	case KindClass:
		return n.FQN, ""
	case KindMethod:
		return n.Parent.FQN, n.Name
	case KindCase:
		return n.Parent.Parent.FQN, n.Parent.Name
	}
	return "", ""
}

// Tree holds one node per project.
type Tree struct {
	Projects []*Node
	leaves   map[*Node]map[string]*Node // by project, then display name
}

// New returns an empty tree.
func New() *Tree {
	return &Tree{leaves: map[*Node]map[string]*Node{}}
}

// AddProject adds a project node for the project file at path, expanded,
// or returns the existing one.
func (t *Tree) AddProject(path string) *Node {
	for _, p := range t.Projects {
		if p.Path == path {
			return p
		}
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	p := &Node{Kind: KindProject, Name: name, FQN: name, Path: path, Expanded: true}
	t.Projects = append(t.Projects, p)
	t.leaves[p] = map[string]*Node{}
	return p
}

// SetTests replaces the tests under project with names (display names from
// --list-tests). Results and expansion of nodes that still exist are kept.
// New classes start collapsed, so a fresh project reads as a list of
// classes.
func (t *Tree) SetTests(project *Node, names []string) {
	old := map[string]*Node{}
	for _, n := range collect(project) {
		old[nodeKey(n)] = n
	}
	project.Children = nil
	t.leaves[project] = map[string]*Node{}
	for _, name := range names {
		t.Leaf(project, name)
	}
	for _, n := range collect(project) {
		if prev, ok := old[nodeKey(n)]; ok {
			n.Expanded = prev.Expanded
			if n.IsLeaf() && prev.IsLeaf() {
				n.status, n.Result = prev.status, prev.Result
			}
		}
	}
}

func nodeKey(n *Node) string { return string(rune('0'+n.Kind)) + n.FQN }

func collect(n *Node) []*Node {
	out := []*Node{n}
	for _, c := range n.Children {
		out = append(out, collect(c)...)
	}
	return out
}

// Leaf returns the leaf for the test with display name under project,
// creating the path to it when it is new (a test that appeared since the
// tree was listed).
func (t *Tree) Leaf(project *Node, name string) *Node {
	if n, ok := t.leaves[project][name]; ok {
		return n
	}
	ns, class, method, args := SplitName(name)
	classFQN := join(ns, class)
	classNode := child(project, KindClass, shortClass(project.Name, classFQN), classFQN, false)
	methodNode := child(classNode, KindMethod, method, join(classFQN, method), false)
	leaf := methodNode
	if args != "" {
		// A method that previously stood alone becomes the parent of its rows.
		if methodNode.status != StatusNone || methodNode.Result != nil {
			methodNode.status, methodNode.Result = StatusNone, nil
		}
		leaf = child(methodNode, KindCase, args, name, false)
	}
	t.leaves[project][name] = leaf
	return leaf
}

// Lookup returns the leaf for the test with display name under project,
// or nil.
func (t *Tree) Lookup(project *Node, name string) *Node {
	return t.leaves[project][name]
}

// shortClass drops the project name from the front of a class's fully
// qualified name ("Shop.Core.Tests.Pricing.MoneyTests" in project
// Shop.Core.Tests becomes "Pricing.MoneyTests").
func shortClass(project, fqn string) string {
	if rest, ok := strings.CutPrefix(fqn, project+"."); ok && rest != "" {
		return rest
	}
	return fqn
}

func join(a, b string) string {
	if a == "" {
		return b
	}
	return a + "." + b
}

// child returns parent's child with the given kind and FQN, inserting it in
// name order when it is missing.
func child(parent *Node, kind Kind, name, fqn string, expanded bool) *Node {
	for _, c := range parent.Children {
		if c.Kind == kind && c.FQN == fqn {
			return c
		}
	}
	n := &Node{Kind: kind, Name: name, FQN: fqn, Parent: parent, Expanded: expanded}
	i := sort.Search(len(parent.Children), func(i int) bool {
		return strings.ToLower(parent.Children[i].Name) > strings.ToLower(name)
	})
	parent.Children = append(parent.Children, nil)
	copy(parent.Children[i+1:], parent.Children[i:])
	parent.Children[i] = n
	return n
}

// SplitName breaks a display name such as "Ns.Sub.Class.Method(n: 1)" into
// its namespace, class, method and argument list ("(n: 1)"; empty for a
// plain test). Dots inside parentheses or angle brackets do not split, so
// generic and parameterised names hold together. A name with no namespace
// gets an empty one; a bare name is treated as a method of an unnamed class.
func SplitName(name string) (ns, class, method, args string) {
	base := name
	if i := indexTopLevel(name, '('); i >= 0 {
		base, args = name[:i], name[i:]
	}
	base, method = splitLast(base)
	ns, class = splitLast(base)
	return ns, class, method, args
}

// splitLast splits s at its last top-level dot.
func splitLast(s string) (head, tail string) {
	depth := 0
	for i := len(s) - 1; i >= 0; i-- {
		switch s[i] {
		case ')', '>', ']':
			depth++
		case '(', '<', '[':
			depth--
		case '.':
			if depth == 0 {
				return s[:i], s[i+1:]
			}
		}
	}
	return "", s
}

// indexTopLevel returns the index of the first c outside angle brackets.
func indexTopLevel(s string, c byte) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			depth++
		case '>':
			depth--
		case c:
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// Visible flattens the tree into the rows to draw. Without a filter,
// collapsed nodes hide their children. With a query, only leaves whose
// display name (or project name) contains it, case-insensitively, are
// shown; with a status other than StatusNone, only leaves in that status.
// Matching leaves bring their ancestors along regardless of expansion, and
// projects without a match are left out.
func (t *Tree) Visible(query string, status Status) []*Node {
	var rows []*Node
	q := strings.ToLower(query)
	for _, p := range t.Projects {
		if q == "" && status == StatusNone {
			rows = appendVisible(rows, p)
		} else {
			rows = appendMatching(rows, p, q, status)
		}
	}
	return rows
}

func appendVisible(rows []*Node, n *Node) []*Node {
	rows = append(rows, n)
	if n.Expanded {
		for _, c := range n.Children {
			rows = appendVisible(rows, c)
		}
	}
	return rows
}

func appendMatching(rows []*Node, n *Node, q string, status Status) []*Node {
	if n.IsLeaf() {
		byName := q == "" || strings.Contains(strings.ToLower(n.FQN), q) || strings.Contains(strings.ToLower(n.Project().Name), q)
		byStatus := status == StatusNone || n.status == status
		if byName && byStatus {
			return append(rows, n)
		}
		return rows
	}
	start := len(rows)
	rows = append(rows, n)
	for _, c := range n.Children {
		rows = appendMatching(rows, c, q, status)
	}
	if len(rows) == start+1 {
		return rows[:start] // nothing matched beneath: drop the node itself
	}
	return rows
}

// Leaves returns every leaf of the tree in order.
func (t *Tree) Leaves() []*Node {
	var out []*Node
	for _, p := range t.Projects {
		out = append(out, p.Leaves()...)
	}
	return out
}

// FoldByResult expands the interior nodes under project that hold a failure
// and collapses the rest, so a finished run reads like a report: passing
// classes take one line, failing ones show their tests.
func (t *Tree) FoldByResult(project *Node) {
	for _, n := range collect(project) {
		if n.Kind == KindProject || n.IsLeaf() {
			continue
		}
		n.Expanded = n.Counts().Failed > 0
	}
}

// Counts tallies every leaf in the tree.
func (t *Tree) Counts() Counts {
	var c Counts
	for _, p := range t.Projects {
		pc := p.Counts()
		c.Total += pc.Total
		c.Running += pc.Running
		c.Passed += pc.Passed
		c.Failed += pc.Failed
		c.Skipped += pc.Skipped
	}
	return c
}

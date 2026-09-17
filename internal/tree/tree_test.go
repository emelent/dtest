package tree

import (
	"strings"
	"testing"
	"time"

	"dtest/internal/dotnet"
)

func TestSplitName(t *testing.T) {
	cases := []struct{ in, ns, class, method, args string }{
		{"Alpha.Tests.MathTests.Adds", "Alpha.Tests", "MathTests", "Adds", ""},
		{"Alpha.Tests.MathTests.Theory(n: 1)", "Alpha.Tests", "MathTests", "Theory", "(n: 1)"},
		{"Ns.C.M(s: \"a.b\", x: 1.5)", "Ns", "C", "M", "(s: \"a.b\", x: 1.5)"},
		{"Ns.Outer+Inner.M", "Ns", "Outer+Inner", "M", ""},
		{"Ns.Gen<System.Int32>.M<System.String>(x: 1)", "Ns", "Gen<System.Int32>", "M<System.String>", "(x: 1)"},
		{"Class.Method", "", "Class", "Method", ""},
		{"Method", "", "", "Method", ""},
	}
	for _, c := range cases {
		ns, class, method, args := SplitName(c.in)
		if ns != c.ns || class != c.class || method != c.method || args != c.args {
			t.Errorf("SplitName(%q) = %q %q %q %q", c.in, ns, class, method, args)
		}
	}
}

func names(rows []*Node) string {
	var parts []string
	for _, r := range rows {
		parts = append(parts, strings.Repeat(" ", r.Depth())+r.Name)
	}
	return strings.Join(parts, "|")
}

func sample() (*Tree, *Node) {
	tr := New()
	p := tr.AddProject("/src/Alpha.Tests/Alpha.Tests.csproj")
	tr.SetTests(p, []string{
		"Alpha.Tests.MathTests.Theory(n: 2)",
		"Alpha.Tests.MathTests.Adds",
		"Alpha.Tests.MathTests.Theory(n: 1)",
		"Alpha.Tests.SlowTests.Waits",
		"Alpha.Other.X.Y",
	})
	return tr, p
}

func TestBuildAndVisible(t *testing.T) {
	tr, p := sample()
	if p.Name != "Alpha.Tests" {
		t.Errorf("project name = %q", p.Name)
	}
	// Projects start expanded with their classes collapsed; class labels drop
	// the project-name prefix.
	got := names(tr.Visible("", StatusNone))
	want := "Alpha.Tests| Alpha.Other.X| MathTests| SlowTests"
	if got != want {
		t.Fatalf("Visible =\n%s\nwant\n%s", got, want)
	}
	for _, c := range p.Children {
		c.Expanded = true
	}
	got = names(tr.Visible("", StatusNone))
	want = "Alpha.Tests| Alpha.Other.X|  Y| MathTests|  Adds|  Theory| SlowTests|  Waits"
	if got != want {
		t.Fatalf("expanded classes =\n%s\nwant\n%s", got, want)
	}
	theory := tr.Lookup(p, "Alpha.Tests.MathTests.Theory(n: 1)").Parent
	if theory.Kind != KindMethod || theory.IsLeaf() || len(theory.Children) != 2 || theory.Children[0].Name != "(n: 1)" {
		t.Fatalf("theory node = %+v", theory)
	}
	theory.Expanded = true
	if got := names(tr.Visible("", StatusNone)); !strings.Contains(got, "Theory|   (n: 1)|   (n: 2)|") {
		t.Errorf("expanded theory: %s", got)
	}
	if c := p.Counts(); c.Total != 5 {
		t.Errorf("Total = %d", c.Total)
	}
	// Filtering shows matching leaves and their ancestors only, and the
	// project name matches too.
	got = names(tr.Visible("waits", StatusNone))
	if got != "Alpha.Tests| SlowTests|  Waits" {
		t.Errorf("filtered = %s", got)
	}
	if got := len(tr.Visible("alpha.tests", StatusNone)); got != 10 {
		t.Errorf("project-name filter rows = %d", got)
	}
	if got := names(tr.Visible("nomatch", StatusNone)); got != "" {
		t.Errorf("no match = %s", got)
	}
}

func TestFiltersAndClass(t *testing.T) {
	tr, p := sample()
	leaf := tr.Lookup(p, "Alpha.Tests.MathTests.Theory(n: 1)")
	if f := leaf.Filter(); f != "FullyQualifiedName=Alpha.Tests.MathTests.Theory" {
		t.Errorf("case filter = %q", f)
	}
	class := leaf.Parent.Parent
	if f := class.Filter(); f != "FullyQualifiedName~Alpha.Tests.MathTests." {
		t.Errorf("class filter = %q", f)
	}
	if class.Parent != p || class.Name != "MathTests" || class.FQN != "Alpha.Tests.MathTests" {
		t.Errorf("class node = %+v", class)
	}
	if f := p.Filter(); f != "" {
		t.Errorf("project filter = %q", f)
	}
	if c, m := leaf.Class(); c != "Alpha.Tests.MathTests" || m != "Theory" {
		t.Errorf("case Class() = %q %q", c, m)
	}
	if c, m := class.Class(); c != "Alpha.Tests.MathTests" || m != "" {
		t.Errorf("class Class() = %q %q", c, m)
	}
	if leaf.Project() != p {
		t.Error("Project() should reach the project node")
	}
}

func TestStatusRollup(t *testing.T) {
	tr, p := sample()
	adds := tr.Lookup(p, "Alpha.Tests.MathTests.Adds")
	class := adds.Parent
	if class.Status() != StatusNone {
		t.Fatal("fresh class should be StatusNone")
	}
	adds.SetStatus(StatusPassed)
	if class.Status() != StatusPassed || p.Status() != StatusPassed {
		t.Error("one passed leaf should make ancestors passed")
	}
	tr.Lookup(p, "Alpha.Tests.MathTests.Theory(n: 2)").SetStatus(StatusFailed)
	if class.Status() != StatusFailed {
		t.Error("a failed leaf should win over passed")
	}
	tr.Lookup(p, "Alpha.Tests.MathTests.Theory(n: 1)").SetStatus(StatusRunning)
	if class.Status() != StatusRunning || tr.Counts().Running != 1 {
		t.Error("a running leaf should win over failed")
	}
	// Interior nodes ignore SetStatus.
	class.SetStatus(StatusPassed)
	if class.Status() != StatusRunning {
		t.Error("SetStatus on an interior node must be ignored")
	}
	// Queued sits between running and failed in the roll-up.
	tr.Lookup(p, "Alpha.Tests.MathTests.Theory(n: 1)").SetStatus(StatusQueued)
	if class.Status() != StatusQueued || class.Counts().Queued != 1 {
		t.Errorf("queued roll-up = %v", class.Status())
	}
	tr.Lookup(p, "Alpha.Tests.SlowTests.Waits").SetStatus(StatusRunning)
	if p.Status() != StatusRunning {
		t.Error("running beats queued")
	}
}

func TestDuration(t *testing.T) {
	tr, p := sample()
	if p.Duration() != 0 {
		t.Fatal("unrun tree should have no duration")
	}
	adds := tr.Lookup(p, "Alpha.Tests.MathTests.Adds")
	adds.Result = &dotnet.Result{Duration: 1500 * time.Millisecond}
	row := tr.Lookup(p, "Alpha.Tests.MathTests.Theory(n: 1)")
	row.Result = &dotnet.Result{Duration: 250 * time.Millisecond}
	if adds.Duration() != 1500*time.Millisecond {
		t.Errorf("leaf = %v", adds.Duration())
	}
	if got := adds.Parent.Duration(); got != 1750*time.Millisecond {
		t.Errorf("class = %v", got)
	}
	if got := row.Parent.Duration(); got != 250*time.Millisecond {
		t.Errorf("theory method = %v", got)
	}
	if got := p.Duration(); got != 1750*time.Millisecond {
		t.Errorf("project = %v", got)
	}
}

func TestSetTestsKeepsStateAndLeafAddsNew(t *testing.T) {
	tr, p := sample()
	adds := tr.Lookup(p, "Alpha.Tests.MathTests.Adds")
	adds.SetStatus(StatusFailed)
	adds.Result = &dotnet.Result{Name: adds.FQN, Outcome: dotnet.OutcomeFailed}
	adds.Parent.Expanded = true

	tr.SetTests(p, []string{"Alpha.Tests.MathTests.Adds", "Alpha.Tests.MathTests.New"})
	adds2 := tr.Lookup(p, "Alpha.Tests.MathTests.Adds")
	if adds2 == adds {
		t.Fatal("SetTests should rebuild nodes")
	}
	if adds2.Status() != StatusFailed || adds2.Result == nil || !adds2.Parent.Expanded {
		t.Errorf("state not carried over: %+v", adds2)
	}
	if tr.Lookup(p, "Alpha.Tests.SlowTests.Waits") != nil {
		t.Error("removed test should be gone")
	}
	n := tr.Leaf(p, "Alpha.Tests.Fresh.Class.Method")
	if n == nil || n.Kind != KindMethod || tr.Lookup(p, "Alpha.Tests.Fresh.Class.Method") != n {
		t.Error("Leaf should add an unlisted test")
	}
	if got := len(tr.Leaves()); got != 3 {
		t.Errorf("Leaves = %d", got)
	}
}

func TestVisibleByStatus(t *testing.T) {
	tr, p := sample()
	tr.Lookup(p, "Alpha.Tests.MathTests.Theory(n: 2)").SetStatus(StatusFailed)
	tr.Lookup(p, "Alpha.Tests.SlowTests.Waits").SetStatus(StatusSkipped)
	if got := names(tr.Visible("", StatusFailed)); got != "Alpha.Tests| MathTests|  Theory|   (n: 2)" {
		t.Errorf("failed only = %s", got)
	}
	if got := names(tr.Visible("", StatusSkipped)); got != "Alpha.Tests| SlowTests|  Waits" {
		t.Errorf("skipped only = %s", got)
	}
	// A text query and a status filter combine.
	if got := names(tr.Visible("theory", StatusSkipped)); got != "" {
		t.Errorf("combined = %s", got)
	}
}

func TestFoldByResult(t *testing.T) {
	tr, p := sample()
	for _, c := range p.Children {
		c.Expanded = true
	}
	tr.Lookup(p, "Alpha.Tests.MathTests.Theory(n: 2)").SetStatus(StatusFailed)
	tr.Lookup(p, "Alpha.Tests.SlowTests.Waits").SetStatus(StatusPassed)
	tr.FoldByResult(p)
	got := names(tr.Visible("", StatusNone))
	want := "Alpha.Tests| Alpha.Other.X| MathTests|  Adds|  Theory|   (n: 1)|   (n: 2)| SlowTests"
	if got != want {
		t.Fatalf("folded =\n%s\nwant\n%s", got, want)
	}
}

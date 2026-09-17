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
	// Projects, namespaces and classes start expanded; methods with cases do not.
	got := names(tr.Visible(""))
	want := "Alpha.Tests| Alpha.Other|  X|   Y| Alpha.Tests|  MathTests|   Adds|   Theory|  SlowTests|   Waits"
	if got != want {
		t.Fatalf("Visible =\n%s\nwant\n%s", got, want)
	}
	theory := tr.Lookup(p, "Alpha.Tests.MathTests.Theory(n: 1)").Parent
	if theory.Kind != KindMethod || theory.IsLeaf() || len(theory.Children) != 2 || theory.Children[0].Name != "(n: 1)" {
		t.Fatalf("theory node = %+v", theory)
	}
	theory.Expanded = true
	if got := names(tr.Visible("")); !strings.Contains(got, "Theory|    (n: 1)|    (n: 2)|") {
		t.Errorf("expanded theory: %s", got)
	}
	if c := p.Counts(); c.Total != 5 {
		t.Errorf("Total = %d", c.Total)
	}
	// Filtering shows matching leaves and their ancestors only.
	got = names(tr.Visible("waits"))
	if got != "Alpha.Tests| Alpha.Tests|  SlowTests|   Waits" {
		t.Errorf("filtered = %s", got)
	}
	if got := names(tr.Visible("nomatch")); got != "" {
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
	if f := class.Parent.Filter(); f != "FullyQualifiedName~Alpha.Tests." {
		t.Errorf("namespace filter = %q", f)
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
	adds.Parent.Expanded = false
	adds.Marked = true

	tr.SetTests(p, []string{"Alpha.Tests.MathTests.Adds", "Alpha.Tests.MathTests.New"})
	adds2 := tr.Lookup(p, "Alpha.Tests.MathTests.Adds")
	if adds2 == adds {
		t.Fatal("SetTests should rebuild nodes")
	}
	if adds2.Status() != StatusFailed || adds2.Result == nil || !adds2.Marked || adds2.Parent.Expanded {
		t.Errorf("state not carried over: %+v", adds2)
	}
	if tr.Lookup(p, "Alpha.Tests.SlowTests.Waits") != nil {
		t.Error("removed test should be gone")
	}
	n := tr.Leaf(p, "Alpha.Tests.Fresh.Class.Method")
	if n == nil || n.Kind != KindMethod || tr.Lookup(p, "Alpha.Tests.Fresh.Class.Method") != n {
		t.Error("Leaf should add an unlisted test")
	}
	if got := len(tr.Marked()); got != 1 {
		t.Errorf("Marked = %d", got)
	}
	tr.ClearMarks()
	if len(tr.Marked()) != 0 {
		t.Error("ClearMarks")
	}
	// Collapsing everything keeps the projects open so their namespaces show.
	tr.SetExpandedAll(false)
	if got := names(tr.Visible("")); got != "Alpha.Tests| Alpha.Tests| Alpha.Tests.Fresh" {
		t.Errorf("collapsed all = %s", got)
	}
}

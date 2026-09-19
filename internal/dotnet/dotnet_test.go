package dotnet

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseList(t *testing.T) {
	out := "Test run for /x/Alpha.Tests.dll (.NETCoreApp,Version=v10.0)\r\n" +
		"The following Tests are available:\r\n" +
		"    Alpha.Tests.MathTests.Adds\r\n" +
		"    Alpha.Tests.MathTests.Theory(n: 1)\r\n" +
		"\r\n" +
		"Test run for /x/Beta.Tests.dll\r\n" +
		"The following Tests are available:\r\n" +
		"    Beta.Tests.UnitTest1.Test1\r\n"
	got := ParseList(out)
	want := []string{"Alpha.Tests.MathTests.Adds", "Alpha.Tests.MathTests.Theory(n: 1)", "Beta.Tests.UnitTest1.Test1"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("ParseList = %v, want %v", got, want)
	}
	if got := ParseList("No test is available in /x/Lib.dll"); len(got) != 0 {
		t.Fatalf("expected no tests, got %v", got)
	}
}

func TestReadLines(t *testing.T) {
	long := strings.Repeat("x", 5*1024*1024)
	var got []string
	readLines(strings.NewReader("a\r\n"+long+"\nlast"), func(l string) { got = append(got, l) })
	if len(got) != 3 || got[0] != "a" || len(got[1]) != len(long) || got[2] != "last" {
		t.Fatalf("readLines: %d lines, lens %d %d", len(got), len(got[0]), len(got[1]))
	}
}

func TestParseResultLine(t *testing.T) {
	cases := []struct {
		line    string
		name    string
		outcome Outcome
		dur     time.Duration
	}{
		{"  Passed Alpha.Tests.MathTests.Adds [1 ms]", "Alpha.Tests.MathTests.Adds", OutcomePassed, time.Millisecond},
		{"  Passed Alpha.Tests.MathTests.Theory(n: 1) [< 1 ms]", "Alpha.Tests.MathTests.Theory(n: 1)", OutcomePassed, time.Millisecond},
		{"  Failed Alpha.Tests.MathTests.Fails [1 s]", "Alpha.Tests.MathTests.Fails", OutcomeFailed, time.Second},
		{"  Skipped Alpha.Tests.MathTests.Skip", "Alpha.Tests.MathTests.Skip", OutcomeSkipped, 0},
		{"  Passed Ns.C.M(a: [1, 2]) [2 m 3 s]", "Ns.C.M(a: [1, 2])", OutcomePassed, 2*time.Minute + 3*time.Second},
	}
	for _, c := range cases {
		r, ok := ParseResultLine(c.line)
		if !ok || r.Name != c.name || r.Outcome != c.outcome || r.Duration != c.dur {
			t.Errorf("ParseResultLine(%q) = %+v %v", c.line, r, ok)
		}
	}
	for _, line := range []string{"Passed foo", "  Error Message:", "[xUnit.net 00:00:00.06]     Alpha.Tests.MathTests.Fails [FAIL]", "Total tests: 5"} {
		if _, ok := ParseResultLine(line); ok {
			t.Errorf("%q should not parse", line)
		}
	}
}

const sampleTRX = "\xef\xbb\xbf" + `<?xml version="1.0" encoding="utf-8"?>
<TestRun id="1" xmlns="http://microsoft.com/schemas/VisualStudio/TeamTest/2010">
  <Results>
    <UnitTestResult testName="Alpha.Tests.MathTests.Adds" duration="00:00:00.0013986" outcome="Passed" />
    <UnitTestResult testName="Alpha.Tests.MathTests.Fails" duration="00:00:01.5000000" outcome="Failed">
      <Output>
        <StdOut>hello</StdOut>
        <ErrorInfo>
          <Message>Assert.Equal() Failure
Expected: 5</Message>
          <StackTrace>   at Alpha.Tests.MathTests.Fails() in /src/Alpha.Tests/UnitTest1.cs:line 12
   at System.Reflection.MethodBaseInvoker.InterpretedInvoke_Method(Object obj, IntPtr* args)</StackTrace>
        </ErrorInfo>
      </Output>
    </UnitTestResult>
    <UnitTestResult testName="Alpha.Tests.MathTests.Skip" duration="00:00:00" outcome="NotExecuted" />
  </Results>
</TestRun>`

func TestParseTRX(t *testing.T) {
	results, err := ParseTRX(strings.NewReader(sampleTRX))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results", len(results))
	}
	if r := results[0]; r.Name != "Alpha.Tests.MathTests.Adds" || r.Outcome != OutcomePassed || r.Duration != 1398600 {
		t.Errorf("results[0] = %+v", r)
	}
	r := results[1]
	if r.Outcome != OutcomeFailed || r.Duration != 1500*time.Millisecond || r.Output != "hello" || !strings.HasPrefix(r.Message, "Assert.Equal() Failure") {
		t.Errorf("results[1] = %+v", r)
	}
	if loc, ok := r.FailureLocation(); !ok || loc.File != "/src/Alpha.Tests/UnitTest1.cs" || loc.Line != 12 {
		t.Errorf("FailureLocation = %+v %v", loc, ok)
	}
	if results[2].Outcome != OutcomeSkipped {
		t.Errorf("results[2] = %+v", results[2])
	}
}

func TestFilters(t *testing.T) {
	if got := FilterExact("Ns.Class.Method"); got != "FullyQualifiedName=Ns.Class.Method" {
		t.Errorf("FilterExact = %q", got)
	}
	if got := FilterPrefix("Ns.Class."); got != "FullyQualifiedName~Ns.Class." {
		t.Errorf("FilterPrefix = %q", got)
	}
	if got := FilterExact("Ns.C.M(x)"); got != `FullyQualifiedName=Ns.C.M\(x\)` {
		t.Errorf("escaped FilterExact = %q", got)
	}
	if got := JoinFilters([]string{"a", "b"}); got != "a|b" {
		t.Errorf("JoinFilters = %q", got)
	}
}

func TestProjects(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("A.Tests/A.Tests.csproj", `<Project><ItemGroup><PackageReference Include="Microsoft.NET.Test.Sdk" /></ItemGroup></Project>`)
	write("B.Tests/B.Tests.csproj", `<Project><PropertyGroup><IsTestProject>true</IsTestProject></PropertyGroup></Project>`)
	write("Lib/Lib.csproj", `<Project></Project>`)
	write("S.sln", "Microsoft Visual Studio Solution File\r\n"+
		`Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "A.Tests", "A.Tests\A.Tests.csproj", "{1}"`+"\r\n"+
		`Project("{2150E333-8FDC-42A3-9474-1A3956D46DE8}") = "Folder", "Folder", "{2}"`+"\r\n"+
		`Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "Lib", "Lib\Lib.csproj", "{3}"`+"\r\n")
	write("S.slnx", `<Solution><Folder Name="/t/"><Project Path="B.Tests/B.Tests.csproj" /></Folder><Project Path="Lib/Lib.csproj" /><Project Path="A.Tests/A.Tests.csproj" /></Solution>`)

	got, err := Projects(filepath.Join(dir, "S.sln"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || filepath.Base(got[0]) != "A.Tests.csproj" {
		t.Errorf("sln projects = %v", got)
	}
	got, err = Projects(filepath.Join(dir, "S.slnx"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || filepath.Base(got[0]) != "A.Tests.csproj" || filepath.Base(got[1]) != "B.Tests.csproj" {
		t.Errorf("slnx projects = %v", got)
	}
	got, err = Projects(filepath.Join(dir, "Lib/Lib.csproj"))
	if err != nil || len(got) != 1 {
		t.Errorf("project = %v %v", got, err)
	}
	if _, err := Projects(filepath.Join(dir, "S.txt")); err == nil {
		t.Error("expected an error for an unknown extension")
	}
	// A solution with no recognisable test projects falls back to all of them.
	write("L.slnx", `<Solution><Project Path="Lib/Lib.csproj" /></Solution>`)
	if got, _ := Projects(filepath.Join(dir, "L.slnx")); len(got) != 1 {
		t.Errorf("fallback projects = %v", got)
	}
}

func TestFindTarget(t *testing.T) {
	dir := t.TempDir()
	touch := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := FindTarget(dir); err == nil {
		t.Fatal("empty directory should be an error")
	}
	touch("Zeta.csproj")
	touch("Alpha.fsproj")
	os.Mkdir(filepath.Join(dir, "Ignored.sln"), 0o755) // a directory, not a file
	got, err := FindTarget(dir)
	if err != nil || filepath.Base(got) != "Alpha.fsproj" {
		t.Errorf("first project: %q %v", got, err)
	}
	touch("Shop.slnx")
	touch("Aardvark.sln")
	got, err = FindTarget(dir)
	if err != nil || filepath.Base(got) != "Aardvark.sln" {
		t.Errorf("solution should win, first by name: %q %v", got, err)
	}
}

func TestLocate(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "bin"), 0o755)
	os.WriteFile(filepath.Join(dir, "bin", "MathTests.cs"), []byte("public class MathTests { public void Adds() {} }"), 0o644)
	src := "namespace Alpha.Tests;\n\npublic class Other {}\n\npublic partial class MathTests\n{\n    [Fact]\n    public void Adds() => Assert.True(true);\n\n    [Theory]\n    public void Theory(int n) {}\n}\n"
	os.WriteFile(filepath.Join(dir, "UnitTest1.cs"), []byte(src), 0o644)
	os.WriteFile(filepath.Join(dir, "More.cs"), []byte("namespace Alpha.Tests;\npublic partial class MathTests\n{\n    public void Extra() {}\n}\n"), 0o644)

	loc, ok := Locate(dir, "Alpha.Tests.MathTests", "Adds")
	if !ok || filepath.Base(loc.File) != "UnitTest1.cs" || loc.Line != 8 {
		t.Errorf("Adds: %+v %v", loc, ok)
	}
	loc, ok = Locate(dir, "Alpha.Tests.MathTests", "Extra")
	if !ok || filepath.Base(loc.File) != "More.cs" || loc.Line != 4 {
		t.Errorf("Extra (partial class): %+v %v", loc, ok)
	}
	// Without a method the first file (in walk order) declaring the class wins.
	loc, ok = Locate(dir, "Alpha.Tests.Outer+MathTests`1", "")
	if !ok || filepath.Base(loc.File) != "More.cs" || loc.Line != 2 {
		t.Errorf("class: %+v %v", loc, ok)
	}
	if _, ok := Locate(dir, "Alpha.Tests.Missing", "Adds"); ok {
		t.Error("Missing should not be found")
	}
}

func TestReadExcerpt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "UnitTest1.cs")
	var src strings.Builder
	for n := 1; n <= 10; n++ {
		fmt.Fprintf(&src, "line %d\n", n)
	}
	os.WriteFile(path, []byte(src.String()), 0o644)

	ex, ok := ReadExcerpt(path, 6, 2, 3)
	if !ok || ex.First != 4 || ex.Focus != 2 || len(ex.Lines) != 6 || ex.Lines[ex.Focus] != "line 6" {
		t.Fatalf("middle of the file: %+v %v", ex, ok)
	}
	// Both ends are clamped, and the focus moves with the start.
	if ex, ok := ReadExcerpt(path, 1, 2, 3); !ok || ex.First != 1 || ex.Focus != 0 || len(ex.Lines) != 4 {
		t.Errorf("first line: %+v %v", ex, ok)
	}
	if ex, ok := ReadExcerpt(path, 10, 2, 3); !ok || ex.First != 8 || ex.Focus != 2 || len(ex.Lines) != 3 {
		t.Errorf("last line: %+v %v", ex, ok)
	}
	// A line past the end of the file, as a stale stack trace names, is a miss.
	if _, ok := ReadExcerpt(path, 11, 2, 3); ok {
		t.Error("line 11 of a 10-line file should not be found")
	}
	if _, ok := ReadExcerpt(filepath.Join(dir, "Gone.cs"), 3, 2, 3); ok {
		t.Error("a missing file should not be found")
	}
	if _, ok := ReadExcerpt(path, 0, 2, 3); ok {
		t.Error("line 0 should not be found")
	}
	// Tabs are expanded so the gutter and the caret line up with the code.
	os.WriteFile(path, []byte("\tif (x)\n\t\tAssert.True(y);\n"), 0o644)
	if ex, ok := ReadExcerpt(path, 2, 1, 0); !ok || ex.Lines[0] != "    if (x)" || ex.Lines[1] != "        Assert.True(y);" {
		t.Errorf("tabs: %q %v", ex.Lines, ok)
	}
}

// A .NET stack trace lists the innermost frame first, so an exception thrown
// below the test points at where it was thrown rather than where the test
// called in. Frames without source information are stepped over.
func TestFailureLocationPicksTheThrowSite(t *testing.T) {
	r := Result{StackTrace: strings.Join([]string{
		"   at System.Linq.ThrowHelper.ThrowNoElementsException()",
		"   at Shop.Core.Pricing.Money.Add(Money other) in /src/Shop.Core/Pricing/Money.cs:line 12",
		"   at Shop.Core.Tests.MoneyTests.Add_Mismatched() in /tests/Shop.Core.Tests/MoneyTests.cs:line 20",
	}, "\n")}
	if loc, ok := r.FailureLocation(); !ok || loc.File != "/src/Shop.Core/Pricing/Money.cs" || loc.Line != 12 {
		t.Errorf("FailureLocation = %+v %v, want the throw site", loc, ok)
	}
	if _, ok := (Result{StackTrace: "   at System.Linq.ThrowHelper.ThrowNoElementsException()"}).FailureLocation(); ok {
		t.Error("a stack trace with no source information has no location")
	}
}

// A run over a solution leaves one results file per test project, so all of
// them are read. This is what keeps a failure in one project from losing its
// message and stack trace to another project finishing later.
func TestReadTRXDir(t *testing.T) {
	dir := t.TempDir()
	if results, err := readTRXDir(dir); results != nil || err != nil {
		t.Errorf("empty directory = %v %v", results, err)
	}
	// The names are the ones dotnet's trx logger picks when it is given no
	// LogFileName, including the suffix it adds to avoid a collision.
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	other := strings.ReplaceAll(sampleTRX, "Alpha.Tests.MathTests", "Beta.Tests.ApiTests")
	write("_Mac_2026-09-19_23_50_33.trx", sampleTRX)
	write("_Mac_2026-09-19_23_50_33[1].trx", other)
	write("notes.txt", "not a results file")

	results, err := readTRXDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 6 {
		t.Fatalf("got %d results from two files, want 6", len(results))
	}
	// Both projects' failures keep the detail the console logger cannot give.
	failed := map[string]Result{}
	for _, r := range results {
		if r.Outcome == OutcomeFailed {
			failed[r.Name] = r
		}
	}
	for _, name := range []string{"Alpha.Tests.MathTests.Fails", "Beta.Tests.ApiTests.Fails"} {
		r, ok := failed[name]
		if !ok {
			t.Fatalf("%s missing from %v", name, failed)
		}
		if r.Message == "" || r.StackTrace == "" {
			t.Errorf("%s has no details: %+v", name, r)
		}
	}
	// A file that will not parse costs only its own results.
	write("broken.trx", "<TestRun><Results>truncated")
	results, err = readTRXDir(dir)
	if err == nil {
		t.Error("the unreadable file should be reported")
	}
	if len(results) != 6 {
		t.Errorf("got %d results alongside a broken file, want the other 6", len(results))
	}
}

// Package dotnet drives the dotnet CLI: it finds test projects, lists their
// tests, runs them while streaming output, and reads the results back.
package dotnet

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// projectExts are the project file extensions dotnet test accepts.
var projectExts = map[string]bool{".csproj": true, ".fsproj": true, ".vbproj": true}

// IsProjectFile reports whether path names a project file.
func IsProjectFile(path string) bool {
	return projectExts[strings.ToLower(filepath.Ext(path))]
}

// IsSolutionFile reports whether path names a solution file (.sln or .slnx).
func IsSolutionFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".sln" || ext == ".slnx"
}

// Projects returns the absolute paths of the test projects reachable from
// path: the project itself when path is a project file, or every test
// project in a solution. When no project in a solution looks like a test
// project every project is returned, so listing still has something to show.
func Projects(path string) ([]string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if IsProjectFile(abs) {
		if _, err := os.Stat(abs); err != nil {
			return nil, err
		}
		return []string{abs}, nil
	}
	if !IsSolutionFile(abs) {
		return nil, fmt.Errorf("%s: not a solution (.sln, .slnx) or project (.csproj, .fsproj, .vbproj) file", path)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	var refs []string
	if strings.EqualFold(filepath.Ext(abs), ".slnx") {
		refs, err = parseSlnx(data)
	} else {
		refs = parseSln(data)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	dir := filepath.Dir(abs)
	var all, tests []string
	for _, ref := range refs {
		p := filepath.Join(dir, filepath.FromSlash(strings.ReplaceAll(ref, `\`, "/")))
		if !IsProjectFile(p) {
			continue
		}
		all = append(all, p)
		if IsTestProject(p) {
			tests = append(tests, p)
		}
	}
	if len(tests) == 0 {
		tests = all
	}
	sort.Strings(tests)
	return tests, nil
}

// slnProject matches a project line of a classic .sln file:
// Project("{GUID}") = "Name", "Path\To\Name.csproj", "{GUID}"
var slnProject = regexp.MustCompile(`^Project\("\{[^}]*\}"\)\s*=\s*"[^"]*",\s*"([^"]+)"`)

// parseSln returns the project paths of a classic solution file, relative to
// the solution directory and as written (usually with backslashes).
func parseSln(data []byte) []string {
	var refs []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		if m := slnProject.FindStringSubmatch(strings.TrimSpace(sc.Text())); m != nil {
			refs = append(refs, m[1])
		}
	}
	return refs
}

// slnxFolder is a <Solution> or <Folder> element of an XML solution file;
// both hold projects and nested folders.
type slnxFolder struct {
	Projects []struct {
		Path string `xml:"Path,attr"`
	} `xml:"Project"`
	Folders []slnxFolder `xml:"Folder"`
}

func (f slnxFolder) paths() []string {
	var refs []string
	for _, p := range f.Projects {
		refs = append(refs, p.Path)
	}
	for _, sub := range f.Folders {
		refs = append(refs, sub.paths()...)
	}
	return refs
}

// parseSlnx returns the project paths of an XML solution file.
func parseSlnx(data []byte) ([]string, error) {
	var root slnxFolder
	if err := xml.Unmarshal(stripBOM(data), &root); err != nil {
		return nil, fmt.Errorf("parsing slnx: %w", err)
	}
	return root.paths(), nil
}

// testMarkers are strings whose presence in a project file mark it as a test
// project: the VSTest SDK, the newer testing platform, or a test framework.
var testMarkers = []string{
	"microsoft.net.test.sdk",
	"microsoft.testing.platform",
	"<istestproject>true",
	`include="xunit`,
	`include="nunit"`,
	`include="mstest`,
	`include="tunit`,
}

// IsTestProject reports whether the project file at path looks like a test
// project. It reads the file only; properties inherited from
// Directory.Build.props are not seen.
func IsTestProject(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := strings.ToLower(string(data))
	for _, marker := range testMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func stripBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
}

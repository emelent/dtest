package dotnet

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Location is a position in a source file.
type Location struct {
	File string
	Line int // 1-based; 0 when unknown
}

// sourceExts are the source files searched for test declarations.
var sourceExts = map[string]bool{".cs": true, ".fs": true, ".vb": true}

// skipDirs are never searched.
var skipDirs = map[string]bool{"bin": true, "obj": true, "node_modules": true, ".git": true}

// Locate finds the declaration of a test in the sources under dir: the line
// of the method when method is not empty, otherwise the line of the class.
// class is the fully qualified class name; nested ("Outer+Inner") and
// generic ("Name`1") forms are reduced to the innermost simple name.
func Locate(dir, class, method string) (Location, bool) {
	simple := simpleClassName(class)
	if simple == "" {
		return Location{}, false
	}
	classRe := regexp.MustCompile(`\b(class|record|struct|interface|type|module)\s+` + regexp.QuoteMeta(simple) + `\b`)
	var methodRe *regexp.Regexp
	if method != "" {
		methodRe = regexp.MustCompile(`\b` + regexp.QuoteMeta(method) + `\s*[(<]`)
	}
	var classOnly Location
	found := false
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] || (strings.HasPrefix(d.Name(), ".") && path != dir) {
				return filepath.SkipDir
			}
			return nil
		}
		if !sourceExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || !classRe.Match(data) {
			return nil
		}
		classLine, methodLine := 0, 0
		sc := bufio.NewScanner(bytes.NewReader(data))
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for n := 1; sc.Scan(); n++ {
			line := sc.Bytes()
			if classLine == 0 && classRe.Match(line) {
				classLine = n
			}
			if methodRe != nil && methodLine == 0 && classLine > 0 && methodRe.Match(line) {
				methodLine = n
				break
			}
		}
		if methodRe == nil && classLine > 0 {
			classOnly, found = Location{File: path, Line: classLine}, true
			return filepath.SkipAll
		}
		if methodLine > 0 {
			classOnly, found = Location{File: path, Line: methodLine}, true
			return filepath.SkipAll
		}
		if classOnly.File == "" && classLine > 0 {
			classOnly = Location{File: path, Line: classLine} // partial class: keep looking for the method
		}
		return nil
	})
	if found {
		return classOnly, true
	}
	return classOnly, classOnly.File != ""
}

// simpleClassName reduces "Ns.Outer+Inner`1" to "Inner".
func simpleClassName(class string) string {
	if i := strings.LastIndexAny(class, ".+"); i >= 0 {
		class = class[i+1:]
	}
	if i := strings.IndexAny(class, "`<"); i >= 0 {
		class = class[:i]
	}
	return class
}

// Excerpt is a run of consecutive source lines around a position, as a code
// frame draws them.
type Excerpt struct {
	First int      // 1-based number of Lines[0]
	Focus int      // index into Lines of the line the excerpt is centred on
	Lines []string // the source, tabs expanded
}

// ReadExcerpt reads the lines of file around line: above of them before it
// and below after, clamped to the start and the end of the file. It reports
// false when the file cannot be read or is shorter than line, which is what
// a stack trace from an older build looks like.
func ReadExcerpt(file string, line, above, below int) (Excerpt, bool) {
	if line < 1 {
		return Excerpt{}, false
	}
	f, err := os.Open(file)
	if err != nil {
		return Excerpt{}, false
	}
	defer f.Close()
	first, last := max(1, line-above), line+below
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for n := 1; n <= last && sc.Scan(); n++ {
		if n >= first {
			lines = append(lines, expandTabs(sc.Text()))
		}
	}
	// The focus line itself has to be there; a frame around nothing would
	// point at whatever happens to be at the end of the file.
	if line-first >= len(lines) {
		return Excerpt{}, false
	}
	return Excerpt{First: first, Focus: line - first, Lines: lines}, true
}

// tabWidth is what a tab is expanded to. The frame is drawn into a fixed
// gutter and its caret counts columns, so a tab left in the text would put
// the two out of step with each other.
const tabWidth = 4

func expandTabs(s string) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r != '\t' {
			b.WriteRune(r)
			col++
			continue
		}
		pad := tabWidth - col%tabWidth
		b.WriteString(strings.Repeat(" ", pad))
		col += pad
	}
	return b.String()
}

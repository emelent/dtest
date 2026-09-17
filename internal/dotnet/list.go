package dotnet

import (
	"context"
	"fmt"
	"strings"
)

// Options configure how dotnet is invoked.
type Options struct {
	Configuration string // -c value; empty for the project default
	NoBuild       bool   // pass --no-build
}

func (o Options) args() []string {
	var args []string
	if o.Configuration != "" {
		args = append(args, "-c", o.Configuration)
	}
	if o.NoBuild {
		args = append(args, "--no-build")
	}
	return args
}

// ListTests runs `dotnet test --list-tests` for project and returns the test
// names it reports, in the display form used by the console logger and the
// TRX file (theory rows carry their arguments, e.g. "Ns.Class.Method(n: 1)").
func ListTests(ctx context.Context, project string, opts Options) ([]string, error) {
	args := append([]string{"test", project, "--list-tests", "--nologo"}, opts.args()...)
	out, err := command(ctx, "dotnet", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("dotnet test --list-tests %s: %s", project, lastLine(out, err))
	}
	return ParseList(string(out)), nil
}

// listHeader precedes the test names in --list-tests output.
const listHeader = "The following Tests are available:"

// ParseList extracts test names from --list-tests output: the indented lines
// that follow the header, up to the next line that is not indented.
func ParseList(out string) []string {
	var names []string
	in := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, listHeader):
			in = true
		case !in:
		case strings.TrimSpace(line) == "":
		case line[0] == ' ' || line[0] == '\t':
			names = append(names, strings.TrimSpace(line))
		default:
			in = false
		}
	}
	return names
}

// lastLine returns the last non-empty line of out, or err's text when there
// is none, so a failed command is summarised in one line.
func lastLine(out []byte, err error) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(lines[i]); s != "" {
			return s
		}
	}
	return err.Error()
}

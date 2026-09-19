package dotnet

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Outcome is the result of one test case.
type Outcome int

const (
	OutcomeUnknown Outcome = iota
	OutcomePassed
	OutcomeFailed
	OutcomeSkipped
)

// String returns the outcome as the console logger prints it.
func (o Outcome) String() string {
	switch o {
	case OutcomePassed:
		return "Passed"
	case OutcomeFailed:
		return "Failed"
	case OutcomeSkipped:
		return "Skipped"
	}
	return "Unknown"
}

// Result is the outcome of one test case.
type Result struct {
	Name       string // display name, as --list-tests prints it
	Outcome    Outcome
	Duration   time.Duration
	Message    string // failure message
	StackTrace string
	Output     string // captured standard output
}

// FailureLocation returns the source position of the first stack frame that
// has one, in the "at X in /path/file.cs:line N" form of .NET stack traces.
func (r Result) FailureLocation() (Location, bool) {
	if m := stackLocation.FindStringSubmatch(r.StackTrace); m != nil {
		line, _ := strconv.Atoi(m[2])
		return Location{File: m[1], Line: line}, true
	}
	return Location{}, false
}

var stackLocation = regexp.MustCompile(` in (.+?):line (\d+)`)

// Event is something that happened during a run: a [LineEvent],
// a [ResultEvent] or, last of all, a [DoneEvent].
type Event interface{ isEvent() }

// LineEvent is one line of dotnet's output.
type LineEvent struct{ Text string }

// ResultEvent reports a test finishing, parsed from the console logger as
// the run progresses. The final results come with DoneEvent.
type ResultEvent struct{ Result Result }

// DoneEvent ends a run. Results are the outcomes read from the TRX files —
// one per test project, so a solution leaves several — and are nil when
// dotnet wrote none, for example after a build failure.
// Err is set when the process could not run or was cancelled; a run with
// failing tests exits non-zero but that alone is not reported as an error.
type DoneEvent struct {
	Results []Result
	Err     error
}

func (LineEvent) isEvent()   {}
func (ResultEvent) isEvent() {}
func (DoneEvent) isEvent()   {}

// command builds the dotnet commands; tests replace it.
var command = exec.CommandContext

// Run executes the tests of project that match filter (every test when
// filter is empty), calling emit for each event in order and finishing with
// a DoneEvent. Cancelling ctx kills dotnet and its test hosts.
func Run(ctx context.Context, project, filter string, opts Options, emit func(Event)) {
	emit(run(ctx, project, filter, opts, emit))
}

func run(ctx context.Context, project, filter string, opts Options, emit func(Event)) DoneEvent {
	dir, err := os.MkdirTemp("", "dtest-")
	if err != nil {
		return DoneEvent{Err: err}
	}
	defer os.RemoveAll(dir)
	args := []string{
		"test", project, "--nologo",
		"--logger", "console;verbosity=normal",
		// Deliberately no LogFileName: a run over a solution is one dotnet
		// invocation covering every test project, and they all write into
		// this directory, so a name of our choosing has each project
		// overwrite the last. Only the project that finished last would keep
		// its results, and every failure in the others would be left with
		// what the console logger gives — a name and an outcome, no message
		// and no stack trace, so no details and no code frame. The logger's
		// own names are unique, and readTRXDir reads all of them.
		"--logger", "trx",
		"--results-directory", dir,
	}
	if filter != "" {
		args = append(args, "--filter", filter)
	}
	args = append(args, opts.args()...)
	emit(LineEvent{Text: "$ dotnet " + strings.Join(args, " ")})

	cmd := command(ctx, "dotnet", args...)
	setProcessGroup(cmd)
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		return DoneEvent{Err: fmt.Errorf("starting dotnet: %w", err)}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		readLines(pr, func(line string) {
			emit(LineEvent{Text: line})
			if r, ok := ParseResultLine(line); ok {
				emit(ResultEvent{Result: r})
			}
		})
	}()
	waitErr := cmd.Wait()
	pw.Close()
	<-done

	results, trxErr := readTRXDir(dir)
	if ctx.Err() != nil {
		return DoneEvent{Results: results, Err: fmt.Errorf("run cancelled")}
	}
	if results == nil && waitErr != nil {
		// No results and a non-zero exit: the build or the host failed.
		return DoneEvent{Err: fmt.Errorf("dotnet test failed: %v", firstError(waitErr, trxErr))}
	}
	return DoneEvent{Results: results}
}

// readLines calls fn for every line of r until it ends. Unlike a Scanner it
// has no line-length limit, so a pathological line cannot stop the reader
// and leave dotnet blocked on a full pipe.
func readLines(r io.Reader, fn func(string)) {
	br := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			fn(strings.TrimRight(line, "\r\n"))
		}
		if err != nil {
			return
		}
	}
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// resultLine matches the console logger's per-test lines at normal
// verbosity, e.g. "  Passed Ns.Class.Method(n: 1) [12 ms]".
var resultLine = regexp.MustCompile(`^\s+(Passed|Failed|Skipped)\s+(.+?)(?:\s+\[([^\]]+)\])?\s*$`)

// ParseResultLine parses one console logger line into a Result. The second
// return value is false for any other line.
func ParseResultLine(line string) (Result, bool) {
	m := resultLine.FindStringSubmatch(line)
	if m == nil {
		return Result{}, false
	}
	r := Result{Name: m[2], Duration: parseConsoleDuration(m[3])}
	switch m[1] {
	case "Passed":
		r.Outcome = OutcomePassed
	case "Failed":
		r.Outcome = OutcomeFailed
	case "Skipped":
		r.Outcome = OutcomeSkipped
	}
	return r, true
}

// parseConsoleDuration reads durations like "12 ms", "< 1 ms", "1 s" or
// "1 m 2 s". Anything unreadable is zero.
func parseConsoleDuration(s string) time.Duration {
	fields := strings.Fields(strings.ReplaceAll(s, "<", ""))
	var total time.Duration
	for i := 0; i+1 < len(fields); i += 2 {
		n, err := strconv.ParseFloat(strings.ReplaceAll(fields[i], ",", "."), 64)
		if err != nil {
			return 0
		}
		unit, ok := map[string]time.Duration{"ms": time.Millisecond, "s": time.Second, "m": time.Minute, "h": time.Hour}[fields[i+1]]
		if !ok {
			return 0
		}
		total += time.Duration(n * float64(unit))
	}
	return total
}

// Build runs `dotnet build` for path (a solution or project), streaming its
// output to emit.
func Build(ctx context.Context, path string, opts Options, emit func(LineEvent)) error {
	args := []string{"build", path, "--nologo", "-v", "minimal"}
	if opts.Configuration != "" {
		args = append(args, "-c", opts.Configuration)
	}
	emit(LineEvent{Text: "$ dotnet " + strings.Join(args, " ")})
	cmd := command(ctx, "dotnet", args...)
	setProcessGroup(cmd)
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting dotnet: %w", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		readLines(pr, func(line string) { emit(LineEvent{Text: line}) })
	}()
	err := cmd.Wait()
	pw.Close()
	<-done
	if err != nil {
		return fmt.Errorf("dotnet build failed: %w", err)
	}
	return nil
}

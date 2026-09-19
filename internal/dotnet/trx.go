package dotnet

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// trxRun is the part of a Visual Studio test results file (.trx) we read.
type trxRun struct {
	Results struct {
		Items []trxResult `xml:"UnitTestResult"`
	} `xml:"Results"`
}

type trxResult struct {
	TestName string `xml:"testName,attr"`
	Outcome  string `xml:"outcome,attr"`
	Duration string `xml:"duration,attr"`
	Output   struct {
		StdOut    string `xml:"StdOut"`
		StdErr    string `xml:"StdErr"`
		ErrorInfo struct {
			Message    string `xml:"Message"`
			StackTrace string `xml:"StackTrace"`
		} `xml:"ErrorInfo"`
	} `xml:"Output"`
}

// readTRXDir parses every results file dotnet left in dir. A run over a
// solution writes one per test project, so the run's results are all of them
// together. No files at all yields nil results and a nil error, since dotnet
// writes none when the run never started. One unreadable file costs only its
// own results: the rest of the projects still reported, and losing their
// failures' messages to a neighbour's bad XML would be the very thing this
// is here to prevent.
func readTRXDir(dir string) ([]Result, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.trx"))
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	sort.Strings(paths) // so a run reports in the same order twice
	var all []Result
	var firstErr error
	for _, path := range paths {
		results, err := readTRX(path)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		all = append(all, results...)
	}
	return all, firstErr
}

// readTRX parses the results file at path. A missing file yields nil results
// and a nil error, since dotnet writes none when the run never started.
func readTRX(path string) ([]Result, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseTRX(f)
}

// ParseTRX reads the per-test results out of a .trx document.
func ParseTRX(r io.Reader) ([]Result, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var run trxRun
	if err := xml.Unmarshal(stripBOM(data), &run); err != nil {
		return nil, fmt.Errorf("parsing trx: %w", err)
	}
	results := make([]Result, 0, len(run.Results.Items))
	for _, it := range run.Results.Items {
		results = append(results, Result{
			Name:       it.TestName,
			Outcome:    trxOutcome(it.Outcome),
			Duration:   parseTRXDuration(it.Duration),
			Message:    strings.TrimSpace(it.Output.ErrorInfo.Message),
			StackTrace: strings.TrimRight(it.Output.ErrorInfo.StackTrace, "\n "),
			Output:     strings.TrimRight(joinNonEmpty(it.Output.StdOut, it.Output.StdErr), "\n"),
		})
	}
	return results, nil
}

func joinNonEmpty(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "\n")
}

func trxOutcome(s string) Outcome {
	switch s {
	case "Passed":
		return OutcomePassed
	case "Failed", "Error", "Timeout", "Aborted":
		return OutcomeFailed
	case "NotExecuted", "Inconclusive", "NotRunnable":
		return OutcomeSkipped
	}
	return OutcomeUnknown
}

// parseTRXDuration reads the "hh:mm:ss.fffffff" durations of a .trx file.
func parseTRXDuration(s string) time.Duration {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	sec, err3 := strconv.ParseFloat(parts[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec*float64(time.Second))
}

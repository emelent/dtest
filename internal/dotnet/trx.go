package dotnet

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
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

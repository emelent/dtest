package ui

import "charm.land/lipgloss/v2"

// Colours are ANSI palette indexes so they follow the terminal theme.
var (
	colGreen  = lipgloss.Color("2")
	colRed    = lipgloss.Color("1")
	colYellow = lipgloss.Color("3")
	colCyan   = lipgloss.Color("6")
	colGrey   = lipgloss.Color("8")
	colWhite  = lipgloss.Color("15")
	colBlack  = lipgloss.Color("0")
)

var (
	stylePassed   = lipgloss.NewStyle().Foreground(colGreen)
	styleFailed   = lipgloss.NewStyle().Foreground(colRed)
	styleSkipped  = lipgloss.NewStyle().Foreground(colYellow)
	styleRunning  = lipgloss.NewStyle().Foreground(colCyan)
	styleQueued   = lipgloss.NewStyle().Foreground(colGrey)
	styleDim      = lipgloss.NewStyle().Foreground(colGrey)
	styleRule     = lipgloss.NewStyle().Foreground(colGrey).Faint(true)
	styleQuick    = lipgloss.NewStyle().Foreground(colGreen)
	styleSlow     = lipgloss.NewStyle().Foreground(colYellow)
	styleBold     = lipgloss.NewStyle().Bold(true)
	styleKey      = lipgloss.NewStyle().Foreground(colCyan).Bold(true)
	styleCommand  = lipgloss.NewStyle().Foreground(colCyan)
	styleLogError = lipgloss.NewStyle().Foreground(colRed)
	styleExpected = lipgloss.NewStyle().Foreground(colGreen)
	styleActual   = lipgloss.NewStyle().Foreground(colRed)
	styleLocation = lipgloss.NewStyle().Foreground(colCyan)

	// Badges, as vitest draws FAIL.
	badgeFail = lipgloss.NewStyle().Background(colRed).Foreground(colWhite).Bold(true).Padding(0, 1)
	badgeInfo = lipgloss.NewStyle().Background(colCyan).Foreground(colBlack).Bold(true).Padding(0, 1)
)

// Cursor line backgrounds, as raw SGR codes so they can be re-applied after
// the resets inside a styled line: 256-colour greys, brighter for the
// focused pane.
const (
	bgFocused   = "\x1b[48;5;237m"
	bgUnfocused = "\x1b[48;5;235m"
)

// Status glyphs, as vitest uses them, plus one for queued tests.
const (
	iconPassed  = "✓"
	iconFailed  = "×"
	iconSkipped = "↓"
	iconNone    = "·"
	iconQueued  = "⧗" // an hourglass: scheduled in a run that has not started
	iconArrow   = "❯" // marks a source location line in the log
	iconOpen    = "▾" // an expanded project, class or theory
	iconClosed  = "▸" // a collapsed one
	rule        = "⎯"
)

// Tree guides, drawn down the left of the tree so the nesting reads at a
// glance. Each level below the project costs one three-column step.
const (
	guideBranch = "├─ " // a child with more siblings under the same parent
	guideLast   = "└─ " // the last child at its level
	guideBar    = "│  " // a level that carries on past this row
	guideGap    = "   " // one that has ended
)

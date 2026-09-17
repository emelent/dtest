package ui

import "charm.land/lipgloss/v2"

// Colours are ANSI palette indexes so they follow the terminal theme.
var (
	colGreen  = lipgloss.Color("2")
	colRed    = lipgloss.Color("1")
	colYellow = lipgloss.Color("3")
	colBlue   = lipgloss.Color("4")
	colCyan   = lipgloss.Color("6")
	colGrey   = lipgloss.Color("8")
	colBar    = lipgloss.Color("236")
	colCursor = lipgloss.Color("238")
)

var (
	stylePassed   = lipgloss.NewStyle().Foreground(colGreen)
	styleFailed   = lipgloss.NewStyle().Foreground(colRed).Bold(true)
	styleSkipped  = lipgloss.NewStyle().Foreground(colYellow)
	styleRunning  = lipgloss.NewStyle().Foreground(colCyan)
	styleDim      = lipgloss.NewStyle().Foreground(colGrey)
	styleMark     = lipgloss.NewStyle().Foreground(colYellow).Bold(true)
	styleTitle    = lipgloss.NewStyle().Bold(true)
	styleFocus    = lipgloss.NewStyle().Bold(true).Foreground(colBlue)
	styleHeader   = lipgloss.NewStyle().Background(colBar).Bold(true)
	styleBar      = lipgloss.NewStyle().Background(colBar)
	styleError    = lipgloss.NewStyle().Background(colBar).Foreground(colRed).Bold(true)
	styleCursor   = lipgloss.NewStyle().Background(colCursor).Bold(true)
	styleKey      = lipgloss.NewStyle().Foreground(colCyan).Bold(true)
	styleCommand  = lipgloss.NewStyle().Foreground(colCyan)
	styleLogError = lipgloss.NewStyle().Foreground(colRed)
)

// Status icons shown in the tree.
const (
	iconPassed  = "✓"
	iconFailed  = "✗"
	iconSkipped = "○"
	iconNone    = "·"
	iconMark    = "●"
)

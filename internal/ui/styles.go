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
	styleDim      = lipgloss.NewStyle().Foreground(colGrey)
	styleBold     = lipgloss.NewStyle().Bold(true)
	styleKey      = lipgloss.NewStyle().Foreground(colCyan).Bold(true)
	styleCommand  = lipgloss.NewStyle().Foreground(colCyan)
	styleLogError = lipgloss.NewStyle().Foreground(colRed)
	styleExpected = lipgloss.NewStyle().Foreground(colGreen)
	styleActual   = lipgloss.NewStyle().Foreground(colRed)
	styleLocation = lipgloss.NewStyle().Foreground(colCyan)

	// Badges, as vitest draws RUN / PASS / FAIL.
	badgeRun  = lipgloss.NewStyle().Background(colYellow).Foreground(colBlack).Bold(true).Padding(0, 1)
	badgePass = lipgloss.NewStyle().Background(colGreen).Foreground(colBlack).Bold(true).Padding(0, 1)
	badgeFail = lipgloss.NewStyle().Background(colRed).Foreground(colWhite).Bold(true).Padding(0, 1)
	badgeInfo = lipgloss.NewStyle().Background(colCyan).Foreground(colBlack).Bold(true).Padding(0, 1)
)

// Status glyphs, as vitest uses them.
const (
	iconPassed  = "✓"
	iconFailed  = "×"
	iconSkipped = "↓"
	iconNone    = "·"
	iconArrow   = "❯"
	iconOpen    = "▾" // an expanded project, class or theory
	iconClosed  = "▸" // a collapsed one
	rule        = "⎯"
)

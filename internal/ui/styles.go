package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// The palette. The greys and the cyan are ANSI indexes, so they follow the
// terminal theme. The three outcome colours are not: a theme's own red and
// green are meant to shout, and a screen that is mostly results wants them
// muted, so these are washed 256-colour shades picked to sit at the same
// weight as each other. A config file may replace any of them; see
// [colorRoles] for the names it uses.
var (
	colGreen  color.Color // sage
	colRed    color.Color // brick
	colYellow color.Color // amber
	colCyan   color.Color
	colGrey   color.Color
	colWhite  color.Color
	colBlack  color.Color

	// Cursor line backgrounds, as raw SGR codes so they can be re-applied
	// after the resets inside an already-styled line.
	bgFocused   string // the cursor line of the focused pane
	bgUnfocused string // and of the other one
	bgSelected  string // a run of log lines picked out to copy
)

// The styles, rebuilt by [buildStyles] whenever the palette changes.
var (
	stylePassed   lipgloss.Style
	styleFailed   lipgloss.Style
	styleSkipped  lipgloss.Style
	styleRunning  lipgloss.Style
	styleQueued   lipgloss.Style
	styleDim      lipgloss.Style
	styleRule     lipgloss.Style
	styleQuick    lipgloss.Style
	styleSlow     lipgloss.Style
	styleElapsed  lipgloss.Style
	styleBold     lipgloss.Style
	styleKey      lipgloss.Style
	styleCommand  lipgloss.Style
	styleLogError lipgloss.Style
	styleExpected lipgloss.Style
	styleActual   lipgloss.Style
	styleLocation lipgloss.Style

	// Badges, as vitest draws FAIL.
	badgeFail lipgloss.Style
	badgeInfo lipgloss.Style
)

func init() { resetTheme() }

// resetTheme puts the palette back to its defaults.
func resetTheme() {
	colGreen, colRed, colYellow = lipgloss.Color("108"), lipgloss.Color("131"), lipgloss.Color("137")
	colCyan, colGrey = lipgloss.Color("6"), lipgloss.Color("8")
	colWhite, colBlack = lipgloss.Color("15"), lipgloss.Color("0")
	bgFocused, bgUnfocused, bgSelected = bgSeq("237"), bgSeq("235"), bgSeq("60")
	buildStyles()
}

// buildStyles derives every style from the palette.
func buildStyles() {
	stylePassed = lipgloss.NewStyle().Foreground(colGreen)
	styleFailed = lipgloss.NewStyle().Foreground(colRed)
	styleSkipped = lipgloss.NewStyle().Foreground(colYellow)
	styleRunning = lipgloss.NewStyle().Foreground(colCyan)
	styleQueued = lipgloss.NewStyle().Foreground(colGrey)
	styleDim = lipgloss.NewStyle().Foreground(colGrey)
	styleRule = lipgloss.NewStyle().Foreground(colGrey).Faint(true)
	styleQuick = lipgloss.NewStyle().Foreground(colGreen)
	styleSlow = lipgloss.NewStyle().Foreground(colYellow)
	styleElapsed = lipgloss.NewStyle().Foreground(colCyan).Faint(true)
	styleBold = lipgloss.NewStyle().Bold(true)
	styleKey = lipgloss.NewStyle().Foreground(colCyan).Bold(true)
	styleCommand = lipgloss.NewStyle().Foreground(colCyan)
	styleLogError = lipgloss.NewStyle().Foreground(colRed)
	styleExpected = lipgloss.NewStyle().Foreground(colGreen)
	styleActual = lipgloss.NewStyle().Foreground(colRed)
	styleLocation = lipgloss.NewStyle().Foreground(colCyan)
	badgeFail = lipgloss.NewStyle().Background(colRed).Foreground(colWhite).Bold(true).Padding(0, 1)
	badgeInfo = lipgloss.NewStyle().Background(colCyan).Foreground(colBlack).Bold(true).Padding(0, 1)
	buildLogRules()
}

// Status glyphs, as vitest uses them, plus one for queued tests.
const (
	iconPassed  = "✓"
	iconFailed  = "×"
	iconSkipped = "↓"
	iconNone    = "·"
	iconQueued  = "⧗" // an hourglass: scheduled in a run that has not started
	iconArrow   = "❯" // marks a source location line in the log
	iconFrame   = ">" // marks the failing line of a code frame, as jest does
	iconCaret   = "^" // points at it from the line below
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

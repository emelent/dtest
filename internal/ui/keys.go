package ui

import (
	tea "charm.land/bubbletea/v2"

	"dtest/internal/tree"
)

// handleKey dispatches a key press: help overlay, filter entry, then the
// report keys.
func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		m.Shutdown()
		return m, tea.Quit
	}
	if m.help {
		m.help = false
		return m, nil
	}
	if m.filtering {
		return m.handleFilterKey(msg)
	}
	if key != "g" {
		defer func() { m.pendingG = false }()
	}
	r, _ := m.current()
	n := r.node
	page := m.report.Height()
	switch key {
	case "q":
		m.Shutdown()
		return m, tea.Quit
	case "?":
		m.help = true
		return m, nil

	// Motion.
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "g":
		if m.pendingG {
			m.cursor = 0
			m.pendingG = false
		} else {
			m.pendingG = true
			return m, nil
		}
	case "G", "end":
		m.cursor = len(m.rows) - 1
		m.clampCursor()
	case "ctrl+d", "pgdown":
		m.move(page / 2)
	case "ctrl+u", "pgup":
		m.move(-page / 2)
	case "ctrl+e":
		m.report.ScrollDown(1)
		return m, nil
	case "ctrl+y":
		m.report.ScrollUp(1)
		return m, nil

	// Folding.
	case "h", "left":
		if n == nil || r.failure {
			return m, nil
		}
		if !n.IsLeaf() && n.Expanded && m.query == "" {
			n.Expanded = false
		} else if n.Parent != nil {
			m.refresh()
			m.selectNode(n.Parent)
		}
	case "l", "right":
		if n == nil || r.failure || n.IsLeaf() {
			return m, nil
		}
		if !n.Expanded && m.query == "" {
			n.Expanded = true
		} else if len(n.Children) > 0 {
			m.move(1)
		}
	case "space":
		if n != nil && !r.failure && !n.IsLeaf() && m.query == "" {
			n.Expanded = !n.Expanded
		}

	// Running.
	case "enter", "r":
		if n != nil {
			return m, m.enqueue([]*tree.Node{n})
		}
	case "a":
		return m, m.enqueue(m.tree.Projects)
	case "f":
		return m, m.runFailed()
	case "x":
		return m, m.cancelRun()
	case "n":
		return m, m.nextFailed(true)
	case "N":
		return m, m.nextFailed(false)

	// Other.
	case "o":
		return m, m.openInEditor()
	case "t", "/":
		m.filtering = true
	case "esc":
		m.query = ""
	case "v":
		m.showOutput = !m.showOutput
		if m.showOutput {
			m.refresh()
			m.report.GotoBottom()
			return m, nil
		}
	case "ctrl+r":
		return m, m.reload()
	default:
		return m, nil
	}
	m.refresh()
	return m, nil
}

// handleFilterKey edits the filter; the report narrows as it is typed.
func (m *Model) handleFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.filtering = false
	case "esc":
		m.filtering = false
		m.query = ""
	case "backspace":
		if m.query != "" {
			m.query = m.query[:len(m.query)-1]
		}
	case "down", "ctrl+n", "ctrl+j":
		m.move(1)
	case "up", "ctrl+p", "ctrl+k":
		m.move(-1)
	default:
		// Printable text only; shift is part of typing capitals, other
		// modifiers mean a shortcut.
		if msg.Text != "" && msg.Mod&^tea.ModShift == 0 {
			m.query += msg.Text
		}
	}
	m.refresh()
	return m, nil
}

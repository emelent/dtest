package ui

import (
	tea "charm.land/bubbletea/v2"

	"dtest/internal/tree"
)

// handleKey dispatches a key press: help, filter entry, pane switching,
// then the focused pane's keys, then keys that work everywhere.
func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		m.Shutdown()
		return m, tea.Quit
	}
	if m.help {
		m.help = false
		m.refresh()
		return m, nil
	}
	if m.filtering {
		return m.handleFilterKey(msg)
	}
	if key != "g" {
		defer func() { m.pendingG = false }()
	}

	// Pane focus follows vim directions: the log is above the tree, the
	// stats are to the right of the tree. Terminals often send ctrl+h as
	// backspace.
	switch key {
	case "ctrl+k":
		m.focus = paneLog
	case "ctrl+j":
		if m.focus == paneLog {
			m.focus = paneTree
		}
	case "ctrl+l":
		if m.focus == paneTree {
			m.focus = paneStats
		}
	case "ctrl+h", "backspace":
		if m.focus == paneStats {
			m.focus = paneTree
		}
	default:
		var handled bool
		var cmd tea.Cmd
		switch m.focus {
		case paneTree:
			handled, cmd = m.handleTreeKey(key)
		case paneLog:
			handled, cmd = m.handleLogKey(key)
		}
		if !handled {
			return m.handleCommonKey(key)
		}
		if cmd != nil {
			return m, cmd
		}
	}
	m.refresh()
	return m, nil
}

// handleTreeKey moves through and folds the tree.
func (m *Model) handleTreeKey(key string) (bool, tea.Cmd) {
	n := m.current()
	page := m.treeView.Height()
	switch key {
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "g":
		if m.pendingG {
			m.move(-len(m.rows))
			m.pendingG = false
		} else {
			m.pendingG = true
		}
	case "G", "end":
		m.move(len(m.rows))
	case "ctrl+d", "pgdown":
		m.move(page / 2)
	case "ctrl+u", "pgup":
		m.move(-page / 2)
	case "h", "left":
		if n == nil {
			return true, nil
		}
		if !n.IsLeaf() && n.Expanded && m.query == "" {
			n.Expanded = false
		} else if n.Parent != nil {
			m.selectNode(n.Parent)
		}
	case "l", "right":
		if n == nil || n.IsLeaf() {
			return true, nil
		}
		if !n.Expanded && m.query == "" {
			n.Expanded = true
		} else if len(n.Children) > 0 {
			m.move(1)
		}
	case "space":
		if n != nil && !n.IsLeaf() && m.query == "" {
			n.Expanded = !n.Expanded
		}
	case "enter", "r":
		if n != nil {
			return true, m.enqueue([]*tree.Node{n})
		}
	case "n":
		return true, m.nextFailed(true)
	case "N":
		return true, m.nextFailed(false)
	case "t", "/":
		m.filtering = true
	case "esc":
		m.query = ""
	default:
		return false, nil
	}
	return true, nil
}

// handleLogKey moves between failure entries and scrolls the log.
func (m *Model) handleLogKey(key string) (bool, tea.Cmd) {
	switch key {
	case "j", "down", "n":
		m.moveFailure(1)
	case "k", "up", "N":
		m.moveFailure(-1)
	case "g":
		if m.pendingG {
			m.moveFailure(-len(m.fails))
			m.pendingG = false
		} else {
			m.pendingG = true
		}
	case "G", "end":
		m.moveFailure(len(m.fails))
	case "ctrl+d", "pgdown":
		m.log.HalfPageDown()
		return true, nil
	case "ctrl+u", "pgup":
		m.log.HalfPageUp()
		return true, nil
	case "ctrl+e":
		m.log.ScrollDown(1)
		return true, nil
	case "ctrl+y":
		m.log.ScrollUp(1)
		return true, nil
	case "enter", "r":
		if f := m.currentFailure(); f != nil {
			return true, m.enqueue([]*tree.Node{f})
		}
	default:
		return false, nil
	}
	return true, nil
}

// handleCommonKey covers actions that work from any pane.
func (m *Model) handleCommonKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q":
		m.Shutdown()
		return m, tea.Quit
	case "?":
		m.help = true
	case "a":
		return m, m.enqueue(m.tree.Projects)
	case "f":
		return m, m.runFailed()
	case "x":
		return m, m.cancelRun()
	case "o":
		return m, m.openInEditor()
	case "v":
		m.showOutput = !m.showOutput
		m.refresh()
		if m.showOutput {
			m.log.GotoBottom()
		}
		return m, nil
	case "ctrl+r":
		return m, m.reload()
	}
	m.refresh()
	return m, nil
}

// handleFilterKey edits the tree filter; the tree narrows as it is typed.
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

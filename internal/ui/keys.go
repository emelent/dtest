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
	switch key {
	case "ctrl+j", "ctrl+k":
		// Two panes, so either direction is the other pane.
		if m.focus == paneLog {
			m.focus = paneTree
		} else {
			m.focus = paneLog
		}
	default:
		var handled bool
		var cmd tea.Cmd
		if m.focus == paneTree {
			handled, cmd = m.handleTreeKey(key)
		} else {
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

// handleTreeKey moves through and folds the tree; the log follows the
// selection.
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
	case "t", "/":
		m.filtering = true
	case "esc":
		m.query = ""
	default:
		return false, nil
	}
	return true, nil
}

// handleLogKey scrolls the log with vim motions.
func (m *Model) handleLogKey(key string) (bool, tea.Cmd) {
	switch key {
	case "j", "down", "ctrl+e":
		m.log.ScrollDown(1)
	case "k", "up", "ctrl+y":
		m.log.ScrollUp(1)
	case "g":
		if m.pendingG {
			m.log.GotoTop()
			m.pendingG = false
		} else {
			m.pendingG = true
		}
	case "G", "end":
		m.log.GotoBottom()
	case "ctrl+d", "pgdown":
		m.log.HalfPageDown()
	case "ctrl+u", "pgup":
		m.log.HalfPageUp()
	case "ctrl+f":
		m.log.PageDown()
	case "ctrl+b":
		m.log.PageUp()
	default:
		return false, nil
	}
	return true, nil
}

// handleCommonKey covers actions that work from either pane.
func (m *Model) handleCommonKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q":
		m.Shutdown()
		return m, tea.Quit
	case "?":
		m.help = true
	case "enter", "r":
		if n := m.current(); n != nil {
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

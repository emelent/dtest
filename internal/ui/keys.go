package ui

import (
	tea "charm.land/bubbletea/v2"

	"dtest/internal/tree"
)

// handleKey dispatches a key press: help overlay, search entry, then the
// focused pane, then keys that work anywhere.
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
	if m.searching {
		return m.handleSearchKey(msg)
	}
	if key != "g" {
		defer func() { m.pendingG = false }()
	}
	if m.focus == paneLog {
		if handled, cmd := m.handleLogKey(key); handled {
			return m, cmd
		}
	} else if handled, cmd := m.handleTreeKey(key); handled {
		return m, cmd
	}
	return m.handleCommonKey(key)
}

// handleTreeKey moves through and folds the tree with vim motions.
func (m *Model) handleTreeKey(key string) (bool, tea.Cmd) {
	n := m.current()
	page := m.bodyHeight()
	switch key {
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
			return true, nil
		}
	case "G", "end":
		m.cursor = len(m.rows) - 1
		m.clampCursor()
	case "ctrl+d":
		m.move(page / 2)
	case "ctrl+u":
		m.move(-page / 2)
	case "ctrl+f", "pgdown":
		m.move(page)
	case "ctrl+b", "pgup":
		m.move(-page)
	case "h", "left":
		if n == nil {
			return true, nil
		}
		if !n.IsLeaf() && n.Expanded && m.query == "" {
			n.Expanded = false
			m.rebuildRows()
		} else if i := m.indexOf(n.Parent); i >= 0 {
			m.cursor = i
		}
	case "l", "right":
		if n == nil || n.IsLeaf() {
			return true, nil
		}
		if !n.Expanded && m.query == "" {
			n.Expanded = true
			m.rebuildRows()
		} else if len(n.Children) > 0 {
			m.move(1)
		}
	case "enter":
		if n == nil {
			return true, nil
		}
		if n.IsLeaf() {
			return true, m.enqueue([]*tree.Node{n})
		}
		n.Expanded = !n.Expanded
		m.rebuildRows()
	case "m":
		if n != nil {
			n.Marked = !n.Marked
			m.move(1)
		}
	case "H":
		m.tree.SetExpandedAll(false)
		m.rebuildRows()
	case "L":
		m.tree.SetExpandedAll(true)
		m.rebuildRows()
	case "tab":
		m.focus = paneLog
	case "/":
		m.searching = true
	default:
		return false, nil
	}
	m.refreshLog()
	return true, nil
}

// handleLogKey scrolls the log pane.
func (m *Model) handleLogKey(key string) (bool, tea.Cmd) {
	switch key {
	case "j", "down":
		m.log.ScrollDown(1)
	case "k", "up":
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
	case "ctrl+d":
		m.log.HalfPageDown()
	case "ctrl+u":
		m.log.HalfPageUp()
	case "ctrl+f", "pgdown":
		m.log.PageDown()
	case "ctrl+b", "pgup":
		m.log.PageUp()
	case "h", "left", "tab", "shift+tab", "esc":
		m.focus = paneTree
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
	case "r":
		if marked := m.tree.Marked(); len(marked) > 0 {
			return m, m.enqueue(marked)
		}
		if n := m.current(); n != nil {
			return m, m.enqueue([]*tree.Node{n})
		}
	case "R":
		return m, m.enqueue(m.tree.Projects)
	case "x":
		return m, m.cancelRun()
	case "o":
		return m, m.openInEditor()
	case "f":
		return m, m.nextWithStatus(tree.StatusFailed, true)
	case "F":
		return m, m.nextWithStatus(tree.StatusFailed, false)
	case "s":
		return m, m.nextWithStatus(tree.StatusSkipped, true)
	case "S":
		return m, m.nextWithStatus(tree.StatusSkipped, false)
	case "e":
		return m, m.rerunFailed()
	case "v":
		m.fullLog = !m.fullLog
		m.refreshLog()
	case "u":
		m.tree.ClearMarks()
	case "esc":
		if m.query != "" {
			m.query = ""
			m.rebuildRows()
		} else {
			m.tree.ClearMarks()
		}
	case "ctrl+r":
		return m, m.reload()
	}
	return m, nil
}

// handleSearchKey edits the / query; the tree filters as it is typed.
func (m *Model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.searching = false
	case "esc":
		m.searching = false
		m.query = ""
		m.rebuildRows()
	case "backspace":
		if m.query != "" {
			m.query = m.query[:len(m.query)-1]
			m.rebuildRows()
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
			m.rebuildRows()
		}
	}
	m.refreshLog()
	return m, nil
}

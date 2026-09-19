package ui

import (
	tea "charm.land/bubbletea/v2"

	"dtest/internal/tree"
)

// handleKey turns a key press into an [Action] and dispatches it: help and
// the filter prompt first, since they swallow everything, then the focused
// pane, then the actions that work from either.
func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	act := action(msg.String())
	if act == actForceQuit {
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
	// gg jumps to the top, so the first press only arms the second.
	if act != actTop {
		defer func() { m.pendingG = false }()
	}
	var handled bool
	var cmd tea.Cmd
	if m.focus == paneTree {
		handled, cmd = m.handleTreeAction(act)
	} else {
		handled, cmd = m.handleLogAction(act)
	}
	if !handled {
		return m.handleCommonAction(act)
	}
	if cmd != nil {
		return m, cmd
	}
	m.refresh()
	return m, nil
}

// handleTreeAction moves through and folds the tree; the log follows the
// selection.
func (m *Model) handleTreeAction(act Action) (bool, tea.Cmd) {
	n := m.current()
	page := m.treeView.Height()
	switch act {
	case actDown:
		m.move(1)
	case actUp:
		m.move(-1)
	case actTop:
		if m.pendingG {
			m.move(-len(m.rows))
			m.pendingG = false
		} else {
			m.pendingG = true
		}
	case actBottom:
		m.move(len(m.rows))
	case actHalfDown:
		m.move(page / 2)
	case actHalfUp:
		m.move(-page / 2)
	case actCollapse:
		if n == nil {
			return true, nil
		}
		if !n.IsLeaf() && n.Expanded && !m.filtered() {
			n.Expanded = false
		} else if n.Parent != nil {
			m.selectNode(n.Parent)
		}
	case actExpand:
		if n == nil || n.IsLeaf() {
			return true, nil
		}
		if !n.Expanded && !m.filtered() {
			n.Expanded = true
		} else if len(n.Children) > 0 {
			m.move(1)
		}
	case actExpandAll:
		// A filtered tree shows every match already, so folding is off there.
		if !m.filtered() {
			m.tree.SetExpanded(true)
		}
	case actCollapseAll:
		if !m.filtered() {
			m.tree.SetExpanded(false)
		}
	case actToggleFold:
		if n != nil && !n.IsLeaf() && !m.filtered() {
			n.Expanded = !n.Expanded
		}
	case actFilter:
		m.filtering = true
	case actFocus:
		return true, m.focusNode()
	case actUnfocus:
		return true, m.unfocusNode()
	case actClear:
		m.query = ""
		m.statusFilter = tree.StatusNone
	default:
		return false, nil
	}
	return true, nil
}

// handleLogAction moves the log's cursor line with vim motions, scrolls
// under it, and selects and copies from it.
func (m *Model) handleLogAction(act Action) (bool, tea.Cmd) {
	page := m.log.Height()
	switch act {
	case actDown:
		m.moveLog(1)
	case actUp:
		m.moveLog(-1)
	case actTop:
		if m.pendingG {
			m.moveLog(-len(m.logLines))
			m.pendingG = false
		} else {
			m.pendingG = true
		}
	case actBottom:
		m.moveLog(len(m.logLines))
	case actHalfDown:
		m.moveLog(page / 2)
	case actHalfUp:
		m.moveLog(-page / 2)
	case actPageDown:
		m.moveLog(page)
	case actPageUp:
		m.moveLog(-page)
	case actScrollDown:
		m.log.ScrollDown(1)
	case actScrollUp:
		m.log.ScrollUp(1)
	case actSelect:
		// Linewise, as vim's V is: the motions above extend it from here.
		if m.selAnchor >= 0 {
			m.selAnchor = -1
		} else {
			m.selAnchor = m.logCursor
		}
	case actCopy:
		return true, m.copyLog()
	case actClear:
		if m.selAnchor < 0 {
			return false, nil
		}
		m.selAnchor = -1
	default:
		return false, nil
	}
	return true, nil
}

// handleCommonAction covers what works from either pane.
func (m *Model) handleCommonAction(act Action) (tea.Model, tea.Cmd) {
	switch act {
	case actSwitchPane:
		// Two panes, so either direction is the other pane.
		if m.focus == paneLog {
			m.focus = paneTree
		} else {
			m.focus = paneLog
		}
	case actQuit:
		m.Shutdown()
		return m, tea.Quit
	case actHelp:
		m.help = true
	case actRun:
		if n := m.current(); n != nil {
			return m, m.enqueue([]*tree.Node{n})
		}
	case actRunAll:
		// The solution in one run, or, with the view focused, whatever is
		// in focus: there it stands in for the whole suite.
		return m, m.enqueue([]*tree.Node{m.root()})
	case actShowAll:
		m.query = ""
		m.statusFilter = tree.StatusNone
	case actRunFailed:
		return m, m.runFailed()
	case actOnlyFailed:
		m.toggleStatusFilter(tree.StatusFailed)
	case actOnlySkipped:
		m.toggleStatusFilter(tree.StatusSkipped)
	case actCancel:
		return m, m.cancelRun()
	case actNextFailure:
		return m, m.nextFailed(true)
	case actPrevFailure:
		return m, m.nextFailed(false)
	case actOpenInEditor:
		return m, m.openInEditor()
	case actToggleOutput:
		m.showOutput = !m.showOutput
		m.refresh()
		if m.showOutput {
			m.log.GotoBottom()
		}
		return m, nil
	case actReload:
		return m, m.reload()
	}
	m.refresh()
	return m, nil
}

// handleFilterKey edits the tree filter; the tree narrows as it is typed.
// The prompt reads keys itself rather than through the keymap: while it is
// open almost every key is text, and enter, esc and backspace mean what
// they mean in any prompt, so they are not the config's to move.
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

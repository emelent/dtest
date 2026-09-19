package ui

import (
	"fmt"
	"sort"
	"strings"
)

// An Action is what a key press asks for. Keys are bound to these rather
// than handled directly, so a config file can move them around without the
// handlers knowing anything about which key arrived.
type Action string

const (
	actNone Action = ""

	// Motions, which act on whichever pane has focus.
	actDown     Action = "down"
	actUp       Action = "up"
	actTop      Action = "top" // pressed twice, as vim's gg is
	actBottom   Action = "bottom"
	actHalfDown Action = "half-page-down"
	actHalfUp   Action = "half-page-up"
	actPageDown Action = "page-down"
	actPageUp   Action = "page-up"

	// The tree.
	actExpand      Action = "expand"
	actCollapse    Action = "collapse"
	actExpandAll   Action = "expand-all"
	actCollapseAll Action = "collapse-all"
	actToggleFold  Action = "toggle-fold"
	actFilter      Action = "filter"

	// The log.
	actScrollDown Action = "scroll-down"
	actScrollUp   Action = "scroll-up"
	actSelect     Action = "select"
	actCopy       Action = "copy"

	// Either pane.
	actSwitchPane   Action = "switch-pane"
	actRun          Action = "run"
	actRunAll       Action = "run-all"
	actRunFailed    Action = "run-failed"
	actOnlyFailed   Action = "only-failed"
	actOnlySkipped  Action = "only-skipped"
	actClear        Action = "clear" // back out of a filter or a selection
	actShowAll      Action = "show-all"
	actCancel       Action = "cancel"
	actNextFailure  Action = "next-failure"
	actPrevFailure  Action = "previous-failure"
	actOpenInEditor Action = "open-in-editor"
	actToggleOutput Action = "toggle-output"
	actReload       Action = "reload"
	actHelp         Action = "help"
	actQuit         Action = "quit"
	actForceQuit    Action = "force-quit"
)

// defaultKeys binds every action to the keys it answers to out of the box.
// A config file replaces an action's whole list, so rebinding one action
// never disturbs another.
var defaultKeys = map[Action][]string{
	actDown:     {"j", "down"},
	actUp:       {"k", "up"},
	actTop:      {"g"},
	actBottom:   {"G", "end"},
	actHalfDown: {"ctrl+d", "pgdown"},
	actHalfUp:   {"ctrl+u", "pgup"},
	actPageDown: {"ctrl+f"},
	actPageUp:   {"ctrl+b"},

	actExpand:      {"l", "right"},
	actCollapse:    {"h", "left"},
	actExpandAll:   {"L"},
	actCollapseAll: {"H"},
	actToggleFold:  {"space"},
	actFilter:      {"t", "/"},

	actScrollDown: {"ctrl+e"},
	actScrollUp:   {"ctrl+y"},
	actSelect:     {"V"},
	actCopy:       {"y"},

	actSwitchPane:   {"ctrl+j", "ctrl+k", "tab", "shift+tab"},
	actRun:          {"enter", "r"},
	actRunAll:       {"A"},
	actRunFailed:    {"F"},
	actOnlyFailed:   {"f"},
	actOnlySkipped:  {"s"},
	actClear:        {"esc"},
	actShowAll:      {"a"},
	actCancel:       {"x"},
	actNextFailure:  {"n"},
	actPrevFailure:  {"N"},
	actOpenInEditor: {"o"},
	actToggleOutput: {"v"},
	actReload:       {"ctrl+r"},
	actHelp:         {"?"},
	actQuit:         {"q"},
	actForceQuit:    {"ctrl+c"},
}

// bindings is what each action answers to, in the order it was bound, and
// keymap is the reverse lookup a key press goes through. Both are package
// variables because the bindings are a property of the program rather than
// of a particular model.
var (
	bindings map[Action][]string
	keymap   map[string]Action
)

func init() { resetKeys() }

// resetKeys puts the bindings back to their defaults.
func resetKeys() {
	built, err := buildKeymap(defaultKeys)
	if err != nil {
		panic("dtest: the default keys clash: " + err.Error()) // a bug, caught by its test
	}
	bindings, keymap = defaultKeys, built
}

// buildKeymap inverts action-to-keys into the key-to-action lookup, and
// refuses a key that two actions both claim.
func buildKeymap(binds map[Action][]string) (map[string]Action, error) {
	built := map[string]Action{}
	for _, act := range sortedActions(binds) {
		for _, k := range binds[act] {
			if k == "" {
				return nil, fmt.Errorf("keys.%s: empty key name", act)
			}
			if other, taken := built[k]; taken {
				return nil, fmt.Errorf("keys.%s: %q is already bound to %s", act, k, other)
			}
			built[k] = act
		}
	}
	return built, nil
}

// ApplyKeys rebinds actions from a config's [keys] table. An action's list
// replaces its default outright, and binding it to nothing unbinds it. An
// unknown action name, or a key already claimed by another action, is an
// error and nothing is changed, so a half-applied keymap cannot leave the
// program unusable.
func ApplyKeys(binds map[string][]string) error {
	if len(binds) == 0 {
		return nil
	}
	next := map[Action][]string{}
	for act, keys := range defaultKeys {
		next[act] = keys
	}
	for _, name := range sortedKeys(binds) {
		act := Action(name)
		if _, ok := defaultKeys[act]; !ok {
			return fmt.Errorf("keys.%s: no such action, try one of %s", name, strings.Join(actionNames(), ", "))
		}
		next[act] = binds[name]
	}
	// Build the lookup first, so a clash is caught before anything changes.
	built, err := buildKeymap(next)
	if err != nil {
		return err
	}
	bindings, keymap = next, built
	return nil
}

// action is what key asks for, or actNone when nothing is bound to it.
func action(key string) Action { return keymap[key] }

// keyFor is the first key bound to an action, for the help screen, or a
// dash when the config has unbound it.
func keyFor(act Action) string {
	if keys := bindings[act]; len(keys) > 0 {
		return keys[0]
	}
	return "–"
}

// keysFor names every key bound to an action, "j / down".
func keysFor(acts ...Action) string {
	var keys []string
	for _, act := range acts {
		keys = append(keys, keyFor(act))
	}
	return strings.Join(keys, " / ")
}

func actionNames() []string {
	names := make([]string, 0, len(defaultKeys))
	for act := range defaultKeys {
		names = append(names, string(act))
	}
	sort.Strings(names)
	return names
}

func sortedActions(m map[Action][]string) []Action {
	out := make([]Action, 0, len(m))
	for act := range m {
		out = append(out, act)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

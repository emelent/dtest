# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A terminal UI for running .NET tests, written in Go on bubbletea v2, laid out
in the style of vitest. It drives the `dotnet` CLI: builds once, lists every
test, runs whatever you select, and shows results in a tree over a log pane.
`spec.txt` holds the original brief; `README.md` documents the finished
behaviour and is the place to update when behaviour changes.

## Commands

```bash
make build            # go build -o dtest, with the version stamped in
make test             # go test ./...
make fmt              # gofmt -l -w .
make vet              # go vet ./...
make install          # build, then sudo mv to /usr/local/bin
make clean            # remove the binary and dist/
make build-all        # cross-compile into dist/ for the release platforms

go test ./internal/ui -run TestRunFlow -v          # one test
go test ./internal/ui -run 'TestFocus'             # a group, by prefix
./dtest sample/Shop.slnx                           # a sample solution lives in sample/
DTEST_CONFIG=/path/config.toml ./dtest sample/Shop.slnx
```

CI (`.github/workflows/ci.yml`) gates on exactly four things, in order:
`gofmt -l .` must print nothing, then `go vet ./...`, `go test ./...`,
`go build ./...`. Run all four before calling a change done.

Running the real app needs the `dotnet` SDK; the tests do not, since every
external call is stubbed.

## Architecture

Five packages. The dependency direction is one way: `ui` → `tree`, `dotnet`,
`editor`; `main` → `config`, `ui`, `dotnet`.

- **`internal/dotnet`** wraps the CLI. Finds solutions and projects, lists
  tests (`dotnet test --list-tests`), runs them, and parses results twice
  over: from the console logger line by line as they happen
  (`ParseResultLine`) and from the TRX files when the run ends — plural,
  because a solution-wide run has every test project write its own into the
  results directory, so `Run` passes the trx logger no `LogFileName` (a name
  of ours is shared, and each project overwrites the last) and `readTRXDir`
  reads the lot. Only the TRX carries a failure's message and stack trace,
  so losing one costs that project its failure details. A run emits
  `LineEvent`, `ResultEvent` and finally `DoneEvent` through a callback.
  `source.go` greps the sources to locate a test's declaration for the
  editor hand-off.
- **`internal/tree`** is the node model: **solution root → project → class →
  method → case**. The root is a real node (`KindRoot`) standing for the
  solution file, so `Node.Project()` returns nil for it and callers must
  handle that. Status, counts and duration roll up from the leaves.
  `Visible`/`VisibleFrom` flatten the tree into the rows to draw, honouring
  folds and filters.
- **`internal/ui`** is the bubbletea model and everything drawn.
- **`internal/editor`** hands a file and line to a running Neovim over its
  socket, or launches one in the terminal.
- **`internal/config`** reads the optional TOML file that rebinds keys and
  repaints colours.

### How a run works

`enqueue` turns selected nodes into `runRequest`s and `pump` starts them one
at a time; everything queued since the last idle moment is one **batch**,
which is what the footer reports. Running the root is special: it is one
`dotnet test` over the whole solution rather than one per project, so results
come back mixed and `tree.LeafIn` files each under the project that listed
it, falling back to a name-prefix match for a test that was never listed.

dotnet work happens in goroutines that feed a channel; `waitMsg` turns the
channel into a `tea.Cmd`. Output lines are **not** drawn as they arrive —
`spinner.TickMsg` calls `refresh()` while `m.busy()`, so a burst of lines
costs one rebuild rather than one each.

### Rendering

`refresh()` rebuilds both panes into viewports. `treeSig`/`logSig` are
joined-content fingerprints that skip `SetContentLines` when nothing changed,
so touching them matters for flicker. The log keeps `logLines` (ANSI-stripped,
one per logical line) and `logStarts` (first display row of each, after
wrapping) — the mouse and the selection both map through `logStarts`.

The screen is fixed rows: header, log title, log body, tree title, tree body,
two-line footer. `layout()` divides what is left between the panes;
`logTop()`, `treeTop()` and `paneAt()` map a mouse Y back to a pane and a row.

### Keys go through actions

`internal/ui/actions.go` maps key → `Action` (`"run-all"`, `"collapse"`, …),
and the handlers in `keys.go` switch on the action, never on the key. Adding
a binding means adding an `Action`, a default in `defaultKeys`, a case in the
right handler, and a `helpRow`. The help screen looks bindings up when it
draws, so never hardcode a key in user-facing text — use `keyFor(act)`.

The filter prompt is deliberately outside the keymap: while it is open almost
every key is literal text.

### Package-level state, and the trap it sets

The keymap (`bindings`, `keymap`) and the whole palette (colours, styles,
background escapes) are package variables, set by `init()` via `resetKeys()`
and `resetTheme()`, and replaced at startup by `ApplyKeys`/`ApplyColors`.

**Anything that captures a style must be built inside `buildStyles()`.** A
package-level `var` holding a style is initialised before `init()` runs and
will silently hold a zero style — this already broke the log-colouring rules
once, hence `buildLogRules()`.

Tests that call `ApplyKeys` or `ApplyColors` must `t.Cleanup(resetKeys)` or
`t.Cleanup(resetTheme)`, since the state outlives the test.

### Visual conventions

Worth preserving when touching the drawing code, because several of them were
asked for specifically:

- The three outcome colours are washed 256-colour shades, not the terminal's
  red/green/yellow. The tree draws every colour a shade back (`inTree`) so the
  footer is the line that carries.
- Durations follow vitest: green under 300ms (`slowDuration`), yellow over,
  with the unit a faded shade of the number's colour. `treeDuration` is the
  dimmer variant.
- The tree root shows only what the solution holds; results and times belong
  to the rows below it. A group's log shows its failures, not its tally,
  which the row it was selected from already carries.
- Exactly one spinner, on the running project. The log never animates.

## Testing

`internal/ui` is tested through the real model, not through mocks of it. The
externals are package variables (`runTests`, `listTests`, `locate`, `now`, …)
that tests swap out; `stubRun` and `finishRun` drive a fake run, and `now` is
stubbed to make times deterministic.

Helpers worth knowing before writing a new test: `newTestModel` (140×40, one
project, **expanded** — projects are collapsed by default in the real app),
`press`, `view` (ANSI-stripped), `treeRows`, `footerRow`, `clipboardOf`.

Assert against styles rather than escape sequences: `styleFailed.Render("x")`,
not `"\x1b[31mx"`. Repainting the palette is a normal change and should not
break tests that are about layout or behaviour.

## Writing code here

`spec.txt` asks for Effective Go. Beyond that, the existing code has a strong
comment convention worth matching: comments explain **why**, not what, and
carry the reasoning behind a choice that looks arbitrary (why a project is
never auto-folded, why the timer is wall-clock, why the filter prompt bypasses
the keymap). Doc comments on exported and unexported identifiers alike.

Update `README.md` alongside behaviour changes — it documents the layout, the
keys table, the config format and the colour rules, and it drifts quickly.

## Other agent configs

This machine has an OpenAI Codex config (`~/.codex/config.toml`) and a Gemini
CLI config (`~/.gemini/settings.json`). If you want their MCP servers, slash
commands, subagents or instructions available here, reply `/import` to scan
and list what is importable, then `/import --yes=<digest>` with the digest the
scan prints. If `/import` is not available on this surface, run
`claude import` from a terminal instead.

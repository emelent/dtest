# dtest

Run a .NET solution's tests from the terminal, with a UI inspired by the JS
test runners — vitest and jest. `dtest` is a Go TUI built on
[bubbletea v2](https://github.com/charmbracelet/bubbletea) that drives the
`dotnet` CLI: it builds once, lists every test, and runs whatever you select
while the screen updates live.

```
  DTEST  Shop                                                                                       press ? for help
⎯⎯ Log  Shop.Api.Tests ⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯
  × Shop.Api.Tests (16 tests | 1 failed | 1 skipped) 1.1s

   FAIL  Shop.Api.Tests › Middleware.RateLimitMiddlewareTests › OverLimit_Returns429 0.001s

    Assert.Equal() Failure: Values differ
    Expected: 429
    Actual:   428
    ❯ tests/Shop.Api.Tests/Middleware/RateLimitMiddlewareTests.cs:12

      10 │         var app = Build(limit: 1);
      11 │         await app.Get("/orders");
    > 12 │         Assert.Equal(429, (await app.Get("/orders")).Status);
         │         ^
      13 │     }
      14 │
      15 │     [Fact]
⎯⎯ Tests ⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯
  ▾ Shop (2 projects | 49 tests)
  ├─ ▾ Shop.Api.Tests (16 tests | 1 failed | 1 skipped) 1.1s
  │  ├─ ▸ Controllers.OrdersControllerTests (3 tests) 0.305s
  │  ├─ ▸ Integration.CheckoutFlowTests (2 tests | 1 skipped) 0.803s
  │  ├─ ▾ Middleware.RateLimitMiddlewareTests (2 tests | 1 failed) 0.002s
  │  │  ├─ × OverLimit_Returns429 0.001s
  │  │  └─ ✓ UnderLimit_Passes 0.001s
  │  └─ ▸ Middleware.AuthMiddlewareTests (5 tests) 0.004s
  └─ ▸ Shop.Core.Tests (33 tests | 1 skipped) 1.2s
 Ran 49 tests in 2.3s at 21:36:37
 1 failed | 46 passed | 2 skipped
```

## Install

Needs Go 1.25+ to build, the `dotnet` SDK on your PATH with test projects on
the VSTest runner (`Microsoft.NET.Test.Sdk` with xUnit, NUnit or MSTest), and
`nvim` for `o`.

```sh
make build           # ./dtest
make install         # /usr/local/bin/dtest (uses sudo)
```

## Usage

```
dtest [flags] [Solution.sln | Solution.slnx | Project.csproj]
```

Without a file, dtest uses a solution file in the current directory, or else
the first project file there (both in name order). It builds once on start and
lists each test project with `dotnet test --list-tests`.

| Flag | Description |
| --- | --- |
| `-c, --configuration name` | Build configuration passed to dotnet |
| `--no-build` | Never build; list and run against existing binaries |
| `--nvim-socket path` | Neovim server socket for `o`; defaults to `$nvim_sock`. Without a listening server, `o` opens nvim in the terminal |
| `-V, --version` | Print the version |

Test projects are recognised by a `Microsoft.NET.Test.Sdk`,
`Microsoft.Testing.Platform`, xUnit, NUnit, MSTest or TUnit reference, or
`<IsTestProject>true</IsTestProject>`; when a solution has none of those, every
project is listed.

## Layout

Two panes, switched with `ctrl+j` / `ctrl+k` or `tab` / `shift+tab`. The mouse
works in both: a click focuses a pane and moves its cursor, clicking a selected
tree row folds it, and the wheel scrolls whatever it is over without taking
focus.

**Log** (top, about 70% of the height) shows what the tree selects — for a
test its verdict, message, failing location, the source around it, stack trace
and captured output; for a group every failure beneath it, without the tally
its row already carries. Build errors come first and long lines wrap.
Selections copy over OSC 52, so they reach the clipboard of the machine running
the terminal even across `ssh`.

**Tests** (bottom) is the tree: a root standing for the solution, then
projects, classes, methods and theory rows, with counts and durations rolled
up. Projects start collapsed. After a run, passing classes fold and failing
ones open — a project is never folded shut under you. `i` makes the selected
project or class the root, and from then on the app behaves as though its tests
were the only ones, down to what `A` runs and where `n` looks; `I` steps back
out one level.

**Footer**, the last two lines: `Ran 12 tests in 0.6s at 21:36:37` over
`1 failed | 11 passed | 0 skipped`, counted in live and held in one shape from
the first result to the last, so the lines settle rather than change form when
the batch ends. The timer is wall clock, so it keeps moving through a slow
test. Status messages take the top line while there is something to say.

## Keys

Press `?` in the app for this list, which shows whatever keys are bound. All of
them can be changed; see [Config](#config).

| Key | Action |
| --- | --- |
| `ctrl+j` / `ctrl+k`, `tab` | Switch between the log and the tree (a click in a pane does too) |
| `j` / `k`, `↓` / `↑` | Move through the tree, or the lines of the log |
| `gg` / `G` | Top / bottom |
| `ctrl+d` / `ctrl+u` | Half page |
| `ctrl+e` / `ctrl+y` | Scroll the log without moving its cursor |
| `V` then `y` | Select lines of the log with the motions above and copy them; `esc` drops the selection |
| `y` | Copy the log line under the cursor |
| `l` / `h` | Expand / collapse a project, class or theory (`h` on a collapsed node selects its parent) |
| `L` / `H` | Expand / collapse the whole tree |
| `i` / `I` | Focus on the selected project or class, treating its tests as the only ones; step back out |
| `space` | Toggle a fold |
| `enter`, `r` | Run the selected node; on the root that is the whole solution in one `dotnet test` |
| `A` | Run the whole solution, or whatever is in focus, the same as running the root |
| `F` | Re-run only the failed tests |
| `f` / `s` | Show only the failed / skipped tests |
| `a` | Show all tests again (`esc` does too) |
| `x` | Cancel the running tests and drop the queue |
| `n` / `N` | Next / previous failed test |
| `o` | Open in Neovim: the stack frame under the log cursor, else a failed test's failing line, else the declaration |
| `t`, `/` | Filter the tree by test or project name; `enter` keeps the filter, `esc` clears every filter |
| `v` | Show the raw dotnet output of the selected project instead of its results |
| `ctrl+r` | Rebuild and list the tests again |
| `q`, `ctrl+c` | Quit |

## How it behaves

- **Runs.** `A` and the root run the whole solution in one `dotnet test`;
  anything smaller is one per project, and runs queue up. The filters are
  `FullyQualifiedName~Ns.Class.` and `FullyQualifiedName=Ns.Class.Method`. A
  theory row cannot be addressed on its own, so running one runs its method.
- **Results** come from the console logger as they land and from the TRX files
  at the end, so a test added since the last reload still appears. Only the TRX
  carries a failure's message and stack trace, so a solution run lets the logger
  name the files and reads them all — one name of ours would have each project
  overwrite the last.
- **While running**, the project row spins, everything beneath it turns cyan,
  and tests waiting in a queued run sit behind an hourglass, ⧗.
- **Failures** are headed by a `FAIL` line and breadcrumb with the rest indented
  under it, so a group's log reads as blocks rather than a wall. One that throws
  is described by its exception: type in bold, message after, inner exception
  under a dimmed `----`.
- **Code frames** follow every failure: six lines of source around the offending
  line in a number gutter, the failing line marked `>` at full weight with a
  caret under the start of its statement, since a .NET trace names a line but
  never a column. On a throw the frame lands on the deepest frame with a file
  and line. It is left out when the source cannot be read.
- **Durations** are `0.032s`, `1.5s`, `35s`, then `2m34s`; green up to 300ms and
  yellow beyond, with the unit a faded shade of the number's colour. A group
  shows the sum of its tests.
- **Colours** are washed shades rather than the terminal's red, green and
  yellow, drawn a shade back again in the tree and with grey counts, so the
  footer is the line that carries.

## Config

Keys and colours come from a TOML file: `$DTEST_CONFIG` if it is set, else
`dtest/config.toml` under `$XDG_CONFIG_HOME` or `~/.config`, and `--config
path` overrides both. No file, a missing table and a missing entry all just
keep the defaults.

```toml
[keys]
# An action's list replaces its default outright, so rebinding one never
# disturbs another. One key or several; an empty list unbinds the action.
quit      = ["q", "Q"]
run-all   = "R"
copy      = "y"
expand-all = []

[colors]
# Either a 256-colour index or a #rrggbb hex.
failed    = "#d78787"
passed    = "108"
skipped   = "137"
running   = "6"
dim       = "8"
selection = "60"
```

The actions are `down`, `up`, `top`, `bottom`, `half-page-down`,
`half-page-up`, `page-down`, `page-up`, `expand`, `collapse`, `expand-all`,
`collapse-all`, `toggle-fold`, `filter`, `scroll-down`, `scroll-up`, `select`,
`copy`, `switch-pane`, `run`, `run-all`, `run-failed`, `only-failed`,
`only-skipped`, `clear`, `show-all`, `cancel`, `focus`, `unfocus`,
`next-failure`, `previous-failure`, `open-in-editor`, `toggle-output`,
`reload`, `help`, `quit` and `force-quit`. Key names are the ones bubbletea
reports: a letter, or `enter`, `esc`, `space`, `tab`, `up`, `pgdown`, `ctrl+d`,
`shift+tab` and so on. `top` is pressed twice, the way `gg` is. The filter
prompt is not configurable: while it is open almost every key is text.

The colour roles are `passed`, `failed`, `skipped`, `running`, `dim` and
`badge` for text, and `cursor`, `cursor-unfocused` and `selection` for the
backgrounds painted behind a whole line. An unknown action or colour, or one
key two actions both want, stops dtest at startup with the offending line;
nothing is half-applied.

## Neovim

`o` opens the stack frame under the log cursor, else a failed test's failing
line, else the selected node's declaration, found by scanning the project's
sources. It goes to the Neovim listening on the socket named by `nvim_sock`
(`NVIM_SOCK` works too) or `--nvim-socket`, as in [ghpr](../ghpr):

```sh
export nvim_sock=/tmp/nvim.shop.sock
nvim --listen "$nvim_sock"
dtest sample/Shop.slnx        # in another pane; o now sends files to that Neovim
```

dtest hands `--remote-expr` an `edit +LINE` and then, inside tmux, selects the
window named `code`. With nothing listening, `nvim +LINE file` opens in the
dtest terminal and dtest resumes when you quit it. macOS caps socket paths at
about 100 characters, so keep the socket under `/tmp`.

## Try it

`sample/` holds a solution to play with: nine libraries and ten xUnit projects,
about 155 tests over several folders (namespaces) each, with theories, six
deliberate failures, five skipped tests and a few slow ones. Six of the ten
projects are green, so the tree has a mix to look at.

```sh
make build
./dtest sample/Shop.slnx
cd sample && ../dtest        # or let it find Shop.slnx itself
```

## Development

```sh
make test            # go test ./...
make fmt vet         # gofmt and go vet
make build-all       # cross-compile into dist/
```

CI gates on gofmt leaving nothing to format, then `go vet`, the tests and a
build.

**Commit messages drive releases.** Every push to `main` works the next version
out of the Conventional Commit messages since the last `v*` tag, then tags,
cross-compiles for Linux, macOS and Windows and publishes a GitHub release with
archives and checksums. `feat!:` or a `BREAKING CHANGE` footer bumps the major
version, `feat:` the minor, `fix:` / `perf:` / `refactor:` / `revert:` and
unprefixed messages the patch; `chore:`, `docs:`, `ci:`, `test:`, `style:` and
`build:` alone release nothing. Preview the next tag with
`.github/scripts/next-version.sh`.

```
main.go              flags, target discovery, tea.NewProgram
internal/dotnet      solution/project discovery, --list-tests, streamed runs, TRX parsing, source lookup, filters
internal/tree        solution → project → class → method → case nodes, status roll-up, durations, folding, visible rows
internal/editor      Neovim hand-off (--remote-expr + tmux) and in-terminal launch
internal/ui          the bubbletea model: two panes, actions and keymap, palette, summary and status, output colouring
internal/config      the TOML file that rebinds keys and repaints colours
```

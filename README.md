# dtest

Run the tests of a .NET solution or project from the terminal, with a UI in
the spirit of vitest. `dtest` is a Go TUI built on
[bubbletea v2](https://github.com/charmbracelet/bubbletea) that drives the
`dotnet` CLI: it builds once, lists every test, and runs whatever you select
while the screen updates live.

```
  DTEST  Shop
⎯⎯ Log  Shop.Api.Tests ⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯
  × Shop.Api.Tests (16 tests | 1 failed | 1 skipped) 1.1s

   FAIL  Shop.Api.Tests › Middleware.RateLimitMiddlewareTests › OverLimit_Returns429 0.001s
Assert.Equal() Failure: Values differ
Expected: 429
Actual:   428
 ❯ tests/Shop.Api.Tests/Middleware/RateLimitMiddlewareTests.cs:12



⎯⎯ Tests ⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯
  ▾ All Tests (49 tests)
  ├─ ▾ Shop.Api.Tests (16 tests | 1 failed | 1 skipped) 1.1s               Test Projects   1 passed (2)
  │  ├─ ▸ Controllers.OrdersControllerTests (3 tests) 0.305s               Tests           49
  │  ├─ ▸ Integration.CheckoutFlowTests (2 tests | 1 skipped) 0.803s       Failed          1
  │  ├─ ▾ Middleware.RateLimitMiddlewareTests (2 tests | 1 failed) 0.002s  Skipped         2
  │  │  ├─ × OverLimit_Returns429 0.001s                                   Passed          46
  │  │  └─ ✓ UnderLimit_Passes 0.001s                                      Start at        21:36:37
  │  └─ ▸ Middleware.AuthMiddlewareTests (5 tests) 0.004s                  Duration        2.3s
  └─ ▾ Shop.Core.Tests (33 tests | 1 skipped) 1.2s
     ├─ ▸ Inventory.StockTests (8 tests) 0.201s
     └─ ▸ Pricing.MoneyTests (12 tests) 0.400s                             press ? to show help, press q to quit
```

## Layout

Two panes; `ctrl+j` and `ctrl+k` switch between them, wrapping around, and
so do `tab` and `shift+tab` for terminals or tmux setups that reserve
ctrl+j/k for their own pane movement.

- **Log** (top, about 70% of the height) shows the results of whatever the
  tree selects. For a test: its verdict, message (expected values green,
  actual values red), failing location, stack trace and captured output.
  For a project, class or theory: its passed / failed / skipped tally and
  every failure beneath it.
  Build errors show first. Long lines wrap. With the log focused, `j`/`k`,
  `gg`/`G` and `ctrl+d`/`ctrl+u` move a cursor line through it (the title
  shows its position, `12/80`), and `ctrl+e`/`ctrl+y` scroll without moving
  it. `v` swaps in the raw `dotnet` output of the selected project,
  coloured by kind.
- **Tests** (bottom) is the tree of projects, classes, methods and theory
  rows under a single **All Tests** root, drawn with branch lines (`├─`,
  `└─`, `│`) so the nesting reads at a glance, with vitest-style counts and
  durations on every group, and the stats along its right-hand side, labels
  down one edge and values in a column. The root's own row just says how
  many tests there are; selecting it shows the whole solution at once, its
  rolled-up tally and every failure under it, and running it runs every
  project. The stats come in two halves. At the top of the pane sits what
  dtest is doing (building, listing, or a status message) and how many test
  projects the solution holds, failures left out since the run summary below
  is where they go. The total number of tests is not repeated there: the
  root of the tree carries it. Hanging from the bottom is the last batch of
  runs: how many tests it covered, how many of those failed, were skipped
  and passed, when it started and how long its tests took, then the key
  hint. So running one class reports that class, and the totals above stay
  put. The stats hide unless the tree keeps at least 65% of the width. After
  a run, passing classes fold to one line and failing ones open.

## Features

- Run the selected root, project, class, method or test; `A` runs everything, as does running the root, `F` re-runs only what failed; runs queue up, one `dotnet test` per project
- Live status while tests run: the project row spins and everything running beneath it turns cyan, tests waiting in a queued run are greyed out behind an hourglass, ⧗; ✓ × ↓ arrive as each result comes in; projects, classes and theories carry a fold arrow (▾ / ▸) in the colour of their status, with counts and durations rolled up
- Open the selected test in Neovim, either a running instance (via the socket in `$nvim_sock`) or one launched in place; a failed test opens at the failing line
- Filter the tree by test or project name as you type, or narrow it to the failed (`f`) or skipped (`s`) tests

## Requirements

- Go 1.25 or newer to build
- The `dotnet` SDK on your PATH, with test projects using the VSTest runner (`Microsoft.NET.Test.Sdk` with xUnit, NUnit or MSTest)
- `nvim` for `o`

## Install

```sh
make build           # ./dtest
make install         # /usr/local/bin/dtest (uses sudo)
```

## Usage

```
dtest [flags] [Solution.sln | Solution.slnx | Project.csproj]
```

Without a file, dtest uses a solution file in the current directory, or
else the first project file there (both in name order).

| Flag | Description |
| --- | --- |
| `-c, --configuration name` | Build configuration passed to dotnet |
| `--no-build` | Never build; list and run against existing binaries |
| `--nvim-socket path` | Neovim server socket for `o`; defaults to `$nvim_sock`. Without a listening server, `o` opens nvim in the terminal |
| `-V, --version` | Print the version |

On start the solution is built once and each test project is listed with
`dotnet test --list-tests`. Test projects are recognised by a
`Microsoft.NET.Test.Sdk`, `Microsoft.Testing.Platform`, xUnit, NUnit,
MSTest or TUnit reference, or `<IsTestProject>true</IsTestProject>`; when a
solution has none of those every project is listed.

## Keys

Press `?` in the app for this list.

| Key | Action |
| --- | --- |
| `ctrl+j` / `ctrl+k`, `tab` | Switch between the log and the tree |
| `j` / `k`, `↓` / `↑` | Move through the tree, or the lines of the log |
| `gg` / `G` | Top / bottom |
| `ctrl+d` / `ctrl+u` | Half page |
| `ctrl+e` / `ctrl+y` | Scroll the log without moving its cursor |
| `l` / `h` | Expand / collapse a project, class or theory (`h` on a collapsed node selects its parent) |
| `L` / `H` | Expand / collapse the whole tree |
| `space` | Toggle a fold |
| `enter`, `r` | Run the selected project, class or test |
| `A` | Run every project |
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

Runs use `dotnet test --filter`: `FullyQualifiedName~Ns.Class.` for a class
and `FullyQualifiedName=Ns.Class.Method` for a method. A theory row cannot
be addressed on its own, so running one runs its method. Results come from
the console logger as they happen and from a TRX file when the run ends, so
a test that was not listed (added since the last reload) still appears.

Durations are `0.032s` below a second, `1.5s` below ten, `35s` below a
minute, then `2m34s`. They are coloured as vitest colours them: green up to
300ms, yellow beyond it, so slow tests stand out, with the unit in a faded
shade of the number's own colour. A group shows the sum of its tests' times,
and so does the summary's Duration, over the last batch of runs. Neither
counts the time spent outside the tests themselves.

## Neovim

`o` opens the file and line named by the log line under the cursor when it
is a stack frame (so an exception thrown deep in the code under test opens
there), else a failed test at the failing line from its stack trace, else
the selected node at its declaration, found by scanning the project's
source files. Either way the location is sent to
the Neovim listening on the socket named by the `nvim_sock` environment
variable (`NVIM_SOCK` works too) or by `--nvim-socket`. Start Neovim
listening on the socket, as in [ghpr](../ghpr):

```sh
export nvim_sock=/tmp/nvim.shop.sock
nvim --listen "$nvim_sock"
dtest sample/Shop.slnx        # in another pane; o now sends files to that Neovim
```

`o` runs `nvim --server <socket> --remote-expr "execute('edit +LINE ' . fnameescape('/path/file.cs'))"`,
then, inside tmux, selects the window named `code` in the current session.
When `nvim_sock` is not set, or nothing is listening on it, `nvim +LINE file`
opens in the dtest terminal and dtest resumes when you quit it. macOS limits
socket paths to about 100 characters, so keep the socket under `/tmp`.

## Try it

`sample/` holds a small solution to play with: a library and two xUnit
projects whose tests are spread over several folders (namespaces), with
theories, three deliberate failures, two skipped tests and a few slow tests.

```sh
make build
./dtest sample/Shop.slnx
cd sample && ../dtest        # or let it find Shop.slnx itself
```

## Development

```sh
make test            # go test ./...
make fmt vet         # gofmt and go vet
make build-all       # cross-compile into dist/ for the platforms the release publishes
```

Pull requests run `.github/workflows/ci.yml`: gofmt must leave nothing to
format, and `go vet`, the tests and a build must pass.

**Commit messages drive releases.** Every push to `main` runs
`.github/workflows/release.yml`, which tests, works out the next version
from the Conventional Commit messages since the last `v*` tag, tags,
cross-compiles for Linux, macOS and Windows and publishes a GitHub release
with archives and checksums. `feat!:` or a `BREAKING CHANGE` footer bumps
the major version, `feat:` the minor, `fix:` / `perf:` / `refactor:` /
`revert:` and unprefixed messages the patch; `chore:`, `docs:`, `ci:`,
`test:`, `style:` and `build:` alone release nothing. Preview the next tag
with `.github/scripts/next-version.sh`.

```
main.go              flags, target discovery, tea.NewProgram
internal/dotnet      solution/project discovery, --list-tests, streamed runs, TRX parsing, source lookup, filters
internal/tree        root → project → class → method → case nodes, status roll-up, durations, folding, visible rows
internal/editor      Neovim hand-off (--remote-expr + tmux) and in-terminal launch
internal/ui          the bubbletea model: three panes, keys, summary and status, output colouring
```

# dtest

Run the tests of a .NET solution or project from the terminal, with a UI in
the spirit of vitest. `dtest` is a Go TUI built on
[bubbletea v2](https://github.com/charmbracelet/bubbletea) that drives the
`dotnet` CLI: it builds once, lists every test, and runs whatever you select
while the screen updates live.

```
  DTEST  Shop
⎯⎯ Failed Tests 1 ⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯
❯  FAIL  Shop.Api.Tests › Middleware.RateLimitMiddlewareTests › OverLimit_Returns429
Assert.Equal() Failure: Values differ
Expected: 429
Actual:   428
 ❯ tests/Shop.Api.Tests/Middleware/RateLimitMiddlewareTests.cs:12



⎯⎯ Tests ⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯│⎯⎯ Summary ⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯
❯ × Shop.Api.Tests (16 tests | 1 failed | 1 skipped) 1.1s  │ Test Projects  1 failed | 1 passed (2)
    ✓ Controllers.OrdersControllerTests (3 tests) 0.305s   │         Tests  1 failed | 46 passed | 2 skipped (49)
    ✓ Integration.CheckoutFlowTests (2 tests | 1 skipped)  │      Start at  21:21:48
    × Middleware.RateLimitMiddlewareTests (2 tests | 1 fai…│      Duration  4.0s (tests 2.4s)
      × OverLimit_Returns429 0.001s                        │
      ✓ UnderLimit_Passes 0.002s                           │  FAIL  Tests failed.
  ✓ Shop.Core.Tests (33 tests | 1 skipped) 1.2s            │       press ? to show help, press q to quit
```

## Layout

Three panes, switched with vim directions on ctrl: `ctrl+k` up to the log,
`ctrl+j` down to the tree, `ctrl+l` right to the summary, `ctrl+h` back left.

- **Log** (top, about 70% of the height): the minimal test log. Each failed
  test with its message (expected values green, actual values red) and the
  source location from its stack trace, plus any build errors. `v` swaps in
  the raw `dotnet` output, coloured by kind.
- **Tests** (bottom left): the tree of projects, classes, methods and theory
  rows, with vitest-style counts and durations on every group. After a run,
  passing classes fold to one line and failing ones open.
- **Summary** (bottom right): vitest's closing block of projects, tests,
  start time and duration, then the `RUN` / `PASS` / `FAIL` state and the
  key hint.

## Features

- Run the selected project, class, method or test; `a` runs everything, `f` re-runs only what failed; runs queue up, one `dotnet test` per project
- Live status while tests run: a spinner on the running tests, ✓ × ↓ as each result comes in, counts and durations rolled up to every parent
- Open the selected test in Neovim, either a running instance (via the socket in `$nvim_sock`) or one launched in place; from the log pane, the failing line
- Filter the tree by test or project name as you type

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
| `ctrl+k` / `ctrl+j` | Focus the log above / the tree below |
| `ctrl+h` / `ctrl+l` | Focus the tree / the summary |
| `j` / `k`, `↓` / `↑` | Move; in the log, between failures (the tree follows) |
| `gg` / `G` | First / last entry |
| `ctrl+d` / `ctrl+u` | Half page |
| `ctrl+e` / `ctrl+y` | Scroll the log without moving its selection |
| `l` / `h` | Expand / collapse a project, class or theory (`h` on a collapsed node selects its parent) |
| `space` | Toggle a fold |
| `enter`, `r` | Run the selected project, class or test |
| `a` | Run every project |
| `f` | Re-run only the failed tests |
| `x` | Cancel the running tests and drop the queue |
| `n` / `N` | Next / previous failed test |
| `o` | Open the selection in Neovim; from the log pane, the failing line |
| `t`, `/` | Filter the tree by test or project name; `enter` keeps the filter, `esc` clears it |
| `v` | Show or hide the raw dotnet output in the log pane |
| `ctrl+r` | Rebuild and list the tests again |
| `q`, `ctrl+c` | Quit |

Runs use `dotnet test --filter`: `FullyQualifiedName~Ns.Class.` for a class
and `FullyQualifiedName=Ns.Class.Method` for a method. A theory row cannot
be addressed on its own, so running one runs its method. Results come from
the console logger as they happen and from a TRX file when the run ends, so
a test that was not listed (added since the last reload) still appears.

Durations are `0.032s` below a second, `1.5s` below ten, `35s` below a
minute, then `2m34s`. A group shows the sum of its tests' times; the
summary's Duration is the wall-clock time of the whole batch of runs.

## Neovim

`o` looks for the class and method declaration in the project's source files
(falling back to the failure position in the stack trace) and sends it to
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
```

```
main.go              flags, target discovery, tea.NewProgram
internal/dotnet      solution/project discovery, --list-tests, streamed runs, TRX parsing, source lookup, filters
internal/tree        project → class → method → case nodes, status roll-up, durations, folding, visible rows
internal/editor      Neovim hand-off (--remote-expr + tmux) and in-terminal launch
internal/ui          the bubbletea model: three panes, keys, summary and status, output colouring
```

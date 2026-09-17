# dtest

Run the tests of a .NET solution or project from a tree in the terminal.
`dtest` is a Go TUI built on [bubbletea v2](https://github.com/charmbracelet/bubbletea)
that drives the `dotnet` CLI: it builds once, lists every test, and runs
whatever part of the tree you select while streaming the output.

```
 dtest  Sample   ✓ 4  ✗ 1  ○ 0   6 tests
 Tests                                    │ Test: Fails
 ▾ ✗ Alpha.Tests                    1.5s  │Alpha.Tests.MathTests.Fails
   ▾ ✗ Alpha.Tests                  1.5s  │
     ▾ ✗ MathTests                 0.004s │✗ Failed in 0.001s
         ✓ Adds                    0.001s │
         ✗ Fails                   0.001s │Message
       ▸ ✓ Theory                  0.002s │Assert.Equal() Failure: Values differ
     ▾ ✓ SlowTests                  1.5s  │Expected: 5
         ✓ Waits                    1.5s  │Actual:   4
```

## Features

- Tree from project down to namespace, class, method and theory row
- Run a project, namespace, class, method or a set of marked nodes; runs queue up, one `dotnet test` per project
- Live status while tests run: a spinner on the running tests, ✓ / ✗ / ○ as each result comes in, counts rolled up to every parent
- Log pane with the streamed `dotnet test` output coloured by kind (results green, red and yellow, errors red, runner chatter dim), or the message, stack trace and captured output of the selected test
- Open the selected test's source in Neovim, either a running instance (via the socket in `$nvim_sock`) or one launched in place
- Vim motions throughout; `/` filters the tree as you type

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
dtest [flags] <Solution.sln | Solution.slnx | Project.csproj>
```

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
| `j` / `k`, `↓` / `↑` | Move |
| `gg` / `G` | First / last row |
| `ctrl+d` / `ctrl+u`, `ctrl+f` / `ctrl+b` | Half page / page |
| `h` / `l` | Collapse (or go to the parent) / expand (or enter) |
| `enter` | Toggle a folder; run a test |
| `H` / `L` | Collapse / expand everything |
| `m` | Mark the node for a run (● in the margin) |
| `u`, `esc` | Clear marks (`esc` clears the filter first) |
| `r` | Run the marked nodes, or the node under the cursor |
| `R` | Run every project |
| `e` | Re-run every test that failed |
| `x` | Cancel the running tests and drop the queue |
| `f` / `F` | Next / previous failed test |
| `s` / `S` | Next / previous skipped test |
| `/` | Filter the tree by name; `enter` keeps the filter, `esc` clears it |
| `o` | Open the test in Neovim |
| `tab` | Focus the log pane (`j`/`k`, `ctrl+d`/`u`, `g`/`G` scroll; `h`, `esc` or `tab` return) |
| `ctrl+r` | Rebuild and list the tests again |
| `q`, `ctrl+c` | Quit |

Runs use `dotnet test --filter`: `FullyQualifiedName~Ns.Class.` for a class
or namespace and `FullyQualifiedName=Ns.Class.Method` for a method. A
theory row cannot be addressed on its own, so running one runs its method.
Results come from the console logger as they happen and from a TRX file
when the run ends, so a test that was not listed (added since the last
reload) still appears.

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

and `o` runs `nvim --server <socket> --remote-expr "execute('edit +LINE ' . fnameescape('/path/file.cs'))"`,
then, inside tmux, selects the window named `code` in the current session.
When `nvim_sock` is not set, or nothing is listening on it, `nvim +LINE file`
opens in the dtest terminal and dtest resumes when you quit it.

## Try it

`sample/` holds a small solution to play with: a library and two xUnit
projects whose tests are spread over several folders (namespaces), with
theories, three deliberate failures, two skipped tests and a few slow tests.

```sh
make build
./dtest sample/Shop.slnx
```

## Development

```sh
make test            # go test ./...
make fmt vet         # gofmt and go vet
```

```
main.go              flags, tea.NewProgram
internal/dotnet      solution/project discovery, --list-tests, streamed runs, TRX parsing, source lookup, filters
internal/tree        project → namespace → class → method → case nodes, status roll-up, visible rows
internal/editor      Neovim hand-off (--remote-expr + tmux) and in-terminal launch
internal/ui          the bubbletea model: tree pane, log/detail pane, keys, rendering
```

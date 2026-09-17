// dtest is a terminal UI for running the tests of a .NET solution or
// project, built on bubbletea and the dotnet CLI.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"dtest/internal/dotnet"
	"dtest/internal/ui"
)

// version is injected at build time via -ldflags "-X main.version=v1.2.3".
var version = "dev"

func usage() {
	fmt.Fprint(os.Stderr, `dtest – run .NET tests from a tree in the terminal

Usage:
  dtest [flags] [Solution.sln | Solution.slnx | Project.csproj]

Without a file, a solution in the current directory is used, or else its
first project file. The solution or project is built once, its test
projects are listed, and the tests appear as project → namespace → class →
method → case. Press ? in the app for the keys.

Flags:
  -c, --configuration name   build configuration passed to dotnet (default: the project's)
  --no-build                 never build; list and run against existing binaries
  --nvim-socket path         Neovim server socket for "o"; defaults to $nvim_sock. Without a
                             listening server, "o" opens nvim in this terminal
  -V, --version              print the version and exit
  -h, --help                 show this help
`)
}

func main() {
	cfg := ui.Config{Version: version}
	var help, showVersion bool
	flag.StringVar(&cfg.Options.Configuration, "c", "", "")
	flag.StringVar(&cfg.Options.Configuration, "configuration", "", "")
	flag.BoolVar(&cfg.Options.NoBuild, "no-build", false, "")
	flag.StringVar(&cfg.Socket, "nvim-socket", "", "")
	flag.BoolVar(&showVersion, "V", false, "")
	flag.BoolVar(&showVersion, "version", false, "")
	flag.BoolVar(&help, "h", false, "")
	flag.BoolVar(&help, "help", false, "")
	flag.Usage = usage
	flag.Parse()
	if help {
		usage()
		return
	}
	if showVersion {
		fmt.Println("dtest " + version)
		return
	}
	switch flag.NArg() {
	case 0:
		target, err := dotnet.FindTarget(".")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, "pass a .sln, .slnx or project file, or run dtest in a directory that has one")
			os.Exit(2)
		}
		cfg.Target = target
	case 1:
		cfg.Target = flag.Arg(0)
	default:
		usage()
		os.Exit(2)
	}
	if !dotnet.IsSolutionFile(cfg.Target) && !dotnet.IsProjectFile(cfg.Target) {
		fmt.Fprintf(os.Stderr, "%s: expected a .sln, .slnx, .csproj, .fsproj or .vbproj file\n", cfg.Target)
		os.Exit(2)
	}
	if _, err := os.Stat(cfg.Target); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	model := ui.New(cfg)
	final, err := tea.NewProgram(model).Run()
	model.Shutdown()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if fm, ok := final.(*ui.Model); ok && fm.Fatal() != nil {
		fmt.Fprintln(os.Stderr, fm.Fatal())
		os.Exit(1)
	}
}

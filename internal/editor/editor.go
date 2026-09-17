// Package editor hands source locations to Neovim: to a running instance
// listening on a socket when there is one (and brings its tmux window
// forward), or by launching nvim in the terminal otherwise.
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// TmuxWindow is the window in the current tmux session that holds the editor.
const TmuxWindow = "code"

// execCommand builds the commands run by Open; tests replace it.
var execCommand = exec.Command

// SocketEnv is the environment variable naming the Neovim server socket;
// its upper-case form is accepted too.
const SocketEnv = "nvim_sock"

// ResolveSocket picks the socket to talk to: the explicit one when given,
// otherwise $nvim_sock (or $NVIM_SOCK). It is empty when neither is set.
func ResolveSocket(explicit string) string {
	for _, s := range []string{explicit, os.Getenv(SocketEnv), os.Getenv(strings.ToUpper(SocketEnv))} {
		if s != "" {
			return s
		}
	}
	return ""
}

// HasServer reports whether a Neovim server socket exists at sock.
func HasServer(sock string) bool {
	if sock == "" {
		return false
	}
	_, err := os.Stat(sock)
	return err == nil
}

// EditExpr is the Vimscript expression evaluated on the server to open path
// at line (1-based; 0 leaves the cursor alone). An expression, unlike keys
// sent with --remote-send, is not subject to the user's mappings and works
// from any mode. fnameescape runs on the server so odd file names are handled.
func EditExpr(path string, line int) string {
	at := ""
	if line > 0 {
		at = fmt.Sprintf("+%d ", line)
	}
	return fmt.Sprintf("execute('edit %s' . fnameescape('%s'))", at, strings.ReplaceAll(path, "'", "''"))
}

// Open asks the Neovim listening on sock to edit path at line, then, when
// running inside tmux, switches the current session to the editor window.
func Open(sock, path string, line int) error {
	if out, err := execCommand("nvim", "--server", sock, "--remote-expr", EditExpr(path, line)).CombinedOutput(); err != nil {
		return fmt.Errorf("nvim --remote-expr: %s", firstLine(out, err))
	}
	if os.Getenv("TMUX") == "" {
		return nil
	}
	// ":name" targets a window in the current session.
	if out, err := execCommand("tmux", "select-window", "-t", ":"+TmuxWindow).CombinedOutput(); err != nil {
		return fmt.Errorf("tmux select-window %s: %s", TmuxWindow, firstLine(out, err))
	}
	return nil
}

// LaunchCmd is the command that opens path at line in a fresh Neovim taking
// over the terminal, for when no server is listening.
func LaunchCmd(path string, line int) *exec.Cmd {
	args := []string{}
	if line > 0 {
		args = append(args, fmt.Sprintf("+%d", line))
	}
	args = append(args, path)
	cmd := execCommand("nvim", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd
}

func firstLine(out []byte, err error) string {
	if s := strings.TrimSpace(string(out)); s != "" {
		s, _, _ = strings.Cut(s, "\n")
		return s
	}
	return err.Error()
}

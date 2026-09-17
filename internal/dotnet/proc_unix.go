//go:build unix

package dotnet

import (
	"os/exec"
	"syscall"
	"time"
)

// setProcessGroup puts the command in its own process group and makes
// cancellation kill the whole group, so the test hosts dotnet spawns die
// with it.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 2 * time.Second
}

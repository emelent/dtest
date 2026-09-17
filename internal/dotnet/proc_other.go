//go:build !unix

package dotnet

import "os/exec"

// setProcessGroup is a no-op where process groups are unavailable; only the
// dotnet process itself is killed on cancellation.
func setProcessGroup(cmd *exec.Cmd) {}

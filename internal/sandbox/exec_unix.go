//go:build !windows

package sandbox

import (
	"bytes"
	"context"
	"os/exec"
	"syscall"
)

// runCommandTree runs a command via sh -c in its own process group; when the
// context is done the whole group is killed.
func runCommandTree(ctx context.Context, dir, cmd string, env []string) Result {
	c := exec.Command("sh", "-c", cmd)
	c.Dir = dir
	c.Env = env
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var buf bytes.Buffer
	c.Stdout = &buf
	c.Stderr = &buf
	if err := c.Start(); err != nil {
		return Result{ExitCode: -1, Err: err}
	}
	pid := c.Process.Pid
	return waitFor(ctx, c, &buf, func() {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	})
}

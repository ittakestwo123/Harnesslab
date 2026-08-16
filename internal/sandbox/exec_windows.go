//go:build windows

package sandbox

import (
	"bytes"
	"context"
	"os"
	"os/exec"
)

// runCommandTree runs a command via a temporary batch file inside a job
// object. When the context is done the job is closed, which terminates the
// whole process tree (the batch file's descendants included).
func runCommandTree(ctx context.Context, dir, cmd string, env []string) Result {
	bat, err := writeBatch(cmd)
	if err != nil {
		return Result{ExitCode: -1, Err: err}
	}
	defer os.Remove(bat)

	var job *windowsJob
	if j, err := newWindowsJob(); err == nil {
		job = j
		defer j.close()
	}

	c := exec.Command("cmd", "/c", bat)
	c.Dir = dir
	c.Env = env
	var buf bytes.Buffer
	c.Stdout = &buf
	c.Stderr = &buf
	if err := c.Start(); err != nil {
		return Result{ExitCode: -1, Err: err}
	}
	if job != nil {
		_ = job.assignByPID(c.Process.Pid)
	}
	return waitFor(ctx, c, &buf, func() {
		if job != nil {
			job.close()
		}
	})
}

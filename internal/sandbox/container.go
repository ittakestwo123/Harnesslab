package sandbox

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

// dockerSandbox runs commands inside a container with the workspace mounted
// read-write at /workspace. Requires a local Docker daemon.
type dockerSandbox struct {
	spec Spec
}

const dockerWorkspace = "/workspace"

func (s *dockerSandbox) Run(ctx context.Context, cmd Command) Result {
	if cmd.Command == "" {
		return Result{ExitCode: -1, Err: fmt.Errorf("sandbox: empty command")}
	}
	if runtime.GOOS == "windows" {
		// Docker Desktop on Windows: the workspace path is the host path;
		// mounting is handled by Docker Desktop's file sharing.
	}
	image := s.spec.Image
	if image == "" {
		image = "alpine:latest"
	}

	args := []string{"run", "--rm"}
	switch s.spec.Network {
	case "none":
		args = append(args, "--network", "none")
	case "allowlist":
		// Host network is the least-surprise default for allowlist mode;
		// real egress filtering needs a userland proxy.
		args = append(args, "--network", "host")
	default: // restricted
		args = append(args, "--network", "none")
	}
	args = append(args,
		"-v", fmt.Sprintf("%s:%s", filepath.Clean(cmd.Dir), dockerWorkspace),
		"-w", dockerWorkspace,
		image,
	)
	// The command runs via sh -c inside the container.
	args = append(args, "sh", "-c", cmd.Command)

	c := exec.CommandContext(ctx, "docker", args...)
	out, err := c.CombinedOutput()
	return Result{ExitCode: exitCode(err), Output: redactSecrets(string(out)), Err: err}
}

func (s *dockerSandbox) Close() error { return nil }

// bwrapSandbox runs commands inside a bubblewrap sandbox (Linux only): the
// workspace is bound read-write, system paths read-only, and the network is
// disabled unless explicitly allowed.
type bwrapSandbox struct {
	spec Spec
}

const bwrapWorkspace = "/workspace"

func (s *bwrapSandbox) Run(ctx context.Context, cmd Command) Result {
	if cmd.Command == "" {
		return Result{ExitCode: -1, Err: fmt.Errorf("sandbox: empty command")}
	}
	timeout := s.spec.Timeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	args := []string{
		"--die-with-parent",
		"--new-session",
		"--unshare-pid",
		"--unshare-uts",
		"--unshare-ipc",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/bin", "/bin",
		"--ro-bind", "/lib", "/lib",
		"--ro-bind", "/lib64", "/lib64",
		"--ro-bind", "/etc", "/etc",
	}
	if s.spec.Network == "none" || s.spec.Network == "restricted" {
		args = append(args, "--unshare-net")
	}
	args = append(args,
		"--bind", filepath.Clean(cmd.Dir), bwrapWorkspace,
		"--chdir", bwrapWorkspace,
		"sh", "-c", cmd.Command,
	)

	c := exec.CommandContext(ctx, "bwrap", args...)
	out, err := c.CombinedOutput()
	return Result{ExitCode: exitCode(err), Output: redactSecrets(string(out)), Err: err}
}

func (s *bwrapSandbox) Close() error { return nil }

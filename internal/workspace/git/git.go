// Package git implements a Workspace backed by git worktrees: a local mirror
// of the repository is cloned once, and each run gets a detached worktree
// registered against the mirror. This is fast (no per-run clone) and keeps
// concurrent runs fully isolated.
package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/log"

	"github.com/ittakestwo123/Harnesslab/internal/workspace"
)

// Workspace manages git-worktree-backed workspaces under a root directory.
type Workspace struct {
	rootDir string
}

// New creates a Workspace whose mirror and worktrees live under rootDir.
func New(rootDir string) *Workspace {
	return &Workspace{rootDir: rootDir}
}

// Create clones (once, into a mirror) and registers a detached worktree for
// the given run id. With an empty Repo it creates a scratch directory instead.
func (w *Workspace) Create(ctx context.Context, id string, s workspace.Spec) (*workspace.Instance, error) {
	inst := &workspace.Instance{ID: id, Repo: s.Repo, Commit: s.Commit}

	if s.Repo == "" {
		dir := filepath.Join(w.rootDir, id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("workspace: create scratch dir: %w", err)
		}
		inst.Root = dir
		return inst, nil
	}

	mirror := filepath.Join(w.rootDir, "mirror.git")
	if err := w.ensureMirror(ctx, mirror, s.Repo); err != nil {
		return nil, err
	}

	wtDir := filepath.Join(w.rootDir, "worktrees", id)
	commit := s.Commit
	if commit == "" {
		commit = "HEAD"
	}
	if err := w.git(ctx, "", "--git-dir", mirror, "worktree", "add", "--detach", wtDir, commit); err != nil {
		return nil, fmt.Errorf("workspace: add worktree %s @ %s: %w", wtDir, commit, err)
	}
	inst.Root = wtDir
	return inst, nil
}

// ensureMirror makes sure a valid bare mirror exists. Creation is guarded by
// a lock file: the holder clones and only removes the lock when done, so
// concurrent workers wait for the lock to disappear before using the mirror
// (a clone-in-progress can look valid before all refs are fetched).
func (w *Workspace) ensureMirror(ctx context.Context, mirror, repo string) error {
	if w.mirrorValid(ctx, mirror) {
		return nil
	}
	if err := os.MkdirAll(w.rootDir, 0o755); err != nil {
		return fmt.Errorf("workspace: mkdir %s: %w", w.rootDir, err)
	}
	lock := mirror + ".lock"
	f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		// Another worker is cloning; wait until it releases the lock.
		deadline := time.Now().Add(2 * time.Minute)
		for {
			if _, statErr := os.Stat(lock); os.IsNotExist(statErr) {
				break
			}
			if ctx.Err() != nil {
				return fmt.Errorf("workspace: wait for mirror: %w", ctx.Err())
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("workspace: timed out waiting for mirror %s", mirror)
			}
			time.Sleep(500 * time.Millisecond)
		}
		if w.mirrorValid(ctx, mirror) {
			return nil
		}
		return fmt.Errorf("workspace: mirror %s missing after lock release", mirror)
	}
	f.Close()
	defer os.Remove(lock)
	// Clone with retries: transient network/proxy failures (e.g.
	// SSL_ERROR_SYSCALL) must not fail the whole benchmark wave.
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		_ = os.RemoveAll(mirror) // clear any partial clone from a failed attempt
		if err := w.git(ctx, "", "clone", "--mirror", repo, mirror); err == nil {
			log.Infof("workspace: mirror ready at %s", mirror)
			return nil
		} else {
			lastErr = err
			if attempt < maxAttempts {
				select {
				case <-time.After(time.Duration(attempt) * 3 * time.Second):
				case <-ctx.Done():
					return fmt.Errorf("workspace: clone mirror %s: %w", repo, ctx.Err())
				}
			}
		}
	}
	return fmt.Errorf("workspace: clone mirror %s after %d attempts: %w", repo, maxAttempts, lastErr)
}

// mirrorValid reports whether mirror is a usable bare git repository.
func (w *Workspace) mirrorValid(ctx context.Context, mirror string) bool {
	if _, err := os.Stat(mirror); err != nil {
		return false
	}
	return w.git(ctx, "", "--git-dir", mirror, "rev-parse", "--is-bare-repository") == nil
}

// Snapshot captures HEAD and the porcelain status of inst.
func (w *Workspace) Snapshot(ctx context.Context, inst *workspace.Instance) (*workspace.Snapshot, error) {
	head, err := w.gitOut(ctx, inst.Root, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("workspace: rev-parse: %w", err)
	}
	status, _ := w.gitOut(ctx, inst.Root, "status", "--porcelain")
	return &workspace.Snapshot{Commit: head, Status: status}, nil
}

// Diff captures the working-tree diff of inst, including untracked files
// (which `git diff` does not cover).
func (w *Workspace) Diff(ctx context.Context, inst *workspace.Instance) (*workspace.Diff, error) {
	patch, err := w.gitOut(ctx, inst.Root, "diff")
	if err != nil {
		return nil, fmt.Errorf("workspace: diff: %w", err)
	}
	stat, err := w.gitOut(ctx, inst.Root, "diff", "--stat")
	if err != nil {
		return nil, fmt.Errorf("workspace: diff --stat: %w", err)
	}
	var untracked []string
	if out, err := w.gitOut(ctx, inst.Root, "ls-files", "--others", "--exclude-standard"); err == nil {
		for _, line := range strings.Split(out, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				untracked = append(untracked, line)
			}
		}
	}
	return &workspace.Diff{Patch: patch, Stat: stat, Untracked: untracked}, nil
}

// Destroy removes the worktree and the run's directory.
func (w *Workspace) Destroy(ctx context.Context, inst *workspace.Instance) error {
	if inst.Root == "" {
		return nil
	}
	if inst.Repo != "" {
		mirror := filepath.Join(w.rootDir, "mirror.git")
		_ = w.git(ctx, "", "--git-dir", mirror, "worktree", "remove", "--force", inst.Root)
	}
	return os.RemoveAll(inst.Root)
}

func (w *Workspace) git(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %v: %w: %s", args, err, trim(out))
	}
	return nil
}

func (w *Workspace) gitOut(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %v: %w", args, err)
	}
	return trim(out), nil
}

func trim(b []byte) string {
	s := string(b)
	if len(s) > 512 {
		return s[:512] + "..."
	}
	return s
}

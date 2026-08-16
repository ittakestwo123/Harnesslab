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
	if _, err := os.Stat(mirror); os.IsNotExist(err) {
		if err := os.MkdirAll(w.rootDir, 0o755); err != nil {
			return nil, fmt.Errorf("workspace: mkdir %s: %w", w.rootDir, err)
		}
		if err := w.git(ctx, "", "clone", "--mirror", s.Repo, mirror); err != nil {
			return nil, fmt.Errorf("workspace: clone mirror %s: %w", s.Repo, err)
		}
		log.Infof("workspace: mirror ready at %s", mirror)
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

// Snapshot captures HEAD and the porcelain status of inst.
func (w *Workspace) Snapshot(ctx context.Context, inst *workspace.Instance) (*workspace.Snapshot, error) {
	head, err := w.gitOut(ctx, inst.Root, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("workspace: rev-parse: %w", err)
	}
	status, _ := w.gitOut(ctx, inst.Root, "status", "--porcelain")
	return &workspace.Snapshot{Commit: head, Status: status}, nil
}

// Diff captures the working-tree diff of inst.
func (w *Workspace) Diff(ctx context.Context, inst *workspace.Instance) (*workspace.Diff, error) {
	patch, err := w.gitOut(ctx, inst.Root, "diff")
	if err != nil {
		return nil, fmt.Errorf("workspace: diff: %w", err)
	}
	stat, err := w.gitOut(ctx, inst.Root, "diff", "--stat")
	if err != nil {
		return nil, fmt.Errorf("workspace: diff --stat: %w", err)
	}
	return &workspace.Diff{Patch: patch, Stat: stat}, nil
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

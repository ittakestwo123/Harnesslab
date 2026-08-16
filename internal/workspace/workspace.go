// Package workspace isolates the repository state each run operates on.
// Stage 1 ships a Git worktree implementation; Docker/container/remote
// workspaces can implement the same interface later.
package workspace

import (
	"context"
	"strings"
)

// Instance is a materialized, isolated workspace for one run.
type Instance struct {
	// ID is the run/workspace id.
	ID string
	// Root is the directory the agent operates in.
	Root string
	// Repo is the source repository (empty for scratch workspaces).
	Repo string
	// Commit is the pinned commit (empty means default branch HEAD).
	Commit string
}

// Spec describes how to create a workspace.
type Spec struct {
	// Repo is a git URL or local path. Empty creates a scratch directory.
	Repo string
	// Commit pins a specific revision. Empty means default branch HEAD.
	Commit string
}

// Snapshot captures the workspace state at a point in time.
type Snapshot struct {
	Commit string
	// Status is `git status --porcelain` output (may be empty).
	Status string
}

// Diff captures the working-tree changes of a workspace.
type Diff struct {
	// Patch is the full working-tree diff.
	Patch string
	// Stat is the diffstat summary.
	Stat string
	// Untracked lists new files created in the workspace (not covered by
	// `git diff`).
	Untracked []string
}

// Changed reports whether the workspace was actually modified: a non-empty
// patch/stat or new untracked files.
func (d *Diff) Changed() bool {
	if d == nil {
		return false
	}
	return strings.TrimSpace(d.Patch) != "" || strings.TrimSpace(d.Stat) != "" || len(d.Untracked) > 0
}

// Workspace creates and manages isolated workspaces.
type Workspace interface {
	// Create materializes a new workspace for id.
	Create(ctx context.Context, id string, s Spec) (*Instance, error)

	// Snapshot captures the current state of inst.
	Snapshot(ctx context.Context, inst *Instance) (*Snapshot, error)

	// Diff captures the working-tree changes of inst.
	Diff(ctx context.Context, inst *Instance) (*Diff, error)

	// Destroy removes inst and its resources.
	Destroy(ctx context.Context, inst *Instance) error
}

// Package sqlitestore implements store.Store on SQLite (pure-Go driver,
// no CGO). It also persists raw run events for future dashboards.
package sqlitestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/ittakestwo123/Harnesslab/internal/store"
)

const schema = `
CREATE TABLE IF NOT EXISTS runs (
	id              TEXT PRIMARY KEY,
	task            TEXT NOT NULL DEFAULT '',
	harness_version TEXT NOT NULL DEFAULT '',
	harness_name    TEXT NOT NULL DEFAULT '',
	repository      TEXT NOT NULL DEFAULT '',
	commit_sha      TEXT NOT NULL DEFAULT '',
	workspace       TEXT NOT NULL DEFAULT '',
	trace_path      TEXT NOT NULL DEFAULT '',
	started_at      INTEGER NOT NULL,
	finished_at     INTEGER NOT NULL DEFAULT 0,
	status          TEXT NOT NULL,
	success         INTEGER NOT NULL DEFAULT 0,
	input_tokens    INTEGER NOT NULL DEFAULT 0,
	output_tokens   INTEGER NOT NULL DEFAULT 0,
	tool_calls      INTEGER NOT NULL DEFAULT 0,
	model_calls     INTEGER NOT NULL DEFAULT 0,
	cost_usd        REAL NOT NULL DEFAULT 0,
	duration_ms     INTEGER NOT NULL DEFAULT 0,
	spec_yaml       TEXT NOT NULL DEFAULT '',
	workspace_patch TEXT NOT NULL DEFAULT '',
	verification_passed INTEGER NOT NULL DEFAULT 0,
	workspace_changed   INTEGER NOT NULL DEFAULT 0,
	verification        TEXT NOT NULL DEFAULT '',
	environment         TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS events (
	id        TEXT NOT NULL,
	run_id    TEXT NOT NULL,
	parent_id TEXT NOT NULL DEFAULT '',
	type      TEXT NOT NULL,
	ts        INTEGER NOT NULL,
	payload   TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (run_id, id)
);
CREATE INDEX IF NOT EXISTS idx_events_run ON events(run_id);
`

// Store persists runs and events in SQLite.
type Store struct {
	db *sql.DB
}

// New opens (creating if needed) a SQLite database at path.
func New(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("sqlitestore: mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: open %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlitestore: ping %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlitestore: schema: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// migrate adds columns that older databases are missing.
func (s *Store) migrate() error {
	cols, err := s.columnNames("runs")
	if err != nil {
		return err
	}
	for _, c := range []struct{ name, typ string }{
		{"spec_yaml", "TEXT NOT NULL DEFAULT ''"},
		{"workspace_patch", "TEXT NOT NULL DEFAULT ''"},
		{"verification_passed", "INTEGER NOT NULL DEFAULT 0"},
		{"workspace_changed", "INTEGER NOT NULL DEFAULT 0"},
		{"verification", "TEXT NOT NULL DEFAULT ''"},
		{"environment", "TEXT NOT NULL DEFAULT ''"},
	} {
		if cols[c.name] {
			continue
		}
		if _, err := s.db.Exec("ALTER TABLE runs ADD COLUMN " + c.name + " " + c.typ); err != nil {
			return fmt.Errorf("sqlitestore: migrate %s: %w", c.name, err)
		}
	}
	return nil
}

func (s *Store) columnNames(table string) (map[string]bool, error) {
	rows, err := s.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: pragma: %w", err)
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var (
			cid     int
			name    string
			typ     string
			notnull int
			dflt    any
			pk      int
		)
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, fmt.Errorf("sqlitestore: scan pragma: %w", err)
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// CreateRun inserts a run.
func (s *Store) CreateRun(ctx context.Context, run *store.Run) error {
	verJSON, _ := json.Marshal(run.Verification)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO runs (id, task, harness_version, harness_name, repository, commit_sha,
			workspace, trace_path, started_at, finished_at, status, success,
			input_tokens, output_tokens, tool_calls, model_calls, cost_usd, duration_ms,
			spec_yaml, workspace_patch, verification_passed, workspace_changed, verification,
			environment)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		run.ID, run.Task, run.HarnessVersion, run.HarnessName, run.Repository, run.Commit,
		run.Workspace, run.TracePath, unixMillis(run.StartedAt), unixMillis(run.FinishedAt),
		string(run.Status), boolInt(run.Metrics.Success), run.Metrics.InputTokens,
		run.Metrics.OutputTokens, run.Metrics.ToolCalls, run.Metrics.ModelCalls,
		run.Metrics.CostUSD, run.Metrics.DurationMS, run.SpecYAML, run.WorkspacePatch,
		boolInt(run.Metrics.VerificationPassed), boolInt(run.Metrics.WorkspaceChanged),
		string(verJSON), run.Environment,
	)
	return wrap(err)
}

// UpdateRun replaces the run row.
func (s *Store) UpdateRun(ctx context.Context, run *store.Run) error {
	verJSON, _ := json.Marshal(run.Verification)
	res, err := s.db.ExecContext(ctx, `
		UPDATE runs SET task=?, harness_version=?, harness_name=?, repository=?, commit_sha=?,
			workspace=?, trace_path=?, started_at=?, finished_at=?, status=?, success=?,
			input_tokens=?, output_tokens=?, tool_calls=?, model_calls=?, cost_usd=?, duration_ms=?,
			spec_yaml=?, workspace_patch=?, verification_passed=?, workspace_changed=?, verification=?,
			environment=?
		WHERE id=?`,
		run.Task, run.HarnessVersion, run.HarnessName, run.Repository, run.Commit,
		run.Workspace, run.TracePath, unixMillis(run.StartedAt), unixMillis(run.FinishedAt),
		string(run.Status), boolInt(run.Metrics.Success), run.Metrics.InputTokens,
		run.Metrics.OutputTokens, run.Metrics.ToolCalls, run.Metrics.ModelCalls,
		run.Metrics.CostUSD, run.Metrics.DurationMS, run.SpecYAML, run.WorkspacePatch,
		boolInt(run.Metrics.VerificationPassed), boolInt(run.Metrics.WorkspaceChanged),
		string(verJSON), run.Environment, run.ID,
	)
	if err != nil {
		return wrap(err)
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return s.CreateRun(ctx, run)
	}
	return wrap(err)
}

// GetRun reads one run.
func (s *Store) GetRun(ctx context.Context, id string) (*store.Run, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, task, harness_version, harness_name, repository, commit_sha, workspace,
			trace_path, started_at, finished_at, status, success, input_tokens,
			output_tokens, tool_calls, model_calls, cost_usd, duration_ms,
			spec_yaml, workspace_patch, verification_passed, workspace_changed, verification,
			environment
		FROM runs WHERE id=?`, id)
	run, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("sqlitestore: run %s not found", id)
	}
	return run, err
}

// ListRuns returns all runs ordered by start time (newest first).
func (s *Store) ListRuns(ctx context.Context) ([]*store.Run, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, task, harness_version, harness_name, repository, commit_sha, workspace,
			trace_path, started_at, finished_at, status, success, input_tokens,
			output_tokens, tool_calls, model_calls, cost_usd, duration_ms,
			spec_yaml, workspace_patch, verification_passed, workspace_changed, verification,
			environment
		FROM runs ORDER BY started_at DESC`)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	var runs []*store.Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, wrap(rows.Err())
}

// AppendEvent persists one raw event (payload is JSON-encoded RunEvent).
func (s *Store) AppendEvent(ctx context.Context, runID, parentID, evType string, ts time.Time, payload []byte) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO events (id, run_id, parent_id, type, ts, payload)
		VALUES (?,?,?,?,?,?)`,
		fmt.Sprintf("%s-%d", runID, ts.UnixNano()), runID, parentID, evType,
		unixMillis(ts), string(payload),
	)
	return wrap(err)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRun(row scanner) (*store.Run, error) {
	var (
		r            store.Run
		started, fin int64
		status       string
		success      int
		verPassed    int
		wsChanged    int
		verJSON      string
	)
	if err := row.Scan(&r.ID, &r.Task, &r.HarnessVersion, &r.HarnessName, &r.Repository,
		&r.Commit, &r.Workspace, &r.TracePath, &started, &fin, &status, &success,
		&r.Metrics.InputTokens, &r.Metrics.OutputTokens, &r.Metrics.ToolCalls,
		&r.Metrics.ModelCalls, &r.Metrics.CostUSD, &r.Metrics.DurationMS,
		&r.SpecYAML, &r.WorkspacePatch, &verPassed, &wsChanged, &verJSON,
		&r.Environment); err != nil {
		return nil, wrap(err)
	}
	r.StartedAt = time.UnixMilli(started)
	if fin > 0 {
		r.FinishedAt = time.UnixMilli(fin)
	}
	r.Status = store.Status(status)
	r.Metrics.Success = success != 0
	r.Metrics.VerificationPassed = verPassed != 0
	r.Metrics.WorkspaceChanged = wsChanged != 0
	if verJSON != "" {
		_ = json.Unmarshal([]byte(verJSON), &r.Verification)
	}
	return &r, nil
}

func unixMillis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func wrap(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("sqlitestore: %w", err)
}

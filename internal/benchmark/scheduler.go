package benchmark

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ittakestwo123/Harnesslab/internal/harness/builder"
	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
	"github.com/ittakestwo123/Harnesslab/internal/store"
)

// Options configures a benchmark run.
type Options struct {
	// HarnessDir is the .harness directory (store/traces/workspaces).
	HarnessDir string
	// StoreDriver selects the run store backend.
	StoreDriver string
	// Parallel bounds concurrent agent runs (budget.max_parallel).
	Parallel int
	// MaxTokens bounds cumulative tokens across all runs (budget.max_tokens).
	MaxTokens int64
	// Timeout bounds the whole benchmark.
	Timeout time.Duration
	// Retry re-queues scheduler-level failures (not verification failures)
	// up to this many extra attempts.
	Retry int
}

// Job is one unit of benchmark work: a task executed under a harness variant.
// Repeat identifies which independent repetition this job is (0-based); each
// repetition gets its own run record and is never overwritten.
type Job struct {
	ID      string
	Task    *Task
	Variant Variant
	Repeat  int
}

// Outcome is the result of one job execution.
type Outcome struct {
	Job     *Job
	RunID   string
	Status  store.Status
	Metrics store.Metrics
	Error   error
}

// Scheduler runs jobs on a bounded worker pool with budget control.
type Scheduler struct {
	opts Options
}

// NewScheduler creates a scheduler.
func NewScheduler(opts Options) *Scheduler {
	if opts.Parallel <= 0 {
		opts.Parallel = 2
	}
	return &Scheduler{opts: opts}
}

// Run executes all jobs in waves: the initial batch, then up to Retry waves
// for scheduler-level failures. onOutcome, when non-nil, is called as each
// job settles (used by the CLI for progress).
func (s *Scheduler) Run(ctx context.Context, jobs []Job, onOutcome func(Outcome)) (*Report, error) {
	if len(jobs) == 0 {
		return nil, fmt.Errorf("benchmark: no jobs to run")
	}
	if s.opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.opts.Timeout)
		defer cancel()
	}

	report := &Report{ID: newBenchID(), StartedAt: time.Now()}
	current := jobs
	for attempt := 0; len(current) > 0 && attempt <= s.opts.Retry; attempt++ {
		if ctx.Err() != nil {
			break
		}
		if attempt > 0 {
			log.Printf("benchmark: retry wave %d: %d jobs", attempt, len(current))
		}
		failed, err := s.runBatch(ctx, current, report, onOutcome)
		if err != nil {
			return nil, err
		}
		current = failed
	}
	report.FinishedAt = time.Now()
	report.Finalize()
	return report, nil
}

// runBatch runs a wave of jobs on a bounded worker pool. It returns the jobs
// that failed at the scheduler level (build/run errors) for a possible retry
// wave. Verification failures are legitimate outcomes, not retried.
func (s *Scheduler) runBatch(ctx context.Context, jobs []Job, report *Report, onOutcome func(Outcome)) ([]Job, error) {
	var (
		wg       sync.WaitGroup
		stopFlag atomic.Bool
		tokens   atomic.Int64
		mu       sync.Mutex
		failed   []Job
	)

	jobCh := make(chan Job)
	for i := 0; i < s.opts.Parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				if ctx.Err() != nil || stopFlag.Load() {
					return
				}
				outcome := s.runJob(ctx, &job)
				tokens.Add(outcome.Metrics.InputTokens + outcome.Metrics.OutputTokens)
				if s.opts.MaxTokens > 0 && tokens.Load() >= s.opts.MaxTokens {
					if stopFlag.CompareAndSwap(false, true) {
						log.Printf("benchmark: token budget exceeded (%d >= %d), stopping new jobs", tokens.Load(), s.opts.MaxTokens)
					}
				}
				mu.Lock()
				report.add(outcome)
				if outcome.Error != nil {
					failed = append(failed, job)
				}
				mu.Unlock()
				if onOutcome != nil {
					onOutcome(outcome)
				}
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, j := range jobs {
			// Wait until the token budget allows starting another job.
			for {
				if ctx.Err() != nil || stopFlag.Load() {
					return
				}
				if s.opts.MaxTokens <= 0 || tokens.Load() < s.opts.MaxTokens {
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
			jobCh <- j
		}
		close(jobCh)
	}()

	wg.Wait()
	<-done
	return failed, nil
}

// runJob executes one job: build a harness from the variant spec + task
// override, run the task, and report the outcome.
func (s *Scheduler) runJob(ctx context.Context, job *Job) Outcome {
	spec2 := *job.Variant.Spec
	// Task verification overrides the harness verification — except when the
	// harness deliberately disables verification (ablation baseline H0): then
	// the task's commands stay unapplied so the no-verification condition is
	// actually exercised.
	if (job.Task.Verification.Strategy != "" || len(job.Task.Verification.Commands) > 0) &&
		spec2.Verification.Strategy != spec.VerificationNone {
		spec2.Verification = job.Task.Verification
	}
	if job.Task.Success.IsSet() {
		spec2.Success = job.Task.Success
	}

	h, err := builder.Build(ctx, &spec2, builder.Options{
		HarnessDir:  s.opts.HarnessDir,
		Repo:        job.Task.Repo,
		Commit:      job.Task.Commit,
		UserID:      "bench",
		StoreDriver: s.opts.StoreDriver,
	})
	if err != nil {
		return Outcome{Job: job, Error: fmt.Errorf("build: %w", err)}
	}
	defer h.Close()

	result, err := h.Run(ctx, job.Task.Prompt, nil)
	if err != nil {
		return Outcome{Job: job, RunID: h.RunID, Error: err}
	}
	return Outcome{
		Job:     job,
		RunID:   h.RunID,
		Status:  result.Run.Status,
		Metrics: result.Metrics,
	}
}

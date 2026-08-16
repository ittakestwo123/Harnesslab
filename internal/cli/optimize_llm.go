package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"

	"github.com/ittakestwo123/Harnesslab/internal/benchmark"
	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
	"github.com/ittakestwo123/Harnesslab/internal/optimize"
)

// modelClient adapts the framework model to the optimizer's LLMClient.
type modelClient struct {
	m model.Model
}

func (c *modelClient) Complete(ctx context.Context, system, user string) (string, error) {
	req := &model.Request{
		Messages: []model.Message{
			{Role: model.RoleSystem, Content: system},
			{Role: model.RoleUser, Content: user},
		},
		GenerationConfig: model.GenerationConfig{Stream: false},
	}
	ch, err := c.m.GenerateContent(ctx, req)
	if err != nil {
		return "", err
	}
	var content string
	for r := range ch {
		if r.Error != nil {
			return "", fmt.Errorf("model api: %s", r.Error.Message)
		}
		if len(r.Choices) > 0 {
			content = r.Choices[0].Message.Content
		}
	}
	if content == "" {
		return "", fmt.Errorf("model returned empty completion")
	}
	return content, nil
}

// newLLMClient builds a model client from a ModelSpec. The framework reads
// the API key from the provider env (DEEPSEEK_API_KEY / OPENAI_API_KEY).
func newLLMClient(m spec.ModelSpec) (optimize.LLMClient, error) {
	switch m.Provider {
	case "deepseek":
		return &modelClient{m: openai.New(m.Name, openai.WithVariant(openai.VariantDeepSeek))}, nil
	case "openai", "":
		return &modelClient{m: openai.New(m.Name)}, nil
	default:
		return nil, fmt.Errorf("optimize: unsupported model provider %q for llm generation", m.Provider)
	}
}

// runBenchTasks runs a benchmark over tasks filtered by a taskset file, one
// variant per harness spec, and returns the report. Variants must include a
// "baseline" entry for comparison.
func runBenchTasks(ctx context.Context, tasksDir, tasksetFile, harnessDir string,
	variants []benchmark.Variant, parallel int, maxTokens int64, timeout time.Duration, repeat int) (*benchmark.Report, error) {

	tasks, err := benchmark.LoadTasks(tasksDir)
	if err != nil {
		return nil, err
	}
	if tasksetFile != "" {
		ts, err := benchmark.LoadTaskSet(tasksetFile)
		if err != nil {
			return nil, err
		}
		tasks = benchmark.FilterByTaskSet(tasks, ts)
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("optimize: no tasks after taskset filter")
	}

	var jobs []benchmark.Job
	for _, t := range tasks {
		for _, v := range variants {
			for rep := 0; rep < repeat; rep++ {
				jobs = append(jobs, benchmark.Job{
					ID:      fmt.Sprintf("%s/%s/r%d", t.ID, v.Name, rep),
					Task:    t,
					Variant: v,
					Repeat:  rep,
				})
			}
		}
	}

	sched := benchmark.NewScheduler(benchmark.Options{
		HarnessDir: harnessDir,
		Parallel:   parallel,
		MaxTokens:  maxTokens,
		Timeout:    timeout,
		Retry:      1,
	})
	return sched.Run(ctx, jobs, nil)
}

// evalResults converts a bench report into optimizer EvalResults.
func evalResults(rep *benchmark.Report) []optimize.EvalResult {
	var out []optimize.EvalResult
	for _, v := range rep.Variants {
		if v.Total == 0 {
			continue
		}
		out = append(out, optimize.EvalResult{
			Variant: v.Variant,
			Pass:    float64(v.Passed) / float64(v.Total),
			Tokens:  avg(v.InputTokens+v.OutputTokens, v.Total),
			Cost:    v.CostUSD / float64(v.Total),
			Runs:    v.Total,
		})
	}
	return out
}

// candidateVariants builds bench variants from candidates, prepending the
// current harness as "baseline".
func candidateVariants(base *spec.HarnessSpec, candidates []*optimize.Candidate) []benchmark.Variant {
	variants := []benchmark.Variant{{Name: "baseline", Spec: base}}
	for _, c := range candidates {
		variants = append(variants, benchmark.Variant{
			Name: strings.TrimSuffix(strings.TrimPrefix(c.File, "candidate-"), ".yaml"),
			Spec: c.Spec,
		})
	}
	return variants
}

// writeCandidates persists generated candidates under harnessDir/candidates.
func writeCandidates(harnessDir string, candidates []*optimize.Candidate) error {
	dir := filepath.Join(harnessDir, "candidates")
	for _, c := range candidates {
		name := strings.TrimSuffix(strings.TrimPrefix(c.File, "candidate-"), ".yaml")
		if _, err := optimize.WriteCandidate(dir, name, c.Spec, c.Metadata); err != nil {
			return err
		}
		fmt.Printf("  wrote %s\n", filepath.Join(dir, c.File))
	}
	return nil
}

// printRecommendation renders the final optimizer recommendation.
func printRecommendation(rec optimize.Recommendation) {
	fmt.Println()
	fmt.Printf("Baseline (dev pass %.0f%%, holdout pass %.0f%%, %d runs)\n",
		rec.Baseline.Pass*100, rec.Baseline.Pass*100, rec.Baseline.Runs)
	if len(rec.Accepted) == 0 {
		fmt.Println("Accepted candidates: none — keep the current harness")
	} else {
		fmt.Println("Accepted candidates (dev improvement AND holdout held):")
		for _, a := range rec.Accepted {
			fmt.Printf("  %-24s pass %.0f%% tokens %d cost $%.4f\n", a.Variant, a.Pass*100, a.Tokens, a.Cost)
		}
	}
	if len(rec.Rejected) > 0 {
		fmt.Println("Rejected candidates (dev-only win, holdout regressed or unevaluated):")
		for _, r := range rec.Rejected {
			fmt.Printf("  %-24s pass %.0f%%\n", r.Variant, r.Pass*100)
		}
	}
}

// ensureEnvWarn checks that a provider API key is available for llm mode.
func ensureEnvWarn(provider string) {
	key := ""
	switch provider {
	case "deepseek":
		key = os.Getenv("DEEPSEEK_API_KEY")
	case "openai", "":
		key = os.Getenv("OPENAI_API_KEY")
	}
	if key == "" {
		fmt.Println("WARNING: no API key found for provider", provider, "(DEEPSEEK_API_KEY/OPENAI_API_KEY); llm generation will fail")
	}
}

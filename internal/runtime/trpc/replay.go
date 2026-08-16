package trpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/tool"

	"github.com/ittakestwo123/Harnesslab/internal/replay"
	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
)

// buildReplayToolCallbacks builds tool callbacks that record or replay tool
// calls. BeforeTool with a CustomResult skips the real tool; AfterTool records
// results. Returns nil when replay is disabled.
func buildReplayToolCallbacks(cfg *hlruntime.ReplayConfig, canon *replay.Canonicalizer) *tool.Callbacks {
	if cfg == nil || cfg.Store == nil || cfg.Mode == "" {
		return nil
	}
	cbs := tool.NewCallbacks()
	if cfg.Mode == hlruntime.ReplayRecord {
		cbs.RegisterAfterTool(func(ctx context.Context, args *tool.AfterToolArgs) (*tool.AfterToolResult, error) {
			if args.Error != nil || args.Result == nil {
				return nil, nil
			}
			hash, err := canon.Hash(replay.KindTool, args.ToolName, args.Arguments)
			if err != nil {
				return nil, nil
			}
			out, err := json.Marshal(args.Result)
			if err != nil {
				return nil, nil
			}
			_ = cfg.Store.Put(ctx, replay.Entry{
				Kind: replay.KindTool, InputHash: hash,
				Input: args.Arguments, Output: out, CreatedAt: time.Now(),
			})
			return nil, nil
		})
	} else {
		cbs.RegisterBeforeTool(func(ctx context.Context, args *tool.BeforeToolArgs) (*tool.BeforeToolResult, error) {
			hash, err := canon.Hash(replay.KindTool, args.ToolName, args.Arguments)
			if err != nil {
				return nil, nil
			}
			out, ok, err := cfg.Store.Lookup(ctx, replay.KindTool, hash)
			if err != nil {
				return nil, nil
			}
			if ok {
				var result any
				if err := json.Unmarshal(out, &result); err != nil {
					return nil, nil
				}
				return &tool.BeforeToolResult{CustomResult: result, SkipStateDelta: true}, nil
			}
			if cfg.Mode == hlruntime.ReplayStrict {
				return nil, fmt.Errorf("replay: strict miss for tool %q (hash %s)", args.ToolName, hash)
			}
			return nil, nil // fallback: execute the real tool
		})
	}
	return cbs
}

// replayModel wraps a model.Model to record or replay model calls. Recording
// aggregates a streaming call into its final response and stores it; replay
// serves the recorded response without calling the live model.
type replayModel struct {
	inner model.Model
	cfg   *hlruntime.ReplayConfig
	canon *replay.Canonicalizer
}

func newReplayModel(inner model.Model, cfg *hlruntime.ReplayConfig, canon *replay.Canonicalizer) model.Model {
	return &replayModel{inner: inner, cfg: cfg, canon: canon}
}

// Info implements model.Model.
func (m *replayModel) Info() model.Info { return m.inner.Info() }

// GenerateContent implements model.Model.
func (m *replayModel) GenerateContent(ctx context.Context, req *model.Request) (<-chan *model.Response, error) {
	input, err := canonicalizeModelRequest(m.canon, req)
	if err != nil {
		return nil, err
	}
	hash, err := m.canon.Hash(replay.KindModel, "model", input)
	if err != nil {
		return nil, err
	}
	if m.cfg.Mode == hlruntime.ReplayRecord {
		return m.recordCall(ctx, req, hash, input)
	}
	return m.replayCall(ctx, req, hash, input)
}

// recordCall passes the live stream through while aggregating the final
// response for the replay store.
func (m *replayModel) recordCall(ctx context.Context, req *model.Request, hash string, input []byte) (<-chan *model.Response, error) {
	ch, err := m.inner.GenerateContent(ctx, req)
	if err != nil {
		return nil, err
	}
	out := make(chan *model.Response, 64)
	go func() {
		defer close(out)
		agg := &responseAggregator{}
		for rsp := range ch {
			agg.add(rsp)
			out <- rsp
		}
		if final := agg.final(); final != nil {
			data, err := json.Marshal(final)
			if err == nil {
				_ = m.cfg.Store.Put(ctx, replay.Entry{
					Kind: replay.KindModel, InputHash: hash,
					Input: input, Output: data, CreatedAt: time.Now(),
				})
			}
		}
	}()
	return out, nil
}

// replayCall serves the recorded response without calling the live model.
func (m *replayModel) replayCall(ctx context.Context, req *model.Request, hash string, input []byte) (<-chan *model.Response, error) {
	out, ok, err := m.cfg.Store.Lookup(ctx, replay.KindModel, hash)
	if err == nil && ok {
		var rsp model.Response
		if err := json.Unmarshal(out, &rsp); err == nil {
			ch := make(chan *model.Response, 1)
			ch <- &rsp
			close(ch)
			return ch, nil
		}
	}
	if m.cfg.Mode == hlruntime.ReplayStrict {
		return nil, fmt.Errorf("replay: strict miss for model call (hash %s)", hash)
	}
	return m.inner.GenerateContent(ctx, req)
}

// responseAggregator accumulates a streaming call into its final response.
// The final frame of a stream repeats already-delivered content in
// Message.Content, so Message content is only used when no delta content was
// seen for the call.
type responseAggregator struct {
	content   string
	sawDelta  bool
	usage     *model.Usage
	toolCalls []model.ToolCall
	modelName string
}

func (a *responseAggregator) add(rsp *model.Response) {
	if a.modelName == "" {
		a.modelName = rsp.Model
	}
	if rsp.Usage != nil {
		a.usage = rsp.Usage
	}
	for _, ch := range rsp.Choices {
		if ch.Delta.Content != "" {
			a.content += ch.Delta.Content
			a.sawDelta = true
		}
		if !a.sawDelta && ch.Message.Content != "" {
			a.content += ch.Message.Content
		}
		if len(ch.Message.ToolCalls) > 0 {
			a.toolCalls = ch.Message.ToolCalls
		}
	}
}

// final synthesizes the aggregated completion response for the store.
func (a *responseAggregator) final() *model.Response {
	if a.modelName == "" && a.content == "" && len(a.toolCalls) == 0 {
		return nil
	}
	return &model.Response{
		ID:        fmt.Sprintf("replay-%d", time.Now().UnixNano()),
		Object:    model.ObjectTypeChatCompletion,
		Model:     a.modelName,
		Choices:   []model.Choice{{Message: model.Message{Role: model.RoleAssistant, Content: a.content, ToolCalls: a.toolCalls}}},
		Usage:     a.usage,
		Done:      true,
		Timestamp: time.Now(),
	}
}

// canonicalizeModelRequest serializes a model request deterministically for
// content hashing: messages + sorted tool declarations + generation config.
// Headers and tool instances are excluded on purpose.
func canonicalizeModelRequest(c *replay.Canonicalizer, req *model.Request) ([]byte, error) {
	payload := map[string]any{
		"messages":          req.Messages,
		"tools":             toolDeclarations(req.Tools),
		"config":            req.GenerationConfig,
		"structured_output": req.StructuredOutput,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return c.Normalize(data)
}

func toolDeclarations(tools map[string]tool.Tool) []*tool.Declaration {
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)
	decls := make([]*tool.Declaration, 0, len(tools))
	for _, name := range names {
		if t := tools[name]; t != nil && t.Declaration() != nil {
			decls = append(decls, t.Declaration())
		}
	}
	return decls
}

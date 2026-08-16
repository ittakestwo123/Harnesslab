// Package trpc adapts the tRPC-Agent-Go runner to the HarnessLab runtime
// interface. It normalizes the framework's event stream into HarnessEvents
// and aggregates per-run metrics. Nothing outside this package depends on
// tRPC-Agent-Go types.
package trpc

import (
	"fmt"
	"strings"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"

	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
)

// maxContentLen caps how much content/tool-result text is kept in a trace
// event so trace files stay small and readable.
const maxContentLen = 800

// normalizer converts framework events into normalized run events and
// aggregates run metrics.
type normalizer struct {
	runID string
	seq   int
	step  int

	// state for the current streaming model call
	inModel   bool
	modelName string
	tokensIn  int
	tokensOut int
	content   strings.Builder

	// toolNames maps tool call ID -> tool name so tool.response events
	// (which carry only the ToolID) can be correlated to a tool_end event.
	toolNames map[string]string
	// seenToolIDs dedupes tool starts across streaming deltas and the final
	// aggregated chat.completion frame.
	seenToolIDs map[string]bool

	// lastToolName is the fallback tool_end name when the ToolID is empty.
	lastToolName string

	// metrics
	modelCalls   int
	toolCalls    int
	inputTokens  int64
	outputTokens int64
}

func newNormalizer(runID string) *normalizer {
	return &normalizer{
		runID:       runID,
		toolNames:   map[string]string{},
		seenToolIDs: map[string]bool{},
	}
}

// metrics returns the aggregated counters of this run.
func (n *normalizer) metrics() (modelCalls, toolCalls int, in, out int64) {
	return n.modelCalls, n.toolCalls, n.inputTokens, n.outputTokens
}

// normalize converts one framework event into zero or more run events.
func (n *normalizer) normalize(ev *event.Event) []hlruntime.RunEvent {
	// Some providers deliver API-level failures with an empty Object and a
	// non-nil Error; surface those before the object-type switch.
	if ev.Error != nil {
		return []hlruntime.RunEvent{n.errorEvent(ev, ev.Error.Type)}
	}
	switch ev.Object {
	case model.ObjectTypeChatCompletionChunk:
		return n.handleChunk(ev)
	case model.ObjectTypeChatCompletion:
		return n.handleCompletion(ev)
	case model.ObjectTypeToolResponse:
		return n.handleToolResponse(ev)
	case model.ObjectTypeError:
		return []hlruntime.RunEvent{n.errorEvent(ev, "model_error")}
	default:
		// preprocessing.*, state.update, runner.completion and friends are
		// framework plumbing; they are intentionally not part of the trace.
		return nil
	}
}

// handleChunk processes one streaming chunk. model_start is emitted on the
// first chunk of a call; model_end on the final (Done) chunk.
func (n *normalizer) handleChunk(ev *event.Event) []hlruntime.RunEvent {
	var out []hlruntime.RunEvent

	if !n.inModel {
		n.inModel = true
		n.modelCalls++
		n.modelName = ev.Model
		out = append(out, n.emit(hlruntime.EventModelStart, func(e *hlruntime.RunEvent) {
			e.Model = &hlruntime.ModelEvent{Model: n.modelName}
		}))
	}

	for _, ch := range ev.Choices {
		if ch.Delta.Content != "" {
			n.content.WriteString(ch.Delta.Content)
		}
		for _, tc := range ch.Delta.ToolCalls {
			if tc.ID != "" {
				n.seenToolIDs[tc.ID] = true
			}
			if tc.Function.Name == "" {
				continue
			}
			n.toolCalls++
			n.lastToolName = tc.Function.Name
			n.toolNames[tc.ID] = tc.Function.Name
			out = append(out, n.emit(hlruntime.EventToolStart, func(e *hlruntime.RunEvent) {
				e.Tool = &hlruntime.ToolEvent{Name: tc.Function.Name}
			}))
		}
	}

	if ev.Usage != nil {
		n.tokensIn = ev.Usage.PromptTokens
		n.tokensOut = ev.Usage.CompletionTokens
	}

	if ev.Done {
		n.inModel = false
		n.inputTokens += int64(n.tokensIn)
		n.outputTokens += int64(n.tokensOut)
		text := n.content.String()
		n.content.Reset()
		out = append(out, n.emit(hlruntime.EventModelEnd, func(e *hlruntime.RunEvent) {
			e.Model = &hlruntime.ModelEvent{
				Model:     n.modelName,
				TokensIn:  n.tokensIn,
				TokensOut: n.tokensOut,
				Content:   clip(text),
			}
		}))
	}
	return out
}

// handleCompletion processes a chat.completion response. In streaming mode
// this event is the flow's aggregated frame: it carries the final tool calls
// in Message.ToolCalls (some providers never stream tool-call deltas) and may
// repeat already-streamed content. Content is therefore only accumulated for
// genuinely non-streaming responses, while tool calls are always processed
// (deduplicated by ToolCallID against streaming deltas).
func (n *normalizer) handleCompletion(ev *event.Event) []hlruntime.RunEvent {
	var out []hlruntime.RunEvent
	streaming := n.inModel

	if !n.inModel {
		n.inModel = true
		n.modelCalls++
		n.modelName = ev.Model
		out = append(out, n.emit(hlruntime.EventModelStart, func(e *hlruntime.RunEvent) {
			e.Model = &hlruntime.ModelEvent{Model: n.modelName}
		}))
	}

	if !streaming {
		for _, ch := range ev.Choices {
			if ch.Message.Content != "" {
				n.content.WriteString(ch.Message.Content)
			}
		}
	}

	for _, ch := range ev.Choices {
		for _, tc := range ch.Message.ToolCalls {
			if tc.Function.Name == "" {
				continue
			}
			if tc.ID != "" {
				if n.seenToolIDs[tc.ID] {
					continue
				}
				n.seenToolIDs[tc.ID] = true
			}
			n.toolCalls++
			n.lastToolName = tc.Function.Name
			n.toolNames[tc.ID] = tc.Function.Name
			out = append(out, n.emit(hlruntime.EventToolStart, func(e *hlruntime.RunEvent) {
				e.Tool = &hlruntime.ToolEvent{
					Name:      tc.Function.Name,
					Arguments: string(tc.Function.Arguments),
				}
			}))
		}
	}

	if ev.Usage != nil {
		n.tokensIn = ev.Usage.PromptTokens
		n.tokensOut = ev.Usage.CompletionTokens
	}

	n.inModel = false
	n.inputTokens += int64(n.tokensIn)
	n.outputTokens += int64(n.tokensOut)
	text := n.content.String()
	n.content.Reset()
	out = append(out, n.emit(hlruntime.EventModelEnd, func(e *hlruntime.RunEvent) {
		e.Model = &hlruntime.ModelEvent{
			Model:     n.modelName,
			TokensIn:  n.tokensIn,
			TokensOut: n.tokensOut,
			Content:   clip(text),
		}
	}))
	return out
}

// handleToolResponse converts a tool.response event into a tool_end event.
// The event carries only the ToolCallID; the name is resolved from the map
// built during tool_start.
func (n *normalizer) handleToolResponse(ev *event.Event) []hlruntime.RunEvent {
	result := ""
	toolID := ""
	if len(ev.Choices) > 0 {
		result = ev.Choices[0].Message.Content
		toolID = ev.Choices[0].Message.ToolID
		if toolID == "" {
			toolID = ev.Choices[0].Delta.ToolID
		}
	}
	name := n.toolNames[toolID]
	if name == "" {
		name = n.lastToolName
	}
	return []hlruntime.RunEvent{n.emit(hlruntime.EventToolEnd, func(e *hlruntime.RunEvent) {
		e.Tool = &hlruntime.ToolEvent{Name: name, Result: clip(result)}
	})}
}

// errorEvent converts a framework error event.
func (n *normalizer) errorEvent(ev *event.Event, typ string) hlruntime.RunEvent {
	msg := "unknown error"
	if ev.Error != nil && ev.Error.Message != "" {
		msg = ev.Error.Message
	}
	return n.emit(hlruntime.EventError, func(e *hlruntime.RunEvent) {
		e.Error = &hlruntime.ErrorEvent{Message: clip(msg), Type: typ}
	})
}

// emit builds a run event with sequence-based id and step numbering.
func (n *normalizer) emit(t hlruntime.EventType, mut func(*hlruntime.RunEvent)) hlruntime.RunEvent {
	n.seq++
	n.step++
	e := hlruntime.RunEvent{
		ID:        fmt.Sprintf("%s-%05d", n.runID, n.seq),
		RunID:     n.runID,
		Type:      t,
		Timestamp: time.Now(),
		Step:      n.step,
	}
	if mut != nil {
		mut(&e)
	}
	return e
}

func clip(s string) string {
	if len(s) <= maxContentLen {
		return s
	}
	return s[:maxContentLen] + "..."
}

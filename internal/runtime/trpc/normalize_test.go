package trpc

import (
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"

	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
)

func mkEvent(object string, done bool, content string, usage *model.Usage) *event.Event {
	rsp := &model.Response{
		Object:    object,
		Done:      done,
		Model:     "gpt-5",
		Usage:     usage,
		Timestamp: time.Now(),
	}
	if content != "" {
		rsp.Choices = []model.Choice{{Delta: model.Message{Content: content}}}
	}
	return &event.Event{Response: rsp, Timestamp: time.Now()}
}

func TestNormalizeStreamingModelCall(t *testing.T) {
	n := newNormalizer("run-1")
	var got []hlruntime.RunEvent
	got = append(got, n.normalize(mkEvent(model.ObjectTypeChatCompletionChunk, false, "hel", nil))...)
	got = append(got, n.normalize(mkEvent(model.ObjectTypeChatCompletionChunk, false, "lo", nil))...)
	got = append(got, n.normalize(mkEvent(model.ObjectTypeChatCompletionChunk, true, "!", &model.Usage{
		PromptTokens: 10, CompletionTokens: 5,
	}))...)

	if len(got) != 2 {
		t.Fatalf("events = %d, want 2 (model_start, model_end)", len(got))
	}
	if got[0].Type != hlruntime.EventModelStart {
		t.Fatalf("first event = %s, want model_start", got[0].Type)
	}
	end := got[1]
	if end.Type != hlruntime.EventModelEnd {
		t.Fatalf("last event = %s, want model_end", end.Type)
	}
	if end.Model == nil || end.Model.TokensIn != 10 || end.Model.TokensOut != 5 {
		t.Fatalf("model_end tokens = %+v, want 10/5", end.Model)
	}
	if end.Model == nil || end.Model.Content != "hello!" {
		t.Fatalf("model_end content = %q, want hello!", end.Model.Content)
	}
	mc, tc, in, out := n.metrics()
	if mc != 1 || tc != 0 || in != 10 || out != 5 {
		t.Fatalf("metrics = %d/%d/%d/%d, want 1/0/10/5", mc, tc, in, out)
	}
}

func TestNormalizeAPIError(t *testing.T) {
	n := newNormalizer("run-2")
	// Providers surface API failures with empty Object and non-nil Error.
	ev := &event.Event{
		Response: &model.Response{
			Object: "",
			Error:  &model.ResponseError{Message: "401 unauthorized", Type: "api_error"},
		},
	}
	got := n.normalize(ev)
	if len(got) != 1 || got[0].Type != hlruntime.EventError {
		t.Fatalf("events = %+v, want one error event", got)
	}
	if got[0].Error == nil || got[0].Error.Message != "401 unauthorized" {
		t.Fatalf("error = %+v", got[0].Error)
	}
}

func TestNormalizeStreamingAggregatedFinal(t *testing.T) {
	// Regression: after streaming chunks, the flow emits a final aggregated
	// chat.completion frame whose Message repeats the content. It must not
	// duplicate content and must not emit a second model_start.
	n := newNormalizer("run-x")
	var got []hlruntime.RunEvent
	got = append(got, n.normalize(mkEvent(model.ObjectTypeChatCompletionChunk, false, "2+2 ", nil))...)
	got = append(got, n.normalize(mkEvent(model.ObjectTypeChatCompletionChunk, false, "equals 4.", nil))...)

	final := &event.Event{
		Response: &model.Response{
			Object:  model.ObjectTypeChatCompletion,
			Done:    true,
			Model:   "gpt-5",
			Usage:   &model.Usage{PromptTokens: 10, CompletionTokens: 5},
			Choices: []model.Choice{{Message: model.Message{Content: "2+2 equals 4."}}},
		},
	}
	got = append(got, n.normalize(final)...)

	var modelEnds, modelStarts int
	for _, e := range got {
		switch e.Type {
		case hlruntime.EventModelStart:
			modelStarts++
		case hlruntime.EventModelEnd:
			modelEnds++
			if e.Model == nil || e.Model.Content != "2+2 equals 4." {
				t.Fatalf("model_end content = %+v, want single '2+2 equals 4.'", e.Model)
			}
		}
	}
	if modelStarts != 1 || modelEnds != 1 {
		t.Fatalf("starts=%d ends=%d, want 1/1", modelStarts, modelEnds)
	}
}

func TestNormalizeToolCall(t *testing.T) {
	n := newNormalizer("run-3")
	ev := &event.Event{
		Response: &model.Response{
			Object: model.ObjectTypeChatCompletion,
			Done:   true,
			Model:  "gpt-5",
			Choices: []model.Choice{{
				Message: model.Message{
					ToolCalls: []model.ToolCall{{
						ID: "call_1",
						Function: model.FunctionDefinitionParam{
							Name:      "exec_command",
							Arguments: []byte(`{"cmd":"go test ./..."}`),
						},
					}},
				},
			}},
		},
	}
	got := n.normalize(ev)
	var start *hlruntime.RunEvent
	for i := range got {
		if got[i].Type == hlruntime.EventToolStart {
			start = &got[i]
		}
	}
	if start == nil {
		t.Fatalf("no tool_start event in %+v", got)
	}
	if start.Tool == nil || start.Tool.Name != "exec_command" {
		t.Fatalf("tool_start = %+v", start.Tool)
	}
	_, tc, _, _ := n.metrics()
	if tc != 1 {
		t.Fatalf("tool calls = %d, want 1", tc)
	}
}

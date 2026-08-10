package main

import (
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
)

func ptr[T any](v T) *T { return &v }

func chatReq(model string, reasoning *schemas.ChatReasoning) *schemas.BifrostRequest {
	params := &schemas.ChatParameters{Reasoning: reasoning}
	return &schemas.BifrostRequest{
		ChatRequest: &schemas.BifrostChatRequest{
			Provider: schemas.OpenAI,
			Model:    model,
			Params:   params,
		},
	}
}

func TestPreLLMHook_StripUnsupportedModels(t *testing.T) {
	if err := Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}

	cases := []string{
		"gpt-4.1",
		"openai/gpt-4.1",
		"gpt-4.1-2025-04-14",
		"gpt-4.1-mini",
		"openai/gpt-4.1-mini",
		"gpt-4.1-mini-2025-04-14",
		"gpt-4.1-nano",
		"openai/gpt-4.1-nano",
		"GPT-4.1-NANO",
	}

	for _, model := range cases {
		t.Run(model, func(t *testing.T) {
			req := chatReq(model, &schemas.ChatReasoning{
				Effort:    ptr("high"),
				MaxTokens: ptr(1024),
			})
			req.ChatRequest.Params.ExtraParams = map[string]interface{}{
				"reasoning_effort": "high",
			}

			out, sc, err := PreLLMHook(nil, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sc != nil {
				t.Fatalf("unexpected short-circuit")
			}
			if out.ChatRequest.Params.Reasoning != nil {
				t.Fatalf("expected Reasoning nil, got %+v", out.ChatRequest.Params.Reasoning)
			}
			if _, ok := out.ChatRequest.Params.ExtraParams["reasoning_effort"]; ok {
				t.Fatalf("expected reasoning_effort removed from ExtraParams")
			}
		})
	}

	t.Run("strip_even_when_absent", func(t *testing.T) {
		req := &schemas.BifrostRequest{
			ChatRequest: &schemas.BifrostChatRequest{
				Provider: schemas.OpenAI,
				Model:    "gpt-4.1-mini",
			},
		}
		out, _, err := PreLLMHook(nil, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.ChatRequest.Params != nil && out.ChatRequest.Params.Reasoning != nil {
			t.Fatalf("expected no reasoning injected for gpt-4.1*")
		}
	})
}

func TestPreLLMHook_ForceNoneOnOtherModels(t *testing.T) {
	if err := Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}

	cases := []struct {
		name  string
		model string
		in    *schemas.ChatReasoning
	}{
		{"no_params_reasoning", "gpt-5", nil},
		{"existing_high", "openai/o3-mini", &schemas.ChatReasoning{Effort: ptr("high"), MaxTokens: ptr(2048)}},
		{"stream_same_path", "gpt-5-mini", &schemas.ChatReasoning{Effort: ptr("medium")}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req *schemas.BifrostRequest
			if tc.in == nil {
				req = &schemas.BifrostRequest{
					ChatRequest: &schemas.BifrostChatRequest{
						Provider: schemas.OpenAI,
						Model:    tc.model,
					},
				}
			} else {
				req = chatReq(tc.model, tc.in)
			}

			out, sc, err := PreLLMHook(nil, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sc != nil {
				t.Fatalf("unexpected short-circuit")
			}
			if out.ChatRequest.Params == nil || out.ChatRequest.Params.Reasoning == nil {
				t.Fatalf("expected Reasoning to be set")
			}
			r := out.ChatRequest.Params.Reasoning
			if r.Effort == nil || *r.Effort != "none" {
				t.Fatalf("expected effort=none, got %+v", r.Effort)
			}
			if r.MaxTokens != nil {
				t.Fatalf("expected MaxTokens cleared, got %v", *r.MaxTokens)
			}
			if r.Enabled != nil || r.Display != nil {
				t.Fatalf("expected Enabled/Display cleared")
			}
		})
	}
}

func TestPreLLMHook_NonChatPassthrough(t *testing.T) {
	if err := Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}

	req := &schemas.BifrostRequest{
		ResponsesRequest: &schemas.BifrostResponsesRequest{
			Model: "gpt-5",
		},
	}
	out, sc, err := PreLLMHook(nil, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sc != nil {
		t.Fatalf("unexpected short-circuit")
	}
	if out != req {
		t.Fatalf("expected same request pointer for non-chat")
	}
}

func TestInit_CustomConfig(t *testing.T) {
	err := Init(map[string]interface{}{
		"unsupported_models": []interface{}{"gpt-4o"},
		"force_effort":       "minimal",
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	req := chatReq("gpt-4o", &schemas.ChatReasoning{Effort: ptr("high")})
	out, _, err := PreLLMHook(nil, req)
	if err != nil {
		t.Fatalf("PreLLMHook: %v", err)
	}
	if out.ChatRequest.Params.Reasoning != nil {
		t.Fatalf("expected gpt-4o stripped when configured unsupported")
	}

	req2 := chatReq("gpt-5", &schemas.ChatReasoning{Effort: ptr("high")})
	out2, _, err := PreLLMHook(nil, req2)
	if err != nil {
		t.Fatalf("PreLLMHook: %v", err)
	}
	if out2.ChatRequest.Params.Reasoning.Effort == nil || *out2.ChatRequest.Params.Reasoning.Effort != "minimal" {
		t.Fatalf("expected force_effort=minimal")
	}

	// Restore defaults for other tests in this package
	_ = Init(nil)
}

func TestNormalizeAndMatch(t *testing.T) {
	if got := normalizeModel("OpenAI/GPT-4.1-Mini"); got != "gpt-4.1-mini" {
		t.Fatalf("normalizeModel: got %q", got)
	}
	unsupported := []string{"gpt-4.1"}
	for _, m := range []string{"gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano", "gpt-4.1-mini-2025-04-14"} {
		if !isUnsupportedModel(m, unsupported) {
			t.Fatalf("expected %q to match gpt-4.1 prefix", m)
		}
	}
	if isUnsupportedModel("gpt-4o", unsupported) {
		t.Fatal("gpt-4o should not match gpt-4.1")
	}
	if isUnsupportedModel("gpt-5", unsupported) {
		t.Fatal("gpt-5 should not match gpt-4.1")
	}
}

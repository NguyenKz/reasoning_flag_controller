package main

import (
	"fmt"
	"strings"
	"sync"

	"github.com/maximhq/bifrost/core/schemas"
)

const pluginName = "reasoning-flag-controller"

var (
	cfgMu sync.RWMutex
	// gpt-4.1 prefix matches gpt-4.1, gpt-4.1-mini, gpt-4.1-nano, and dated snapshots.
	unsupportedModels = []string{"gpt-4.1-mini", "gpt-4.1-nano"}
	forceEffort       = "none"
)

// Init is called when the plugin is loaded.
// config may contain:
//   - unsupported_models: []string — models that must not receive reasoning params
//   - force_effort: string — effort value for all other models (default "none")
func Init(config any) error {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	// Reset to defaults on each Init
	unsupportedModels = []string{"gpt-4.1-mini", "gpt-4.1-nano"}
	forceEffort = "none"

	if config == nil {
		return nil
	}

	cfg, ok := config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid config format: expected map[string]interface{}")
	}

	if raw, exists := cfg["unsupported_models"]; exists && raw != nil {
		models, err := toStringSlice(raw)
		if err != nil {
			return fmt.Errorf("unsupported_models: %w", err)
		}
		if len(models) > 0 {
			normalized := make([]string, 0, len(models))
			for _, m := range models {
				m = strings.ToLower(strings.TrimSpace(m))
				if m != "" {
					normalized = append(normalized, m)
				}
			}
			if len(normalized) > 0 {
				unsupportedModels = normalized
			}
		}
	}

	if raw, exists := cfg["force_effort"]; exists && raw != nil {
		s, ok := raw.(string)
		if !ok {
			return fmt.Errorf("force_effort must be a string")
		}
		s = strings.TrimSpace(s)
		if s != "" {
			forceEffort = s
		}
	}

	return nil
}

// GetName returns the plugin's unique identifier.
func GetName() string {
	return pluginName
}

// HTTPTransportPreHook is a no-op (LLM mutation happens in PreLLMHook).
func HTTPTransportPreHook(_ *schemas.BifrostContext, _ *schemas.HTTPRequest) (*schemas.HTTPResponse, error) {
	return nil, nil
}

// HTTPTransportPostHook is a no-op.
func HTTPTransportPostHook(_ *schemas.BifrostContext, _ *schemas.HTTPRequest, _ *schemas.HTTPResponse) error {
	return nil
}

// HTTPTransportStreamChunkHook passes chunks through unchanged.
func HTTPTransportStreamChunkHook(_ *schemas.BifrostContext, _ *schemas.HTTPRequest, chunk *schemas.BifrostStreamChunk) (*schemas.BifrostStreamChunk, error) {
	return chunk, nil
}

// PreRequestHook is a no-op (routing is unchanged).
func PreRequestHook(_ *schemas.BifrostContext, _ *schemas.BifrostRequest) error {
	return nil
}

// PreLLMHook runs once per provider attempt: Bifrost → this hook → real provider.
// Chat Completions + stream only (req.ChatRequest).
//
// Rules:
//  1. gpt-4.1* (base/mini/nano/dated): always strip reasoning, whether client sent it or not.
//  2. Every other model: always force reasoning.effort (default "none"), even if client omitted it.
func PreLLMHook(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) (*schemas.BifrostRequest, *schemas.LLMPluginShortCircuit, error) {
	if req == nil || req.ChatRequest == nil {
		return req, nil, nil
	}

	model := normalizeModel(req.ChatRequest.Model)
	cfgMu.RLock()
	unsupported := unsupportedModels
	effort := forceEffort
	cfgMu.RUnlock()

	if isUnsupportedModel(model, unsupported) {
		stripReasoning(req.ChatRequest)
		if ctx != nil {
			ctx.Log(schemas.LogLevelDebug, fmt.Sprintf("stripped reasoning for unsupported model %q", model))
		}
	} else {
		forceReasoningEffort(req.ChatRequest, effort)
		if ctx != nil {
			ctx.Log(schemas.LogLevelDebug, fmt.Sprintf("forced reasoning.effort=%q for model %q", effort, model))
		}
	}

	return req, nil, nil
}

// PostLLMHook is a no-op.
func PostLLMHook(_ *schemas.BifrostContext, resp *schemas.BifrostResponse, bifrostErr *schemas.BifrostError) (*schemas.BifrostResponse, *schemas.BifrostError, error) {
	return resp, bifrostErr, nil
}

// Cleanup is called on Bifrost shutdown.
func Cleanup() error {
	return nil
}

func stripReasoning(chatReq *schemas.BifrostChatRequest) {
	if chatReq.Params != nil {
		chatReq.Params.Reasoning = nil
		if chatReq.Params.ExtraParams != nil {
			delete(chatReq.Params.ExtraParams, "reasoning")
			delete(chatReq.Params.ExtraParams, "reasoning_effort")
			delete(chatReq.Params.ExtraParams, "reasoning_max_tokens")
			delete(chatReq.Params.ExtraParams, "reasoning_display")
		}
	}
}

func forceReasoningEffort(chatReq *schemas.BifrostChatRequest, effort string) {
	if chatReq.Params == nil {
		chatReq.Params = &schemas.ChatParameters{}
	}
	effortCopy := effort
	chatReq.Params.Reasoning = &schemas.ChatReasoning{
		Effort: &effortCopy,
	}
	if chatReq.Params.ExtraParams != nil {
		delete(chatReq.Params.ExtraParams, "reasoning_effort")
		delete(chatReq.Params.ExtraParams, "reasoning_max_tokens")
		delete(chatReq.Params.ExtraParams, "reasoning_display")
	}
}

// normalizeModel strips a provider prefix (e.g. "openai/gpt-4.1-mini" → "gpt-4.1-mini")
// and lowercases the result.
func normalizeModel(model string) string {
	model = strings.TrimSpace(model)
	if i := strings.LastIndex(model, "/"); i >= 0 && i+1 < len(model) {
		model = model[i+1:]
	}
	return strings.ToLower(model)
}

// isUnsupportedModel returns true if model matches any unsupported prefix
// (so dated snapshots like gpt-4.1-mini-2025-04-14 match gpt-4.1-mini).
func isUnsupportedModel(model string, unsupported []string) bool {
	for _, prefix := range unsupported {
		if prefix == "" {
			continue
		}
		if model == prefix || strings.HasPrefix(model, prefix+"-") || strings.HasPrefix(model, prefix+"_") {
			return true
		}
	}
	return false
}

func toStringSlice(raw interface{}) ([]string, error) {
	switch v := raw.(type) {
	case []string:
		return v, nil
	case []interface{}:
		out := make([]string, 0, len(v))
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("element %d is not a string", i)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("expected []string, got %T", raw)
	}
}

func main() {}

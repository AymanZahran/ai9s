package discover

import (
	"strconv"
	"strings"

	"github.com/AymanZahran/ai9s/internal/model"
)

func noteClaudeUsage(u *model.Usage, message map[string]any) {
	usage, _ := message["usage"].(map[string]any)
	if usage == nil {
		return
	}
	in := asInt(usage["input_tokens"])
	cacheRead := asInt(usage["cache_read_input_tokens"])
	cacheWrite := asInt(usage["cache_creation_input_tokens"])
	u.Output += asInt(usage["output_tokens"])
	u.CacheRead += cacheRead
	u.CacheWrite += cacheWrite
	if ctx := in + cacheRead + cacheWrite; ctx > 0 {
		u.Context = ctx
	}
	if details, ok := usage["output_tokens_details"].(map[string]any); ok {
		u.Reasoning += asInt(details["thinking_tokens"])
	}
}

func noteClaudeCost(u *model.Usage, obj map[string]any) {
	if c := asFloat(obj["totalCostUSD"]); c > u.CostUSD {
		u.CostUSD = c
	}
}

// noteTurnUsage adds one turn. Context becomes the latest input plus cache.
// Output, cache, and reasoning accumulate. Cost is added only when the
// object records USD. A credit balance is not a cost.
func noteTurnUsage(u *model.Usage, usage map[string]any, includeCost bool) {
	if u == nil || usage == nil {
		return
	}
	in := num(pick(usage, "input", "input_tokens", "inputTokens"))
	cacheRead := num(pick(usage, "cacheRead", "cache_read", "cache_read_input_tokens", "cacheReadTokens"))
	cacheWrite := num(pick(usage, "cacheWrite", "cache_write", "cache_creation", "cache_creation_input_tokens", "cacheCreationTokens"))
	u.Output += num(pick(usage, "output", "output_tokens", "outputTokens"))
	u.CacheRead += cacheRead
	u.CacheWrite += cacheWrite
	u.Reasoning += num(pick(usage, "reasoning", "reasoning_tokens", "thinkingTokens", "thinking_tokens"))
	if ctx := in + cacheRead + cacheWrite; ctx > 0 {
		u.Context = ctx
	}
	if includeCost {
		if c := costOf(usage["cost"]); c > 0 {
			u.CostUSD += c
		}
	}
}

func pick(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func costOf(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0
		}
		return f
	case map[string]any:
		for _, k := range []string{"total", "totalUSD", "usd"} {
			if raw, ok := t[k]; ok {
				return costOf(raw)
			}
		}
	}
	return 0
}

func noteCodexTokens(u *model.Usage, info map[string]any) {
	if info == nil {
		return
	}
	if n := asInt(info["model_context_window"]); n > 0 {
		u.Window = n
	}
	if total, ok := info["total_token_usage"].(map[string]any); ok {
		u.Input = asInt(total["input_tokens"])
		u.Output = asInt(total["output_tokens"])
		u.CacheRead = asInt(total["cached_input_tokens"])
		u.CacheWrite = asInt(total["cache_write_input_tokens"])
		u.Reasoning = asInt(total["reasoning_output_tokens"])
		u.Total = asInt(total["total_tokens"])
	}
	if last, ok := info["last_token_usage"].(map[string]any); ok {
		ctx := asInt(last["input_tokens"]) + asInt(last["cached_input_tokens"]) + asInt(last["cache_write_input_tokens"])
		if ctx == 0 {
			ctx = asInt(last["total_tokens"])
		}
		if ctx > 0 {
			u.Context = ctx
		}
	}
}

func noteCopilotData(u *model.Usage, data map[string]any) {
	if data == nil {
		return
	}
	if n := asInt(data["totalPremiumRequests"]); n > 0 {
		u.Requests = n
	}
	if effort := asString(data["reasoningEffort"]); effort != "" {
		u.Effort = effort
	}
	harvestCopilot(u, data)
}

func harvestCopilot(u *model.Usage, v any) {
	switch node := v.(type) {
	case map[string]any:
		if _, ok := node["prompt_tokens"]; ok {
			if n := asInt(node["prompt_tokens"]); n > 0 {
				u.Context = n
			}
			if _, ok := node["cache_read"]; ok {
				u.CacheRead = asInt(node["cache_read"])
			}
			if _, ok := node["cache_write"]; ok {
				u.CacheWrite = asInt(node["cache_write"])
			}
		}
		for _, child := range node {
			harvestCopilot(u, child)
		}
	case []any:
		for _, child := range node {
			harvestCopilot(u, child)
		}
	}
}

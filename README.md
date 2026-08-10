# Reasoning Flag Controller

Bifrost Go plugin that controls the Chat Completions `reasoning` flag before the request reaches the provider.

| Model | Action |
|-------|--------|
| All `gpt-4.1*` (`gpt-4.1`, `mini`, `nano`, dated snapshots) | Always strip `reasoning` (whether client sent it or not) |
| All other models | Always force `reasoning.effort=none` (even if client omitted it) |

Flow: **Bifrost → `PreLLMHook` → real provider**. Applies to Chat Completions + stream (`req.ChatRequest`).

## Build

Requires Go **1.26.5** matching your Bifrost binary, and Linux or macOS.

```bash
make deps
make build   # → build/reasoning-flag-controller.so
```

Pin `github.com/maximhq/bifrost/core` in `go.mod` to the **exact** version used by your Bifrost binary:

```bash
go version -m /path/to/bifrost-http | grep bifrost/core
go get github.com/maximhq/bifrost/core@vX.Y.Z
go mod tidy
make build
```

## Configure Bifrost

```json
{
  "plugins": [
    {
      "enabled": true,
      "name": "reasoning-flag-controller",
      "path": "./build/reasoning-flag-controller.so",
      "config": {
        "unsupported_models": ["gpt-4.1"],
        "force_effort": "none"
      }
    }
  ]
}
```

| Config key | Default | Description |
|------------|---------|-------------|
| `unsupported_models` | `["gpt-4.1"]` | Prefix list — models that must not receive reasoning params |
| `force_effort` | `"none"` | Effort value forced on all other models |

Add other non-reasoning models (e.g. `gpt-4o`) to `unsupported_models` if they also reject `reasoning`.

## Test

```bash
make test
```

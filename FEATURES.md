# Features

Open feature requests and follow-up ideas that are real extensions beyond the
current `agent-mux-v2` behavior.

## Proposed

- ~~Role-level skills~~ — **SHIPPED (2026-03-28)**. `Skills []string` on
  `RoleConfig`, merged with CLI `--skill` flags via `mergeSkills()`.

- Gemini adapter: response capture broken
  - Current state: Gemini dispatches return truncated/empty `response` field
    despite generating 1000+ output tokens. `turns: 0`, `tool_calls: []`.
  - Root cause: the adapter calls `gemini -p <prompt> -o stream-json` but
    stdout parsing drops the actual content. Only tail fragments survive.
  - Fix needed: audit `internal/engine/adapter/gemini.go` stream-json
    parsing — the NDJSON scanner likely misses the final response event
    or conflates stream events with the terminal result.

- Gemini adapter: no tool calling
  - Current state: Gemini dispatches produce 0 file reads, 0 commands,
    0 tool calls. The gemini CLI doesn't expose a tool-use surface
    comparable to Codex or Claude.
  - Impact: Gemini variants are reasoning-only (all context must be in
    the prompt). Cannot read files or run commands.
  - Options: (a) accept this as a known limitation and document Gemini
    as prompt-contained-reasoning-only, (b) investigate if `gemini` CLI
    supports tool definitions via config/flags, (c) build a tool-use
    shim that pre-reads files into the prompt context.

- Hanging bash detection (per-command timeout)
  - Current state: only global silence watchdog. A legit 10-min Rust build
    is indistinguishable from a hung curl. No per-command timeout.
  - Proposed: track `tool_start` → `tool_end` pairs in `loop.go`. If
    `tool_end` hasn't arrived in N seconds, emit `long_command_warning`
    event with command name. Optionally classify known-long commands
    (cargo, make, nvcc) for extended grace.
  - Effort: ~40 lines in `loop.go`, no harness changes.

- Session-local daemon / JSON-RPC control plane
  - Current state: one-shot CLI dispatch is enough for basic bot integration.
  - Possible extension: optional per-session daemon with a small JSON-RPC
    surface for centralized streaming/control plus `attach`, `list`,
    `inspect`, and `signal` across multiple live dispatches.
  - Why: not needed now, but potentially useful for Jenkins/operator workflows
    and better caller-death tolerance.

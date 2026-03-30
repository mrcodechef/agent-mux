# Configuration and Roles

TOML config structure, role authoring, variant resolution, and skill injection.

---

## Config File Locations

Loaded in order (later wins on conflicts):

1. **Global:** `~/.agent-mux/config.toml`
2. **Global machine-local:** `~/.agent-mux/config.local.toml`
3. **Project:** `<cwd>/.agent-mux/config.toml`
4. **Project machine-local:** `<cwd>/.agent-mux/config.local.toml`
5. **Explicit:** `--config <path>` (skips implicit lookup above)

`config.local.toml` files are for machine-local secrets and model overrides.
Add them to `.gitignore`.

When `--config` points to a directory, agent-mux looks for
`<dir>/.agent-mux/config.toml` then `<dir>/config.toml`.

Legacy fallback: `~/.config/agent-mux/config.toml` (emits deprecation warning).

---

## Config Structure

```toml
[defaults]
engine = "codex"
model = "gpt-5.4"
effort = "high"
sandbox = "danger-full-access"
response_max_chars = 16000
max_depth = 2
allow_subdispatch = true

[liveness]
heartbeat_interval_sec = 15
silence_warn_seconds = 90
silence_kill_seconds = 180
long_command_silence_seconds = 540

[timeout]
low = 120
medium = 600
high = 1800
xhigh = 2700
grace = 60

[async]
poll_interval = "60s"

[models]
codex = ["gpt-5.4", "gpt-5.4-mini", "gpt-5.3-codex-spark", "gpt-5.2-codex"]
claude = ["claude-opus-4-6", "claude-sonnet-4-6"]
gemini = ["gemini-3-flash-preview", "gemini-3.1-pro-preview", "gemini-2.5-pro", "gemini-2.5-flash"]

[hooks]
deny = ["DROP TABLE", "vault.sh export"]
warn = ["rm -rf", "git push --force", "curl", "wget"]
event_deny_action = "kill"

[skills]
search_paths = ["~/.claude/skills", "~/thinking/pratchett-os/coordinator/.claude/skills"]
```

### Merge behavior

| What | Rule |
|------|------|
| Scalar fields (`engine`, `model`, `timeout`) | Last explicit definition wins |
| `[models].<engine>`, `skills`, `hooks.deny/warn` | Overlay replaces entire list |
| `[roles.<name>.variants.<v>]` | Additive — new variants added, collisions deep-merged |
| `[pipelines.<name>]` | Overlay replaces entire pipeline |

---

## Roles

A role bundles engine, model, effort, timeout, skills, and system prompt.

```toml
[roles.researcher]
engine = "claude"
model = "claude-opus-4-6"
effort = "high"
timeout = 900
skills = ["web-search", "pratchett-read"]
system_prompt_file = "prompts/researcher.md"
```

### Role fields

| Field | Type | Notes |
|-------|------|-------|
| `engine` | string | Override default engine |
| `model` | string | Override default model |
| `effort` | string | Override default effort |
| `timeout` | int | Override timeout (seconds) |
| `skills` | string[] | Skills to inject (merged with CLI `--skill`) |
| `system_prompt_file` | string | Path relative to config dir |

`system_prompt_file` resolves to `<configDir>/prompts/researcher.md`.

---

## Variants

Variants inherit all parent role fields, override only what they set.

```toml
[roles.lifter]
engine = "codex"
model = "gpt-5.4"
effort = "high"
timeout = 1800

[roles.lifter.variants.claude]
engine = "claude"
model = "claude-sonnet-4-6"
# effort and timeout inherited

[roles.lifter.variants.spark]
model = "gpt-5.3-codex-spark"
effort = "medium"
timeout = 600
```

Use variants when task semantics are the same but you want a different
engine/model. Use separate roles when system prompt, skills, or effort
differ fundamentally.

### Live variant table

| Role | Base Engine | Variants |
|------|------------|----------|
| `scout` | codex/gpt-5.4-mini | `gemini` |
| `explorer` | codex/gpt-5.4 | `claude`, `gemini` |
| `researcher` | claude/opus-4-6 | `codex`, `gemini` |
| `architect` | claude/opus-4-6 | `codex` (xhigh), `gemini` |
| `lifter` | codex/gpt-5.4 | `claude`, `gemini`, `mini`, `spark` |
| `lifter-deep` | codex/gpt-5.4 xhigh | `claude`, `gemini` |
| `grunt` | codex/gpt-5.4-mini | `gemini`, `spark` |
| `batch` | codex/gpt-5.4-mini | (none) |
| `auditor` | codex/gpt-5.4 xhigh | `claude`, `gemini` |
| `writer` | codex/gpt-5.4 | (none) |
| `handoff-extractor` | codex/gpt-5.4-mini | `deep`, `claude` |

---

## Profile/Coordinator System

`--profile=name` loads an orchestrator persona from agents/ directories.

### Search order

1. `<cwd>/.claude/agents/<name>.md`
2. `<cwd>/agents/<name>.md`
3. `<cwd>/.agent-mux/agents/<name>.md`
4. `~/.agent-mux/agents/<name>.md`

### Format

```markdown
---
engine: claude
model: claude-opus-4-6
effort: high
timeout: 900
skills: [web-search]
---
You are a senior code reviewer. Focus on correctness and edge cases.
```

If `<name>.toml` exists beside `<name>.md`, it loads as a config overlay.

### Prompt composition order

1. Profile body (from `--profile`)
2. System prompt file content (from `--system-prompt-file`)
3. Inline system prompt text (from `--system-prompt`)

---

## Skill Injection

### Resolution order

1. `<cwd>/.claude/skills/<name>/SKILL.md`
2. `<configDir>/.claude/skills/<name>/SKILL.md`
3. Each path in `[skills] search_paths`: `<path>/<name>/SKILL.md`

First match wins. Tilde expansion (`~`) supported.

### Behavior

- Content wrapped in `<skill name="...">` XML blocks, prepended to prompt
- If `<skillDir>/scripts/` exists, added to Codex `--add-dir`
- Role skills merge with CLI/JSON skills (role first, then explicit)
- Duplicate names deduplicated
- `--skip-skills` bypasses injection while keeping role's engine/model/effort

### Discovering skills

```bash
agent-mux config skills        # tabular: NAME, PATH, SOURCE
agent-mux config skills --json # JSON array
```

---

## Hooks

Pattern-based deny/warn rules on prompts and events.

```toml
[hooks]
deny = ["DROP TABLE", "vault.sh export"]
warn = ["rm -rf", "git push --force"]
event_deny_action = "kill"
```

**Limitation:** Event-level matching can false-positive during harness
orientation (e.g., Codex reading files containing denied patterns). Prompt-level
deny is reliable; event-level deny is experimental.

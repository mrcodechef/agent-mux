# Configuration and Roles

TOML config structure, role authoring, variant resolution, and skill injection.

---

## Config File Locations

Loaded in order (later wins on conflicts):

1. **Global:** `~/.agent-mux/config.toml`
2. **Global machine-local:** `~/.agent-mux/config.local.toml`
3. **Project:** `<cwd>/.agent-mux/config.toml`
4. **Project machine-local:** `<cwd>/.agent-mux/config.local.toml`

`--config <path>` is the sole source when set (skips all implicit lookup).

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
permission_mode = ""
max_depth = 2

[liveness]
heartbeat_interval_sec = 15
silence_warn_seconds = 90
silence_kill_seconds = 180

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

### Section reference

| Section | Fields |
|---------|--------|
| `[defaults]` | `engine`, `model`, `effort`, `sandbox`, `permission_mode`, `max_depth` |
| `[liveness]` | `heartbeat_interval_sec`, `silence_warn_seconds`, `silence_kill_seconds` |
| `[timeout]` | `low`, `medium`, `high`, `xhigh`, `grace` (all seconds, must be > 0) |
| `[async]` | `poll_interval` (Go duration string, e.g. `"60s"`) |
| `[models]` | `<engine> = [...]` model allowlists per engine |
| `[hooks]` | `deny`, `warn` (string arrays), `event_deny_action` (`"kill"`) |
| `[skills]` | `search_paths` (string array, tilde expansion supported) |
| `[roles.<name>]` | Role definitions (see below) |

### Merge behavior

| What | Rule |
|------|------|
| Scalar fields (`engine`, `model`, `effort`, etc.) | Last explicit definition wins |
| `[models].<engine>` | Deduplicated union |
| `skills.search_paths` | Deduplicated union |
| `hooks.deny`, `hooks.warn` | Deduplicated union |
| `[roles.<name>]` | Additive — new roles added, existing roles deep-merged |
| `[roles.<name>.variants.<v>]` | Additive — new variants added, collisions deep-merged |

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
| `timeout` | int | Override timeout in seconds (must be > 0) |
| `skills` | string[] | Skills to inject (merged with CLI `--skill`) |
| `system_prompt_file` | string | Path relative to config source directory |

`system_prompt_file` resolves to `<configDir>/<path>` or
`<configDir>/prompts/<filename>` (fallback when path has no directory component).

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

Variant fields: same as role (`engine`, `model`, `effort`, `timeout`,
`skills`, `system_prompt_file`). Unset fields inherit from parent.

Use variants when task semantics are the same but you want a different
engine/model. Use separate roles when system prompt, skills, or effort
differ fundamentally.

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
2. Role system prompt file (from `system_prompt_file` in role config)
3. CLI system prompt text (from `--system-prompt` / `--system-prompt-file`)

`coordinator` in JSON is an alias for `profile`. If both are set to
different values, agent-mux returns an error.

---

## Skill Injection

### Resolution order

1. `<cwd>/.claude/skills/<name>/SKILL.md`
2. `<configDir>/.claude/skills/<name>/SKILL.md` (if configDir differs from cwd)
3. Each path in `[skills] search_paths`: `<path>/<name>/SKILL.md`

First match wins. Tilde expansion (`~`) supported in search_paths.

### Behavior

- Content wrapped in `<skill name="...">` XML blocks, prepended to prompt
- If `<skillDir>/scripts/` exists, added to `--add-dir` for all engines
- Role skills merge with CLI/JSON skills (CLI skills first, then role skills)
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

- **deny**: Blocks dispatch before launch if prompt or system prompt matches
- **warn**: Logged but not blocking
- **event_deny_action**: `"kill"` — kills harness if a denied pattern appears in events

Hooks also inject a policy instruction into the prompt when rules are configured.

**Limitation:** Event-level matching can false-positive during harness
orientation (e.g., Codex reading files containing denied patterns). Prompt-level
deny is reliable; event-level deny is experimental.

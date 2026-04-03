# Configuration and Roles

TOML config structure, flat role authoring, profile loading, skills, and hooks.

---

## Config File Locations

Implicit config discovery uses exactly two files, later winning on conflict:

1. `~/.agent-mux/config.toml`
2. `<cwd>/.agent-mux/config.toml`

`--config` is the sole config source when set.

There is no XDG fallback, no `config.local.toml`, and no companion merge file.

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
codex = ["gpt-5.4", "gpt-5.4-mini"]
claude = ["claude-opus-4-6", "claude-sonnet-4-6"]
gemini = ["gemini-2.5-pro", "gemini-3.1-pro-preview"]

[hooks]
pre_dispatch = ["~/.agent-mux/hooks/pre-dispatch.sh"]
on_event = ["~/.agent-mux/hooks/on-event.sh"]
event_deny_action = "kill"

[skills]
search_paths = ["~/.claude/skills"]

[roles.lifter]
engine = "codex"
model = "gpt-5.4"
effort = "high"
timeout = 1800
skills = ["agent-mux"]
system_prompt_file = "prompts/lifter.md"

[roles.lifter-claude]
engine = "claude"
model = "claude-sonnet-4-6"
effort = "high"
timeout = 1800
skills = ["agent-mux"]
```

### Section reference

| Section | Fields |
|---------|--------|
| `[defaults]` | `engine`, `model`, `effort`, `sandbox`, `permission_mode`, `max_depth` |
| `[liveness]` | `heartbeat_interval_sec`, `silence_warn_seconds`, `silence_kill_seconds` |
| `[timeout]` | `low`, `medium`, `high`, `xhigh`, `grace` |
| `[async]` | `poll_interval` |
| `[models]` | per-engine model lists |
| `[hooks]` | `pre_dispatch`, `on_event`, `event_deny_action` |
| `[skills]` | `search_paths` |
| `[roles.<name>]` | flat role definitions |

### Merge behavior

| What | Rule |
|------|------|
| Scalar fields | Last explicit definition wins |
| `[models].<engine>` | Deduplicated union |
| `skills.search_paths` | Deduplicated union |
| `hooks.pre_dispatch`, `hooks.on_event` | Deduplicated union |
| `[roles.<name>]` | Additive map; existing roles are field-merged |

Roles are flat. There is no variant resolution or inheritance layer.

---

## Roles

A role bundles engine, model, effort, timeout, skills, and an optional system
prompt file.

```toml
[roles.auditor]
engine = "codex"
model = "gpt-5.4"
effort = "xhigh"
timeout = 1800
skills = ["agent-mux", "review"]
system_prompt_file = "prompts/auditor.md"
```

### Role fields

| Field | Type | Notes |
|-------|------|-------|
| `engine` | string | Engine override |
| `model` | string | Model override |
| `effort` | string | Effort override |
| `timeout` | int | Timeout in seconds |
| `skills` | string[] | Skills to inject |
| `system_prompt_file` | string | Relative to the config source directory |

`system_prompt_file` resolution:

1. `<configDir>/<path>`
2. `<configDir>/prompts/<filename>` when the configured value has no directory

Use a new role name when you want a different engine or model. Example:
`lifter` and `lifter-claude` are separate role definitions, not a role plus a
variant.

---

## Profiles

`--profile=<name>` loads a coordinator persona from one of these locations:

1. `<cwd>/.claude/agents/<name>.md`
2. `<cwd>/agents/<name>.md`
3. `<cwd>/.agent-mux/agents/<name>.md`
4. `~/.agent-mux/agents/<name>.md`

There is no companion TOML overlay beside the profile file.

### Profile format

```markdown
---
engine: claude
model: claude-opus-4-6
effort: high
timeout: 900
skills: [review]
---
You are a code reviewer. Prioritize regressions and edge cases.
```

The profile body becomes the system prompt only when the dispatch did not
already supply an explicit system prompt.

### System prompt composition

Current behavior is:

1. explicit `system_prompt` / CLI system prompt, if present
2. otherwise the profile body, if present
3. then prepend the role `system_prompt_file`, if present

`coordinator` in stdin JSON is an alias for `profile`. Different values cause
an error.

---

## Skill Injection

### Resolution order

1. `<cwd>/.claude/skills/<name>/SKILL.md`
2. `<configDir>/.claude/skills/<name>/SKILL.md` if `configDir != cwd`
3. each `[skills].search_paths` entry: `<path>/<name>/SKILL.md`

First match wins.

### Behavior

- skill content is wrapped in `<skill name="...">` blocks and prepended to the prompt
- if `<skillRoot>/<name>/scripts/` exists, that directory is added to `engine_opts["add-dir"]`
- duplicate skill names are removed
- `--skip-skills` disables skill injection but does not disable role/profile resolution

Discover skills with:

```bash
agent-mux config skills
agent-mux config skills --json
```

---

## Hooks

Hooks are executable scripts configured in `[hooks]`. They are not pattern
lists.

```toml
[hooks]
pre_dispatch = ["~/.agent-mux/hooks/pre-dispatch.sh"]
on_event = ["~/.agent-mux/hooks/on-event.sh"]
event_deny_action = "warn"
```

### pre_dispatch

Each script receives JSON on stdin:

```json
{
  "phase": "pre_dispatch",
  "prompt": "...",
  "system_prompt": "..."
}
```

Environment variables:

- `HOOK_PHASE=pre_dispatch`
- `HOOK_PROMPT`
- `HOOK_SYSTEM_PROMPT`

### on_event

Each script receives JSON on stdin:

```json
{
  "phase": "event",
  "text": "...",
  "command": "...",
  "tool": "...",
  "file_path": "/absolute/path"
}
```

Environment variables:

- `HOOK_PHASE=event`
- `HOOK_COMMAND`
- `HOOK_FILE_PATH`
- `HOOK_TOOL`
- `HOOK_TEXT`

### Exit codes and deny action

| Exit code | Meaning |
|-----------|---------|
| `0` | allow |
| `1` | block |
| `2` | warn |

The script's stderr becomes the reason string.

`event_deny_action` controls what happens when an `on_event` hook returns
`1`:

- `kill`: treat the event as denied and fail the dispatch
- `warn`: downgrade the deny to a warning

`pre_dispatch` exit `1` blocks launch. `pre_dispatch` exit `2` does not block.

### Path handling

- script paths are executed exactly as configured
- `~/` is expanded to the user's home directory
- relative paths are not rebased to the config file directory

Hooks do not inject extra policy text into the prompt. `PromptInjection()`
returns an empty string in the current implementation.

# TOML Config Guide

## Config File Locations

Source of truth: `internal/config/config.go` and its `configPaths()` function.

Implicit discovery checks exactly two files, in order:

1. `~/.agent-mux/config.toml`
2. `<cwd>/.agent-mux/config.toml`

Project config overlays global config. Missing files are skipped.

If `--config <path>` is set, implicit discovery is skipped:

- file path: load that file
- directory path: try `<dir>/.agent-mux/config.toml`, then `<dir>/config.toml`

## Config Structure

```toml
[defaults]
engine = "codex"
model = "gpt-5.4"
effort = "high"
sandbox = "danger-full-access"
permission_mode = ""
max_depth = 2

[skills]
search_paths = ["~/.claude/skills"]

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

[models]
codex = ["gpt-5.4", "gpt-5.4-mini"]
claude = ["claude-sonnet-4-6"]

[hooks]
pre_dispatch = ["./.agent-mux/hooks/pre-dispatch.sh"]
on_event = ["./.agent-mux/hooks/on-event.sh"]
event_deny_action = "kill"

[async]
poll_interval = "60s"

[roles.scout]
engine = "codex"
model = "gpt-5.4-mini"
effort = "low"
timeout = 180
skills = ["repo-map"]
system_prompt_file = "prompts/scout.md"
```

### Section Reference

| Section | Fields |
| --- | --- |
| `[defaults]` | `engine`, `model`, `effort`, `sandbox`, `permission_mode`, `max_depth` |
| `[skills]` | `search_paths` |
| `[liveness]` | `heartbeat_interval_sec`, `silence_warn_seconds`, `silence_kill_seconds` |
| `[timeout]` | `low`, `medium`, `high`, `xhigh`, `grace` |
| `[models]` | `<engine> = [...]` |
| `[hooks]` | `pre_dispatch`, `on_event`, `event_deny_action` |
| `[async]` | `poll_interval` |
| `[roles.<name>]` | Flat role definitions |

### Merge Behavior

| What | Rule |
| --- | --- |
| Scalars in `[defaults]`, `[liveness]`, `[timeout]`, `[async]` | Last explicit definition wins |
| `[skills].search_paths` | Deduplicated append |
| `[models].<engine>` | Deduplicated append |
| `[hooks].pre_dispatch`, `[hooks].on_event` | Deduplicated append |
| `[hooks].event_deny_action` | Last explicit definition wins |
| Existing roles | Deep-merged per field |
| `roles.<name>.skills` | Whole-list replacement when explicitly defined |

## Roles

Source: `config.RoleConfig` in `internal/config/config.go`.

| Field | Type | Notes |
| --- | --- | --- |
| `engine` | string | Override default engine |
| `model` | string | Override default model |
| `effort` | string | Override default effort |
| `timeout` | int | Override timeout in seconds; must be `> 0` when set |
| `skills` | string[] | Skills injected for this role |
| `system_prompt_file` | string | Relative file under the role's config source directory |

Roles are flat entries in `[roles]`. Names like `lifter-claude` are just role names.

`agent-mux config roles` lists those flat names directly from `cfg.Roles`.

`system_prompt_file` must be relative. Absolute paths are rejected. If the value is a bare filename, agent-mux also tries `prompts/<filename>` under the same config source directory.

## Hooks

Source: `internal/hooks/hooks.go`.

Hooks are executable scripts:

```toml
[hooks]
pre_dispatch = ["./.agent-mux/hooks/pre-dispatch.sh"]
on_event = ["./.agent-mux/hooks/on-event.sh"]
event_deny_action = "warn"
```

Behavior:

- Scripts receive JSON on `stdin`.
- Pre-dispatch env vars: `HOOK_PHASE`, `HOOK_PROMPT`, `HOOK_SYSTEM_PROMPT`
- Event env vars: `HOOK_PHASE`, `HOOK_COMMAND`, `HOOK_FILE_PATH`, `HOOK_TOOL`, `HOOK_TEXT`
- Exit `0` allows.
- Exit `1` blocks.
- Exit `2` warns.
- For event hooks, a block becomes `warn` instead of `deny` only when `event_deny_action = "warn"`.

## Profiles

Source: `internal/config/coordinator.go`.

`--profile=name` loads `<name>.md` from:

1. `<cwd>/.claude/agents/`
2. `<cwd>/agents/`
3. `<cwd>/.agent-mux/agents/`
4. `~/.agent-mux/agents/`

Recognized frontmatter fields are `engine`, `model`, `effort`, `skills`, and `timeout`.

Profiles are markdown-only.

## Skill Injection

Source: `internal/config/skills.go`.

Resolution order for `<name>/SKILL.md`:

1. `<cwd>/.claude/skills/`
2. `<configDir>/.claude/skills/` when the role came from a different config tree
3. configured `[skills].search_paths`

First match wins. Search paths support `~` expansion.

Loaded skill content is wrapped in `<skill name="...">` blocks and prepended to the prompt. If a skill directory has `scripts/`, that directory is returned for path injection.

### Discovery Commands

```bash
agent-mux config
agent-mux config --sources
agent-mux config roles
agent-mux config roles --json
agent-mux config models
agent-mux config skills
agent-mux config skills --json
```

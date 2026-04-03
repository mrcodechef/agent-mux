# Configuration

agent-mux loads TOML config from a fixed set of locations and merges the files it finds on top of hardcoded defaults from `DefaultConfig()`. The source of truth for file discovery is `configPaths()` in `internal/config/config.go`.

## Config File Discovery

Implicit discovery checks exactly two paths, in this order:

1. `~/.agent-mux/config.toml`
2. `<cwd>/.agent-mux/config.toml`

Rules:

- Project config overlays global config.
- Missing files are skipped.
- `cwd` is absolutized before project lookup.

If `--config <path>` is set, implicit discovery is skipped and that path becomes the sole config source:

- If `<path>` is a file, agent-mux loads that file.
- If `<path>` is a directory, agent-mux tries `<path>/.agent-mux/config.toml` and then `<path>/config.toml`.

## TOML Schema

Supported top-level sections:

```toml
[defaults]
[skills]
[models]
[liveness]
[timeout]
[hooks]
[async]
[roles.NAME]
```

Example:

```toml
[defaults]
engine = "codex"
model = "gpt-5.4"
effort = "high"
sandbox = "danger-full-access"
max_depth = 2

[skills]
search_paths = ["~/.claude/skills"]

[models]
codex = ["gpt-5.4", "gpt-5.4-mini"]
claude = ["claude-sonnet-4-6"]

[timeout]
low = 120
medium = 600
high = 1800
xhigh = 2700
grace = 60

[liveness]
heartbeat_interval_sec = 15
silence_warn_seconds = 90
silence_kill_seconds = 180

[hooks]
pre_dispatch = ["./.agent-mux/hooks/pre-dispatch.sh"]
on_event = ["./.agent-mux/hooks/on-event.sh"]
event_deny_action = "warn"

[async]
poll_interval = "60s"

[roles.scout]
engine = "codex"
model = "gpt-5.4-mini"
effort = "low"
timeout = 180
skills = ["repo-map"]
system_prompt_file = "prompts/scout.md"

[roles.lifter-claude]
engine = "claude"
model = "claude-sonnet-4-6"
effort = "high"
timeout = 900
system_prompt_file = "prompts/lifter.md"
```

### `[defaults]`

`[defaults]` maps to `DefaultsConfig`:

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `engine` | string | `""` | Default engine |
| `model` | string | `""` | Default model |
| `effort` | string | `"high"` | Default effort |
| `sandbox` | string | `"danger-full-access"` | Default sandbox mode |
| `permission_mode` | string | `""` | Default permission mode |
| `max_depth` | int | `2` | Default recursive dispatch depth |

Defaults apply only when the resolved dispatch still leaves a field unset.

### `[skills]`

`[skills]` maps to:

```go
type SkillsConfig struct {
	SearchPaths []string `toml:"search_paths"`
}
```

`search_paths` is an additive list of extra skill roots. Config merge appends and deduplicates these paths, and `~` expansion is supported.

### `[models]`

`[models]` is `map[string][]string`, keyed by engine. Merge behavior is additive per engine: overlay entries append to the existing list and duplicates are removed in first-seen order.

### `[liveness]`

`[liveness]` maps to `LivenessConfig`:

| Key | Type | Default |
| --- | --- | --- |
| `heartbeat_interval_sec` | int | `15` |
| `silence_warn_seconds` | int | `90` |
| `silence_kill_seconds` | int | `180` |

These values are copied into engine options when the dispatch did not already set them explicitly.

### `[timeout]`

`[timeout]` maps to `TimeoutConfig`:

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `low` | int | `120` | Used for `effort="low"` |
| `medium` | int | `600` | Used for `effort="medium"` |
| `high` | int | `1800` | Used for `effort="high"` and unknown efforts |
| `xhigh` | int | `2700` | Used for `effort="xhigh"` |
| `grace` | int | `60` | Post-timeout grace period |

Explicitly configured timeout values must be greater than zero. That validation also applies to `roles.<name>.timeout`.

### `[hooks]`

`[hooks]` maps to `HooksConfig` in `internal/hooks/hooks.go`:

```go
type HooksConfig struct {
	PreDispatch     []string `toml:"pre_dispatch"`
	OnEvent         []string `toml:"on_event"`
	EventDenyAction string   `toml:"event_deny_action"`
}
```

Example:

```toml
[hooks]
pre_dispatch = ["./.agent-mux/hooks/pre-dispatch.sh"]
on_event = ["./.agent-mux/hooks/on-event.sh"]
event_deny_action = "kill"
```

Rules:

- Hooks are executable scripts referenced by path.
- `pre_dispatch` scripts run before launch.
- `on_event` scripts run for harness events.
- Paths are trimmed, empty entries are ignored, and leading `~/` is expanded.
- `event_deny_action` is normalized to `"warn"` or `"kill"`. Any value other than `"warn"` behaves as `"kill"`.

Hook input:

- Scripts receive JSON on `stdin`.
- Pre-dispatch JSON has `phase`, `prompt`, and `system_prompt`.
- Event JSON has `phase`, `text`, `command`, `tool`, and `file_path`.

Hook environment variables:

- Pre-dispatch: `HOOK_PHASE`, `HOOK_PROMPT`, `HOOK_SYSTEM_PROMPT`
- Event: `HOOK_PHASE`, `HOOK_COMMAND`, `HOOK_FILE_PATH`, `HOOK_TOOL`, `HOOK_TEXT`

Exit codes from `runHook()`:

- `0`: allow
- `1`: block
- `2`: warn

Evaluator behavior:

- Pre-dispatch hooks only block on exit `1`. Exit `2` is treated as a warning result internally but is not surfaced as a prompt warning.
- Event hooks return `warn` on exit `2`.
- Event hooks return `deny` on exit `1` unless `event_deny_action = "warn"`, in which case the result is downgraded to `warn`.
- Non-exit failures default to allow.

Hooks do not inject prompt policy text. `PromptInjection()` currently returns an empty string.

### `[async]`

`[async]` maps to:

```go
type AsyncConfig struct {
	PollInterval string `toml:"poll_interval"`
}
```

`poll_interval` is a Go duration string. `AsyncPollInterval()` falls back to `60s` when the value is empty, invalid, or shorter than `1s`.

## Roles

Roles are flat entries under `[roles]`. `ResolveRole()` does a direct lookup of `cfg.Roles[roleName]` and returns that record.

Role names are just keys. Names such as `lifter-claude` or `grunt-spark` are separate role definitions.

`agent-mux config roles` shows those flat role names exactly as stored in `cfg.Roles`.

### Role Fields

`[roles.<name>]` maps to:

```go
type RoleConfig struct {
	Engine           string   `toml:"engine"`
	Model            string   `toml:"model"`
	Effort           string   `toml:"effort"`
	Timeout          int      `toml:"timeout"`
	Skills           []string `toml:"skills"`
	SystemPromptFile string   `toml:"system_prompt_file"`
	SourceDir        string   `toml:"-"`
}
```

| Field | Type | Notes |
| --- | --- | --- |
| `engine` | string | Role engine |
| `model` | string | Role model |
| `effort` | string | Role effort |
| `timeout` | int | Role timeout in seconds; must be `> 0` when set |
| `skills` | `[]string` | Role skill list |
| `system_prompt_file` | string | Relative path to a prompt file stored with the config |

`system_prompt_file` behavior:

- Absolute paths are rejected.
- The file is resolved relative to the source directory of the config file that defined the role.
- If the configured value has no directory component, agent-mux also tries `prompts/<filename>` under that source directory.

Role skills and request skills are merged at dispatch time with deduplication. Request-supplied skills keep priority order, then role skills are appended.

## Merge Semantics

Config merge is field-wise and defined-wins:

```go
func merge[T comparable](dst *T, value T, defined bool) {
	var zero T
	if defined || value != zero {
		*dst = value
	}
}
```

Meaning:

- An explicitly defined overlay value wins, even when it is `""` or `0`.
- An absent zero value does not clear the earlier value.
- TOML metadata is used to tell the difference.

Conflict rules:

| Section or field | Merge behavior |
| --- | --- |
| Scalars in `[defaults]`, `[liveness]`, `[timeout]`, `[async]` | Last explicit definition wins |
| `[skills].search_paths` | Append and deduplicate |
| `[models].<engine>` | Append and deduplicate |
| `[hooks].pre_dispatch`, `[hooks].on_event` | Append and deduplicate |
| `[hooks].event_deny_action` | Last explicit definition wins |
| New roles | Added by name |
| Existing role scalars | Last explicit definition wins |
| `roles.<name>.skills` | Replace the whole list when explicitly defined |

## Profiles

Profiles are loaded by `LoadProfile(name, cwd)` from markdown files only. Search order for `<name>.md` is:

1. `<cwd>/.claude/agents/`
2. `<cwd>/agents/`
3. `<cwd>/.agent-mux/agents/`
4. `~/.agent-mux/agents/`

Recognized YAML frontmatter fields:

- `engine`
- `model`
- `effort`
- `skills`
- `timeout`

The markdown body becomes `CoordinatorSpec.SystemPrompt`. If `timeout` is explicitly present, it must be greater than zero.

Profiles are markdown-only. Loading a profile reads the matched `<name>.md` file and its frontmatter.

In `--stdin` mode, `coordinator` is accepted as an alias for `profile`.

## Skill Injection

`LoadSkills()` resolves skill names in this order:

1. `<cwd>/.claude/skills/<name>/SKILL.md`
2. `<configDir>/.claude/skills/<name>/SKILL.md`, when `configDir != ""` and `configDir != cwd`
3. Each configured `search_path` as `<search_path>/<name>/SKILL.md`

Behavior:

- First readable match wins.
- Duplicate requested skill names are removed by first occurrence.
- Loaded content is wrapped as `<skill name="NAME"> ... </skill>`.
- If the resolved skill directory contains `scripts/`, that directory is added to the dispatch path list.
- Missing skills produce an error that includes the searched paths and discovered skill names.

## Config Inspection Commands

Useful commands:

```bash
agent-mux config
agent-mux config --sources
agent-mux config roles
agent-mux config roles --json
agent-mux config models
agent-mux config skills
agent-mux config skills --json
```

`agent-mux config roles` reports flat role names from `[roles]`. It does not expand or synthesize names from any overlay system.

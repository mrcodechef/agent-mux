# Configuration Setup Guide

Practical walkthrough for configuring agent-mux. For the schema reference, see [config-guide.md](config-guide.md).

## Directory Structure

```text
.agent-mux/
  config.toml    # roles, models, timeouts, hooks
  prompts/       # role system prompt files
  hooks/         # executable hook scripts
  agents/        # optional profile markdown files
```

Implicit config lookup uses only:

- Global: `~/.agent-mux/config.toml`
- Project: `<cwd>/.agent-mux/config.toml`

Project config overlays global config.

## Getting Started

Create a project config and a prompt file:

```bash
mkdir -p .agent-mux/prompts
```

Write `.agent-mux/config.toml`:

```toml
[defaults]
engine = "codex"
model = "gpt-5.4"
effort = "high"

[roles.scout]
model = "gpt-5.4-mini"
effort = "low"
timeout = 180
system_prompt_file = "prompts/scout.md"
```

Create `.agent-mux/prompts/scout.md`, then verify the resolved dispatch:

```bash
agent-mux preview -R=scout "Find all TODOs in src/"
```

## Global vs Project Config

Resolution order for implicit lookup:

```text
hardcoded defaults
  -> ~/.agent-mux/config.toml
  -> <cwd>/.agent-mux/config.toml
```

If `--config <path>` is set, that path is the only config source:

- file path: load it directly
- directory path: try `.agent-mux/config.toml`, then `config.toml`

Merge rules:

| What | Merge behavior |
| --- | --- |
| Scalar fields | Last explicit definition wins |
| `[models].<engine>` | Append and deduplicate |
| `[skills].search_paths` | Append and deduplicate |
| `[hooks].pre_dispatch`, `[hooks].on_event` | Append and deduplicate |
| `[roles.<name>]` | Deep-merged per field |
| `roles.<name>.skills` | Whole-list replacement when explicitly defined |

Use global config for personal defaults and model lists. Use project config for repo-specific roles, prompts, hooks, and skills.

## Defining Roles

A role bundles engine, model, effort, timeout, skills, and a system prompt file under one name.

```toml
[roles.researcher]
engine = "claude"
model = "claude-sonnet-4-6"
effort = "high"
timeout = 900
skills = ["web-search", "repo-map"]
system_prompt_file = "prompts/researcher.md"
```

`system_prompt_file` must be relative to the config source directory. If it is a bare filename, agent-mux also tries `prompts/<filename>`.

If you want multiple engine-specific presets, define multiple roles:

```toml
[roles.lifter-codex]
engine = "codex"
model = "gpt-5.4"
effort = "high"
timeout = 1800
system_prompt_file = "prompts/lifter.md"

[roles.lifter-claude]
engine = "claude"
model = "claude-sonnet-4-6"
effort = "high"
timeout = 1800
system_prompt_file = "prompts/lifter.md"
```

These are separate roles. There is no base-plus-overlay role mechanism.

## Hooks

Hooks are executable scripts referenced by path in `[hooks]`.

```toml
[hooks]
pre_dispatch = ["./.agent-mux/hooks/pre-dispatch.sh"]
on_event = ["./.agent-mux/hooks/on-event.sh"]
event_deny_action = "kill"
```

Make the scripts executable:

```bash
chmod +x .agent-mux/hooks/pre-dispatch.sh .agent-mux/hooks/on-event.sh
```

Simple pre-dispatch example:

```bash
#!/usr/bin/env bash
set -euo pipefail

if [[ "${HOOK_PROMPT:-}" == *"production database"* ]]; then
  echo "production database work requires manual review" >&2
  exit 1
fi
```

Hook behavior:

- Scripts receive JSON on `stdin`.
- Pre-dispatch env vars include `HOOK_PHASE`, `HOOK_PROMPT`, and `HOOK_SYSTEM_PROMPT`.
- Event env vars include `HOOK_PHASE`, `HOOK_COMMAND`, `HOOK_FILE_PATH`, `HOOK_TOOL`, and `HOOK_TEXT`.
- Exit `0` allows.
- Exit `1` blocks.
- Exit `2` warns.
- For event hooks, `event_deny_action = "warn"` downgrades a block result to a warning.

The hook policy lives in the script.

## Profiles

Profiles are markdown files loaded with `--profile=name`. Search order:

1. `<cwd>/.claude/agents/<name>.md`
2. `<cwd>/agents/<name>.md`
3. `<cwd>/.agent-mux/agents/<name>.md`
4. `~/.agent-mux/agents/<name>.md`

Example:

```markdown
---
engine: claude
model: claude-sonnet-4-6
effort: high
timeout: 900
skills:
  - web-search
---
You are a senior code reviewer. Focus on correctness and test coverage.
```

Profiles are markdown-only.

## Skills Configuration

Configure extra search roots under `[skills]`:

```toml
[skills]
search_paths = ["~/.claude/skills", "/opt/team-skills"]
```

Skill resolution order:

1. `<cwd>/.claude/skills/<name>/SKILL.md`
2. `<configDir>/.claude/skills/<name>/SKILL.md` when `configDir` differs from `cwd`
3. `<search_path>/<name>/SKILL.md`

First match wins.

Inspect the resolved config:

```bash
agent-mux config
agent-mux config --sources
agent-mux config roles
agent-mux config skills
```

`agent-mux config roles` shows flat role names from `[roles]`.

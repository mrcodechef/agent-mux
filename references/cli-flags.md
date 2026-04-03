# CLI Flags and DispatchSpec Reference

This reference mirrors the current command surface in `cmd/agent-mux/main.go`, `cmd/agent-mux/lifecycle.go`, and `internal/types/types.go`.

## Top-Level Commands

Valid commands:

- `dispatch` (default)
- `preview`
- `help`
- `list`
- `status`
- `result`
- `inspect`
- `wait`
- `steer`
- `config`

`--signal`, `--stdin`, and `--version` are control flags, not standalone commands.

## Normal Dispatch Flag Set

These flags are registered when `--stdin` is not enabled.

| Flag | Short | Type | Default | Notes |
| --- | --- | --- | --- | --- |
| `--engine` | `-E` | string | from config | `codex`, `claude`, `gemini` |
| `--role` | `-R` | string | empty | Role name |
| `--profile` |  | string | empty | Profile name |
| `--cwd` | `-C` | string | current dir | Working directory |
| `--model` | `-m` | string | from config | Model override |
| `--effort` | `-e` | string | resolved later | `low`, `medium`, `high`, `xhigh` |
| `--timeout` | `-t` | int | resolved later | Timeout seconds |
| `--system-prompt` | `-s` | string | empty | Inline system prompt |
| `--system-prompt-file` |  | string | empty | System prompt file |
| `--prompt-file` |  | string | empty | Prompt file |
| `--skill` |  | string[] | empty | Repeatable skill name |
| `--context-file` |  | string | empty | Context file |
| `--artifact-dir` |  | string | auto | Runtime artifact directory |
| `--recover` |  | string | empty | Prior dispatch ID |
| `--signal` |  | string | empty | Deliver a signal message |
| `--config` |  | string | empty | Config path override |
| `--full` | `-f` | bool | `true` | Full access mode |
| `--no-full` |  | bool | `false` | Disable full access |
| `--max-depth` |  | int | `2` | Recursive dispatch limit |
| `--skip-skills` |  | bool | `false` | Skip skill injection |
| `--stdin` |  | bool | `false` | Switch to stdin mode |
| `--yes` |  | bool | `false` | Skip confirmation |
| `--version` |  | bool | `false` | Print version |
| `--verbose` | `-v` | bool | `false` | Verbose stderr |
| `--stream` | `-S` | bool | `false` | Stream structured events |
| `--async` |  | bool | `false` | Ack-first async flow |

### Engine-Specific Flags

| Flag | Short | Engine | Type | Default | Notes |
| --- | --- | --- | --- | --- | --- |
| `--sandbox` |  | Codex | string | `danger-full-access` | Sandbox mode |
| `--reasoning` | `-r` | Codex | string | `medium` | Reasoning effort |
| `--add-dir` |  | Codex | string[] | empty | Repeatable writable directory |
| `--permission-mode` |  | Claude/Gemini | string | empty | Permission or approval mode |
| `--max-turns` |  | Claude | int | `0` | Maximum turns |

## `--stdin` Flag Set

When `--stdin` is enabled, agent-mux parses a reduced flag set:

| Flag | Short | Notes |
| --- | --- | --- |
| `--stdin` |  | Enter stdin mode |
| `--yes` | `-y` | Skip confirmation |
| `--verbose` | `-v` | Verbose stderr |
| `--stream` |  | Stream structured events |
| `--async` |  | Ack-first async flow |
| `--config` |  | Config path override |

The normal dispatch flag set is not registered in stdin mode. Most execution settings must be carried in the JSON payload.

## `--stdin` JSON Fields

### Core `DispatchSpec` Keys

Source of truth: `types.DispatchSpec`.

| JSON key | Type | Default when omitted |
| --- | --- | --- |
| `dispatch_id` | `string` | generated ULID |
| `engine` | `string` | resolved later |
| `model` | `string` | resolved later |
| `effort` | `string` | resolved later |
| `prompt` | `string` | required |
| `system_prompt` | `string` | empty |
| `cwd` | `string` | current working directory |
| `artifact_dir` | `string` | `dispatch.DefaultArtifactDir(dispatch_id) + "/"` |
| `context_file` | `string` | empty |
| `timeout_sec` | `int` | resolved later |
| `grace_sec` | `int` | `60` |
| `max_depth` | `int` | resolved later |
| `depth` | `int` | `0` |
| `full_access` | `bool` | `true` |
| `engine_opts` | `map[string]any` | empty |

### Additional Top-Level JSON Keys

These are consumed in stdin mode but are not part of `types.DispatchSpec`:

| JSON key | Meaning |
| --- | --- |
| `role` | Role name |
| `profile` | Profile name |
| `coordinator` | Alias for `profile` |
| `skills` | Explicit skills |
| `skip_skills` | Skip skill injection |
| `recover` | Previous dispatch ID to continue |

`coordinator` and `profile` may both appear only when they agree.

## Command Summary

```text
agent-mux [flags] <prompt>
agent-mux dispatch [flags] <prompt>
agent-mux preview [flags] <prompt>
agent-mux help

agent-mux list [--limit N] [--status <status>] [--engine <engine>] [--json]
agent-mux status <dispatch_id> [--json]
agent-mux result <dispatch_id> [--json] [--artifacts] [--no-wait]
agent-mux inspect <dispatch_id> [--json]
agent-mux wait <dispatch_id> [--json] [--poll 60s] [--config <path>] [--cwd <dir>]

agent-mux steer <dispatch_id> abort
agent-mux steer <dispatch_id> nudge ["message"]
agent-mux steer <dispatch_id> redirect "<instructions>"
agent-mux steer <dispatch_id> extend <seconds>

agent-mux config [--config <path>] [--cwd <dir>]
agent-mux config --sources
agent-mux config roles [--json]
agent-mux config models [--json]
agent-mux config skills [--json]

agent-mux --signal <dispatch_id> "<message>"
agent-mux --stdin < spec.json
agent-mux --version
agent-mux -- help
```

## Persistence Layout

Durable persistence is only:

```text
~/.agent-mux/dispatches/<dispatch_id>/
  meta.json
  result.json
```

Runtime artifacts are separate:

```text
<artifact_dir>/
  _dispatch_ref.json
  status.json
  events.jsonl
  inbox.md
  control.json
  host.pid
  full_output.md
  <worker-created files>
```

`_dispatch_ref.json` replaces the old artifact-local metadata file. It is just a pointer to the durable store.

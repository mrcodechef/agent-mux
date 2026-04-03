# CLI Flags and DispatchSpec Reference

Complete flag table, subcommands, --stdin JSON fields, and precedence rules.

---

## CLI Flags

### Dispatch flags (all engines)

| Flag | Short | Type | Default | Notes |
|------|-------|------|---------|-------|
| `--engine` | `-E` | string | from config | `codex`, `claude`, `gemini` |
| `--role` | `-R` | string | unset | Role name from config.toml |
| `--variant` | | string | unset | Variant within a role (requires `--role`) |
| `--profile` | | string | unset | Coordinator persona (loads agents/<name>.md) |
| `--cwd` | `-C` | string | current dir | Working directory for the harness |
| `--model` | `-m` | string | from role/config | Model override |
| `--effort` | `-e` | string | `high` | `low`, `medium`, `high`, `xhigh` |
| `--timeout` | `-t` | int | effort-mapped | Timeout in seconds |
| `--system-prompt` | `-s` | string | unset | Appended system context |
| `--system-prompt-file` | | string | unset | File loaded as system prompt |
| `--prompt-file` | | string | unset | Prompt from file instead of positional arg |
| `--context-file` | | string | unset | Large context file; injects read preamble |
| `--skill` | | string[] | `[]` | Repeatable; loads SKILL.md from skill dirs |
| `--skip-skills` | | bool | `false` | Skip skill injection (keep role engine/model/effort) |
| `--config` | | string | unset | Explicit config path (overrides default lookup) |
| `--artifact-dir` | | string | auto | Override artifact directory |
| `--recover` | | string | unset | Dispatch ID to continue from |
| `--signal` | | string | unset | Dispatch ID to send a message to |
| `--stream` | `-S` | bool | `false` | Stream all events to stderr |
| `--async` | | bool | `false` | Fire-and-forget; returns ack immediately |
| `--full` | `-f` | bool | `true` | Full access mode (Codex sandbox) |
| `--no-full` | | bool | `false` | Disable full access mode |
| `--max-depth` | | int | `2` | Maximum recursive dispatch depth |
| `--stdin` | | bool | `false` | Read DispatchSpec JSON from stdin |
| `--yes` | | bool | `false` | Skip TTY confirmation |
| `--verbose` | `-v` | bool | `false` | Raw harness lines on stderr |
| `--version` | | bool | | Print version JSON |

### Engine-specific flags

| Flag | Short | Type | Default | Applies to | Notes |
|------|-------|------|---------|-----------|-------|
| `--sandbox` | | string | `danger-full-access` | Codex | `danger-full-access`, `workspace-write`, `read-only` |
| `--reasoning` | `-r` | string | `medium` | Codex | Codex reasoning effort |
| `--add-dir` | | string[] | `[]` | Codex, Claude, Gemini | Repeatable additional writable directories |
| `--permission-mode` | | string | from config | Claude, Gemini | Claude: `default`, `acceptEdits`, `bypassPermissions`, `plan`. Gemini: maps to `--approval-mode` (default `yolo`) |
| `--max-turns` | | int | unset | Claude | Maximum conversation turns |

---

## Subcommands

| Invocation | Purpose |
|------------|---------|
| `agent-mux [flags] <prompt>` | dispatch (default) |
| `agent-mux dispatch [flags] <prompt>` | dispatch (explicit) |
| `agent-mux preview [flags] <prompt>` | resolve without executing |
| `agent-mux config [sub] [flags]` | config introspection |
| `agent-mux list [flags]` | list recent dispatches |
| `agent-mux status <id> [--json]` | single dispatch status |
| `agent-mux result <id> [--json]` | retrieve dispatch response |
| `agent-mux inspect <id> [--json]` | deep dispatch view |
| `agent-mux wait [--poll <dur>] <id>` | block until async dispatch completes |
| `agent-mux steer <id> <action> [args]` | mid-flight steering |
| `agent-mux help` | top-level usage |

### Config subcommands

| Invocation | Purpose |
|------------|---------|
| `agent-mux config` | dump resolved config JSON |
| `agent-mux config --sources` | show loaded config file paths |
| `agent-mux config roles [--json]` | role catalog with engines, models, timeouts |
| `agent-mux config models [--json]` | model allowlist per engine |
| `agent-mux config skills [--json]` | discovered skills with paths and sources |

Config subcommands accept `--config` and `--cwd` for config/project override.

### Lifecycle flags

| Subcommand | Flag | Type | Default | Notes |
|------------|------|------|---------|-------|
| `list` | `--limit` | int | 20 | Max records (0 = all) |
| `list` | `--status` | string | unset | Filter: `completed`, `failed`, `timed_out` |
| `list` | `--engine` | string | unset | Filter: `codex`, `claude`, `gemini` |
| `list` | `--json` | bool | false | NDJSON output |
| `status` | `--json` | bool | false | Full record as JSON |
| `result` | `--json` | bool | false | JSON output |
| `result` | `--artifacts` | bool | false | List artifact files |
| `result` | `--no-wait` | bool | false | Error if still running |
| `inspect` | `--json` | bool | false | Full inspection payload |
| `wait` | `--poll` | string | 60s | Check interval (Go duration: `30s`, `1m`) |
| `wait` | `--json` | bool | false | JSON result when done |
| `wait` | `--config` | string | unset | Config path for poll interval lookup |

### Steer actions

| Action | Args | Notes |
|--------|------|-------|
| `abort` | none | SIGTERM to host PID (async) or control.json (foreground) |
| `nudge` | `[message]` | Default: "Please wrap up your current work and provide a final summary." |
| `redirect` | `"<instructions>"` | Required. Reprioritizes the worker. |
| `extend` | `<seconds>` | Extends watchdog kill threshold. Positive integer. |

---

## DispatchSpec JSON Fields (--stdin)

Pipe a JSON object. `prompt` is required and must be non-empty.

### Core fields

| JSON key | Type | Required | Default | Notes |
|----------|------|----------|---------|-------|
| `prompt` | string | yes | - | The task prompt |
| `cwd` | string | - | shell cwd | Harness working directory |
| `engine` | string | - | from role/config | `codex`, `claude`, `gemini` |
| `model` | string | - | from role/config | Model override |
| `effort` | string | - | `high` | `low`, `medium`, `high`, `xhigh` |
| `system_prompt` | string | - | - | System prompt text |
| `context_file` | string | - | - | Path to large context file |
| `role` | string | - | - | Resolves engine/model/effort/timeout from config |
| `variant` | string | - | - | Variant within a role |
| `profile` | string | - | - | Coordinator persona name |
| `coordinator` | string | - | - | Alias for `profile` (conflicts if both set differently) |
| `skills` | string[] | - | `[]` | Skill names to inject |
| `skip_skills` | bool | - | `false` | Skip skill injection |
| `recover` | string | - | - | Dispatch ID to continue from |

### Control fields

| JSON key | Type | Default | Notes |
|----------|------|---------|-------|
| `dispatch_id` | string | auto ULID | Unique dispatch identifier |
| `artifact_dir` | string | auto | Override artifact directory |
| `timeout_sec` | int | effort-mapped | Override in seconds (must be > 0) |
| `grace_sec` | int | 60 | Grace period in seconds (must be > 0) |
| `max_depth` | int | 2 | Recursive dispatch limit |
| `depth` | int | 0 | Current recursion depth |
| `full_access` | bool | true | Codex sandbox override |
| `engine_opts` | object | `{}` | Per-engine overrides (sandbox, reasoning, permission-mode, add-dir, max-turns) |

### engine_opts keys

| Key | Type | Engine | Notes |
|-----|------|--------|-------|
| `sandbox` | string | Codex | `danger-full-access`, `workspace-write`, `read-only` |
| `reasoning` | string | Codex | Reasoning effort level |
| `permission-mode` | string | Claude, Gemini | Permission/approval mode |
| `max-turns` | int | Claude | Maximum conversation turns |
| `add-dir` | string[] | All | Additional writable directories |

---

## Persistence Paths

| Path | Contents |
|------|----------|
| `~/.agent-mux/dispatches/<dispatch_id>/meta.json` | Dispatch metadata (ID, engine, model, role, cwd, timeout, started_at) |
| `~/.agent-mux/dispatches/<dispatch_id>/result.json` | Full dispatch result with response, activity, metadata |
| `<artifact_dir>/status.json` | Live status (state, elapsed, tools, files changed) |
| `<artifact_dir>/events.jsonl` | NDJSON event log |
| `<artifact_dir>/host.pid` | PID of async dispatch process |
| `<artifact_dir>/control.json` | Steering control (abort, extend_kill_seconds) |
| `<artifact_dir>/inbox.md` | Coordinator mailbox for signal/steer injection |
| `<artifact_dir>/full_output.md` | Full response when truncation occurred |
| `<artifact_dir>/meta.json` | Artifact-local dispatch meta (legacy/runtime) |

Default artifact root: `$XDG_RUNTIME_DIR/agent-mux/<id>/` or `/tmp/agent-mux-<uid>/<id>/`.

---

## Precedence Order

For `engine`, `model`, and `effort`:

```
CLI flags / JSON explicit values
  > --role (resolved from merged TOML config)
  > --profile coordinator frontmatter scalars
  > merged config [defaults]
  > hardcoded defaults (effort="high")
```

For `timeout`:

```
Explicit timeout_sec in JSON / CLI --timeout
  > role.timeout from config
  > profile/coordinator frontmatter timeout
  > timeout table for chosen effort level
```

Config file loading order (later wins on conflicts):

```
~/.agent-mux/config.toml (global)
  > ~/.agent-mux/config.local.toml (global machine-local)
  > <cwd>/.agent-mux/config.toml (project)
  > <cwd>/.agent-mux/config.local.toml (project machine-local)
```

`--config <path>` is the sole source when set (skips implicit lookup).

In `--stdin` mode: JSON payload is the primary source. CLI dispatch flags
are merged only for fields the JSON doesn't set.

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Error (config, dispatch, signal, recovery) |
| `2` | Usage error (bad flags, missing prompt) |
| `130` | Cancelled at TTY confirmation |

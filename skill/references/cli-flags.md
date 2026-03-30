# CLI Flags and DispatchSpec Reference

Complete flag table, --stdin JSON fields, and precedence rules.

---

## CLI Flags

### Common (all engines)

| Flag | Short | Type | Default | Notes |
|------|-------|------|---------|-------|
| `--engine` | `-E` | string | from config | `codex`, `claude`, `gemini` |
| `--cwd` | `-C` | string | current dir | Working directory for the harness |
| `--model` | `-m` | string | from role/config | Model override |
| `--effort` | `-e` | string | `high` | `low`, `medium`, `high`, `xhigh` |
| `--timeout` | `-t` | int | effort-mapped | Timeout in seconds |
| `--role` | `-R` | string | unset | Role name from config.toml |
| `--variant` | | string | unset | Variant within a role |
| `--system-prompt` | `-s` | string | unset | Appended system context |
| `--system-prompt-file` | | string | unset | File loaded as system prompt |
| `--prompt-file` | | string | unset | Prompt from file instead of positional arg |
| `--context-file` | | string | unset | Large context file; injects read preamble |
| `--skill` | | string[] | `[]` | Repeatable; loads SKILL.md from skill dirs |
| `--skip-skills` | | bool | `false` | Skip skill injection (keep role engine/model/effort) |
| `--profile` | | string | unset | Coordinator persona (loads agents/<name>.md) |
| `--coordinator` | | string | unset | Legacy alias for `--profile` |
| `--pipeline` | `-P` | string | unset | Named pipeline from config |
| `--config` | | string | unset | Explicit config path (overrides default lookup) |
| `--artifact-dir` | | string | auto | Override artifact directory |
| `--salt` | | string | auto-generated | Human-readable dispatch salt |
| `--recover` | | string | unset | Dispatch ID to continue from |
| `--signal` | | string | unset | Dispatch ID to send a message to |
| `--stream` | `-S` | bool | `false` | Stream all events to stderr |
| `--async` | | bool | `false` | Fire-and-forget; returns ack immediately |
| `--full` | `-f` | bool | `true` | Full access mode |
| `--no-full` | | bool | `false` | Disable full access |
| `--stdin` | | bool | `false` | Read DispatchSpec JSON from stdin |
| `--max-depth` | | int | `2` | Maximum recursive dispatch depth |
| `--no-subdispatch` | | bool | `false` | Disable recursive dispatch |
| `--verbose` | `-v` | bool | `false` | Raw harness lines on stderr |
| `--yes` | | bool | `false` | Skip TTY confirmation |
| `--version` | | bool | | Print version (no short flag) |

### Codex-specific

| Flag | Short | Type | Default | Notes |
|------|-------|------|---------|-------|
| `--sandbox` | | string | `danger-full-access` | `danger-full-access`, `workspace-write`, `read-only` |
| `--reasoning` | `-r` | string | `medium` | Codex reasoning effort |
| `--add-dir` | | string[] | `[]` | Repeatable additional writable directories |

### Claude-specific

| Flag | Short | Type | Default | Notes |
|------|-------|------|---------|-------|
| `--permission-mode` | | string | from config | `default`, `acceptEdits`, `bypassPermissions`, `plan` |
| `--max-turns` | | int | unset | Maximum conversation turns |

### Gemini-specific

Gemini reuses `--permission-mode` for its `--approval-mode` flag.

---

## DispatchSpec JSON Fields (--stdin)

When using `--stdin`, pipe a JSON object. `prompt` must be non-empty.

### Core fields

| Field | JSON key | Type | Required | Default | Notes |
|-------|----------|------|----------|---------|-------|
| Prompt | `prompt` | string | yes | - | The task prompt |
| Working directory | `cwd` | string | yes | shell cwd | Harness working directory |
| Role | `role` | string | - | - | Resolves engine/model/effort/timeout |
| Variant | `variant` | string | - | - | Engine swap within a role |
| Engine | `engine` | string | role or this | - | `codex`, `claude`, `gemini` |
| Model | `model` | string | - | from role/config | Model override |
| Effort | `effort` | string | - | `high` | `low`, `medium`, `high`, `xhigh` |
| System prompt | `system_prompt` | string | - | - | Appended system context |
| Skills | `skills` | string[] | - | `[]` | Skill names to inject |
| Skip skills | `skip_skills` | bool | - | `false` | Skip skill injection |
| Pipeline | `pipeline` | string | - | - | Named pipeline from config |
| Profile | `profile` | string | - | - | Coordinator persona name |
| Context file | `context_file` | string | - | - | Path to large context file |

### Control fields

| Field | JSON key | Type | Default | Notes |
|-------|----------|------|---------|-------|
| Timeout | `timeout_sec` | int | effort-mapped | Override in seconds |
| Grace period | `grace_sec` | int | 60 | Grace period in seconds |
| Max depth | `max_depth` | int | 2 | Recursive dispatch limit |
| Allow subdispatch | `allow_subdispatch` | bool | true | Recursive dispatch toggle |
| Full access | `full_access` | bool | true | Full filesystem access |
| Salt | `salt` | string | auto | Human-readable identifier |
| Dispatch ID | `dispatch_id` | string | auto ULID | Unique dispatch identifier |
| Artifact dir | `artifact_dir` | string | auto | Override artifact directory |

### Recovery and continuation

| Field | JSON key | Type | Notes |
|-------|----------|------|-------|
| Continue from | `continues_dispatch_id` | string | Prior dispatch ID for recovery |

### Pipeline-internal (set by pipeline orchestrator, not callers)

| Field | JSON key | Type | Notes |
|-------|----------|------|-------|
| Pipeline ID | `pipeline_id` | string | Set for steps within a pipeline |
| Pipeline step | `pipeline_step` | int | Step index within pipeline |
| Parent dispatch | `parent_dispatch_id` | string | Parent dispatch for pipeline steps |
| Receives | `receives` | string | Named output from prior step |
| Pass output as | `pass_output_as` | string | Name for this step's output |
| Parallel | `parallel` | int | Fan-out count |
| Handoff mode | `handoff_mode` | string | `summary_and_refs`, `full_concat`, `refs_only` |
| Depth | `depth` | int | Current recursion depth |

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
  > --config path (explicit overlay)
  > coordinator companion .toml (if --profile is set)
```

In `--stdin` mode, CLI dispatch flags are ignored (warning printed). The JSON
payload is the sole source.

---

## Subcommands

| Invocation | Mode |
|------------|------|
| `agent-mux [flags] <prompt>` | dispatch (default) |
| `agent-mux preview [flags] <prompt>` | preview (resolve without executing) |
| `agent-mux config [sub] [flags]` | config introspection |
| `agent-mux list [flags]` | list recent dispatches |
| `agent-mux status <id>` | single dispatch status |
| `agent-mux result <id>` | retrieve dispatch response |
| `agent-mux inspect <id>` | deep dispatch view |
| `agent-mux wait <id>` | block until async dispatch completes |
| `agent-mux steer <id> <action>` | mid-flight steering |
| `agent-mux gc --older-than <dur>` | garbage-collect old records |

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
| `wait` | `--poll` | string | 60s | Check interval |
| `gc` | `--older-than` | string | required | Duration: `7d`, `24h` |
| `gc` | `--dry-run` | bool | false | Preview without deleting |

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Error (config, dispatch, signal, recovery) |
| `2` | Usage error (bad flags, missing prompt) |
| `130` | Cancelled at TTY confirmation |

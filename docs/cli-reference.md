# CLI Reference

This page is the command and flag reference for the current CLI surface.

## Valid Commands

The top-level command set is:

- `dispatch` (default when no explicit command is given)
- `preview`
- `help`
- `list`
- `status`
- `result`
- `inspect`
- `wait`
- `steer`
- `config`

`--signal`, `--stdin`, and `--version` are control flags on the default dispatch path, not standalone commands.

## Dispatch Flags

These flags are registered in normal dispatch mode.

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
| `--prompt-file` |  | string | empty | Prompt file instead of positional prompt |
| `--skill` |  | string[] | empty | Repeatable skill name |
| `--context-file` |  | string | empty | Context file path |
| `--artifact-dir` |  | string | auto | Runtime artifact directory |
| `--recover` |  | string | empty | Previous dispatch ID to continue |
| `--signal` |  | string | empty | Deliver a message to a running dispatch |
| `--config` |  | string | empty | Config path override |
| `--full` | `-f` | bool | `true` | Full access mode |
| `--no-full` |  | bool | `false` | Disable full access |
| `--max-depth` |  | int | `2` | Recursive dispatch limit |
| `--skip-skills` |  | bool | `false` | Skip skill injection |
| `--stdin` |  | bool | `false` | Read dispatch JSON from stdin |
| `--yes` |  | bool | `false` | Skip interactive confirmation |
| `--version` |  | bool | `false` | Print version |
| `--verbose` | `-v` | bool | `false` | Raw harness lines plus events on stderr |
| `--stream` | `-S` | bool | `false` | All structured events on stderr |
| `--async` |  | bool | `false` | Ack-first async flow; does not daemonize |

## Engine-Specific Flags

| Flag | Short | Engine | Type | Default | Notes |
| --- | --- | --- | --- | --- | --- |
| `--sandbox` |  | Codex | string | `danger-full-access` | Sandbox mode |
| `--reasoning` | `-r` | Codex | string | `medium` | Reasoning effort |
| `--add-dir` |  | Codex | string[] | empty | Repeatable writable directory |
| `--permission-mode` |  | Claude/Gemini | string | empty | Adapter-specific permission or approval mode |
| `--max-turns` |  | Claude | int | `0` | Maximum turns |

## `--stdin` Mode

When `--stdin` is enabled, the CLI uses a reduced flag set:

| Flag | Short | Notes |
| --- | --- | --- |
| `--stdin` |  | Required to enter stdin mode |
| `--yes` | `-y` | Skip confirmation |
| `--verbose` | `-v` | Verbose stderr |
| `--stream` |  | Stream structured events |
| `--async` |  | Async ack-first execution |
| `--config` |  | Config path override |

`--stdin` mode does not register the normal dispatch flag set. The dispatch payload itself carries the execution fields.

### Core JSON Fields

These keys map to `types.DispatchSpec`:

- `dispatch_id`
- `engine`
- `model`
- `effort`
- `prompt`
- `system_prompt`
- `cwd`
- `artifact_dir`
- `context_file`
- `timeout_sec`
- `grace_sec`
- `max_depth`
- `depth`
- `full_access`
- `engine_opts`

### Additional JSON Keys Consumed by `main.go`

- `role`
- `profile`
- `coordinator`
- `skills`
- `skip_skills`
- `recover`

Defaults when omitted:

- `dispatch_id`: generated ULID
- `cwd`: current working directory
- `artifact_dir`: `dispatch.DefaultArtifactDir(dispatch_id) + "/"`
- `full_access`: `true`
- `grace_sec`: `60`

`prompt` is required. `coordinator` is accepted as an alias for `profile`; conflicting values are rejected.

## `preview`

```bash
agent-mux preview [dispatch flags] <prompt>
```

`preview` resolves the dispatch without executing it and emits:

- `dispatch_spec`
- `result_metadata`
- `prompt`
- `control`
- `prompt_preamble`
- `warnings`
- `confirmation_required`

`result_metadata` currently contains `role`, `profile`, and `skills`.

## `config`

```bash
agent-mux config [--config <path>] [--cwd <dir>]
agent-mux config --sources
agent-mux config roles [--json]
agent-mux config models [--json]
agent-mux config skills [--json]
```

Implemented subcommands are exactly:

- `config`
- `config roles`
- `config models`
- `config skills`

`config roles` reports one entry per role. The current command does not emit a variant table.

## Mode Detection

| Invocation | Behavior |
| --- | --- |
| `agent-mux` | top-level help |
| `agent-mux help` | top-level help |
| `agent-mux [flags] <prompt>` | dispatch |
| `agent-mux dispatch [flags] <prompt>` | dispatch |
| `agent-mux preview [flags] <prompt>` | preview |
| `agent-mux list ...` | lifecycle list |
| `agent-mux status <id>` | lifecycle status |
| `agent-mux result <id>` | lifecycle result |
| `agent-mux inspect <id>` | lifecycle inspect |
| `agent-mux wait <id>` | lifecycle wait |
| `agent-mux steer <id> <action> ...` | steering |
| `agent-mux config ...` | config introspection |
| `agent-mux --signal <id> "<message>"` | inbox signal path |
| `agent-mux --stdin < spec.json` | stdin dispatch |
| `agent-mux --version` | version output |
| `agent-mux -- help` | literal prompt `help` |

## Exit Codes

| Code | Meaning |
| --- | --- |
| `0` | Success |
| `1` | Runtime, config, lifecycle, signal, or recovery failure |
| `2` | Usage error |
| `130` | Cancelled at the interactive confirmation prompt |

## Cross-References

- [dispatch.md](./dispatch.md) for `DispatchSpec` and `DispatchResult`
- [async.md](./async.md) for `--async` semantics
- [lifecycle.md](./lifecycle.md) for lifecycle behavior
- [recovery.md](./recovery.md) for persistence and recovery

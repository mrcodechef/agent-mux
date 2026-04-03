# Output Contract

JSON schemas for dispatch results, async ack, preview, lifecycle, and errors.

---

## DispatchResult JSON

Normal dispatch writes one JSON object to `stdout`:

```json
{
  "schema_version": 1,
  "status": "completed",
  "dispatch_id": "01KM...",
  "response": "Worker response text",
  "response_truncated": false,
  "full_output": null,
  "full_output_path": "/path/to/full_output.md",
  "handoff_summary": "Short summary for handoff",
  "artifacts": ["/tmp/agent-mux-501/01KM.../notes.md"],
  "error": null,
  "activity": {
    "files_changed": ["src/parser.go"],
    "files_read": ["src/types.go"],
    "commands_run": ["go test ./..."],
    "tool_calls": ["Read", "Edit", "Bash"]
  },
  "metadata": {
    "engine": "codex",
    "model": "gpt-5.4",
    "role": "lifter",
    "variant": "",
    "profile": "",
    "skills": ["agent-mux"],
    "tokens": { "input": 1234, "output": 567, "reasoning": 89, "cache_read": 0, "cache_write": 0 },
    "turns": 3,
    "cost_usd": 0,
    "session_id": "thread_..."
  },
  "duration_ms": 84231
}
```

### Status values

| `status` | Meaning |
|----------|---------|
| `completed` | Worker exited cleanly (including clean exit during grace) |
| `timed_out` | Soft timeout fired, grace expired, harness stopped |
| `failed` | Validation error, startup problem, adapter failure, or policy denial |

### Top-level fields

| Field | Type | Notes |
|-------|------|-------|
| `schema_version` | int | Always `1` |
| `status` | string | `completed`, `timed_out`, `failed` |
| `dispatch_id` | string | ULID for this run |
| `response` | string | Final response text |
| `response_truncated` | bool | True if response was shortened |
| `full_output` | string/null | Full response text (when available inline) |
| `full_output_path` | string/null | Path to full output file |
| `handoff_summary` | string | Extracted from `## Summary`/`## Handoff` |
| `artifacts` | string[] | Files in artifact dir (excludes internals) |
| `partial` | bool | Present on timed-out runs |
| `recoverable` | bool | Present on timed-out runs |
| `reason` | string | Reason for non-completed status |
| `error` | object/null | Present on failed runs |
| `activity` | object | Files/commands/tool calls observed |
| `metadata` | object | Engine, model, tokens, session info |
| `duration_ms` | int | End-to-end duration in milliseconds |

### DispatchError object

Present when `status` is `failed`:

```json
{
  "code": "engine_not_found",
  "message": "Engine \"bogus\" not found.",
  "hint": "The selected engine is not available.",
  "example": "agent-mux -E=codex -m=gpt-5.4 \"<prompt>\"",
  "retryable": true,
  "partial_artifacts": []
}
```

| Field | Type | Notes |
|-------|------|-------|
| `code` | string | Machine-readable error code |
| `message` | string | Human-readable description |
| `hint` | string | Guidance on what went wrong |
| `example` | string | Example of correct invocation |
| `retryable` | bool | Whether retry may succeed |
| `partial_artifacts` | string[] | Files written before failure |

### Activity object

| Field | Type | Notes |
|-------|------|-------|
| `files_changed` | string[] | Files the worker wrote |
| `files_read` | string[] | Files the worker read |
| `commands_run` | string[] | Shell commands executed |
| `tool_calls` | string[] | Tool names invoked |

### Metadata object

| Field | Type | Notes |
|-------|------|-------|
| `engine` | string | Engine used |
| `model` | string | Model used |
| `role` | string | Role name (if set) |
| `variant` | string | Variant name (if set) |
| `profile` | string | Profile name (if set) |
| `skills` | string[] | Skills injected |
| `tokens` | object | Token usage breakdown |
| `turns` | int | Conversation turns |
| `cost_usd` | float | Estimated cost |
| `session_id` | string | Harness session ID |

### TokenUsage object

| Field | Type | Notes |
|-------|------|-------|
| `input` | int | Input tokens |
| `output` | int | Output tokens |
| `reasoning` | int | Reasoning tokens (Codex only) |
| `cache_read` | int | Cache read tokens (Claude only) |
| `cache_write` | int | Cache write tokens (Claude only) |

---

## Async Ack

When `--async` is set, stdout receives the ack before the worker starts:

```json
{
  "schema_version": 1,
  "kind": "async_started",
  "dispatch_id": "01KMY...",
  "artifact_dir": "/tmp/agent-mux-501/01KMY..."
}
```

`host.pid` and `status.json` are guaranteed on-disk before this ack is emitted.

---

## Preview Output

`agent-mux preview` returns the resolved dispatch shape without executing:

```json
{
  "schema_version": 1,
  "kind": "preview",
  "dispatch_spec": {
    "dispatch_id": "01KM...",
    "engine": "codex",
    "model": "gpt-5.4",
    "effort": "high",
    "cwd": "/repo",
    "artifact_dir": "/tmp/agent-mux-501/01KM.../",
    "timeout_sec": 1800,
    "grace_sec": 60,
    "max_depth": 2,
    "depth": 0,
    "full_access": true
  },
  "result_metadata": {
    "role": "lifter",
    "variant": "",
    "profile": "",
    "skills": ["agent-mux"]
  },
  "prompt": {
    "excerpt": "First 280 chars of prompt...",
    "chars": 1500,
    "truncated": true,
    "system_prompt_chars": 200
  },
  "control": {
    "control_record": "~/.agent-mux/dispatches/01KM.../meta.json",
    "artifact_dir": "/tmp/agent-mux-501/01KM.../"
  },
  "prompt_preamble": ["Relevant context from the coordinator is at $AGENT_MUX_CONTEXT. Read it before starting."],
  "warnings": [],
  "confirmation_required": false
}
```

---

## LiveStatus (status.json)

Written atomically to `<artifact_dir>/status.json` during dispatch:

```json
{
  "state": "running",
  "elapsed_s": 45,
  "last_activity": "tool:Edit",
  "tools_used": 12,
  "files_changed": 3,
  "stdin_pipe_ready": true,
  "ts": "2026-04-03T10:30:00Z",
  "dispatch_id": "01KMY...",
  "session_id": "thread_..."
}
```

| Field | Type | Notes |
|-------|------|-------|
| `state` | string | `running`, `initializing`, `completed`, `failed`, `timed_out`, `orphaned` |
| `elapsed_s` | int | Seconds since dispatch start |
| `last_activity` | string | Most recent activity description |
| `tools_used` | int | Tool calls observed |
| `files_changed` | int | File writes observed |
| `stdin_pipe_ready` | bool | Whether Codex stdin FIFO is open |
| `ts` | string | ISO 8601 timestamp of this update |
| `dispatch_id` | string | Dispatch ID |
| `session_id` | string | Harness session ID |

---

## Signal Ack

`--signal <id> "message"` writes to inbox and returns:

```json
{"status":"ok","dispatch_id":"01KM...","artifact_dir":"/tmp/agent-mux-501/01KM...","message":"Signal delivered to inbox"}
```

Error:
```json
{"status":"error","dispatch_id":"01KM...","message":"...","error":{"code":"recovery_failed","message":"...","hint":"...","example":"","retryable":false}}
```

---

## Lifecycle Subcommand JSON

Pass `--json` for machine-parseable output. Default is tabular.

- **list --json**: NDJSON, one `DispatchRecord` per line
- **status --json**: Single `LiveStatus` or `DispatchRecord` object
- **result --json**: `{"dispatch_id":"...","response":"...","status":"...","session_id":"..."}`
- **inspect --json**: Combined record + response + artifacts + meta + session_id

---

## Event Types

Events in `events.jsonl` and stderr (when `--stream` enabled).
All events carry: `schema_version`, `type`, `dispatch_id`, `ts`.

| Type | Key fields |
|------|------------|
| `dispatch_start` | `engine`, `model`, `effort`, `timeout_sec`, `grace_sec`, `cwd` |
| `dispatch_end` | `status`, `duration_ms` |
| `heartbeat` | `elapsed_s`, `interval_s`, `last_activity` |
| `tool_start` / `tool_end` | `tool`, `args` / `duration_ms` |
| `file_write` / `file_read` | `path` |
| `command_run` | `command` |
| `progress` | `message` |
| `timeout_warning` | `message` |
| `frozen_warning` | `silence_seconds`, `message` |
| `long_command_detected` | `command`, `timeout_seconds`, `message` |
| `response_truncated` | `full_output_path` |
| `error` | `error_code`, `message` |
| `info` | `error_code`, `message` |

Silent mode (default): only `dispatch_start`, `dispatch_end`, `error`,
`frozen_warning`, `frozen_killed`, `timeout_warning`, `long_command_detected`,
`response_truncated`, `preview` emitted to stderr. All events always written
to `events.jsonl`.

---

## Error Codes

| Code | Retryable | Meaning |
|------|-----------|---------|
| `abort_requested` | no | Dispatch aborted via steer or control file |
| `artifact_dir_unwritable` | no | Cannot create/write artifact directory |
| `binary_not_found` | yes | Harness binary not found on PATH |
| `cancelled` | no | Dispatch cancelled before launch at confirmation |
| `config_error` | yes | Config loading or validation failure |
| `engine_not_found` | yes | Unknown engine name |
| `event_denied` | no | Hook denied a harness event |
| `frozen_killed` | no | Harness killed after prolonged silence |
| `internal_error` | no | agent-mux hit an internal invariant failure |
| `interrupted` | no | Context cancelled or signal received |
| `invalid_args` | yes | Invalid arguments or missing required fields |
| `invalid_input` | yes | Input failed validation |
| `max_depth_exceeded` | no | Recursive dispatch depth limit hit |
| `model_not_found` | yes | Unknown model for engine |
| `output_parse_error` | no | Failed to parse streaming harness output |
| `parse_error` | no | Malformed final harness output |
| `process_killed` | no | Harness process killed (generic fallback) |
| `prompt_denied` | no | Hook denied the prompt before launch |
| `recovery_failed` | yes | Existing dispatch state could not be recovered |
| `resume_session_missing` | no | No session ID available for resume |
| `resume_start_failed` | yes | Resume process failed to start |
| `resume_unsupported` | no | Engine does not support resume |
| `signal_killed` | no | Harness killed by OS signal (exit 137/143) |
| `startup_failed` | yes | Harness binary failed to start |

Harness-native codes: Codex `context_length_exceeded`, Claude `result_error`, Gemini `tool_error`.

# Output Contract

This page documents the structured outputs emitted by the current CLI.

## Dispatch Result JSON

Normal dispatch writes one `DispatchResult` object to stdout.

Source of truth: `types.DispatchResult` in `internal/types/types.go`.

```json
{
  "schema_version": 1,
  "status": "completed",
  "dispatch_id": "01K...",
  "response": "Worker response text",
  "response_truncated": false,
  "handoff_summary": "Short summary",
  "artifacts": ["/path/to/artifacts/01K.../notes.md"],
  "partial": false,
  "recoverable": false,
  "reason": "",
  "error": null,
  "activity": {
    "files_changed": [],
    "files_read": [],
    "commands_run": [],
    "tool_calls": []
  },
  "metadata": {
    "engine": "codex",
    "model": "gpt-5.4",
    "role": "lifter",
    "profile": "",
    "skills": [],
    "tokens": {
      "input": 0,
      "output": 0
    },
    "turns": 0,
    "cost_usd": 0,
    "session_id": "thread_..."
  },
  "duration_ms": 84231
}
```

### Top-Level Fields

| Field | Type | Notes |
| --- | --- | --- |
| `schema_version` | int | Always `1` |
| `status` | string | `completed`, `timed_out`, `failed` |
| `dispatch_id` | string | Dispatch identifier |
| `response` | string | Final response text |
| `response_truncated` | bool | Compatibility field; usually `false` |
| `full_output` | string or null | Legacy compatibility field |
| `full_output_path` | string or null | Deprecated legacy stub; do not treat as the active persistence contract |
| `handoff_summary` | string | Extracted handoff summary |
| `artifacts` | string[] | Non-internal artifact paths |
| `partial` | bool | Partial result marker |
| `recoverable` | bool | Recovery hint |
| `reason` | string | Terminal reason |
| `error` | object or null | Structured error |
| `activity` | object | Files, commands, and tools observed |
| `metadata` | object | Engine/model/session metadata |
| `duration_ms` | int64 | End-to-end duration |

### Metadata Fields

| Field | Type | Notes |
| --- | --- | --- |
| `engine` | string | Final engine |
| `model` | string | Final model |
| `role` | string | Resolved role |
| `profile` | string | Resolved profile |
| `skills` | string[] | Injected skills |
| `tokens` | object | Best-effort token accounting |
| `turns` | int | Best-effort turn count |
| `cost_usd` | float | Best-effort cost |
| `session_id` | string | Harness session ID |

## Persistent Store Files

The durable store is:

```text
~/.agent-mux/dispatches/<dispatch_id>/
  meta.json
  result.json
```

`result.json` is `PersistentDispatchResult`: it embeds the `DispatchResult` fields above and adds persisted context such as `started_at`, `ended_at`, `artifact_dir`, `cwd`, `engine`, `model`, `role`, `profile`, `effort`, `session_id`, `response_chars`, and `timeout_sec`.

## Preview Output

`agent-mux preview` emits a `previewResult`.

```json
{
  "schema_version": 1,
  "kind": "preview",
  "dispatch_spec": {
    "dispatch_id": "01K...",
    "engine": "codex",
    "model": "gpt-5.4",
    "effort": "high",
    "cwd": "/repo",
    "context_file": "/repo/context.md",
    "artifact_dir": "/path/to/artifacts/01K.../",
    "timeout_sec": 1800,
    "grace_sec": 60,
    "max_depth": 2,
    "depth": 0,
    "full_access": true
  },
  "result_metadata": {
    "role": "lifter",
    "profile": "",
    "skills": []
  },
  "prompt": {
    "excerpt": "Explain what you would change",
    "chars": 29,
    "truncated": false,
    "system_prompt_chars": 0
  },
  "control": {
    "control_record": "/home/user/.agent-mux/dispatches/01K.../meta.json",
    "artifact_dir": "/path/to/artifacts/01K.../"
  },
  "prompt_preamble": [],
  "warnings": [],
  "confirmation_required": false
}
```

## Async Ack

`--async` emits:

```json
{
  "schema_version": 1,
  "kind": "async_started",
  "dispatch_id": "01K...",
  "artifact_dir": "/path/to/artifacts/01K.../"
}
```

At ack time, `host.pid` and `status.json` already exist. The process keeps running the dispatch in the current process after the ack; the caller must background or supervise it if it wants immediate control return.

## Live `status.json`

Source of truth: `dispatch.LiveStatus`.

```json
{
  "state": "running",
  "elapsed_s": 42,
  "last_activity": "tool_call: Bash",
  "tools_used": 7,
  "files_changed": 3,
  "stdin_pipe_ready": true,
  "ts": "2026-04-03T10:00:42Z",
  "dispatch_id": "01K...",
  "session_id": "thread_..."
}
```

| Field | Type | Notes |
| --- | --- | --- |
| `state` | string | `running`, `completed`, `failed`, `timed_out`; `status` may synthesize `orphaned` |
| `elapsed_s` | int | Seconds since start |
| `last_activity` | string | Last activity label |
| `tools_used` | int | Tool-call count |
| `files_changed` | int | File-write count |
| `stdin_pipe_ready` | bool | Present when Codex soft steering is ready |
| `ts` | string | RFC3339 timestamp |
| `dispatch_id` | string | Dispatch ID |
| `session_id` | string | Harness session ID |

## Lifecycle JSON

Lifecycle successes are thin JSON wrappers, not raw `DispatchResult` objects.

### `list --json`

Emits NDJSON `DispatchRecord` entries, one per line.

Example:

```json
{"id":"01K...","session_id":"thread_...","status":"completed","engine":"codex","model":"gpt-5.4","role":"lifter","started":"2026-04-03T10:00:00Z","ended":"2026-04-03T10:01:24Z","duration_ms":84231,"cwd":"/repo","truncated":false,"response_chars":1250,"artifact_dir":"/path/to/artifacts/01K.../","effort":"high","profile":"","timeout_sec":1800}
```

### `status --json`

- completed dispatch: a `DispatchRecord`
- live dispatch: a `LiveStatus`

### `result --json`

Example:

```json
{
  "dispatch_id": "01K...",
  "response": "Worker response text...",
  "status": "completed",
  "session_id": "thread_..."
}
```

When `--artifacts` is set:

```json
{
  "dispatch_id": "01K...",
  "artifact_dir": "/path/to/artifacts/01K.../",
  "artifacts": ["notes.md"]
}
```

When `--no-wait` is used against a live dispatch:

```json
{
  "error": "dispatch_running",
  "dispatch_id": "01K...",
  "session_id": "thread_...",
  "state": "running"
}
```

### `inspect --json`

```json
{
  "dispatch_id": "01K...",
  "session_id": "thread_...",
  "record": {"id":"01K...","status":"completed","engine":"codex","model":"gpt-5.4"},
  "response": "Worker response text...",
  "artifact_dir": "/path/to/artifacts/01K.../",
  "artifacts": ["notes.md"],
  "meta": {"dispatch_id":"01K...","engine":"codex","model":"gpt-5.4"}
}
```

### `wait --json`

On success, `wait --json` returns the same shape as `result --json`.

### Lifecycle Errors

Failures use the common envelope:

```json
{
  "kind": "error",
  "error": {
    "code": "not_found",
    "message": "no dispatch found for reference \"01K...\"",
    "hint": "",
    "example": "",
    "retryable": true,
    "partial_artifacts": []
  }
}
```

## Control-Path Responses

### `--signal`

Success:

```json
{
  "status": "ok",
  "dispatch_id": "01K...",
  "artifact_dir": "/path/to/artifacts/01K.../",
  "message": "Signal delivered to inbox"
}
```

Failure:

```json
{
  "status": "error",
  "dispatch_id": "01K...",
  "message": "invalid dispatch_id: ...",
  "error": {
    "code": "invalid_input",
    "message": "invalid dispatch_id: ...",
    "hint": "Provide a dispatch ID without path separators or traversal segments.",
    "example": "",
    "retryable": true,
    "partial_artifacts": []
  }
}
```

### `steer`

Examples:

```json
{"action":"abort","dispatch_id":"01K...","mechanism":"sigterm","pid":12345,"delivered":true}
```

```json
{"action":"nudge","dispatch_id":"01K...","mechanism":"stdin_fifo","delivered":true}
```

```json
{"action":"extend","dispatch_id":"01K...","seconds":300,"delivered":true}
```

### `--version`

```json
{"version":"agent-mux v3.2.0"}
```

## stderr Event Stream

Structured events use the shared envelope:

```json
{
  "schema_version": 1,
  "type": "dispatch_start",
  "dispatch_id": "01K...",
  "ts": "2026-04-03T10:00:00Z"
}
```

Common fields by event type:

| Type | Extra fields commonly present |
| --- | --- |
| `dispatch_start` | `engine`, `model`, `effort`, `timeout_sec`, `grace_sec`, `cwd` |
| `dispatch_end` | `status`, `duration_ms` |
| `heartbeat` | `elapsed_s`, `interval_s`, `last_activity` |
| `tool_start` | `tool`, `args` |
| `tool_end` | `tool`, `duration_ms` |
| `file_write` | `path` |
| `file_read` | `path` |
| `command_run` | `command` |
| `progress` | `message` |
| `timeout_warning` | `message` |
| `frozen_warning` | `silence_seconds`, `message` |
| `long_command_detected` | `command`, `timeout_seconds`, `message` |
| `info` | `error_code`, `message` |
| `warning` | `error_code`, `message` |
| `error` | `error_code`, `message` |
| `coordinator_inject` | `message` |
| `response_truncated` | `full_output_path` |

`response_truncated.full_output_path` is a compatibility path for legacy truncation handling. The field exists, but current documentation should not treat it as the primary result contract.

## Error Codes

Common built-in codes include:

- `abort_requested`
- `artifact_dir_unwritable`
- `binary_not_found`
- `cancelled`
- `config_error`
- `engine_not_found`
- `event_denied`
- `frozen_killed`
- `internal_error`
- `interrupted`
- `invalid_args`
- `invalid_input`
- `max_depth_exceeded`
- `model_not_found`
- `output_parse_error`
- `parse_error`
- `process_killed`
- `prompt_denied`
- `recovery_failed`
- `resume_session_missing`
- `resume_start_failed`
- `resume_unsupported`
- `signal_killed`
- `startup_failed`

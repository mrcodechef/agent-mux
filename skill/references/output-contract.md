# Output Contract

JSON schemas for dispatch results, pipeline results, and control-path responses.

---

## Single Dispatch JSON

Normal dispatch writes one JSON object to `stdout`:

```json
{
  "schema_version": 1,
  "status": "completed",
  "dispatch_id": "01KM...",
  "dispatch_salt": "mint-ant-five",
  "trace_token": "AGENT_MUX_GO_01KM...",
  "response": "Worker response text",
  "response_truncated": false,
  "full_output_path": null,
  "handoff_summary": "Short summary for handoff",
  "artifacts": ["/tmp/agent-mux-501/01KM.../notes.md"],
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
| `dispatch_salt` | string | Human-greppable `adjective-noun-digit` salt |
| `response` | string | Final response text (may be truncated) |
| `response_truncated` | bool | True when shortened to `response_max_chars` |
| `full_output_path` | string/null | Path to full output file when truncated |
| `handoff_summary` | string | Extracted from `## Summary`/`## Handoff` |
| `artifacts` | string[] | Files in artifact dir (excludes internals) |
| `partial` | bool | Present on timed-out runs |
| `recoverable` | bool | Present on timed-out runs |
| `error` | object/null | Present on failed runs |
| `activity` | object | Files/commands/tool calls observed |
| `metadata` | object | Engine, model, tokens, session info |
| `duration_ms` | int | End-to-end duration in milliseconds |

### Error object (failed only)

```json
{
  "code": "engine_not_found",
  "message": "Engine \"bogus\" not found.",
  "suggestion": "Valid engines: [codex, claude, gemini]",
  "retryable": true,
  "partial_artifacts": []
}
```

---

## Pipeline JSON

`-P=name` returns a `PipelineResult`, NOT a `DispatchResult`.

```json
{
  "schema_version": 1,
  "pipeline_id": "01KM...",
  "status": "completed",
  "steps": [
    {
      "step_name": "plan",
      "step_index": 0,
      "handoff_mode": "summary_and_refs",
      "workers": [{
        "worker_index": 0,
        "dispatch_id": "01KM...",
        "status": "completed",
        "summary": "Designed the migration plan.",
        "artifact_dir": "/tmp/agent-mux/.../pipeline/step-0/worker-0",
        "output_file": "/tmp/agent-mux/.../pipeline/step-0/worker-0/output.md",
        "duration_ms": 45000
      }],
      "succeeded": 1, "failed": 0, "total_ms": 45000
    }
  ],
  "final_step": { "..." },
  "duration_ms": 120000
}
```

### Pipeline status

| `status` | Meaning |
|----------|---------|
| `completed` | No worker failures in any step |
| `partial` | Some workers failed but each step had at least one success |
| `failed` | A step had zero successful workers; pipeline stopped |

---

## Control-Path Responses

### --signal ack

```json
{"status":"ok","dispatch_id":"01KM...","artifact_dir":"/tmp/agent-mux-501/01KM...","message":"Signal delivered to inbox"}
```

### --version

```json
{"version":"agent-mux v2.0.0-dev"}
```

### -o=text

Human-readable summary for single dispatches only. Pipeline always JSON.

---

## Lifecycle Subcommand JSON

Pass `--json` for machine-parseable output. Default is tabular.

- **list --json**: NDJSON, one `DispatchRecord` per line
- **status --json**: Single `DispatchRecord` object
- **result --json**: `{"dispatch_id":"...","response":"..."}`
- **inspect --json**: Combined record + response + artifacts + meta
- **gc**: Always JSON: `{"kind":"gc","removed":N,"kept":N,"cutoff":"..."}`

---

## Error Codes

### Built-in Codes

| Code | Meaning |
|------|---------|
| `abort_requested` | Dispatch aborted via `ax steer abort` or control file |
| `artifact_dir_unwritable` | Cannot create/write artifact directory |
| `binary_not_found` | Harness binary not found on PATH |
| `cancelled` | Dispatch cancelled before launch at confirmation |
| `config_error` | Config loading or validation failure |
| `engine_not_found` | Unknown engine name |
| `event_denied` | Hook denied a harness event |
| `frozen_killed` | Harness killed after prolonged silence |
| `internal_error` | agent-mux hit an internal invariant failure |
| `interrupted` | Context cancelled or signal received |
| `invalid_args` | Invalid arguments or missing required fields |
| `invalid_input` | Input failed validation |
| `max_depth_exceeded` | Recursive dispatch depth limit hit |
| `model_not_found` | Unknown model for engine |
| `output_parse_error` | Failed to parse streaming harness output |
| `parse_error` | Malformed final harness output prevented a trusted result |
| `process_killed` | Harness process killed (generic fallback) |
| `prompt_denied` | Hook denied the prompt before launch |
| `recovery_failed` | Existing dispatch state could not be recovered |
| `resume_session_missing` | No session ID available for resume |
| `resume_start_failed` | Resume process failed to start |
| `resume_unsupported` | Engine does not support resume |
| `signal_killed` | Harness killed by OS signal (exit 137/143) |
| `startup_failed` | Harness binary failed to start |

### Harness-Native Codes

Additional codes surface directly from the underlying harness:

- Codex: `context_length_exceeded`
- Claude: `result_error`
- Gemini: `tool_error`

Treat `error.suggestion` as backward-compatible guidance derived from `error.hint` and `error.example`.

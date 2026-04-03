# Async Dispatch and Mid-Flight Steering

Fire-and-forget dispatch, result collection, and live control over running workers.

---

## Async Dispatch

### --async flag

When `--async` is present, agent-mux starts the dispatch in the current process,
emits an ack to stdout, then detaches stdout/stderr and runs synchronously.
The calling wrapper (e.g., `run_in_background`) provides backgrounding.

```json
{
  "schema_version": 1,
  "kind": "async_started",
  "dispatch_id": "01JQXYZ...",
  "artifact_dir": "/tmp/agent-mux-501/01JQXYZ..."
}
```

Before the ack: `host.pid` and initial `status.json` are written and fsynced.
Consumers can safely read both immediately after receiving the ack.

The `dispatch_id` is the handle for all subsequent operations.

### Collecting results

**wait** — block until terminal state:
```bash
agent-mux wait 01JQXYZ 2>/dev/null             # default poll 60s
agent-mux wait --poll 30s 01JQXYZ 2>/dev/null   # faster polling
agent-mux wait --json 01JQXYZ 2>/dev/null       # JSON result when done
```

Emits status lines to stderr during polling. Prints final result to stdout.

**result** — retrieve the response:
```bash
agent-mux result 01JQXYZ 2>/dev/null            # blocks until done
agent-mux result --no-wait 01JQXYZ 2>/dev/null  # error if still running
agent-mux result --json 01JQXYZ 2>/dev/null     # structured JSON result
agent-mux result --artifacts 01JQXYZ 2>/dev/null # list artifact files
```

Result reads from `~/.agent-mux/dispatches/<id>/result.json`. Falls back to
`full_output.md` in the artifact directory for legacy dispatches.

### Poll interval precedence

```
CLI --poll flag > config.toml [async].poll_interval > 60s hardcoded default
```

### status.json

The artifact directory contains `status.json`, updated atomically on each event:

| Field | Type | Description |
|-------|------|-------------|
| `state` | string | `running`, `initializing`, `completed`, `failed`, `timed_out`, `orphaned` |
| `elapsed_s` | int | Seconds since dispatch start |
| `last_activity` | string | Most recent activity description |
| `tools_used` | int | Tool calls observed |
| `files_changed` | int | File writes observed |
| `stdin_pipe_ready` | bool | Whether Codex stdin FIFO is open |
| `ts` | string | ISO 8601 timestamp |
| `dispatch_id` | string | Dispatch ID |
| `session_id` | string | Harness session ID |

`orphaned` state: detected when `host.pid` exists but the process is dead.

---

## Steering Commands

`agent-mux steer <dispatch_id> <action> [args]` provides structured control
over running dispatches. Dispatch ID can be a unique prefix.

### steer abort

Kill the worker process. Sends SIGTERM to host PID (async dispatches) or writes
`control.json` with `abort: true` (foreground dispatches).

```bash
agent-mux steer 01JQXYZ abort
```

Response:
```json
{"action":"abort","dispatch_id":"01JQXYZ...","mechanism":"sigterm","pid":12345,"delivered":true}
```

Mechanism is `sigterm` (PID alive) or `control_file` (PID dead or no host.pid).

### steer nudge

Send a wrap-up message. Codex workers on Unix may receive this through the
dispatch-local stdin FIFO (`stdin_fifo` mechanism); other cases fall back to
inbox delivery.

Default message: "Please wrap up your current work and provide a final summary."

```bash
agent-mux steer 01JQXYZ nudge
agent-mux steer 01JQXYZ nudge "Summarize what you have so far"
```

### steer redirect

Redirect the worker with new instructions. Argument is required.

```bash
agent-mux steer 01JQXYZ redirect "Focus on the tests, skip the refactor"
```

Codex redirect via stdin FIFO prepends: "IMPORTANT: The coordinator has
redirected your task. Stop your current approach and follow these new
instructions instead:"

### steer extend

Extend the watchdog kill threshold. Writes `control.json` with
`extend_kill_seconds`. Useful for legitimate long-running operations.

```bash
agent-mux steer 01JQXYZ extend 300
```

Argument: positive integer (seconds).

### Steer delivery mechanisms

| Mechanism | When used |
|-----------|-----------|
| `stdin_fifo` | Codex, running state, stdin pipe ready, PID alive |
| `inbox` | Fallback for all engines; nudge and redirect |
| `sigterm` | Abort when host PID is alive |
| `control_file` | Abort and extend (always via control.json) |

### Steer output format

All steer commands return JSON:

```json
{
  "action": "redirect",
  "dispatch_id": "01JQXYZ...",
  "mechanism": "inbox",
  "delivered": true
}
```

Errors: `{"kind":"error","error":{"code":"...","message":"...","hint":"...","example":"","retryable":false}}`.

---

## Signal System

`--signal` delivers a message to a running dispatch's inbox:

```bash
agent-mux --signal 01JQXYZ "Focus on auth paths; skip tests"
```

### How it works

1. Resolves dispatch reference to artifact directory
2. Message appended to `inbox.md` (atomic with flock)
3. JSON ack returned confirming write
4. LoopEngine checks inbox at event boundaries
5. If harness has a resumable session ID: graceful stop, then resume with
   message injected into the conversation

### Important caveats

- **Ack != delivery.** Ack confirms inbox write, not worker consumption.
- **Not instant.** Waits for an event boundary and a resumable session ID.
- **Keep signals short.** They become a resumed turn.

---

## Streaming Protocol

### Default: silent mode

Only bookend events (`dispatch_start`, `dispatch_end`) and failure events
are emitted to stderr. All events always written to `events.jsonl`.

### --stream / -S

Restores full event streaming to stderr: heartbeats, tool activity, progress.
Useful for human debugging and real-time monitoring.

### --verbose / -v

Adds raw harness lines to stderr output. Breaks pure NDJSON parsing but
useful for debugging harness behavior.

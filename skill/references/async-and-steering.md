# Async Dispatch and Mid-Flight Steering

Async launch, live status, result collection, and steering for running dispatches.

---

## Async Dispatch

### --async ack

`--async` starts the dispatch in the current process, writes the runtime control
files, emits an ack on `stdout`, then continues running in the background.

```json
{
  "schema_version": 1,
  "kind": "async_started",
  "dispatch_id": "01K...",
  "artifact_dir": "/tmp/agent-mux-501/01K.../"
}
```

Before the ack is emitted:

- `<artifact_dir>/host.pid` exists and is fsynced
- `<artifact_dir>/status.json` exists and is fsynced
- `<artifact_dir>/_dispatch_ref.json` points at the durable store
- `~/.agent-mux/dispatches/<id>/meta.json` has been written

`_dispatch_ref.json` is a thin pointer:

```json
{
  "dispatch_id": "01K...",
  "store_dir": "/Users/alice/.agent-mux/dispatches/01K..."
}
```

The durable store is always `~/.agent-mux/dispatches/<id>/`.

### Collecting results

`wait` is the completion primitive:

```bash
agent-mux wait 01K... 2>/dev/null
agent-mux wait --poll 30s 01K... 2>/dev/null
agent-mux wait --json 01K... 2>/dev/null
```

- `wait` blocks until `result.json` exists
- stderr gets periodic status lines while waiting
- `wait --json` prints the same compact lifecycle JSON shape as `result --json`

`result` reads the durable result:

```bash
agent-mux result 01K... 2>/dev/null
agent-mux result --json 01K... 2>/dev/null
agent-mux result --no-wait 01K... 2>/dev/null
agent-mux result --artifacts 01K... 2>/dev/null
```

- plain `result` prints the stored response text
- `--json` prints a compact lifecycle JSON object
- `--artifacts` lists non-internal files in the artifact dir
- `--no-wait` errors if the dispatch is still running

Current source of truth: `~/.agent-mux/dispatches/<id>/result.json`.

### Poll interval precedence

```
CLI --poll
  > [async].poll_interval in config.toml
  > 60s hardcoded default
```

### status.json

`<artifact_dir>/status.json` is updated atomically during the run.

| Field | Type | Meaning |
|-------|------|---------|
| `state` | string | `initializing`, `running`, `completed`, `failed`, `timed_out` |
| `elapsed_s` | int | Seconds since start |
| `last_activity` | string | Most recent activity summary |
| `tools_used` | int | Tool-call count seen so far |
| `files_changed` | int | File-write count seen so far |
| `stdin_pipe_ready` | bool | Codex stdin FIFO bridge is ready |
| `ts` | string | RFC3339 timestamp |
| `dispatch_id` | string | Dispatch ID |
| `session_id` | string | Harness session ID |

`agent-mux status` may synthesize `orphaned` if `host.pid` exists but the
process is no longer alive.

---

## Steering

Steering is unified under `internal/steer`. `steer.Delivery` owns both
soft-delivery channels:

- inbox file in the artifact dir
- FIFO named pipe at `<artifact_dir>/stdin.pipe` on Unix

`agent-mux steer <dispatch_id> <action> [args]` accepts a full dispatch ID or
a unique prefix.

### steer abort

Try SIGTERM against `host.pid`. If there is no live host PID, fall back to
`control.json`.

```bash
agent-mux steer 01K... abort
```

Possible JSON responses:

```json
{"action":"abort","dispatch_id":"01K...","mechanism":"sigterm","pid":12345,"delivered":true}
```

```json
{"action":"abort","dispatch_id":"01K...","mechanism":"control_file","delivered":true}
```

### steer nudge

Send a wrap-up message. Default message:

`Please wrap up your current work and provide a final summary.`

```bash
agent-mux steer 01K... nudge
agent-mux steer 01K... nudge "Summarize what you have so far"
```

Delivery order:

1. Codex stdin FIFO when the worker is running and `stdin_pipe_ready=true`
2. inbox fallback for everything else

Inbox fallback writes `[NUDGE] <message>`.

### steer redirect

Send a new instruction set. Argument is required.

```bash
agent-mux steer 01K... redirect "Focus on tests; skip the refactor"
```

Delivery order is the same as `nudge`.

Inbox fallback writes `[REDIRECT] <message>`.

Typical JSON response:

```json
{"action":"redirect","dispatch_id":"01K...","mechanism":"inbox","delivered":true}
```

### steer extend

Extend the watchdog kill threshold by writing `control.json`.

```bash
agent-mux steer 01K... extend 300
```

Response:

```json
{"action":"extend","dispatch_id":"01K...","seconds":300,"delivered":true}
```

### Steering mechanisms

| Mechanism | When used |
|-----------|-----------|
| `stdin_fifo` | Codex only, running state, stdin bridge ready, host PID alive |
| `inbox` | Fallback path for `nudge` and `redirect` |
| `sigterm` | `abort` when host PID is alive |
| `control_file` | `abort` fallback and all `extend` requests |

---

## Signal Flag

`--signal` is a convenience write to the inbox:

```bash
agent-mux --signal 01K... "Focus on auth paths only" 2>/dev/null
```

Ack shape:

```json
{
  "status": "ok",
  "dispatch_id": "01K...",
  "artifact_dir": "/tmp/agent-mux-501/01K.../",
  "message": "Signal delivered to inbox"
}
```

Ack means the message was written to the inbox, not that the worker has already
consumed it.

---

## Streaming

Default stderr mode is silent except for bookend and failure-family events. All
structured events are always appended to `<artifact_dir>/events.jsonl`.

- `--stream` / `-S`: emit the full NDJSON event stream to stderr
- `--verbose` / `-v`: include raw harness lines as well

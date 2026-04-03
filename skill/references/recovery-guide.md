# Recovery Guide

Recovery flow, runtime layout, inbox mechanics, and watchdog behavior.

---

## Runtime Layout

Every dispatch has:

- a runtime artifact directory
- a durable store entry under `~/.agent-mux/dispatches/<id>/`

### Artifact directory

| File | Purpose |
|------|---------|
| `_dispatch_ref.json` | Thin pointer to the durable store |
| `events.jsonl` | Full NDJSON event log |
| `status.json` | Live status snapshot |
| `host.pid` | PID of the async host process |
| `control.json` | Abort and extend requests |
| `inbox.md` | NDJSON coordinator inbox |
| `stdin.pipe` | Unix FIFO for soft Codex steering |
| worker files | Any artifacts written by the worker |

`_dispatch_ref.json` replaces `_dispatch_meta.json` as the runtime control
record in the artifact dir.

### Durable store

The only durable persistence location is:

`~/.agent-mux/dispatches/<dispatch_id>/`

Files:

| File | Purpose |
|------|---------|
| `meta.json` | persistent dispatch metadata |
| `result.json` | persistent dispatch result |

Lifecycle commands (`list`, `status`, `result`, `inspect`, `wait`) use this
store as their source of truth.

---

## Recovery Workflow

Recovery lives in `internal/dispatch/recovery.go`.

Use `--recover <id>` or `"recover": "<id>"` in stdin JSON.

### Flow

1. resolve the dispatch ID from the durable store
2. read `~/.agent-mux/dispatches/<id>/meta.json` to recover `artifact_dir`
3. resolve artifact-backed metadata via `_dispatch_ref.json`
4. scan the artifact directory for worker-written files
5. build a continuation prompt with dispatch ID, engine, model, prior status, artifact list, and prompt hash
6. run a new dispatch with that continuation prompt prepended

The added recovery prompt already says "continue from where the previous run
left off." Your prompt should only state what remains.

### When recovery is appropriate

- the prior run timed out after writing useful artifacts
- the dispatch was interrupted mid-work
- you want a continuation, not a restart

---

## Dispatch ID Resolution

Lifecycle commands accept a full dispatch ID or a unique prefix.

Resolution is driven from `~/.agent-mux/dispatches/`:

1. search dispatch directories by prefix
2. error if more than one dispatch matches
3. use the matching dispatch's durable metadata to find the artifact dir

---

## Inbox Mechanics

`inbox.md` is a file-backed coordinator mailbox managed by `internal/steer`.

### Write path

- open `inbox.md` with append/create
- take `flock(LOCK_EX)`
- append one NDJSON message with timestamp

### Read path

- open `inbox.md` read-write
- take `flock(LOCK_EX)`
- read all messages
- truncate the file to zero
- return the parsed message list

### Fast path

`HasMessages()` uses a stat check on file size without locking.

### Where the loop checks inbox messages

The loop checks for pending inbox messages:

1. after harness output is scanned
2. on the `250ms` inbox ticker
3. on the `5s` watchdog ticker

That is why steer and `--signal` are not tied to a single polling path.

---

## Liveness Watchdog

### Silence thresholds

| Threshold | Default | Action |
|-----------|---------|--------|
| `silence_warn_seconds` | 90s | emit `frozen_warning`, optionally send stdin nudge |
| `silence_kill_seconds` | 180s | kill the worker and fail the dispatch |

Config source: `[liveness]`.

Per-dispatch override: `engine_opts.heartbeat_interval_sec`,
`engine_opts.silence_warn_seconds`, `engine_opts.silence_kill_seconds`.

### Heartbeats

Heartbeat interval default: `15s`.

### Soft timeout flow

1. at `timeout_sec`, emit `timeout_warning`
2. write a wrap-up message to the inbox telling the worker to write final artifacts to `$AGENT_MUX_ARTIFACT_DIR`
3. start the grace timer
4. if grace expires, stop the worker and return `timed_out`

### Frozen process handling

- if silence crosses the warn threshold, emit `frozen_warning`
- if the adapter supports stdin nudges, send one
- if silence crosses the kill threshold, emit `frozen_killed` and terminate the worker

### Long commands

Long-running commands can temporarily extend the effective kill threshold and
emit `long_command_detected`.

---

## Steering and Recovery Interaction

Soft steering is unified under `internal/steer`:

- inbox delivery for all engines
- FIFO delivery for Codex when `stdin_pipe_ready=true`

If a steer message arrives while a tool is still active, agent-mux can defer
resume/restart until the tool finishes, or force it after the configured
maximum wait.

---

## Failure Decision Tree

```text
status?
 timed_out + files_changed non-empty
   -> --recover=<dispatch_id> with a continuation prompt
 timed_out + files_changed empty
   -> prompt too broad; tighten scope and retry once
 failed + error.retryable
   -> fix the cause and retry once
 failed + not retryable
   -> structural failure; escalate
 second failure on the same step
   -> stop and reframe the work
```

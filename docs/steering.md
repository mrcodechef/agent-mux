# Steering

agent-mux provides mid-flight control over running dispatches through two mechanisms: the signal/inbox system for message delivery, and the `steer` subcommand for structured control actions.

Steering is decoupled from the dispatch path. For most harnesses, messages arrive through the inbox and are consumed at event boundaries. For live Codex workers on Unix, `steer nudge` and `steer redirect` can use a dispatch-local FIFO to inject soft steering over the worker stdin pipe without forcing a resume.

## Signal System

### --signal Flag

```bash
agent-mux --signal <dispatch_id> "Focus on the parser module only"
```

Delivers a message to a running dispatch's inbox. Returns a JSON ack confirming the message was appended, but does not confirm the worker has consumed it.

### Inbox Mechanics

**Append-under-lock** (`WriteInbox`): opens the inbox file with `O_WRONLY|O_APPEND|O_CREATE`, acquires `LOCK_EX` via `syscall.Flock`, appends `message + "\n---\n"`.

**Poll-at-event-boundary**: `HasMessages` is a fast stat check (no lock). Polled in three places:

1. After every harness stdout line in `scanHarnessOutput`
2. On the 250ms inbox ticker
3. On the 5-second watchdog ticker

The dual path (stdout-linked plus timer) ensures that steering messages arrive even when the harness is idle and not producing output.

**Atomic read-and-clear** (`ReadInbox`): opens with `O_RDWR`, acquires `LOCK_EX`, reads all content, truncates to zero, splits on `"\n---\n"`, returns messages.

### Resume Flow

When inbox messages arrive, the LoopEngine:

1. Gracefully stops the current harness process
2. Re-invokes via the adapter's native resume protocol (`ResumeArgs`)
3. The harness maintains its own conversation state; agent-mux does not manage it

This means the message is injected into the harness conversation as a continuation, not as an out-of-band signal.

## Steer Commands

`agent-mux steer <dispatch_id> <action> [args]` provides structured control actions.

Live observation is separate: use `agent-mux status [--json] <dispatch_id>` for one-off liveness checks and `agent-mux wait <dispatch_id>` for completion.

### steer abort

```bash
agent-mux steer 01JQXYZ abort
```

Kills the worker process. Tries `SIGTERM` to the host PID first (for async dispatches). Falls back to writing `control.json` with `abort: true` for foreground dispatches.

### steer nudge

```bash
agent-mux steer 01JQXYZ nudge
agent-mux steer 01JQXYZ nudge "Please summarize what you have so far"
```

Sends a wrap-up message. On live Codex workers with a ready stdin FIFO, agent-mux injects the nudge through `stdin.pipe`. Otherwise it falls back to inbox delivery. Default message: "Please wrap up your current work and provide a final summary."

### steer redirect

```bash
agent-mux steer 01JQXYZ redirect "focus on the tests, skip the refactor"
```

Redirects the worker with new instructions. On live Codex workers with a ready stdin FIFO, agent-mux injects the redirect through `stdin.pipe`. Otherwise it falls back to inbox delivery. The instructions argument is required.

## FIFO Soft Steering For Codex

Codex dispatches on Unix create `stdin.pipe` inside the artifact directory and keep a reader open for the dispatch lifetime. `agent-mux steer <id> nudge|redirect` checks `_dispatch_meta.json`, `status.json`, and `host.pid`; when the worker is still running and `stdin_pipe_ready` is true, the CLI writes a single JSON envelope to the FIFO and closes it immediately.

This path avoids the inbox -> stop -> resume cycle. The LoopEngine receives the envelope, emits `coordinator_inject`, and writes the formatted steer text into the live Codex stdin pipe. If a tool call is active, delivery is deferred until the tool ends.

FIFO soft steering is used only when all of these are true:

- Engine is `codex`
- Platform supports FIFOs (Unix; Windows stays on inbox fallback)
- `status.json` reports `state: "running"`
- `status.json` reports `stdin_pipe_ready: true`
- `host.pid` exists and is still alive

Fallback to inbox happens when any of those checks fail, or when FIFO open/write returns readiness errors such as missing pipe, no reader, or broken pipe.

### steer extend

```bash
agent-mux steer 01JQXYZ extend 300
```

Extends the watchdog kill threshold by writing `control.json` with `extend_kill_seconds`. The watchdog reads this file on each tick and applies the extension. Useful when a legitimate long-running operation needs more time than the configured `silence_kill_seconds`.

## Live Status

```bash
agent-mux status --json 01JQXYZ
```

Reads live status from `status.json` in the artifact directory. Detects orphaned processes where the host PID is dead but the state is still recorded as "running".

## Output Format

All steer commands output a JSON ack:

```json
{
  "action": "redirect",
  "dispatch_id": "01JQXYZ...",
  "mechanism": "stdin_fifo",
  "delivered": true
}
```

`mechanism` reports how the steer action was delivered:

- `stdin_fifo`: live Codex soft steering through `stdin.pipe`
- `inbox`: inbox fallback for non-Codex, non-Unix, not-ready, or failed FIFO delivery
- `sigterm`: `steer abort` delivered through `SIGTERM`
- `control_file`: `steer abort` fallback through `control.json`

Errors follow the standard envelope: `{"kind":"error","error":{...}}`.

## status.json

Running dispatches write `status.json` with live state, activity counters, and steering readiness. For soft steering, the important field is:

- `stdin_pipe_ready`: true when the dispatch-local `stdin.pipe` FIFO is live and safe for `steer nudge` / `steer redirect`

## Cross-References

- [Architecture](./architecture.md) for inbox design and concurrency model
- [Recovery](./recovery.md) for artifact directory layout and control records
- [Async](./async.md) for background dispatch and status.json
- [Dispatch](./dispatch.md) for the DispatchResult contract
- [Engines](./engines.md) for per-adapter resume support

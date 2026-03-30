# Steering

agent-mux provides mid-flight control over running dispatches through two mechanisms: the signal/inbox system for message delivery, and the `steer` subcommand for structured control actions.

Steering is decoupled from the dispatch path. A running worker does not need to be aware of steering capability; messages arrive through the inbox and are consumed at event boundaries.

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

Sends a wrap-up message via inbox. Default message: "Please wrap up your current work and provide a final summary."

### steer redirect

```bash
agent-mux steer 01JQXYZ redirect "focus on the tests, skip the refactor"
```

Redirects the worker with new instructions via inbox. The instructions argument is required.

### steer extend

```bash
agent-mux steer 01JQXYZ extend 300
```

Extends the watchdog kill threshold by writing `control.json` with `extend_kill_seconds`. The watchdog reads this file on each tick and applies the extension. Useful when a legitimate long-running operation needs more time than the configured `silence_kill_seconds`.

### steer status

```bash
agent-mux steer 01JQXYZ status
```

Reads live status from `status.json` in the artifact directory. Detects orphaned processes where the host PID is dead but the state is still recorded as "running".

## Output Format

All steer commands output a JSON ack:

```json
{
  "action": "redirect",
  "dispatch_id": "01JQXYZ...",
  "delivered": true
}
```

Errors follow the standard envelope: `{"kind":"error","error":{...}}`.

## Cross-References

- [Architecture](./architecture.md) for inbox design and concurrency model
- [Recovery](./recovery.md) for artifact directory layout and control records
- [Async](./async.md) for background dispatch and status.json
- [Dispatch](./dispatch.md) for the DispatchResult contract
- [Engines](./engines.md) for per-adapter resume support

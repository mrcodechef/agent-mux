# Recovery Guide

Recovery workflow, artifact layout, signal mechanics, and liveness watchdog.

---

## Artifact Directory Layout

Every dispatch gets an artifact directory. Default location:
`$XDG_RUNTIME_DIR/agent-mux/<dispatch_id>/` or `/tmp/agent-mux-<uid>/<dispatch_id>/`.
Override with `--artifact-dir`.

| File | Purpose |
|------|---------|
| `meta.json` | Runtime dispatch metadata (ID, session_id, engine, model, status, timestamps, prompt_hash) |
| `events.jsonl` | NDJSON event log (all events) |
| `status.json` | Live status for async polling (state, elapsed, tools, files) |
| `host.pid` | PID of async dispatch process |
| `control.json` | Steering control (abort, extend_kill_seconds) |
| `inbox.md` | Coordinator mailbox for signal/steer injection |
| `full_output.md` | Full response when truncation occurred |
| (worker files) | Any files the worker created as artifacts |

### Persistent store

Durable records live at `~/.agent-mux/dispatches/<dispatch_id>/`:

| File | Purpose |
|------|---------|
| `meta.json` | Dispatch metadata (ID, session_id, engine, model, role, variant, profile, cwd, timeout, started_at) |
| `result.json` | Full dispatch result (embeds DispatchResult + envelope: started_at, ended_at, artifact_dir, cwd, engine, model, role, variant, profile, effort, session_id, response_chars, timeout_sec) |

The persistent store is the source of truth for `list`, `status`, `result`,
and `inspect`. Artifact directories are ephemeral runtime state.

---

## Recovery Workflow

Recovery continues a timed-out or interrupted dispatch via `--recover <id>`
or `"recover": "<id>"` in JSON.

### Flow

1. **Resolve** — finds artifact directory via persistent meta at
   `~/.agent-mux/dispatches/<id>/meta.json`, then checks current and legacy
   artifact roots
2. **Reconstruct** — reads `meta.json` from artifact dir, scans artifact files
3. **Build prompt** — generates continuation with: dispatch ID, engine, model,
   previous status, artifact file paths, original prompt hash
4. **Re-dispatch** — recovery prompt replaces `spec.Prompt`, runs normal dispatch path

### When to use

- Prior dispatch timed out after writing useful artifacts
- Dispatch was interrupted (SIGINT/SIGTERM) mid-work
- You want to continue from where the worker left off

### Tips

Your recovery prompt should be the delta, not a full re-brief.
- Good: "Finish the remaining tests and summarize what's left"
- Bad: "Re-explain the entire project from scratch"

---

## Dispatch ID Resolution

All lifecycle commands (`status`, `result`, `inspect`, `wait`, `steer`)
accept a full dispatch ID or a unique prefix.

Resolution order:
1. Search `~/.agent-mux/dispatches/` for matching directory
2. If prefix matches multiple dispatches, return error
3. Resolve artifact directory from persistent meta

---

## Inbox Mechanics

File-based message queue at `<artifact_dir>/inbox.md`.

- **Write:** append with `\n---\n` delimiter, flock(LOCK_EX) for atomicity
- **Read:** exclusive lock, read all, split by delimiter, truncate to zero
- **Check:** stat-only (file size > 0), no lock — for fast non-blocking polling

LoopEngine checks inbox in three places:
1. After every harness stdout line
2. On the 250ms inbox ticker
3. On the 5-second watchdog ticker

Dual path ensures steering arrives even when harness is idle.

---

## Liveness Watchdog

### Silence thresholds

| Threshold | Default | Action |
|-----------|---------|--------|
| `silence_warn_seconds` | 90s | Emit `frozen_warning`, send stdin nudge |
| `silence_kill_seconds` | 180s | Kill harness, return `frozen_killed` error |

Configurable in `[liveness]` section. Per-dispatch override via `engine_opts`.

### Stdin nudge

At warn threshold, writes `"\n"` to Codex's stdin pipe to attempt recovery.
Claude and Gemini do not support stdin nudge (return nil).

### Timeout flow

1. At `timeout_sec`: soft timeout fires, `timeout_warning` event, wrap-up
   message written to inbox, grace timer starts
2. If harness exits cleanly during grace: `completed` (not `timed_out`)
3. If grace expires: SIGTERM -> SIGKILL -> `timed_out`, `partial: true`

Design principle: kill the process, preserve the artifacts. Written files
persist regardless of timeout.

### Long command detection

The watchdog tracks active commands. Known long-running commands (cargo, make,
nvcc, etc.) trigger `long_command_detected` event and extended silence threshold.

---

## Supervisor

Process groups (`Setpgid: true`) ensure grandchildren die with parent.
`GracefulStop`: SIGTERM first, then SIGKILL after grace period. Process group
kill (`Kill(-pgid, ...)`) terminates all descendants atomically.

---

## Failure Decision Tree

```
status?
 timed_out + files_changed non-empty
   -> --recover=<dispatch_id> with continuation prompt
 timed_out + files_changed empty
   -> Prompt too broad. Reframe with tighter scope. Retry ONCE.
 timed_out + heartbeat_count == 0
   -> Worker never started. Config error. Check and retry once.
 failed + error.retryable
   -> Fix cause (wrong flag, missing arg). Retry ONCE.
 failed + not retryable
   -> Structural. Escalate.
 Second failure on same step
   -> STOP. Problem is the prompt or the scope, not the effort level.
```

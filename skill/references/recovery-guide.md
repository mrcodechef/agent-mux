# Recovery Guide

Recovery workflow, signal mechanics, artifact layout, and liveness watchdog.

---

## Artifact Directory Layout

Every dispatch gets an artifact directory at `/tmp/agent-mux-<uid>/<dispatch_id>/`.
Override with `--artifact-dir`. Uses `$XDG_RUNTIME_DIR/agent-mux/` if set.

| File | Purpose |
|------|---------|
| `_dispatch_meta.json` | Dispatch metadata (ID, salt, engine, model, status, timestamps) |
| `events.jsonl` | NDJSON event log (mirrored from stderr) |
| `inbox.md` | Coordinator mailbox for signal/steer injection |
| `full_output.md` | Full response when truncation occurred |
| `status.json` | Live status for async polling |
| (worker files) | Any files the worker created as artifacts |

### Pipeline artifacts

```
/tmp/agent-mux-<uid>/<dispatch_id>/pipeline/
  step-0/worker-0/
    _dispatch_meta.json, events.jsonl, output.md
  step-1/worker-0/
    ...
```

---

## Recovery Workflow

Recovery continues a timed-out or interrupted dispatch via `--recover <id>`
or `continues_dispatch_id` in JSON.

### Flow

1. **Resolve** — reads control record to find artifact directory
2. **Reconstruct** — reads `_dispatch_meta.json` and scans artifacts
3. **Build prompt** — generates continuation with: dispatch ID, engine, model,
   previous status, artifact file paths, original prompt hash
4. **Re-dispatch** — recovery prompt replaces `spec.Prompt`, runs normal path

### When to use

- Prior dispatch timed out after writing useful artifacts
- Dispatch was interrupted (SIGINT/SIGTERM) mid-work
- You want to continue from where the worker left off

### Tips

Your recovery prompt should be the delta, not a full re-brief.
- Good: "Finish the remaining tests and summarize what's left"
- Bad: "Re-explain the entire project from scratch"

---

## Control Records

Map dispatch IDs to artifact directories.
Location: `/tmp/agent-mux-<uid>/control/<dispatch_id>.json`

```json
{
  "dispatch_id": "01KM...",
  "artifact_dir": "/tmp/agent-mux-501/01KM...",
  "dispatch_salt": "mint-ant-five",
  "trace_token": "AGENT_MUX_GO_01KM..."
}
```

Resolution order for `--recover` and `--signal`:
1. Control record at `/tmp/agent-mux-<uid>/control/<id>.json`
2. Legacy default directory at `/tmp/agent-mux-<uid>/<id>/`
3. Error if neither found

---

## Inbox Mechanics

File-based message queue at `<artifact_dir>/inbox.md`.

- **Write:** append with `\n---\n` delimiter, flock(LOCK_EX) for atomicity
- **Read:** exclusive lock, read all, split by delimiter, truncate to zero
- **Check:** `HasMessages()` is stat-only (file size > 0), no lock — for fast
  non-blocking polling at event boundaries

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
| `silence_kill_seconds` | 180s | Kill harness, return `frozen_tool_call` error |
| `long_command_silence_seconds` | 540s | Extended threshold for cargo, make, nvcc, etc. |

The watchdog tracks active commands via `tool_start`/`command_run` (set) and
`tool_end` (clear). Known long-running commands get the extended threshold.

### Stdin nudge

At warn threshold, writes `"\n"` to Codex's stdin pipe to attempt recovery.
Claude and Gemini don't support stdin nudge.

### Timeout flow

1. At `timeout_sec`: soft timeout fires, `timeout_warning` event, wrap-up
   message written to inbox, grace timer starts
2. If harness exits cleanly during grace: `completed` (not `timed_out`)
3. If grace expires: SIGTERM -> SIGKILL -> `timed_out`, `partial: true`

Design principle: kill the process, preserve the artifacts. Written files
persist regardless of timeout.

---

## Supervisor

Process groups (`Setpgid: true`) ensure grandchildren die with parent.
`GracefulStop`: SIGTERM first, then SIGKILL after grace period. Process group
kill (`Kill(-pgid, ...)`) terminates all descendants atomically.

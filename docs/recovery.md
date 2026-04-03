# Recovery

Recovery in agent-mux is built on two separate layers:

- a runtime artifact directory for live files and worker outputs
- a durable dispatch store at `~/.agent-mux/dispatches/<dispatch_id>/`

The current implementation no longer uses the old artifact-local metadata file as the active metadata record.

## Runtime Artifact Directory

Each dispatch has a runtime artifact directory, either chosen explicitly with `--artifact-dir` or derived from the runtime artifact root returned by `sanitize.SecureArtifactRoot()`.

Typical contents:

```text
<artifact_dir>/
  _dispatch_ref.json
  status.json
  events.jsonl
  inbox.md
  control.json
  host.pid
  full_output.md
  <worker-created files>
```

Notes:

- `_dispatch_ref.json` is the current artifact-local pointer file.
- `status.json` is live state, not durable completion state.
- `host.pid` exists for async dispatches.
- `full_output.md` is only a legacy fallback path used by lifecycle commands when no durable result is available.

### `_dispatch_ref.json`

`internal/dispatch/dispatch.go` writes:

```json
{
  "dispatch_id": "01K...",
  "store_dir": "/home/user/.agent-mux/dispatches/01K..."
}
```

This file is intentionally thin. The real metadata is not stored in the artifact directory.

## Durable Persistence

All durable dispatch persistence lives under:

```text
~/.agent-mux/dispatches/<dispatch_id>/
  meta.json
  result.json
```

There is no other persistence location.

### `meta.json`

`meta.json` is `PersistentDispatchMeta` from `internal/dispatch/persistence.go`. It stores:

- `dispatch_id`
- `session_id`
- `engine`
- `model`
- `effort`
- `role`
- `profile`
- `cwd`
- `artifact_dir`
- `started_at`
- `timeout_sec`
- `prompt_hash`

### `result.json`

`result.json` is `PersistentDispatchResult`. It embeds `DispatchResult` and adds persisted context such as:

- `started_at`
- `ended_at`
- `artifact_dir`
- `cwd`
- `engine`
- `model`
- `role`
- `profile`
- `effort`
- `session_id`
- `response_chars`
- `timeout_sec`

`wait` and other lifecycle commands treat `result.json` as the durable completion signal.

## Recovery Flow

`--recover <dispatch_id>` and stdin `recover` use this flow:

1. Resolve the artifact directory with `ResolveArtifactDir`.
2. Read dispatch metadata with `ReadDispatchMeta(artifactDir)`.
3. Scan non-internal artifacts from the artifact directory.
4. Build a continuation prompt with `BuildRecoveryPrompt`.
5. Re-dispatch through the normal dispatch path.

### Artifact Directory Resolution

`ResolveArtifactDir` checks:

1. `~/.agent-mux/dispatches/<dispatch_id>/meta.json` and its `artifact_dir`
2. the current runtime artifact root for `<dispatch_id>`
3. the legacy runtime path `/tmp/agent-mux/<dispatch_id>`

That legacy `/tmp/agent-mux/<dispatch_id>` fallback is only for artifact lookup compatibility. It is not a persistence location.

### Metadata Resolution

`ReadDispatchMeta(artifactDir)` now prefers `_dispatch_ref.json`, then loads the durable `meta.json` and `result.json` for the referenced dispatch.

Artifact-local legacy metadata files are fallback compatibility only.

## Recovery Prompt Contents

`BuildRecoveryPrompt` includes:

- the previous dispatch ID
- the previous engine and model
- the previous terminal status when known
- the artifact file list
- the original prompt hash when known
- the caller's new instruction appended at the end

The new prompt should be the delta for the next attempt, not a full re-brief.

## Timeouts and Partial Results

When a run ends in `timed_out` or `failed`, agent-mux still preserves:

- the accumulated response text
- the scanned artifact list
- structured failure metadata
- recoverability markers when continuation is viable

The durable write target for terminal results is always `~/.agent-mux/dispatches/<dispatch_id>/result.json`.

## Liveness and Orphans

The runtime watchdog updates `status.json` during execution. Lifecycle commands use it for live inspection, and `status` may mark a dispatch as `orphaned` when `host.pid` exists but the process is gone before a durable result was written.

## Cross-References

- [async.md](./async.md) for live `status.json` behavior
- [lifecycle.md](./lifecycle.md) for `status`, `result`, `inspect`, and `wait`
- [dispatch.md](./dispatch.md) for `DispatchResult` and stdin `recover`

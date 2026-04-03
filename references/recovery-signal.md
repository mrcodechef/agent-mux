# Recovery and Signal Guide

This guide covers the current recovery, signal, and steering paths.

## Runtime Artifact Directory

Each dispatch has a runtime artifact directory under the configured `artifact_dir` or the default runtime root from `sanitize.SecureArtifactRoot()`.

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

`_dispatch_ref.json` is the current artifact-local pointer file. The old artifact-local metadata file is no longer the active contract.

## Durable Store

All durable persistence lives only in:

```text
~/.agent-mux/dispatches/<dispatch_id>/
  meta.json
  result.json
```

`DispatchesDir()` resolves to `~/.agent-mux/dispatches`.

### `meta.json`

`meta.json` is `PersistentDispatchMeta` and stores fields including:

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

`result.json` is `PersistentDispatchResult`. It includes the full `DispatchResult` plus persisted context such as:

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

## `_dispatch_ref.json`

The artifact-local pointer looks like:

```json
{
  "dispatch_id": "01K...",
  "store_dir": "/home/user/.agent-mux/dispatches/01K..."
}
```

Lifecycle and recovery code use this pointer to resolve the durable store. The real metadata is in `meta.json` and `result.json`.

## Recovery Workflow

Recovery starts from `--recover <dispatch_id>` or stdin `recover`.

Current flow:

1. Resolve the artifact directory from durable `meta.json`, then the current runtime artifact root, then the legacy `/tmp/agent-mux/<dispatch_id>` fallback.
2. Read dispatch metadata with `ReadDispatchMeta(artifactDir)`.
3. Scan non-internal artifacts from the artifact directory.
4. Build a continuation prompt with `BuildRecoveryPrompt`.
5. Dispatch again through the normal execution path.

`ReadDispatchMeta` now prefers `_dispatch_ref.json` and the durable store. Legacy artifact-local metadata files are fallback compatibility only.

### Recovery Prompt Shape

`BuildRecoveryPrompt` includes:

- previous dispatch ID
- previous engine and model
- previous status when known
- scanned artifact paths
- original prompt hash when known
- the caller's new instruction appended at the end

## Signal Workflow

`agent-mux --signal <dispatch_id> "<message>"` does one thing synchronously: write a message to the resolved inbox path and return an acknowledgement.

Success:

```json
{
  "status": "ok",
  "dispatch_id": "01K...",
  "artifact_dir": "/path/to/artifacts/01K.../",
  "message": "Signal delivered to inbox"
}
```

Important details:

- the ack confirms the inbox write, not that the worker has acted on it yet
- the running loop consumes inbox messages later at event boundaries
- adapters with resume support may restart around the injected message once the session is resumable

## Steer Workflow

`agent-mux steer` provides four actions:

- `abort`
- `nudge`
- `redirect`
- `extend`

### Delivery Mechanisms

Current behavior:

- `abort`: `SIGTERM` to `host.pid` when available, otherwise write `control.json`
- `nudge` and `redirect`: try Codex `stdin_fifo` first, then fall back to `inbox.md`
- `extend`: write `control.json`

### `control.json`

```json
{
  "abort": true,
  "extend_kill_seconds": 300,
  "updated_at": "2026-04-03T10:01:00Z"
}
```

The watchdog reads this file while the dispatch is running.

## Live State and Orphans

`status.json` is the live state file. `host.pid` is used for async orphan detection. Lifecycle commands may report a dispatch as `orphaned` when the PID is gone before a durable `result.json` appears.

## Summary

- `_dispatch_ref.json` is the active artifact-local pointer file.
- Durable metadata is only `~/.agent-mux/dispatches/<dispatch_id>/meta.json`.
- Durable results are only `~/.agent-mux/dispatches/<dispatch_id>/result.json`.

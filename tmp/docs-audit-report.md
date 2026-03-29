---
date: 2026-03-28
engine: coordinator
status: complete
---

# Documentation Audit Report — agent-mux Go v2

Cross-referenced all 12 documentation files against the Go codebase.
Grouped by file, then a consolidated fix list at the end.

---

## 1. README.md

### [DRIFT] README.md:84 — Event count and names

Doc says: "15 NDJSON event types on stderr: `dispatch_start`, `heartbeat`, `tool_start`, `tool_end`, `file_write`, `timeout_warning`, and more."

Code (`internal/event/event.go`) emits these event types via Emitter methods:
- `dispatch_start`, `dispatch_end`, `heartbeat`, `tool_start`, `tool_end`,
  `file_write`, `file_read`, `command_run`, `progress`, `timeout_warning`,
  `frozen_warning`, `error`, `response_truncated`

Code (`internal/engine/loop.go`) also emits:
- `coordinator_inject`, `warning`

That's **15** total, which matches the count. The listed examples are correct.
**No drift here** — the "and more" covers the unlisted ones.

### [DRIFT] README.md:88 — Liveness supervision interval

Doc says: "5-second watchdog cycle detects hung harnesses."

Code (`internal/config/config.go:113`): `HeartbeatIntervalSec: 15` (the watchdog
cycle in `loop.go` uses a 5-second ticker for the watchdog goroutine — this is
separate from heartbeat).

This is actually correct — the watchdog is a 5s cycle. **No drift.**

### [MISSING] README.md — config.local.toml not mentioned

Code (`internal/config/config.go:298,321`) loads `config.local.toml` as a
machine-local overlay at both global and project levels. README does not
mention this. Minor — README is a summary, but the config resolution
section at line 49 should include it.

---

## 2. DOCS.md

### [DRIFT] DOCS.md:164 — response_max_chars default

Doc says: `response_max_chars` default is `2000`.

Code (`internal/config/config.go:105`): `ResponseMaxChars: 16000`.

**The default was changed to 16000 but DOCS.md still says 2000.**

### [DRIFT] DOCS.md:178 — Models merge semantics

Doc says: "Overlay replaces the entire list per engine key (not appended)."

Code (`internal/config/config.go:192`):
```go
base.Models[engine] = deduplicateStrings(append(base.Models[engine], models...))
```

**Models are UNION-merged (appended + deduplicated), NOT replaced.** This is
a significant documentation drift from the Phase 5 merge semantics change.

### [MISSING] DOCS.md config section — config.local.toml layer

Code (`internal/config/config.go:296-321`) loads four paths:
1. `~/.agent-mux/config.toml`
2. `~/.agent-mux/config.local.toml` (NEW — machine-local global)
3. `<cwd>/.agent-mux/config.toml`
4. `<cwd>/.agent-mux/config.local.toml` (NEW — machine-local project)

DOCS.md Section 3 only lists three tiers (global, project, --config).
**Missing the config.local.toml layers.**

### [DRIFT] DOCS.md:263 — Hooks merge semantics

Doc says: "`[hooks].deny` / `.warn` — Overlay list fully replaces base list"

Code (`internal/config/config.go:227-228`):
```go
base.Hooks.Deny = deduplicateStrings(append(base.Hooks.Deny, overlay.Hooks.Deny...))
base.Hooks.Warn = deduplicateStrings(append(base.Hooks.Warn, overlay.Hooks.Warn...))
```

**Hooks deny/warn are UNION-merged (appended + deduplicated), NOT replaced.**
Same Phase 5 change as models.

### [DRIFT] DOCS.md:258 — Models merge semantics (merge table)

The merge table says: "`[models].<engine>` — Overlay list fully replaces base list"

Per code (see above), this is now **union-merge with dedup**. Same drift.

### [DRIFT] DOCS.md:301 — EnvVars return type

Doc says: `EnvVars(spec *DispatchSpec) []string`

Code (`internal/types/types.go:281`): `EnvVars(spec *DispatchSpec) ([]string, error)`

**EnvVars returns `([]string, error)`, not just `[]string`.**

### [DRIFT] DOCS.md:332 — Codex resume args format

Doc says: `["exec", "resume", "-m", <model>, "--json", <sessionID>, <message>]`

Code (`internal/engine/adapter/codex.go:270-276`):
```go
args := []string{"exec", "resume"}
if spec != nil && spec.Model != "" {
    args = append(args, "-m", spec.Model)
}
args = append(args, "--json", sessionID, message)
```

Doc puts `-m` before `--json` unconditionally. Code only adds `-m` when
model is non-empty. The structure is actually: `exec resume [-m model] --json <id> <msg>`.
**Minor drift — doc shows `-m` as always present.**

### [MISSING] DOCS.md — `full_output_path` field

Code (`internal/types/types.go:27`): `FullOutputPath *string \`json:"full_output_path,omitempty"\``

DOCS.md does not document the `full_output_path` field separately from
`full_output`. The types have both `FullOutput` and `FullOutputPath` as
distinct fields.

---

## 3. SKILL.md

### [DRIFT] SKILL.md:339 — response_max_chars default

Doc says: `response_max_chars` default is `2000`.

Code: default is `16000`. Same drift as DOCS.md.

### No other drift detected — SKILL.md is well-aligned with current CLI.

---

## 4. references/cli-flags.md

### [DRIFT] cli-flags.md:58 — `--add-dir` short flag

Doc says: `--add-dir` has short flag `-d`.

Code (`cmd/agent-mux/main.go:1189`): `fs.Var(&flags.addDirs, "add-dir", ...)`

**There is no `-d` short alias for `--add-dir`.** It is registered only as
`add-dir` with no `bindStr` call that would create a short form.

### [DRIFT] cli-flags.md:103 — response_max_chars default in DispatchSpec

Doc says: `response_max_chars` default is `4000`.

Code: default is `16000` (config.go:105). The unset sentinel in CLI is `-1`
(main.go:1148), which resolves to the config default.

**Doc says 4000, code says 16000.** (Note: cli-flags.md says 4000 while
DOCS.md says 2000 — neither matches the code's 16000.)

### [MISSING] cli-flags.md — config.local.toml in precedence

The precedence section (line 175-179) lists:
```
~/.agent-mux/config.toml (global)
  > <cwd>/.agent-mux/config.toml (project)
  > --config path
  > coordinator companion .toml
```

Missing `config.local.toml` layers.

---

## 5. references/output-contract.md

### [MISSING] output-contract.md — `full_output_path` field

Code (`internal/types/types.go:27`): has `FullOutputPath *string` as a
separate field from `FullOutput`.

Doc line 83 says `full_output` is "Path to `full_output.md` when truncated"
— but in code, `FullOutput` holds the *content* (or nil), while
`FullOutputPath` holds the *path*. The output-contract conflates these.

### [MISSING] output-contract.md — `missing_api_key` not in error codes

README.md:101 references `MISSING_API_KEY` as a warning. The error codes
table in output-contract.md does not list it. If the code still uses it
(even as warning), it should be documented.

### [DRIFT] output-contract.md:244 — version format

Doc says `--version` returns: "Plain text: `agent-mux v2.0.0-dev`"

Code (`cmd/agent-mux/main.go:188-189`): emits JSON:
```go
emitResult(stdout, map[string]any{"version": version})
```

**`--version` outputs JSON, not plain text.**

### [MISSING] output-contract.md — Lifecycle subcommand output shapes

The output-contract does not document the JSON shapes for `list`, `status`,
`result`, `inspect`, or `gc` subcommands. These are implemented in
`lifecycle.go` and emit structured JSON with `--json` flag.

### [DRIFT] output-contract.md:240 — Signal failure format

Doc says: "Failure: plain text error on stderr, non-zero exit."

Code (`cmd/agent-mux/main.go:193-208`): signal failures emit structured
JSON to stdout via `emitResult(stdout, buildSignalErrorAck(...))`.

**Signal failures produce JSON on stdout, not plain text on stderr.**

---

## 6. references/config-guide.md

### [DRIFT] config-guide.md:48 — Liveness defaults

Doc shows in the config example:
```toml
silence_warn_seconds = 120
silence_kill_seconds = 240
```

Code (`internal/config/config.go:114-115`):
```go
SilenceWarnSeconds: 90,
SilenceKillSeconds: 180,
```

**Doc shows 120/240, code defaults are 90/180.**

### [DRIFT] config-guide.md:98 — response_max_chars default

Doc says: `response_max_chars` default is `2000`.

Code: default is `16000`. Same recurring drift.

### [DRIFT] config-guide.md:131 — event_deny_action default

Doc says: `event_deny_action` default is `deny`.

Code (`internal/config/config.go:76`): `EventDenyAction string` with no
explicit default in `DefaultConfig()`. The zero value is `""` (empty string).

**Doc says default is `"deny"`, code has no default (empty string).**
The actual behavior when empty depends on `hooks.go` interpretation.

### [DRIFT] config-guide.md — Models merge semantics

Not explicitly stated in config-guide.md, but the Config Structure example
implies replacement. Per code, models are **union-merged**. Should be
documented.

### [MISSING] config-guide.md — config.local.toml

Not documented. Same gap as other files.

---

## 7. references/engine-comparison.md

### [DRIFT] engine-comparison.md:52 — Codex resume args

Doc says:
```bash
codex exec resume --id <thread_id> --json "message"
```

Code (`internal/engine/adapter/codex.go:270-276`):
```go
args := []string{"exec", "resume"}
// ...
args = append(args, "--json", sessionID, message)
```

**No `--id` flag.** The session ID is a positional argument, not `--id <thread_id>`.
Correct form: `codex exec resume [-m model] --json <sessionID> <message>`

### [DRIFT] engine-comparison.md:152 — Response max chars default

Doc says: "Response max chars: `2000`"

Code: default is `16000`. Same recurring drift.

---

## 8. references/prompting-guide.md

### No drift detected.

Prompting guide is advice-oriented with no concrete code claims.
No stale v1/TS references found.

---

## 9. references/pipeline-guide.md

### No drift detected.

Pipeline guide accurately reflects `internal/pipeline/types.go` structure,
validation rules, handoff modes, and fan-out semantics.

---

## 10. references/recovery-signal.md

### [STALE] recovery-signal.md:124 — Binary name

Doc says: `agent-mux-v2 --signal=01KM...`

Should be: `agent-mux --signal=01KM...`

**Stale reference to `agent-mux-v2` binary name.**

### [DRIFT] recovery-signal.md:17-20 — Artifact directory location

Doc says default location is: `/tmp/agent-mux/<dispatch_id>/`

Code (`internal/sanitize/sanitize.go:83-98`): `SecureArtifactRoot()` returns:
- `$XDG_RUNTIME_DIR/agent-mux/` (if set)
- `/tmp/agent-mux-<uid>/` (fallback)

**Default is `/tmp/agent-mux-<uid>/`, NOT `/tmp/agent-mux/`.** The per-UID
suffix was added in the SPEC_V3 security sweep. Legacy path
`/tmp/agent-mux/` exists only for backward-compat resolution.

### [DRIFT] recovery-signal.md:154 — Control record location

Doc says: `/tmp/agent-mux/control/<dispatch_id>.json`

Code: control root is `currentArtifactRoot() + "/control/"` which is
`/tmp/agent-mux-<uid>/control/` (or XDG path). Legacy
`/tmp/agent-mux/control/` is only checked as fallback.

---

## 11. FEATURES.md

### [STALE] FEATURES.md:70-84 — Lifecycle commands marked as "Proposed"

Doc proposes: `status`, `result`, `list` subcommands.

Code (`cmd/agent-mux/lifecycle.go`): **All three are implemented**, plus
`inspect` and `gc`. These should be moved from "Proposed" to "Shipped".

### [STALE] FEATURES.md:46-59 — Response truncation marked as "Proposed"

Doc proposes raising default to 8000-16000 chars.

Code (`internal/config/config.go:105`): default is now `16000`. Item (a)
is implemented. Items (b), (c), (d) are partially addressed:
- (b) `response_truncated` event is emitted via `EmitResponseTruncated()`
- (d) `full_output_path` field exists in result JSON

Should be marked as partially shipped.

### [STALE] FEATURES.md:61-68 — "BUG: result JSON goes to stderr"

This was a known bug. Looking at current code (`cmd/agent-mux/main.go`),
`emitResult` writes to `stdout` parameter. Need to verify if this is
fixed — the code structure suggests it is, since `run()` passes `os.Stdout`
as the stdout writer.

---

## 12. CHANGELOG.md

### [DRIFT] CHANGELOG.md:12 — Event names are wrong

Doc lists event types: `dispatch_start`, `dispatch_end`, `tool_start`,
`tool_end`, `agent_turn`, `file_written`, `command_run`, `hook_triggered`,
`signal_received`, `timeout_warning`, `liveness_ping`, `recovery_hint`,
`pipeline_step`

Actual event types from code:
- `dispatch_start`, `dispatch_end`, `heartbeat`, `tool_start`, `tool_end`,
  `file_write`, `file_read`, `command_run`, `progress`, `timeout_warning`,
  `frozen_warning`, `error`, `response_truncated`, `coordinator_inject`,
  `warning`

Wrong names: `agent_turn` (no such event), `file_written` (should be
`file_write`), `hook_triggered` (no such event — it's `warning`),
`signal_received` (no such event — it's `coordinator_inject`),
`liveness_ping` (should be `heartbeat`), `recovery_hint` (no such event),
`pipeline_step` (no such event).

**7 of 13 listed event names are wrong.**

### [DRIFT] CHANGELOG.md:26 — Status values

Doc says: "status field (`success` | `partial` | `error` | `timeout`)"

Code (`internal/types/types.go:12-16`): `"completed"`, `"timed_out"`, `"failed"`

**Status values are wrong.** Should be `completed`, `timed_out`, `failed`.

### [DRIFT] CHANGELOG.md:13 — Hooks description

Doc says: "per-role `[hooks]` blocks"

Code: hooks are global `[hooks]` in config, NOT per-role. There is only
one `HooksConfig` struct on the top-level `Config`.

### [MISSING] CHANGELOG.md — Phase 5 features not documented

The following implemented features have no changelog entry:
- `inspect` subcommand
- `gc` subcommand (with `--older-than`, `--dry-run`)
- `list --engine` filter
- `result --artifacts` flag
- Orphan reaper (`supervisor/reaper_darwin.go`, `reaper_linux.go`)
- `config.local.toml` support
- `response_max_chars` default raised to 16000
- Models union-merge semantics
- Hooks union-merge semantics
- `full_output_path` field in result JSON

---

## Consolidated Fix List

### Critical (behavioral mismatch)

| # | File(s) | Issue | Fix |
|---|---------|-------|-----|
| 1 | DOCS.md:164, SKILL.md:339, config-guide.md:98, cli-flags.md:103, engine-comparison.md:152 | `response_max_chars` default wrong | Change to `16000` everywhere |
| 2 | DOCS.md:178,258, config-guide.md | Models merge: says "replace", code does union | Document as union-merge with dedup |
| 3 | DOCS.md:263 | Hooks deny/warn merge: says "replace", code does union | Document as union-merge with dedup |
| 4 | CHANGELOG.md:12 | 7 of 13 event names are wrong | Replace with actual event type names |
| 5 | CHANGELOG.md:26 | Status values wrong (`success`/`partial`/`error`/`timeout`) | Fix to `completed`/`timed_out`/`failed` |
| 6 | output-contract.md:244 | `--version` said to be plain text, is JSON | Document JSON output |
| 7 | output-contract.md:240 | Signal failure said to be plain text stderr, is JSON stdout | Fix |
| 8 | engine-comparison.md:52 | Codex resume uses `--id` flag (doesn't exist) | Fix to positional arg |

### Important (missing documentation)

| # | File(s) | Issue | Fix |
|---|---------|-------|-----|
| 9 | All config docs | `config.local.toml` not documented | Add machine-local layer to all config resolution docs |
| 10 | CHANGELOG.md | Phase 5 features not in changelog | Add `inspect`, `gc`, `list --engine`, `result --artifacts`, reaper, config.local.toml, etc. |
| 11 | FEATURES.md:70-84 | `status`/`result`/`list` still "Proposed" — all shipped | Move to "Shipped" |
| 12 | FEATURES.md:46-59 | Response truncation partly shipped | Update status |
| 13 | output-contract.md | No docs for lifecycle subcommand JSON shapes | Add `list`, `status`, `result`, `inspect`, `gc` output shapes |
| 14 | output-contract.md | `full_output_path` field undocumented | Add to dispatch result schema |

### Minor

| # | File(s) | Issue | Fix |
|---|---------|-------|-----|
| 15 | recovery-signal.md:124 | `agent-mux-v2` stale binary name | Change to `agent-mux` |
| 16 | recovery-signal.md:17,154 | Artifact dir shown as `/tmp/agent-mux/` | Update to `/tmp/agent-mux-<uid>/` (or XDG) |
| 17 | config-guide.md:48 | Liveness example shows 120/240, defaults are 90/180 | Fix example values |
| 18 | config-guide.md:131 | `event_deny_action` default said to be `"deny"`, code has no default | Fix or clarify |
| 19 | cli-flags.md:58 | `--add-dir` listed with short flag `-d` (doesn't exist) | Remove `-d` from table |
| 20 | DOCS.md:301 | `EnvVars` return type shown as `[]string`, is `([]string, error)` | Fix signature |
| 21 | CHANGELOG.md:13 | Hooks described as "per-role", are global | Fix to "global" |

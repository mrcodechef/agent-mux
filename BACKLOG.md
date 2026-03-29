# Backlog

Open bugs, feature requests, spec gaps, and known limitations for agent-mux.
Replaces `FEATURES.md` (preserved at `_archive/FEATURES.md`).

**Prefix key:** `B-` bugs · `F-` features · `S-` spec gaps · `L-` limitations
**Priority:** P0 (next up) · P1 (soon) · P2 (planned) · P3 (parked)
**Rule:** Every decision, prioritization change, or "done" mark includes the session ID where it was decided.

---

## P0 — Next Up

### F-6: Soft steering via stdin before hard kill
**Type:** feature | **Priority:** P0 | **Status:** open — needs implementation
**Location:** `internal/engine/loop.go` (watchdog), adapters
**Decided:** `acabe588` (2026-03-29)

Current liveness: warn at 90s (pure observation), hard kill at 180s. The 90s
gap is dead air — no steering attempt. This was the original design intent of
the LoopEngine architecture, but **stdin pipe was never implemented** — no
`StdinPipe`, no `io.Writer`, no `Write` call to the child process exists
anywhere in `loop.go` or adapters.

The current "recovery" path is inbox-based: write a file → watchdog polls →
kill and relaunch with `ResumeArgs`. This is restart, not steering.

**Implementation needed:**
1. Add `cmd.StdinPipe()` in `startRun` (`loop.go:322-352`)
2. At warn threshold (90s), write nudge to stdin
3. Codex-first: `"\n"` or continuation prompt (stdin-driven by design)
4. Claude/Gemini: lower feasibility, investigate later

**Observed impact:** GSD session `a2608f024a0eb520b` lost a Codex worker that
froze at 244s. A stdin nudge at 90s could have unblocked it without killing.

---

### F-8: Distinct error codes for process_killed
**Type:** feature | **Priority:** P0 | **Status:** open
**Location:** `internal/engine/loop.go`, `internal/types/types.go`
**Decided:** `acabe588` (2026-03-29)

`process_killed` is a generic status covering: frozen detection, OOM, startup
failure, and signal kill. GSD assumed OOM when the actual cause was frozen
detection (`frozen_tool_call` at 244s silence). Distinct codes
(`frozen_killed`, `oom_killed`, `startup_failed`, `signal_killed`) would
enable correct diagnosis without reading logs. Mechanical fix.

---

### F-1: Per-command timeout / hanging bash detection
**Type:** feature | **Priority:** P0 | **Status:** needs investigation
**Location:** `internal/engine/loop.go`
**Decided:** `acabe588` (2026-03-29)

Only a global silence watchdog exists. A legitimate 10-minute Rust build is
indistinguishable from a hung `curl`. Risk of false positive kills is too
high — "job done is holy." The watchdog must not kill agents running
legitimate long-running commands.

**Investigation needed:** Can we distinguish between "process is silent because
it's dead" vs "process is silent because `cargo build` takes 8 minutes"?
Options:
- Track `tool_start` → `tool_end` pairs — if a known-long command is running,
  extend the silence threshold automatically
- Check if the child process is still alive (waitpid/WNOHANG) before killing
- Classify commands: `cargo`, `make`, `nvcc`, `go build` get extended grace

**Effort:** ~40 lines in `loop.go`, but design decision on classification
approach needed first.

---

## P1 — Soon

### S-2: ax-eval instrumentation
**Type:** spec gap | **Priority:** P1 | **Status:** open
**Reference:** `_archive/SPEC-V2.md` — ax-eval section
**Decided:** `acabe588` (2026-03-29)

Build a proper ax-eval testing framework using gpt-5.4-mini high as the
judge. Structured `ax_eval` behavioral events emitted during dispatch:
- `error_correction` — agent noticed and self-corrected an error.
- `tool_retry` — a tool call was retried after failure.
- `scope_reduction` — agent narrowed scope mid-task.

These events feed an evaluation pipeline for measuring dispatch quality.

---

### S-3: ax-eval CI tests (LLM-in-the-loop behavioral tests)
**Type:** spec gap | **Priority:** P1 | **Status:** open (blocked on S-2)
**Reference:** `_archive/SPEC-V2.md`
**Decided:** `acabe588` (2026-03-29)

CI tests that run a live dispatch against a small fixture repo and validate
behavioral outcomes (files changed, commands run, self-correction events)
using gpt-5.4-mini high as judge. Blocked on S-2 (ax-eval events must exist
before they can be asserted against).

---

## P2 — Planned

### F-3: Pipeline orchestration enhancements
**Type:** feature | **Priority:** P2 | **Status:** open (core shipped in 3.0.0)
**Location:** `internal/pipeline/`
**Decided:** `acabe588` (2026-03-29)

Core sequential pipelines + within-step fan-out shipped. **Advanced features
never built** (confirmed via code audit — zero fields for conditions,
branching, or fan-in in `PipelineStep`):

- **Conditional branching** — skip or reroute based on previous step status
- **Fan-in aggregation** — merge parallel fan-out results
- **Pipeline-level timeout** — ceiling on total wall-clock time

---

### S-1: `repeat_escalation` liveness
**Type:** spec gap | **Priority:** P2 | **Status:** open — not implemented
**Reference:** `_archive/SPEC-V2.md`
**Decided:** `acabe588` (2026-03-29)

**Not implemented** (confirmed: zero matches for `repeat_escalation`,
`frozen_escalation`, or `escalat` in codebase). The watchdog has a
warn-then-kill two-stage reaction but no repeat-escalation logic. Related to
F-6 (soft steering) — both touch the liveness system.

---

## P3 — Parked

### B-1: Gemini response capture broken
**Type:** bug | **Priority:** P3 | **Status:** parked
**Location:** `internal/engine/adapter/gemini.go`
**Decided:** `acabe588` (2026-03-29) — "P3, we will do it when adding gemini.
For now we work on codex and claude code engines mostly."

Gemini dispatches return truncated or empty responses. NDJSON parser drops
content. Fix when Gemini becomes a primary engine.

---

### B-2: Hooks — needs rethink and re-implementation
**Type:** bug | **Priority:** P3 | **Status:** parked (disabled in production)
**Location:** `internal/hooks/`
**Decided:** `acabe588` (2026-03-29) — "Hooks are P3, feature we need to
rethink and re-implement."

False positives on workspace reads. Entire hooks system needs redesign with
proper event source scoping before re-enabling.

---

### F-4: Bundled agent auto-install / setup command
**Type:** feature | **Priority:** P3 | **Status:** parked
**Decided:** `acabe588` (2026-03-29)

---

### F-5: Session-local daemon / JSON-RPC control plane
**Type:** feature | **Priority:** P3 | **Status:** parked
**Decided:** `acabe588` (2026-03-29)

---

## Closed

### F-2: `--no-truncate` hard-disable flag — CLOSED
**Status:** closed — truncation removed entirely in P0-1 (`51dbb23`)
**Decided:** `acabe588` (2026-03-29)

---

## Shipped (reference)

All items include session ID where the work was done.

| Item | Shipped | Session | Notes |
|------|---------|---------|-------|
| SPEC_V3 Phase 5 sweep (reaper, inspect, gc, tests) | 3.1.0 | `ecca0bdb` | 4 commits, +3918 lines |
| Docs alignment (22 drift items) | 3.1.0 | `acabe588` | 10 files updated |
| SKILL.md redesign | 3.1.0 | `acabe588` | 402→214 lines, decision-tool rewrite |
| `agent-mux config` subcommand | 3.1.0 | `acabe588` | roles, pipelines, models, skills, --sources |
| `skill_paths` config + enhanced errors | 3.1.0 | `acabe588` | Union-merged search paths, role name in errors |
| `--skip-skills` flag + `-V` removal | 3.1.0 | `acabe588` | Escape hatch + flag conflict fix |
| scripts/ dir wired for Claude/Gemini | 3.1.0 | `acabe588` | addDirs in all adapters |
| GSD agent definition updates | 3.1.0 | `acabe588` | Pre-flight, no raw flags, timeout alignment |
| Remove response truncation | 3.1.0 | `acabe588` | TruncateResponse deleted, default 0 |
| Fix `gc --older-than` parsing | 3.1.0 | `acabe588` | flagTakesValue + 4 tests |
| Lifecycle test coverage | 3.1.0 | `acabe588` | 14 new tests (inspect, gc, config skills) |
| Lifecycle docs | 3.1.0 | `acabe588` | cli-flags.md + DOCS.md |
| Archive specs → `_archive/` | 3.1.0 | `acabe588` | Docs are ground truth |
| BACKLOG.md consolidation | 3.1.0 | `acabe588` | Replaces FEATURES.md |
| F-7: skill loading root cause (F-7) | 3.1.1 | `coordinator` | Root cause was absent search_paths (pre-fix session). search_paths fix covers GSD scenario. Ghost-dir bug in collectSkills fixed + test. |

---
name: agent-mux
description: |
  Dispatch workers across Codex, Claude, and Gemini through one CLI and one
  JSON contract. Tool, not orchestrator — the LLM decides; agent-mux dispatches.
  Use for: spawning workers, running pipelines, recovering timed-out dispatches,
  async background dispatch, mid-flight steering, parsing output contracts,
  and multi-model coordination.
  Keywords: subagent, dispatch, worker, codex, claude, gemini, pipeline, role,
  variant, recover, signal, agent-mux, spawn agent, engine, multi-model, async,
  steer, wait, poll.
---

# agent-mux

Dispatch substrate. One CLI, one JSON contract, three engines. TOML-driven
roles encode engine + model + effort + timeout + skills into one flag. The
calling LLM decides what to do — agent-mux handles the how.

## 1. Design Principles

These are load-bearing. They explain why things work the way they do.

**Tool, not orchestrator.** agent-mux resolves inputs and dispatches. It does
not decide strategy. Multi-step behavior lives in TOML-defined pipelines.

**Job done is holy.** Artifact directories and dispatch metadata exist before
the harness starts. Partial work is preserved across timeout, crash, or cancel.

**Errors are steering signals.** Every failure carries a code, message,
suggestion, and retryability flag. Parse them programmatically, not as text.

**Single-shot with curated context.** The default path is one spec into one
engine. Fan-out only happens through pipelines. Front-load context; don't
make the worker discover its own scope.

## 2. Quick Start

Three patterns that cover 90% of dispatches.

**Programmatic (--stdin)** — primary for coordinators:
```bash
printf '{"role":"lifter","prompt":"Fix the parser bug in src/parser.go:45","cwd":"/repo"}' | agent-mux --stdin
```

**Role shorthand** — when you know the role:
```bash
agent-mux -R=lifter -C=/repo "Implement retries in src/http/client.ts"
```

**Async dispatch** — fire and collect later:
```bash
agent-mux -R=lifter --async -C=/repo "Long-running migration"
# Returns immediately with dispatch_id
agent-mux wait <dispatch_id>       # blocks until done
agent-mux result <dispatch_id>     # retrieves response
```

## 3. Dispatch Patterns

> "Which invocation shape fits my task?"

**--stdin (primary)** — JSON from stdin. Use when: programmatic construction,
multiple overrides, recovery context, or CLI flags become unreadable.
```bash
printf '{"role":"lifter","variant":"claude","skills":["react"],"prompt":"...","cwd":"/repo"}' | agent-mux --stdin
```

**-R=role (shorthand)** — You know the role. The common interactive case.
```bash
agent-mux -R=lifter -C=/repo "Implement retries in src/http/client.ts"
```

**Raw flags (escape hatch)** — No role fits.
```bash
agent-mux -E=codex -m=gpt-5.4 -e=high -C=/repo "one-off task"
```

**-P=pipeline** — Multi-step workflow defined in config TOML.
```bash
agent-mux -P=build -C=/repo "Redesign the auth flow"
```
See [references/pipeline-guide.md](references/pipeline-guide.md) for authoring.

## 4. Engine Cognitive Styles

> "Which engine for this task?"

**Claude** resolves ambiguity. Planning, synthesis, review, prompt crafting.
Roles: `architect`, `researcher`. Watch for: skipping validation, over-abstraction.

**Codex** executes precision. Implementation, debugging, focused audits. Needs
surgical scope — one goal, specific files, explicit gate. Roles: `lifter`,
`auditor`, `scout`. Watch for: paralysis on underspecified prompts.
**Sandbox note:** Use `--sandbox none` for reliable multi-file output. The
`workspace-write` sandbox has known file persistence issues when workers write
multiple files. Restrict scope with `--cwd` instead.

**Gemini** provides contrast. Different training, different blind spots. Use as
second opinion or challenge probe. All variants are reasoning-only on this
machine — no file reads, no tool calls. All context must be in the prompt.

**Meta-rule:** If you're sending exploration to Codex or precision execution to
Claude, reconsider.

**Streaming v2:** stderr is silent by default — only bookend events
(`dispatch_start`, `dispatch_end`) and failures are emitted. Use `--stream`
to restore full event output for human debugging.

**Known limitation:** Skill `scripts/` directories only reach Codex workers
currently (Claude/Gemini adapters don't wire `addDirs`).

## 5. Role Quick-Reference

> "Which role matches this task?"

**Scanning:** `scout` (fast, mini) | `explorer` (deep, multi-file, KB-aware)
**Implementation:** `lifter` (standard) | `lifter-deep` (xhigh, hard problems)
**Cheap parallel:** `grunt` (mini) | `batch` (mini, high-volume)
**Research:** `researcher` (Claude, external) | `explorer` (Codex, internal KB)
**Architecture:** `architect` (Claude, strategic reasoning)
**Verification:** `auditor` (Codex xhigh, adversarial review)
**Writing:** `writer` (Codex, voice-matched, full publishing pipeline)
**Specialized:** `handoff-extractor` (session handoff extraction)

Variants swap the engine within a role: `-R=lifter --variant=claude`. Common
variants: `claude`, `gemini`, `mini`, `spark`. Run `agent-mux config roles`
for the live catalog with all engines, models, timeouts, and variants.

## 6. Async Dispatch

> "I need to fire a dispatch and collect the result later."

**Start async:**
```bash
agent-mux -R=lifter --async -C=/repo "Long task"
```
Returns `async_started` JSON immediately with `dispatch_id` and `artifact_dir`.

**Collect results:**
```bash
agent-mux wait <id>                  # block until terminal state, print DispatchResult
agent-mux wait <id> --poll 5s        # custom poll interval
agent-mux result <id>                # get response (blocks by default)
agent-mux result <id> --no-wait      # error if still running
agent-mux result <id> --artifacts    # list artifact files
```

**Mid-flight steering:**
```bash
agent-mux steer <id> status          # read live status from status.json
agent-mux steer <id> abort           # SIGTERM the worker
agent-mux steer <id> nudge           # ask worker to wrap up
agent-mux steer <id> redirect "new instructions here"
agent-mux steer <id> extend 300      # extend watchdog kill threshold
```

**Poll interval precedence:** CLI `--poll` > config.toml `[async].poll_interval` > 60s default.

**Signal (legacy):** `agent-mux --signal=<id> "message"` delivers to inbox.
Ack confirms write, not delivery. Keep signals to one crisp sentence.

## 7. Output Parsing

> "What do I check in the response?"

**Single dispatch** — 4 fields to check:
- `status`: `completed` | `timed_out` | `failed`. Never treat non-completed as done.
- `response`: The worker's answer. Check it's non-empty and not a placeholder.
- `response_truncated` + `full_output_path`: If truncated, read the full file.
- `activity.files_changed`: What the worker actually modified.

**Pipeline** returns `PipelineResult`, NOT `DispatchResult`:
- `status`: `completed` | `partial` | `failed`
- `steps[]`: Per-step worker results with `summary` and `artifact_dir`
- `final_step`: The last step's output

Always parse JSON from stdout. Never parse as text. For full schemas:
[references/output-contract.md](references/output-contract.md).

## 8. Failure & Recovery

> "The dispatch failed or timed out. What now?"

```
status?
 timed_out + files_changed non-empty
   -> --recover=<dispatch_id> with continuation prompt (the delta, not re-brief)
 timed_out + files_changed empty
   -> Prompt too broad. Reframe with tighter scope. Retry ONCE.
 timed_out + heartbeat_count == 0
   -> Worker never started. Config error. Check and retry once.
 failed + error.retryable
   -> Fix cause (wrong flag, missing --network). Retry ONCE.
 failed + not retryable
   -> Structural. Escalate.
 Second failure on same step
   -> STOP. Problem is the prompt or the scope, not the effort level.
      Reframe entirely or escalate.
```

For recovery mechanics, signal delivery, artifact layout, and liveness
watchdog details: [references/recovery-guide.md](references/recovery-guide.md).

## 9. Codex Prompt Discipline

> "How do I get clean output from Codex?"

Codex output quality is a direct function of prompt specificity. Rules:

1. **Exact file paths.** Never "explore src/". If unknown, scout first.
2. **Inline critical context.** The function, the type, the error — not "read file X".
3. **One deliverable.** "Write X to Y" or "modify Z in W". Not "audit and fix."
4. **Word budget.** "3-sentence summary" or "file path + verdict only."
5. **State the verification gate.** How the worker proves it's done.
6. **Use `--context-file` for bulk.** Don't inline giant specs into the prompt.
7. **Pass `--network`/`-n` for web access.** Codex sandbox is offline by default.

BAD: `"Read all files in src/auth/ and identify issues"`
GOOD: `"In src/auth/middleware.go:45-80, validateToken() doesn't handle expired
refresh tokens. Add a check at line 67. refreshToken is at src/auth/refresh.go:12-30."`

## 10. Timeout Alignment

> "My wrapper kills the process before agent-mux can clean up."

Wrapper timeout MUST exceed agent-mux timeout by at least 60 seconds.

| Effort | agent-mux timeout | Wrapper minimum |
|--------|-------------------|-----------------|
| `low` | 120s | 180s (180000ms) |
| `medium` | 600s | 660s (660000ms) |
| `high` | 1800s | 1860s (1860000ms) |
| `xhigh` | 2700s | 2760s (2760000ms) |

Roles set their own timeouts. Check with `agent-mux config roles`.

## 11. Anti-Patterns

- **Raw engine/model/effort when a role fits.** Roles exist. Use them.
- **`--permission-mode` with Codex.** Use `--sandbox` instead.
- **Exploration prompts to Codex.** Use Claude for open-ended work.
- **Wrapper timeout == worker timeout.** Add 60s slack.
- **Ignoring `status` field.** `timed_out` is not `completed`.
- **Pipeline output parsed as DispatchResult.** Different JSON shape.
- **Codex with `workspace-write` sandbox for multi-file output.** Use
  `--sandbox none` + `--cwd` for scope control instead.
- **Cross-CWD skill resolution failure.** When dispatching with `--cwd`
  pointing to a different project, skills search: cwd, configDir, then
  `[skills] search_paths`. Fix: add missing paths to `search_paths` in
  config.toml, or use `--skip-skills` as escape hatch.
  Run `agent-mux config skills` to verify discoverability.
- **Using `-V` for `--version`.** `-V` no longer exists. Use `--version`.

## 12. Reference Index

> "Where do I go for deeper detail?"

**Operational references** (skill/references/):

| Reference | Read when |
|-----------|-----------|
| [cli-flags.md](references/cli-flags.md) | Complete flag table, DispatchSpec JSON fields, precedence |
| [async-and-steering.md](references/async-and-steering.md) | Async dispatch, wait, steer, poll intervals, status.json |
| [output-contract.md](references/output-contract.md) | DispatchResult/PipelineResult schemas, error codes |
| [recovery-guide.md](references/recovery-guide.md) | Recovery decision tree, signal mechanics, artifact layout |
| [prompting-guide.md](references/prompting-guide.md) | Per-engine prompt patterns, word budgets, context loading |
| [config-and-roles.md](references/config-and-roles.md) | TOML structure, role authoring, variants, skill injection |
| [pipeline-guide.md](references/pipeline-guide.md) | Pipeline TOML, fan-out, handoff modes |

**Architecture and internals** (docs/):
`docs/architecture.md`, `docs/dispatch.md`, `docs/engines.md`,
`docs/config.md`, `docs/pipelines.md`, `docs/recovery.md`,
`docs/lifecycle.md`, `docs/async.md`, `docs/steering.md`.

Discover paths: `agent-mux config --sources`.

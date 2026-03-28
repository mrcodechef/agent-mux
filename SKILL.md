---
name: agent-mux
description: |
  Dispatch workers across Codex, Claude, and Gemini through one CLI and one
  JSON contract. Tool, not orchestrator — the LLM decides; agent-mux dispatches.
  Use for: spawning workers, running pipelines, recovering timed-out dispatches,
  parsing output contracts, and multi-model coordination.
  Keywords: subagent, dispatch, worker, codex, claude, gemini, pipeline, role,
  variant, recover, signal, agent-mux, spawn agent, engine, multi-model.
---

# agent-mux

Dispatch substrate. One CLI, one JSON contract, three engines. TOML-driven
roles encode engine + model + effort + timeout + skills into one flag. The
calling LLM decides what to do — agent-mux handles the how.

## 1. First Action

> "What roles and pipelines are available?"

```bash
agent-mux config roles       # live role catalog with engines/models/timeouts
agent-mux config pipelines   # named multi-step workflows
agent-mux config models      # valid models per engine
agent-mux config --sources   # which config files are loaded
```

## 2. Dispatch Pattern Selection

> "Which invocation shape fits my task?"

**Role CLI** (`-R=role`) — You know the role. This is the common case.
```bash
agent-mux -R=lifter -C=/repo "Implement retries in src/http/client.ts"
```

**JSON stdin** (`--stdin`) — Multiple overrides, programmatic construction, or
recovery context. When CLI flags become unreadable.
```bash
printf '{"role":"lifter","variant":"claude","skills":["react"],"prompt":"...","cwd":"/repo"}' | agent-mux --stdin
```

**Raw flags** (`-E= -m=`) — No role fits. Escape hatch.
```bash
agent-mux -E=codex -m=gpt-5.4 -e=high -C=/repo "one-off task"
```

**Named pipeline** (`-P=name`) — Multi-step workflow defined in config.
Invoke-only; see [references/pipeline-guide.md](references/pipeline-guide.md)
for authoring.
```bash
agent-mux -P=build -C=/repo "Redesign the auth flow"
```

## 3. Engine Cognitive Styles

> "Which engine for this task?"

**Claude** resolves ambiguity. Planning, synthesis, review, prompt crafting.
Roles: `architect`, `researcher`. Watch for: skipping validation, over-abstraction.

**Codex** executes precision. Implementation, debugging, focused audits. Needs
surgical scope — one goal, specific files, explicit gate. Roles: `lifter`,
`auditor`, `scout`. Watch for: paralysis on underspecified prompts.

**Gemini** provides contrast. Different training, different blind spots. Use as
second opinion or challenge probe. All variants are reasoning-only on this
machine — no file reads, no tool calls. All context must be in the prompt.

**Meta-rule:** If you're sending exploration to Codex or precision execution to
Claude, reconsider.

**Known limitation:** Skill `scripts/` directories only reach Codex workers
currently (Claude/Gemini adapters don't wire `addDirs`). Skill path resolution
is relative to `--cwd` — when dispatching from unusual cwd, pass explicit
`-C=` pointing to the repo root.

## 4. Role Quick-Reference

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

## 5. Dispatch Flow

> "What's the sequence every dispatch follows?"

1. **Construct command.** Pick pattern from section 2. Role is preferred.
2. **Preview** (for non-trivial dispatches):
   ```bash
   agent-mux preview -R=lifter -C=/repo "Complex task"
   ```
   Outputs the resolved DispatchSpec as JSON without executing. Review engine,
   model, skills, timeout. Skip for routine dispatches with well-known roles.
3. **Execute.** Run without `preview`. Parse stdout JSON.
4. **Parse output.** Check the 4 critical fields (section 6).
5. **Verify.** Does the result satisfy the task's verification gate?
6. **Recover if needed.** Follow section 7 decision tree.

## 6. Output Parsing

> "What do I check in the response?"

**Single dispatch** — 4 fields to check:
- `status`: `completed` | `timed_out` | `failed`. Never treat non-completed as done.
- `response`: The worker's answer. Check it's non-empty and not a placeholder.
- `response_truncated` + `full_output_path`: If truncated, read the full file.
- `activity.files_changed`: What the worker actually modified.

**Pipeline** returns `PipelineResult`, not `DispatchResult`:
- `status`: `completed` | `partial` | `failed`
- `steps[]`: Per-step worker results with `summary` and `artifact_dir`
- `final_step`: The last step's output

Always parse JSON from stdout. Never parse as text. For full schemas:
[references/output-contract.md](references/output-contract.md).

## 7. Failure Recovery

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

**Signal for mid-flight steering** (running dispatch only):
```bash
agent-mux --signal=<dispatch_id> "Narrow scope to parser module only"
```
Ack confirms inbox write, not delivery. Keep signals to one crisp sentence.
See [references/recovery-signal.md](references/recovery-signal.md) for
mechanics.

## 8. Codex Prompt Discipline

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

## 9. Timeout Alignment

> "My wrapper kills the process before agent-mux can clean up."

Wrapper timeout MUST exceed agent-mux timeout by at least 60 seconds.

| Effort | agent-mux timeout | Wrapper minimum |
|--------|-------------------|-----------------|
| `low` | 120s | 180s (180000ms) |
| `medium` | 600s | 660s (660000ms) |
| `high` | 1800s | 1860s (1860000ms) |
| `xhigh` | 2700s | 2760s (2760000ms) |

Roles set their own timeouts. Check with `agent-mux config roles`.

## 10. Anti-Patterns

- **Raw engine/model/effort when a role fits.** Roles exist. Use them.
- **Fire-and-forget non-trivial dispatches.** Preview first.
- **`--permission-mode` with Codex.** Use `--sandbox` instead.
- **Exploration prompts to Codex.** Use Claude for open-ended work.
- **Wrapper timeout == worker timeout.** Add 60s slack.
- **Ignoring `status` field.** `timed_out` is not `completed`.
- **Pipeline output parsed as DispatchResult.** Different JSON shape.

## 11. Reference Index

> "Where do I go for deeper detail?"

| Reference | Read when |
|-----------|-----------|
| [cli-flags.md](references/cli-flags.md) | You need the complete flag table or DispatchSpec JSON fields |
| [config-guide.md](references/config-guide.md) | You need TOML structure, variant tables, config resolution order |
| [output-contract.md](references/output-contract.md) | You need exact JSON schemas for dispatch, pipeline, signal, events |
| [engine-comparison.md](references/engine-comparison.md) | You need engine harness details, permission/sandbox mapping |
| [prompting-guide.md](references/prompting-guide.md) | You are crafting prompts, writing pipeline steps, phrasing recovery |
| [pipeline-guide.md](references/pipeline-guide.md) | You need pipeline TOML structure, fan-out, handoff modes |
| [recovery-signal.md](references/recovery-signal.md) | You need recovery continuation, signal delivery, artifact layout |

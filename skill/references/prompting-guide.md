# Prompting Guide

Per-engine prompt patterns, word budgets, context loading, and recovery phrasing.

---

## General Rules

Every dispatch prompt should provide:

1. **One job** — a single deliverable, not a compound task
2. **Explicit scope** — files, directories, boundaries
3. **A verification gate** — how the worker proves it is done
4. **A clear output shape** — what to report back

Default prompt frame:

```text
Goal:
Scope:
Constraints:
Verification gate:
Expected output:
```

Do not spend tokens explaining agent-mux to the worker. The worker already
has the harness and artifact directory. Prompt the task, not the tool.

---

## Codex Prompting

Codex executes precision. Give it surgical scope and it delivers clean work.
Give it ambiguity and it hallucinates scope or stalls.

### What works

- Exact file paths (never "explore src/")
- Concrete edits with clear boundaries
- Specific commands to run after the edit
- A hard verification gate
- One deliverable per dispatch

### What fails

- "Explore the repo and tell me what you find"
- Multi-goal prompts with fuzzy priorities
- Hidden verification expectations
- Prompts that require discovering scope

### Recommended shape

```text
Implement X in these files:
- src/parser.go
- internal/parser/types.go

Constraints:
- do not change public API names
- keep changes under 150 LOC

Verification gate:
- go test ./internal/parser ./cmd/tool passes
- malformed input now returns ErrInvalidToken

Report: what changed, which tests ran, any follow-up risk
```

### Model guidance

| Model | Use for |
|-------|---------|
| `gpt-5.4` | Primary worker for real code work |
| `gpt-5.4-mini` | Cheaper high-volume parallel workers |
| `gpt-5.3-codex-spark` | Quick scans, light edits, broad fan-out |

---

## Claude Prompting

Claude handles ambiguity, planning, synthesis, and review.

### Recommended shape

```text
Task: Pressure-test the migration plan for auth.
Scope: read the plan, identify top 3 failure modes, propose safer rollout.
Output: verdict first, then revised sequence, then open questions.
```

For read-only work, use `--permission-mode=plan` or the appropriate role.
If the job is implementation at its core, plan in Claude and hand coding
to Codex via a second dispatch or pipeline.

---

## Gemini Prompting

Best as a fast alternate take, not primary implementation.

### Recommended shape

```text
Review this patch for hidden risks.
Focus on: backwards compatibility, deployment hazards, missing tests.
Keep the answer to 5 bullets max.
```

Keep Gemini prompts narrower than Claude prompts. Treat as a contrast probe.

**Important:** Gemini variants are reasoning-only on this machine. No file
reads, no commands, no tool calls. All context must be in the prompt.

---

## Context-Loading Tools

### --skill / "skills"

Reusable runbooks from named skill directories. Keeps prompts composable.
```json
{"role":"lifter","skills":["react","test-writer"],"prompt":"...","cwd":"/repo"}
```

### --context-file / "context_file"

Large briefs already written to disk. Injects preamble telling worker to
read `$AGENT_MUX_CONTEXT` before starting.

### system_prompt / "system_prompt"

Run-level framing, not giant manuals:
```json
{"role":"auditor","system_prompt":"Prioritize regressions over style","prompt":"..."}
```

### --profile / "profile"

Persistent personas with default engine/model. Put stable persona in the
body, dynamic task in the prompt.

---

## Recovery and Signal Phrasing

### Recovery

`--recover` already builds a continuation prompt listing old artifacts.
Your prompt should be the delta.

- Good: "Continue by finishing test coverage for the parser"
- Bad: "Re-explain the entire project from scratch"

### Signals

Signals become a short resumed turn. Keep them crisp.

- Good: "Focus on auth paths only; skip docs."
- Good: "Stop after green tests and summarize remaining edge cases."
- Bad: Multi-paragraph redesigns or contradictory instructions.

---

## Pipeline Step Authoring

### Good patterns

1. `architect` produces plan
2. `lifter` implements
3. `auditor` verifies

### Fan-out guidance

Use `parallel > 1` only when workers are independent (separate files,
modules, or research threads). `worker_prompts` length must match `parallel`.

### Handoff mode selection

| Mode | When to use |
|------|-------------|
| `summary_and_refs` | Default; works for most pipelines |
| `refs_only` | Next worker only needs file paths |
| `full_concat` | Rare; expensive. Full output inline |

Next step should receive minimum context that still lets it succeed.

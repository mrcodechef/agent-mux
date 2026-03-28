# Prompting Guide

## Contents

- General rules
- Codex prompting
- Claude prompting
- Gemini prompting
- Context-loading tools
- Recovery and signal phrasing
- Pipeline step authoring

---

## General Rules

Each worker dispatch should provide:

1. One job
2. Explicit scope (files, directories, boundaries)
3. A verification gate (how the worker proves it is done)
4. A clear output shape (what to report back)

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

Codex is the implementation hammer. It wants precision.

### What works

- Exact files or directories
- Concrete edits with clear boundaries
- Specific commands to run
- A hard verification gate
- One deliverable per dispatch

### What fails

- "Explore the repo and tell me what you find"
- Multi-goal prompts with fuzzy priorities
- Hidden verification expectations
- Giant context blobs that should have been a `--context-file`

### Recommended shape

```text
Implement X in these files:
- src/parser.go
- internal/parser/types.go

Constraints:
- do not change public API names
- keep changes under 150 LOC if possible

Verification gate:
- go test ./internal/parser ./cmd/tool passes
- malformed input now returns ErrInvalidToken

Report:
- what changed
- which tests ran
- any follow-up risk
```

### Model guidance

| Model | Use for |
|-------|---------|
| `gpt-5.4` | Primary worker for real code work |
| `gpt-5.4-mini` | Cheaper high-volume parallel workers |
| `gpt-5.3-codex-spark` | Quick scans, light edits, broad fan-out |

Use `--reasoning=high` for most code work. Reserve `xhigh` for deep audits
or very tricky failures.

---

## Claude Prompting

Claude handles ambiguity, planning, synthesis, and review.

### What works

- Open-ended analysis and tradeoff evaluation
- Review framing and assumption challenging
- Prompt crafting for downstream workers
- Writing, restructuring, and documentation

### Recommended shape

```text
Task:
Pressure-test the migration plan for the auth subsystem.

Scope:
- read the plan
- identify the top 3 failure modes
- propose a safer rollout sequence

Output:
- verdict first
- then the revised sequence
- then open questions
```

For read-only work, use `--permission-mode=plan` or the appropriate role.

If the job is implementation at its core, plan in Claude and hand the coding
to Codex via a second dispatch or a pipeline.

---

## Gemini Prompting

Gemini is best as a fast alternate take, not the primary implementation engine.

### Best uses

- Second opinion on a patch or plan
- Quick challenge of assumptions
- Fast comparison against Codex/Claude output

### Recommended shape

```text
Review this patch for hidden risks.

Focus on:
- backwards compatibility
- deployment hazards
- missing tests

Keep the answer to 5 bullets max.
```

Keep Gemini prompts narrower than Claude prompts. Treat as a contrast probe.

### Important limitation

Gemini variants are reasoning-only on this machine. No file reads, no
commands, no tool calls. All context must be embedded in the prompt.

---

## Context-Loading Tools

### --skill (JSON: "skills")

Use for reusable runbooks or domain instructions from a named skill directory.
Keeps prompts short and composable.

```json
{"role":"lifter","skills":["react","test-writer"],"prompt":"...","cwd":"/repo"}
```

### --context-file (JSON: "context_file")

Use when the coordinator already wrote a large brief to disk. v2 injects a
preamble telling the worker to read `$AGENT_MUX_CONTEXT` before starting.

Good uses: bulky handoff notes, long specs, structured research dumps.

### system_prompt (JSON field)

Use for run-level framing, not giant project manuals:

```json
{"role":"auditor","system_prompt":"Prioritize regressions over style","prompt":"...","cwd":"/repo"}
```

### --profile / "profile" (JSON)

Use for persistent personas with default engine/model settings. Put stable
persona text in the coordinator body, dynamic task details in the prompt.

---

## Recovery and Signal Phrasing

### Recovery

`--recover` / `continues_dispatch_id` already builds a continuation prompt
listing old artifacts. Your extra prompt should be the delta, not a re-brief.

Good:
```text
Continue by finishing test coverage for the parser and summarize what remains.
```

Bad:
```text
Re-explain the entire project and restate the old run from scratch.
```

### Signal

Signals become a short resumed turn inside the harness. Keep them crisp.
The `--signal` ack only confirms the inbox write; injection waits for an
event boundary.

Good:
- `Focus on auth paths only; skip docs.`
- `Stop after green tests and summarize remaining edge cases.`
- `Do not touch migrations; stay inside src/parser/.`

Bad:
- Multi-paragraph redesigns
- Contradictory instructions
- Giant specs that should have been written to disk

---

## Pipeline Step Authoring

### Good pipeline patterns

1. `architect` step produces a plan
2. `lifter` step implements
3. `auditor` step verifies

### Fan-out guidance

Use `parallel > 1` only when workers are independent:
- Separate files or modules
- Separate review slices
- Independent research threads

If you provide `worker_prompts`, the list length must match `parallel`.

### Handoff mode selection

| Mode | When to use |
|------|-------------|
| `summary_and_refs` | Default; works for most pipelines |
| `refs_only` | Next worker only needs file paths |
| `full_concat` | Rare; loads full output into next step (expensive) |

### Key rule

The next step should receive the minimum context that still lets it succeed.
`full_concat` is the expensive option. Prefer `summary_and_refs`.

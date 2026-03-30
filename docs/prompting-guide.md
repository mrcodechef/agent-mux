# Prompting Guide

agent-mux dispatch quality is a direct function of prompt specificity. This guide covers how to write prompts that produce clean results across all three engines.

The core rule: prompt the task, not the tool. The worker already has the harness, the artifact directory, and any injected skills. Your prompt should specify what to do, with what scope, and how to prove it worked.

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

Do not spend tokens explaining agent-mux to the worker. Do not include giant context blobs that should be a `--context-file`.

## Codex Prompting

Codex is the implementation engine. It executes precision. Give it surgical scope and it delivers clean work. Give it ambiguity and it either hallucinates scope or stalls.

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
- Prompts that require the worker to discover its own scope

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
| --- | --- |
| `gpt-5.4` | Primary worker for real code work |
| `gpt-5.4-mini` | Cheaper high-volume parallel workers |
| `gpt-5.3-codex-spark` | Quick scans, light edits, broad fan-out |

Use `--reasoning=high` for most code work. Reserve `xhigh` for deep audits or tricky failures.

### The pre-extraction pattern

For complex tasks, scout first:

1. Use a `scout` or `explorer` to extract: file paths, function signatures, error traces, type definitions
2. Inline the extracted context into the Codex prompt
3. Codex executes with zero exploration overhead

This turns an ambiguous prompt into a surgical one.

## Claude Prompting

Claude resolves ambiguity. It handles planning, synthesis, review, and open-ended analysis.

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

For read-only work, use `--permission-mode=plan` or an appropriate role.

If the job is implementation at its core, plan in Claude and hand the coding to Codex via a second dispatch or a pipeline.

## Gemini Prompting

Gemini is a contrast probe, not the primary implementation engine.

### Best uses

- Second opinion on a patch or plan
- Quick challenge of assumptions
- Fast comparison against Codex or Claude output

### Recommended shape

```text
Review this patch for hidden risks.

Focus on:
- backwards compatibility
- deployment hazards
- missing tests

Keep the answer to 5 bullets max.
```

Keep Gemini prompts narrower than Claude prompts.

### Limitation

Gemini variants are reasoning-only on this machine. No file reads, no commands, no tool calls. All context must be embedded in the prompt or passed via `--context-file`.

## Context-Loading Tools

### --skill (JSON: "skills")

Reusable runbooks from a named skill directory. Keeps prompts short and composable.

```bash
agent-mux -R=lifter --skill react --skill test-writer -C /repo "Implement error boundary"
```

### --context-file (JSON: "context_file")

For large briefs the coordinator wrote to disk. agent-mux injects a preamble telling the worker to read `$AGENT_MUX_CONTEXT` before starting.

Good uses: bulky handoff notes, long specs, structured research dumps.

### system_prompt (JSON field)

Run-level framing, not giant project manuals:

```json
{"role":"auditor","system_prompt":"Prioritize regressions over style","prompt":"...","cwd":"/repo"}
```

### --profile / "profile" (JSON)

Persistent personas with default engine/model settings. Put stable persona text in the profile body, dynamic task details in the prompt.

## Recovery Prompting

`--recover` builds a continuation prompt listing previous artifacts. Your extra prompt should be the delta, not a re-brief.

Good:

```text
Continue by finishing test coverage for the parser and summarize what remains.
```

Bad:

```text
Re-explain the entire project and restate the old run from scratch.
```

## Signal Phrasing

Signals become a short resumed turn inside the harness. Keep them to one crisp sentence.

The `--signal` ack confirms inbox write, not delivery. Injection waits for an event boundary.

Good:

- `Focus on auth paths only; skip docs.`
- `Stop after green tests and summarize remaining edge cases.`
- `Do not touch migrations; stay inside src/parser/.`

Bad:

- Multi-paragraph redesigns
- Contradictory instructions
- Giant specs that should have been written to disk

## Pipeline Step Prompting

### Good pipeline patterns

1. `architect` step produces a plan
2. `lifter` step implements from the plan
3. `auditor` step verifies the implementation

### Fan-out guidance

Use `parallel > 1` only when workers are independent:

- Separate files or modules
- Separate review slices
- Independent research threads

When `worker_prompts` is set, the list length must match `parallel`.

### Handoff mode selection

| Mode | When to use |
| --- | --- |
| `summary_and_refs` | Default; works for most pipelines |
| `refs_only` | Next worker only needs file paths |
| `full_concat` | Rare; loads full output into the next step (expensive) |

The next step should receive the minimum context that still lets it succeed. `full_concat` is the expensive option. Default to `summary_and_refs`.

## Cross-References

- [Dispatch](./dispatch.md) for prompt composition order and DispatchSpec fields
- [Engines](./engines.md) for per-engine system prompt handling
- [Config](./config.md) for skill injection and profile loading
- [Pipelines](./pipelines.md) for pipeline step TOML and handoff modes
- [Steering](./steering.md) for signal delivery mechanics
- [Recovery](./recovery.md) for recovery continuation prompts

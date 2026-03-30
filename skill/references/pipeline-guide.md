# Pipeline Guide

Pipeline TOML structure, fan-out patterns, handoff modes, and step authoring.

---

## Concept

Pipelines are TOML-defined multi-step execution sequences. Steps run
sequentially; workers within a fan-out step run concurrently. Pipelines
are data, not code — adding one requires no recompilation.

Key characteristics:
- Steps execute sequentially
- Workers within a step execute concurrently (up to `max_parallel`)
- Completed step outputs are preserved even when later steps fail
- Returns `PipelineResult`, NOT `DispatchResult`

---

## TOML Structure

```toml
[pipelines.build]
max_parallel = 4

[[pipelines.build.steps]]
name = "plan"
role = "architect"
pass_output_as = "plan"
handoff_mode = "summary_and_refs"

[[pipelines.build.steps]]
name = "implement"
role = "lifter"
receives = "plan"
pass_output_as = "code"
handoff_mode = "refs_only"

[[pipelines.build.steps]]
name = "verify"
role = "auditor"
receives = "code"
```

---

## Step Fields

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | yes | Step identifier |
| `role` | string | recommended | Role from config |
| `variant` | string | no | Role variant |
| `engine` | string | no | Direct engine override |
| `model` | string | no | Direct model override |
| `effort` | string | no | Direct effort override |
| `timeout` | int | no | Timeout in seconds |
| `parallel` | int | no | Fan-out count (0 or 1 = sequential) |
| `worker_prompts` | string[] | no | Per-worker focus (length must match `parallel`) |
| `receives` | string | no | Named output from prior step |
| `pass_output_as` | string | no | Name for this step's output |
| `handoff_mode` | string | no | Default: `summary_and_refs` |

### Validation rules

- At least one step is required
- `receives` must reference a `pass_output_as` from a preceding step
- `pass_output_as` names must be unique across steps
- `worker_prompts` length must match `parallel`
- `parallel` must be >= 1 when set

---

## Fan-Out

Setting `parallel > 1` spawns concurrent workers:

```toml
[[pipelines.research.steps]]
name = "scout"
role = "scout"
parallel = 3
pass_output_as = "leads"
```

Each worker gets its own dispatch ID and artifact directory at
`<pipeline_dir>/step-N/worker-M/`.

### Per-worker prompts

```toml
[[pipelines.tenx.steps]]
name = "fan-out"
role = "grunt"
parallel = 3
worker_prompts = [
  "Focus on src/auth/",
  "Focus on src/api/",
  "Focus on src/storage/"
]
```

Each worker prompt is appended to the base prompt for that worker only.
Workers are capped by `max_parallel` (default 8).

---

## Handoff Modes

Control how step output passes to the next step's prompt.

### summary_and_refs (default)

Next step receives:
- Worker summary (max 2000 chars, from `## Summary`/`## Handoff`)
- Path to full output file
- Artifact directory path

Best for most pipelines. Keeps context lean.

### refs_only

Next step receives:
- Path to full output file
- Artifact directory path

No inline content. Worker must read the file itself.

### full_concat

Next step receives:
- Full worker output text inline in the prompt

Expensive. Use only when next step truly needs entire output.

---

## Prompt Construction

For each step, agent-mux constructs the worker prompt from:

1. **Received handoff text** from the step named in `receives`
2. **Base user prompt** from the original dispatch
3. **Pass-output instruction** if `pass_output_as` is set

Concatenated with double newlines. Workers inherit `context_file` from
the base spec unchanged.

---

## Live Pipelines

Three pipelines in the operational config:

### build

Sequential: architect -> lifter -> auditor

| Step | Role | Handoff |
|------|------|---------|
| plan | architect | summary_and_refs |
| implement | lifter | refs_only |
| verify | auditor | summary_and_refs |

### research

Scout fan-out -> deep dive -> synthesis

| Step | Role | Parallel | Handoff |
|------|------|----------|---------|
| scout | scout | 3 | summary_and_refs |
| deep-dive | researcher | 1 | full_concat |
| synthesize | architect | 1 | summary_and_refs |

### tenx

High-parallelism fan-out -> merge audit

| Step | Role | Parallel | Handoff |
|------|------|----------|---------|
| fan-out | grunt | 8 | refs_only |
| merge-audit | auditor | 1 | summary_and_refs |

---

## Pipeline Invocation

```bash
agent-mux -P=build -C=/repo "Redesign the auth flow"
```

Via JSON:
```json
{"pipeline":"build","prompt":"Redesign the auth flow","cwd":"/repo"}
```

Pipeline mode still needs a top-level engine or config default.

### Artifacts

```
/tmp/agent-mux/<id>/pipeline/
  step-0/worker-0/_dispatch_meta.json, events.jsonl, output.md
  step-1/worker-0/...
  step-1/worker-1/...
```

Each worker writes output to `output.md` in its artifact directory.

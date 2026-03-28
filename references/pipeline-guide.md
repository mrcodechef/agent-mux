# Pipeline Guide

## Contents

- Pipeline concept
- TOML definition structure
- Step fields
- Fan-out and parallel execution
- Handoff modes
- Prompt construction
- Live pipelines (coordinator config)
- Pipeline invocation

---

## Pipeline Concept

Pipelines are TOML-defined multi-step execution sequences. Each step dispatches
one or more workers, and step outputs chain to the next step via handoff text.

Key characteristics:
- Steps execute sequentially
- Workers within a fan-out step execute concurrently
- Completed step outputs are preserved even when later steps fail
- Pipeline mode returns `PipelineResult`, NOT `DispatchResult`

---

## TOML Definition Structure

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
handoff_mode = "summary_and_refs"
```

---

## Step Fields

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | yes | Step identifier |
| `role` | string | recommended | Role from config (resolves engine/model/effort) |
| `variant` | string | no | Role variant to use |
| `engine` | string | no | Override engine for this step |
| `model` | string | no | Override model for this step |
| `effort` | string | no | Override effort for this step |
| `timeout` | int | no | Override timeout in seconds |
| `parallel` | int | no | Fan-out count (0 or 1 = sequential) |
| `worker_prompts` | string[] | no | Per-worker focus for fan-out (length must match `parallel`) |
| `receives` | string | no | Name of prior step's `pass_output_as` |
| `pass_output_as` | string | no | Name for this step's output |
| `handoff_mode` | string | no | `summary_and_refs` (default), `full_concat`, `refs_only` |

### Validation Rules

- At least one step is required
- `receives` must reference a `pass_output_as` from a preceding step
- `pass_output_as` names must be unique across steps
- `worker_prompts` length must match `parallel` when both are set
- `parallel` must be >= 1 when set

---

## Fan-Out and Parallel Execution

Setting `parallel > 1` on a step spawns multiple concurrent workers:

```toml
[[pipelines.research.steps]]
name = "scout"
role = "scout"
parallel = 3
pass_output_as = "leads"
handoff_mode = "summary_and_refs"
```

Workers are capped by `max_parallel` (default 8) from the pipeline config.

Each worker gets:
- Its own dispatch ID
- Its own artifact directory: `<pipeline_dir>/step-N/worker-M/`
- The same base prompt
- Optional per-worker focus via `worker_prompts`

### Per-Worker Prompts

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

---

## Handoff Modes

Handoff controls how a step's output is passed to the next step's prompt.

### summary_and_refs (default)

Next step receives:
- Worker summary (max 2000 chars, extracted from `## Summary`/`## Handoff`)
- Path to full output file
- Artifact directory path

Best for most pipelines. Keeps next step's context lean.

### refs_only

Next step receives:
- Path to full output file
- Artifact directory path

No inline content. Next worker must read the file itself.

### full_concat

Next step receives:
- Full worker output text inline in the prompt

Expensive. Use only when the next step truly needs the entire output and
cannot work from a file reference.

### Rendering

For sequential steps (parallel=1): header + summary/refs/full.
For fan-out steps: header + per-worker sections with index and status.

---

## Prompt Construction

For each step, v2 constructs the worker prompt from:

1. **Received handoff text** from the step named in `receives`
2. **Base user prompt** (from the original dispatch)
3. **Pass-output instruction** if `pass_output_as` is set

These are concatenated with double newlines.

v2 does NOT auto-create a `context_file` for pipeline steps. Workers inherit
any `context_file` from the base spec unchanged.

---

## Live Pipelines (Coordinator Config)

Three pipelines are defined in the operational config:

### build

Sequential: architect -> lifter -> auditor

| Step | Role | Handoff | Notes |
|------|------|---------|-------|
| plan | architect | summary_and_refs | Produces plan |
| implement | lifter | refs_only | Receives plan, produces code |
| verify | auditor | summary_and_refs | Receives code |

### research

Scout fan-out -> deep dive -> synthesis

| Step | Role | Parallel | Handoff | Notes |
|------|------|----------|---------|-------|
| scout | scout | 3 | summary_and_refs | Fan-out scouting |
| deep-dive | researcher | 1 | full_concat | Deep analysis of leads |
| synthesize | architect | 1 | summary_and_refs | Final synthesis |

### tenx

High-parallelism fan-out -> merge audit

| Step | Role | Parallel | Handoff | Notes |
|------|------|----------|---------|-------|
| fan-out | grunt | 8 | refs_only | Broad parallel work |
| merge-audit | auditor | 1 | summary_and_refs | Merge and verify |

---

## Pipeline Invocation

Via JSON (--stdin):

```json
{
  "pipeline": "build",
  "prompt": "Redesign the auth flow",
  "cwd": "/repo",
  "engine": "codex"
}
```

Pipeline mode still needs a top-level engine or config default even if
individual steps override engine/model.

### Pipeline Artifacts

```
/tmp/agent-mux/<dispatch_id>/pipeline/
  step-0/
    worker-0/
      _dispatch_meta.json
      events.jsonl
      output.md
  step-1/
    worker-0/
      ...
    worker-1/
      ...
```

Each worker writes its output to `output.md` in its artifact directory.

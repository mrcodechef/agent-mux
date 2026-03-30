# Pipelines

agent-mux pipelines are multi-step dispatch chains defined in TOML. Steps run sequentially; within a step, workers may fan out in parallel. The Go entry point is `pipeline.ExecutePipeline`.

Pipelines are data, not code. The binary reads the TOML, builds one `DispatchSpec` per worker, and runs them through the same `LoopEngine` used for single dispatches. Adding a pipeline requires no recompilation.

## Pipeline TOML Schema

### PipelineConfig

| Key | Type | Default | Purpose |
| --- | --- | --- | --- |
| `max_parallel` | int | `8` | Maximum concurrent workers across all steps |
| `steps` | []PipelineStep | required | Ordered step definitions |

### PipelineStep

| Field | Type | Effect |
| --- | --- | --- |
| `name` | string | Step label in handoff headers and result output |
| `role` | string | Role name for this step's workers |
| `variant` | string | Variant within the role |
| `engine` | string | Direct engine override (bypasses role) |
| `model` | string | Direct model override (bypasses role) |
| `effort` | string | Direct effort override (bypasses role) |
| `timeout` | int | Overrides `spec.TimeoutSec` when greater than zero |
| `parallel` | int | Worker count; 0 or 1 means sequential (single worker) |
| `worker_prompts` | []string | Per-worker focus directives; length must equal `parallel` |
| `receives` | string | Name of a prior step's output to consume as handoff input |
| `pass_output_as` | string | Name for this step's output, referenced by downstream `receives` |
| `handoff_mode` | string | How output is rendered for the next step |

Example:

```toml
[pipelines.build]
max_parallel = 4

[[pipelines.build.steps]]
name = "plan"
role = "architect"
pass_output_as = "plan"

[[pipelines.build.steps]]
name = "implement"
role = "lifter"
parallel = 3
worker_prompts = ["auth module", "data layer", "API routes"]
receives = "plan"
pass_output_as = "code"

[[pipelines.build.steps]]
name = "review"
role = "auditor"
receives = "code"
```

## Execution Model

`ExecutePipeline` is the single entry point. It stamps a ULID pipeline ID on every worker spec for traceability.

Steps execute in order. The pipeline does not skip ahead or reorder. Within a step, when `parallel > 1`, all workers for that step launch concurrently up to `max_parallel`.

Each worker receives the same base prompt (from the CLI or stdin) plus any handoff context from prior steps. When `worker_prompts` is set, each worker also receives its individual focus directive.

## Fan-Out

Fan-out uses a semaphore channel of size `MaxParallel` with `sync.WaitGroup`. This is intentionally not `errgroup`: `errgroup.WithContext` cancels remaining goroutines on the first error, which is wrong for fan-out where you want all workers to complete and then collect partial results.

Results land in a pre-allocated slice indexed by worker position, so output ordering is deterministic regardless of completion order.

## Partial Success

Three rules govern step outcomes:

| Condition | Step Status | Pipeline Behavior |
| --- | --- | --- |
| All workers failed, none succeeded | `failed` | Pipeline stops |
| Some workers succeeded, some failed | `partial` | Pipeline continues |
| All workers succeeded | `completed` | Pipeline continues |

Both `WorkerCompleted` and `WorkerTimedOut` count as succeeded. A timed-out worker that produced partial artifacts is more useful than a clean failure with nothing.

## Handoff Modes

| Mode | Content Passed to Next Step |
| --- | --- |
| `summary_and_refs` | Summary text plus path to `output.md` and artifact directory. Default. |
| `full_concat` | Full content of `output.md`. Falls back to refs if the file is missing. |
| `refs_only` | Only the `output.md` path and artifact directory path. |

## Validation

`ValidatePipeline` runs before execution and rejects:

- Zero steps
- Forward references in `receives` (a step cannot consume output from a step that hasn't run yet)
- Duplicate `pass_output_as` names
- Mismatched `worker_prompts` and `parallel` lengths

## PipelineResult

Pipeline output is a `PipelineResult`, not a `DispatchResult`. The JSON shape differs.

```go
type PipelineResult struct {
    PipelineID string       `json:"pipeline_id"`   // ULID
    Status     string       `json:"status"`        // "completed" | "partial" | "failed"
    Steps      []StepOutput `json:"steps"`
    FinalStep  *StepOutput  `json:"final_step"`
    DurationMS int64        `json:"duration_ms"`
}

type StepOutput struct {
    StepName    string         `json:"step_name"`
    StepIndex   int            `json:"step_index"`
    PipelineID  string         `json:"pipeline_id"`
    HandoffMode string         `json:"handoff_mode"`
    Workers     []WorkerResult `json:"workers"`
    HandoffText string         `json:"handoff_text"`
    Succeeded   int            `json:"succeeded"`
    Failed      int            `json:"failed"`
    TotalMS     int64          `json:"total_ms"`
}

type WorkerResult struct {
    WorkerIndex int          `json:"worker_index"`
    DispatchID  string       `json:"dispatch_id"`
    Status      WorkerStatus `json:"status"`      // "completed" | "timed_out" | "failed"
    Summary     string       `json:"summary"`     // truncated, max 2000 chars
    ArtifactDir string       `json:"artifact_dir"`
    OutputFile  string       `json:"output_file,omitempty"` // path to output.md
    ErrorCode   string       `json:"error_code,omitempty"`
    ErrorMsg    string       `json:"error_msg,omitempty"`
    DurationMS  int64        `json:"duration_ms"`
}
```

Callers must parse pipeline output as `PipelineResult`. Parsing it as `DispatchResult` will fail.

## Cross-References

- [Architecture](./architecture.md) for system overview and package map
- [Dispatch](./dispatch.md) for the single-dispatch contract that each pipeline worker uses
- [Config](./config.md) for TOML pipeline definitions and merge rules
- [Recovery](./recovery.md) for artifact directories and timeout behavior within pipeline steps
- [CLI Reference](./cli-reference.md) for `--pipeline` flag and invocation

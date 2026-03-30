# Dispatch

Dispatch is the core execution path in agent-mux. It takes one `DispatchSpec`, resolves it into a concrete harness invocation, supervises the run, and returns one normalized `DispatchResult`.

This document is self-contained. If you need to know how a prompt becomes a running worker, what fields are legal on the wire, or which CLI invocation selects which mode, this is the reference.

## Dispatch Flow

1. **Config load**: `config.LoadConfig(flags.config, spec.Cwd)` merges the active config layers before any dispatch-specific resolution happens.
2. **Profile resolution**: if `profile` is set, `config.LoadProfile` applies frontmatter defaults, prepends profile skills, and merges the profile companion config when present.
3. **Role resolution**: if `role` is set, `config.ResolveRole` applies the role and optional variant overlay, prepends the role `system_prompt_file`, and merges role skills with CLI or stdin skills taking precedence.
4. **Defaults application**: unresolved `engine`, `model`, `effort`, `max_depth`, `allow_subdispatch`, and `response_max_chars` fields are filled from config defaults, with `effort` falling back to `"high"`.
5. **Timeout resolution**: `timeout_sec` is taken from the explicit spec, then role or profile timeout, then `config.TimeoutForEffort`, while `grace_sec` falls back to `cfg.Timeout.Grace`.
6. **EngineOpts injection**: liveness settings and the default `permission-mode` are written into `spec.EngineOpts` unless the dispatch already set them.
7. **Skill injection**: unless `skip_skills` is true, `config.LoadSkills` searches `cwd`, the active role config directory, and `[skills].search_paths`, then prepends XML-wrapped skill blocks and adds any skill `scripts/` directories to `engine_opts["add-dir"]`.
8. **Context file preamble**: when `context_file` is set and exists, a fixed preamble telling the worker to read `$AGENT_MUX_CONTEXT` is prepended to the user prompt.
9. **Hook check**: deny hooks can abort the dispatch before execution, and allow or warn rules can inject a safety preamble ahead of the user prompt.
10. **Traceability**: `dispatch.EnsureTraceability` fills `salt` and `trace_token`, and the final prompt later receives the dispatch preamble with trace token, dispatch ID, and artifact-dir hint.

After these ten steps, agent-mux resolves the adapter, validates pipeline mode if requested, writes recovery control records for live dispatches, and hands the materialized spec to `LoopEngine.Dispatch`.

## Prompt Composition Order

### System Prompt Layers

1. Role `system_prompt_file`
2. Profile body (`.md` content outside frontmatter)
3. `--system-prompt-file` content
4. `--system-prompt` string

### User Prompt Layers

1. Hook injection
2. Context file preamble
3. Skill blocks
4. Recovery prompt
5. Original user prompt

At execution time, `dispatch.WithPromptPreamble` prepends the trace token, dispatch ID, and artifact-dir instruction ahead of the composed user prompt.

## DispatchSpec

Current source definition:

```go
type DispatchSpec struct {
	DispatchID          string         `json:"dispatch_id"`
	Salt                string         `json:"salt,omitempty"`
	TraceToken          string         `json:"trace_token,omitempty"`
	Engine              string         `json:"engine"`
	Model               string         `json:"model,omitempty"`
	Effort              string         `json:"effort"`
	Prompt              string         `json:"prompt"`
	SystemPrompt        string         `json:"system_prompt,omitempty"`
	Cwd                 string         `json:"cwd"`
	Skills              []string       `json:"skills,omitempty"`
	SkipSkills          bool           `json:"skip_skills,omitempty"`
	Profile             string         `json:"-"`
	Pipeline            string         `json:"pipeline,omitempty"`
	ContextFile         string         `json:"context_file,omitempty"`
	ArtifactDir         string         `json:"artifact_dir"`
	TimeoutSec          int            `json:"timeout_sec,omitempty"`
	GraceSec            int            `json:"grace_sec,omitempty"`
	Role                string         `json:"role,omitempty"`
	Variant             string         `json:"variant,omitempty"`
	MaxDepth            int            `json:"max_depth"`
	AllowSubdispatch    bool           `json:"allow_subdispatch"`
	Depth               int            `json:"depth"`
	ParentDispatchID    string         `json:"parent_dispatch_id,omitempty"`
	PipelineID          string         `json:"pipeline_id,omitempty"`
	PipelineStep        int            `json:"pipeline_step"`
	ContinuesDispatchID string         `json:"continues_dispatch_id,omitempty"`
	Receives            string         `json:"receives,omitempty"`
	PassOutputAs        string         `json:"pass_output_as,omitempty"`
	Parallel            int            `json:"parallel,omitempty"`
	HandoffMode         string         `json:"handoff_mode,omitempty"`
	ResponseMaxChars    int            `json:"response_max_chars,omitempty"`
	EngineOpts          map[string]any `json:"engine_opts,omitempty"`
	FullAccess          bool           `json:"full_access"`
}
```

`Profile` uses custom JSON marshal and unmarshal behavior. The wire key is `profile`; `coordinator` is accepted as an alias on input; and decoding fails if both are present with different values.

### Field Reference

| Field | JSON key | Type | Meaning |
| --- | --- | --- | --- |
| `DispatchID` | `dispatch_id` | `string` | Stable dispatch identifier; generated if absent in stdin mode. |
| `Salt` | `salt` | `string` | Human-readable dispatch label used in logs and metadata. |
| `TraceToken` | `trace_token` | `string` | Stable trace token injected into the prompt preamble and metadata. |
| `Engine` | `engine` | `string` | Adapter name: `codex`, `claude`, or `gemini`. |
| `Model` | `model` | `string` | Model override after role, profile, and default resolution. |
| `Effort` | `effort` | `string` | Effort bucket used for defaults and timeout mapping. |
| `Prompt` | `prompt` | `string` | Final user task before preamble injection. |
| `SystemPrompt` | `system_prompt` | `string` | Composed system prompt content passed to the adapter. |
| `Cwd` | `cwd` | `string` | Working directory used for config discovery and harness execution. |
| `Skills` | `skills` | `[]string` | Named skills to resolve and inject into the prompt. |
| `SkipSkills` | `skip_skills` | `bool` | Disables skill prompt injection while preserving role defaults. |
| `Profile` | `profile` | `string` | Coordinator profile name; `coordinator` is accepted as an input alias. |
| `Pipeline` | `pipeline` | `string` | Named pipeline from config; switches execution from single dispatch to pipeline mode. |
| `ContextFile` | `context_file` | `string` | Large context file path whose contents are referenced through `$AGENT_MUX_CONTEXT`. |
| `ArtifactDir` | `artifact_dir` | `string` | Artifact root for events, status, metadata, and worker outputs. |
| `TimeoutSec` | `timeout_sec` | `int` | Hard execution timeout in seconds. |
| `GraceSec` | `grace_sec` | `int` | Grace period after timeout before forced termination. |
| `Role` | `role` | `string` | Role name resolved from config. |
| `Variant` | `variant` | `string` | Variant name applied within the selected role. |
| `MaxDepth` | `max_depth` | `int` | Maximum recursive dispatch depth allowed for subdispatch. |
| `AllowSubdispatch` | `allow_subdispatch` | `bool` | Enables or disables recursive dispatch from the worker. |
| `Depth` | `depth` | `int` | Current recursion depth; mainly used for pipeline and subdispatch bookkeeping. |
| `ParentDispatchID` | `parent_dispatch_id` | `string` | Parent dispatch ID for nested runs and pipeline children. |
| `PipelineID` | `pipeline_id` | `string` | Pipeline instance ID stamped onto pipeline worker specs. |
| `PipelineStep` | `pipeline_step` | `int` | Zero-based step index for pipeline workers; defaults to `-1` in stdin mode. |
| `ContinuesDispatchID` | `continues_dispatch_id` | `string` | Previous dispatch being recovered or continued. |
| `Receives` | `receives` | `string` | Named input binding from an earlier pipeline step. |
| `PassOutputAs` | `pass_output_as` | `string` | Name under which this worker output is exposed downstream. |
| `Parallel` | `parallel` | `int` | Fan-out count for parallel workers in one pipeline step. |
| `HandoffMode` | `handoff_mode` | `string` | Pipeline handoff strategy such as `summary_and_refs`, `full_concat`, or `refs_only`. |
| `ResponseMaxChars` | `response_max_chars` | `int` | Max length for stored response before truncation and full-output spillover. |
| `EngineOpts` | `engine_opts` | `map[string]any` | Adapter-specific options such as liveness, permission mode, sandbox, and add-dir values. |
| `FullAccess` | `full_access` | `bool` | Tells adapters whether the run should request full filesystem access. |

### Runnable stdin example

```sh
cat <<'JSON' | agent-mux --stdin
{
  "dispatch_id": "01KTESTDISPATCH0000000000000",
  "engine": "codex",
  "prompt": "Print the resolved prompt only.",
  "cwd": ".",
  "artifact_dir": "/tmp/agent-mux/01KTESTDISPATCH0000000000000",
  "full_access": true,
  "allow_subdispatch": true,
  "max_depth": 2,
  "depth": 0,
  "pipeline_step": -1,
  "effort": "high",
  "grace_sec": 60,
  "handoff_mode": "summary_and_refs"
}
JSON
```

## DispatchResult

Current source definition:

```go
type DispatchResult struct {
	SchemaVersion     int               `json:"schema_version"`
	Status            DispatchStatus    `json:"status"`
	DispatchID        string            `json:"dispatch_id"`
	DispatchSalt      string            `json:"dispatch_salt"`
	TraceToken        string            `json:"trace_token"`
	Response          string            `json:"response"`
	ResponseTruncated bool              `json:"response_truncated"`
	FullOutput        *string           `json:"full_output"`
	FullOutputPath    *string           `json:"full_output_path,omitempty"`
	HandoffSummary    string            `json:"handoff_summary"`
	Artifacts         []string          `json:"artifacts"`
	Partial           bool              `json:"partial,omitempty"`
	Recoverable       bool              `json:"recoverable,omitempty"`
	Reason            string            `json:"reason,omitempty"`
	Error             *DispatchError    `json:"error,omitempty"`
	Activity          *DispatchActivity `json:"activity"`
	Metadata          *DispatchMetadata `json:"metadata"`
	DurationMS        int64             `json:"duration_ms"`
}
```

### Result Fields

| Field | JSON key | Type | Meaning |
| --- | --- | --- | --- |
| `SchemaVersion` | `schema_version` | `int` | Result schema version for callers consuming structured output. |
| `Status` | `status` | `DispatchStatus` | Terminal state: `completed`, `timed_out`, or `failed`. |
| `DispatchID` | `dispatch_id` | `string` | Dispatch identifier for the finished run. |
| `DispatchSalt` | `dispatch_salt` | `string` | Human-readable label associated with the dispatch. |
| `TraceToken` | `trace_token` | `string` | Trace token carried through execution and artifacts. |
| `Response` | `response` | `string` | Final response text returned inline. |
| `ResponseTruncated` | `response_truncated` | `bool` | Indicates the inline response was truncated. |
| `FullOutput` | `full_output` | `*string` | Optional full untruncated response payload. |
| `FullOutputPath` | `full_output_path` | `*string` | Optional absolute path to `full_output.md` when persisted separately. |
| `HandoffSummary` | `handoff_summary` | `string` | Downstream handoff summary extracted from the response. |
| `Artifacts` | `artifacts` | `[]string` | Artifact files found in the dispatch artifact directory. |
| `Partial` | `partial` | `bool` | Marks a partial result when the worker produced usable output before terminal failure. |
| `Recoverable` | `recoverable` | `bool` | Signals whether agent-mux thinks recovery is viable. |
| `Reason` | `reason` | `string` | Short terminal reason string used for timeouts and failure classification. |
| `Error` | `error` | `*DispatchError` | Normalized failure payload when status is not cleanly completed. |
| `Activity` | `activity` | `*DispatchActivity` | Aggregated file, command, and tool activity observed during the run. |
| `Metadata` | `metadata` | `*DispatchMetadata` | Engine, model, usage, and lineage metadata attached to the result. |
| `DurationMS` | `duration_ms` | `int64` | End-to-end runtime in milliseconds. |

### DispatchError

| Field | JSON key | Type | Meaning |
| --- | --- | --- | --- |
| `Code` | `code` | `string` | Stable error code such as `config_error`, `startup_failed`, or `timed_out`-adjacent failure codes. |
| `Message` | `message` | `string` | Human-readable failure summary. |
| `Suggestion` | `suggestion` | `string` | Immediate retry or remediation guidance for the caller. |
| `Retryable` | `retryable` | `bool` | Indicates whether the failure should usually be retried. |
| `PartialArtifacts` | `partial_artifacts` | `[]string` | Artifact paths preserved even though the run failed or timed out. |

### DispatchActivity

| Field | JSON key | Type | Meaning |
| --- | --- | --- | --- |
| `FilesChanged` | `files_changed` | `[]string` | Files the worker wrote or modified. |
| `FilesRead` | `files_read` | `[]string` | Files the worker read. |
| `CommandsRun` | `commands_run` | `[]string` | Shell commands observed from harness events. |
| `ToolCalls` | `tool_calls` | `[]string` | Structured tool names or tool-call identifiers reported by the adapter. |

### DispatchMetadata

| Field | JSON key | Type | Meaning |
| --- | --- | --- | --- |
| `Engine` | `engine` | `string` | Engine adapter that executed the run. |
| `Model` | `model` | `string` | Final model name used by the adapter. |
| `Role` | `role` | `string` | Resolved role name when a role drove the dispatch. |
| `Tokens` | `tokens` | `*TokenUsage` | Token accounting parsed from harness output. |
| `Turns` | `turns` | `int` | Conversation or step count reported by the harness loop. |
| `CostUSD` | `cost_usd` | `float64` | Reported or estimated run cost in USD. |
| `SessionID` | `session_id` | `string` | Harness session ID used for resume or correlation. |
| `PipelineID` | `pipeline_id` | `string` | Pipeline instance ID when the dispatch ran as a pipeline worker. |
| `ParentDispatchID` | `parent_dispatch_id` | `string` | Parent dispatch ID for nested or pipeline-owned runs. |

### TokenUsage

| Field | JSON key | Type | Meaning |
| --- | --- | --- | --- |
| `Input` | `input` | `int` | Input tokens consumed by the run. |
| `Output` | `output` | `int` | Output tokens emitted by the run. |
| `Reasoning` | `reasoning` | `int` | Reasoning-token count when the harness reports it. |
| `CacheRead` | `cache_read` | `int` | Prompt-cache read tokens when reported. |
| `CacheWrite` | `cache_write` | `int` | Prompt-cache write tokens when reported. |

### Runnable preview example

```sh
agent-mux preview --engine codex --cwd . "Explain what you would change"
```

## Mode Detection

| Invocation pattern | Mode |
| --- | --- |
| `agent-mux [flags] <prompt>` | dispatch |
| `agent-mux dispatch [flags] <prompt>` | dispatch |
| `agent-mux preview [flags] <prompt>` | preview |
| `agent-mux --pipeline <name> [flags] <prompt>` | pipeline |
| `agent-mux --recover <dispatch_id> [flags] <prompt>` | recover |
| `agent-mux --signal <dispatch_id> <message>` | signal |
| `agent-mux --stdin` | stdin |
| `agent-mux config [subcommand] [flags]` | config |
| `agent-mux list`, `status`, `result`, `inspect`, `gc` | lifecycle |
| `agent-mux --async [flags] <prompt>` or `agent-mux wait <dispatch_id>` | async |
| `agent-mux steer <dispatch_id> <action> [args]` | steer |

## Exit Codes

| Code | Meaning |
| --- | --- |
| `0` | Success. |
| `1` | Runtime or config failure, including failed dispatch, signal, or recovery. |
| `2` | Usage error, bad flags, or missing required prompt or arguments. |
| `130` | Cancelled at the interactive TTY confirmation gate. |

## Cross-references

- [engines.md](./engines.md) for adapter-specific command construction and behavior.
- [config.md](./config.md) for config merge order, roles, variants, defaults, and skill search paths.
- [recovery.md](./recovery.md) for recovery context, control records, and continuation behavior.
- [cli-reference.md](./cli-reference.md) for the full flag surface and subcommand details.

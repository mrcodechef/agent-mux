---
name: agent-mux
description: |
  Cross-engine dispatch layer for AI coding agents. Use when you need to:
  launch a worker on Codex/Claude/Gemini, recover a timed-out dispatch, steer
  a running worker mid-flight, or coordinate multi-model work. Trigger on:
  agent-mux, dispatch, spawn worker, codex worker, role dispatch, async
  dispatch, steer agent, recover timeout, multi-engine.
---

# agent-mux

Dispatch substrate. One CLI, one JSON contract, three engines. agent-mux
resolves inputs and dispatches; it does not decide strategy.

Current invariants:

- roles are flat; there is no variant system
- implicit config discovery is a two-file merge:
  `~/.agent-mux/config.toml` then `<cwd>/.agent-mux/config.toml`
- durable state lives in `~/.agent-mux/dispatches/<id>/`
- the artifact dir contains `_dispatch_ref.json`, not `_dispatch_meta.json`

## Quickstart

### Discovery

Run this first:

```bash
agent-mux config roles
```

That is the live role catalog for the current project. Do not invent role
names, and do not assume a role has variants.

### Dispatch -> Wait -> Collect

`--async` returns an ack immediately. `wait` is the completion primitive.
`result` reads the stored response.

```bash
agent-mux -R=scout --async -C=/repo "Find deprecated API usages" 2>/dev/null

agent-mux wait --poll 30s <id> 2>/dev/null

agent-mux result --json <id> 2>/dev/null

agent-mux status --json <id> 2>/dev/null

agent-mux inspect --json <id> 2>/dev/null
```

Rules:

- always parse stdout, not stderr
- redirect stderr away when you need machine-readable output
- do not poll `status --json` in a loop for completion; use `wait`

### Structured dispatch spec

Use `--stdin` when the dispatch shape is easier to express as JSON:

```bash
printf '%s' '{"role":"lifter","prompt":"Implement the fix","cwd":"/repo"}' \
  | agent-mux --stdin --async 2>/dev/null
```

In `--stdin` mode, dispatch fields belong in JSON. The CLI is only carrying
transport flags like `--stdin`, `--async`, `--stream`, `--verbose`, `--yes`,
and `--config`.

### Mid-flight steering

```bash
agent-mux steer <id> redirect "Narrow to the parser module only" 2>/dev/null
agent-mux steer <id> nudge 2>/dev/null
agent-mux steer <id> extend 300 2>/dev/null
agent-mux steer <id> abort 2>/dev/null
```

Steering is unified under `internal/steer`. The package manages both:

- inbox delivery in the artifact dir
- FIFO delivery via `stdin.pipe` on Unix when Codex soft steering is available

---

## Role Selection

Treat role names as ordinary flat config keys.

- good mental model: `lifter`, `lifter-claude`, `auditor`
- bad mental model: `lifter` plus a `claude` variant

If you need a different engine/model pairing, use a different role name if the
project defines one.

Profiles are separate from roles:

- roles come from TOML config
- profiles come from `agents/<name>.md`

---

## Output Parsing

On every result, check:

- `status`
- `response`
- `activity.files_changed`
- `metadata.engine` and `metadata.model`

Compatibility fields:

- `response_truncated` remains in the schema but is normally `false`
- `full_output` remains in the schema but is normally `null`
- `full_output_path` is a deprecated compatibility stub; do not build new flow around it

Durable sources of truth:

- `~/.agent-mux/dispatches/<id>/meta.json`
- `~/.agent-mux/dispatches/<id>/result.json`

Runtime state:

- `<artifact_dir>/_dispatch_ref.json`
- `<artifact_dir>/status.json`
- `<artifact_dir>/events.jsonl`

---

## Failure and Recovery

Recovery code lives in `internal/dispatch/recovery.go`.

Use `--recover=<dispatch_id>` when the earlier run already produced useful
files or notes.

Decision rule:

```text
timed_out + useful artifacts exist
  -> recover
timed_out + no useful artifacts
  -> tighten the prompt and retry once
failed + retryable
  -> fix the cause and retry once
failed + not retryable
  -> escalate
```

Your recovery prompt should describe the delta only. agent-mux already injects
the prior dispatch ID, status, engine/model, and artifact list.

---

## Prompt Discipline

### General rules

1. give the worker one job
2. name exact files or directories
3. state hard constraints
4. provide a verification gate
5. state the expected output

### Codex

Use Codex for implementation, debugging, and precise edits.

Good:

- exact file paths
- explicit tests to run
- narrow output requirements

Bad:

- open-ended exploration
- mixed planning plus implementation plus review

### Claude

Use Claude for planning, synthesis, review, and ambiguity reduction.

### Gemini

Use Gemini as a narrow contrast pass or second opinion.

### Auto-injected context

agent-mux may prepend:

- `Relevant context from the coordinator is at $AGENT_MUX_CONTEXT. Read it before starting.`
- `Write intermediate artifacts to $AGENT_MUX_ARTIFACT_DIR.`

If you want a specific scratch file, say so explicitly:

```text
Write your work log to $AGENT_MUX_ARTIFACT_DIR/review-notes.md.
```

### Shell hygiene

- put flags before positional args
- escape worker env vars in prompts when your shell would expand them early
- use `2>/dev/null` when you need clean JSON on stdout

---

## Anti-Patterns

- assuming `config.local.toml` or XDG config lookup exists
- describing roles as variants
- mixing CLI dispatch flags into `--stdin` mode
- treating `_dispatch_ref.json` as the durable result instead of a pointer
- treating `full_output_path` as a live contract
- polling `status --json` instead of using `wait`
- ignoring `status` and reading only `response`

---

## Reference Index

Operational references in `skill/references/`:

| Reference | Read when |
|-----------|-----------|
| [cli-flags.md](references/cli-flags.md) | flags, commands, JSON fields, precedence |
| [async-and-steering.md](references/async-and-steering.md) | async launch, wait, steer, status |
| [output-contract.md](references/output-contract.md) | result schema, preview, lifecycle JSON |
| [recovery-guide.md](references/recovery-guide.md) | recovery flow, runtime layout, watchdog |
| [prompting-guide.md](references/prompting-guide.md) | prompt shapes, auto preamble, workflows |
| [config-and-roles.md](references/config-and-roles.md) | config structure, flat roles, profiles, hooks |

Architecture and internals in `docs/`:

- `docs/architecture.md`
- `docs/dispatch.md`
- `docs/engines.md`
- `docs/config.md`
- `docs/recovery.md`
- `docs/lifecycle.md`
- `docs/async.md`
- `docs/steering.md`

Discover config sources with:

```bash
agent-mux config --sources
```
